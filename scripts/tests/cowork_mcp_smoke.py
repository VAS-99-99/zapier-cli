#!/usr/bin/env python3
"""Exercise an extracted native Cowork package against loopback fixtures only.

Usage: python3 scripts/tests/cowork_mcp_smoke.py /path/to/extracted/plugin
No real account, browser, agent configuration, or inherited secrets are used.
This tests MCP plumbing, not Claude Desktop's upload UI or model decisions.
"""
import http.server
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import threading


requests = []
fixture_failures = []
expired = False
run = {"id": "fixture-run", "status": "error", "startTime": "2026-09-06T08:00:00Z",
       "zap": {"id": "42", "title": "Fixture business"},
       "steps": [{"title": "Catch Hook", "status": "success", "input": {}},
                 {"title": "Re-Route Lead", "status": "error",
                  "input": {"business": "Fixture business", "phone": ""},
                  "error": {"title": "Synthetic failure, no webhook was sent"}}]}


class Fixture(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, value, status=200):
        raw = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        requests.append(("GET", self.path))
        if expired:
            self.reply({"error": "expired fixture session"}, 401)
        elif self.path.split("?")[0] == "/api/v4/session":
            self.reply({"email": "cowork-fixture@example.invalid",
                        "current_account_id": 77, "is_logged_in": True})
        elif self.path.split("?")[0] == "/api/v4/zaps":
            self.reply({"count": 1, "next": None, "results": [
                {"id": 42, "title": "Fixture business", "is_enabled": True}]})
        else:
            fixture_failures.append("unexpected GET " + self.path)
            self.reply({"error": "unexpected fixture path"}, 404)

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        requests.append(("POST", self.path))
        if self.path != "/api/reporting/graphql":
            fixture_failures.append("unexpected POST " + self.path)
            self.reply({"error": "unexpected POST path"}, 400)
        elif not body.get("query", "").startswith("query "):
            fixture_failures.append("non-query POST")
            self.reply({"error": "only GraphQL queries allowed"}, 400)
        elif body.get("operationName") == "ZapRuns":
            self.reply({"data": {"zapRuns": {"edges": [run], "totalCount": 1}}})
        elif body.get("operationName") == "RunDetail":
            self.reply({"data": {"zapRun": run}})
        else:
            fixture_failures.append("unexpected GraphQL operation")
            self.reply({"error": "unexpected operation"}, 400)


def main():
    global expired
    root = Path(sys.argv[1]).resolve()
    config = json.loads((root / ".mcp.json").read_text())["mcpServers"]["zapier"]
    command = config["command"].replace("${CLAUDE_PLUGIN_ROOT}", str(root))
    assert Path(command).is_file(), "MCP executable missing"
    assert config["args"] == ["--transport", "stdio"]
    assert config["env"] == {"ZAPIER_NO_LEARN": "true"}, "unexpected package environment"
    with tempfile.TemporaryDirectory(prefix="cowork smoke ") as temporary:
        home = Path(temporary)
        fixture = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
        threading.Thread(target=fixture.serve_forever, daemon=True).start()
        # Deliberately no user PATH: mirrored tools must find the bundled sibling.
        env = {"PATH": "", "HOME": temporary, "USERPROFILE": temporary,
               "APPDATA": str(home / "roaming"), "LOCALAPPDATA": str(home / "local"),
               "XDG_CONFIG_HOME": str(home / "config"), "XDG_DATA_HOME": str(home / "data"),
               "XDG_CACHE_HOME": str(home / "cache"), "TMPDIR": temporary,
               "ZAPIER_CONFIG": str(home / "fixture-config.json"),
               "ZAPIER_BASE_URL": f"http://127.0.0.1:{fixture.server_port}",
               "ZAPIER_SESSION_COOKIE": "synthetic-fixture-only",
               "PRINTING_PRESS_VERIFY": "1", **config["env"]}
        if "SystemRoot" in os.environ:
            env["SystemRoot"] = os.environ["SystemRoot"]
        proc = subprocess.Popen([command, *config["args"]], cwd=home, env=env,
                                stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, text=True)
        messages = queue.Queue()
        errors = []

        def read_output():
            for line in proc.stdout:
                try:
                    messages.put(json.loads(line))
                except ValueError:
                    messages.put({"invalid_json": line})
        threading.Thread(target=read_output, daemon=True).start()
        threading.Thread(target=lambda: errors.append(proc.stderr.read()), daemon=True).start()
        counter = 0

        def rpc(method, params):
            nonlocal counter
            counter += 1
            proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": counter,
                                         "method": method, "params": params}) + "\n")
            proc.stdin.flush()
            while True:
                message = messages.get(timeout=30)
                assert "invalid_json" not in message, message
                if message.get("id") == counter:
                    assert "error" not in message, message
                    return message["result"]

        try:
            init = rpc("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                                      "clientInfo": {"name": "cowork-smoke", "version": "1"}})
            assert init["serverInfo"]["name"] == "Zapier", init
            proc.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
            proc.stdin.flush()
            listing = rpc("tools/list", {})
            tools = {tool["name"]: tool for tool in listing["tools"]}
            for name in ("session_check", "zaps_list", "runs_list", "runs_get", "diagnose"):
                assert name in tools, (name, sorted(tools))
                assert tools[name]["annotations"]["readOnlyHint"], name
            assert not any(name.startswith("auth") for name in tools), "auth exposed through MCP"
            assert requests == [], "startup or tool discovery queried the API"

            def call(name, args):
                schema = tools[name]["inputSchema"].get("properties", {})
                args = dict(args)
                for flag in ("no-cache", "no-learn"):
                    if flag in schema:
                        args[flag] = True
                result = rpc("tools/call", {"name": name, "arguments": args})
                assert not result.get("isError"), result
                return json.dumps(result)

            identity = call("session_check", {})
            assert "cowork-fixture@example.invalid" in identity and "77" in identity, identity
            assert requests == [("GET", "/api/v4/session")], requests
            # Fixture confirmation checkpoint. Only now simulate the user's read request.
            assert "Fixture business" in call("zaps_list", {})
            assert "fixture-run" in call("runs_list", {"zap": "42"})
            get_schema = tools["runs_get"]["inputSchema"]["properties"]
            run_key = next((key for key, spec in get_schema.items()
                            if spec.get("description", "").startswith("Positional argument ")), None)
            assert run_key, get_schema
            detail = call("runs_get", {run_key: "fixture-run"})
            assert "Re-Route Lead" in detail and "Synthetic failure" in detail, detail
            diag_schema = tools["diagnose"]["inputSchema"]["properties"]
            zap_key = next((key for key, spec in diag_schema.items()
                            if spec.get("description", "").startswith("Positional argument ")), None)
            assert zap_key, diag_schema
            diagnosis = call("diagnose", {zap_key: "Fixture business"})
            assert "Re-Route Lead" in diagnosis and "Synthetic failure" in diagnosis, diagnosis
            expired = True
            result = rpc("tools/call", {"name": "session_check", "arguments": {}})
            assert result.get("isError"), "expired session incorrectly accepted"
            assert not fixture_failures, fixture_failures
            assert "synthetic-fixture-only" not in str(result), "credential leaked"
            print("PASS: native stdio startup and tool discovery made zero API requests")
            print("PASS: fresh fixture identity, sibling CLI without PATH, Zaps/runs/details/diagnosis")
            print("PASS: expired session fails, no auth tool, only fixture GET/query POST traffic")
        finally:
            proc.stdin.close()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.terminate()
                proc.wait(timeout=5)
            fixture.shutdown()
            fixture.server_close()


if __name__ == "__main__":
    main()
