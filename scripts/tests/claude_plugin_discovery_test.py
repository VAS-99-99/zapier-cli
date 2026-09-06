#!/usr/bin/env python3
"""Native fresh-chat discovery test using a local model-response fixture.

No real model, browser, Zapier account, or user agent configuration is used.
This verifies host loading, not a model's choice to invoke the skill.
Requires Claude Code on PATH; not part of the host-independent release gate.
"""
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading

ROOT = Path(__file__).resolve().parents[2]
# Optional native-plugin package check. This remains a Claude Code loader
# check, not proof that a particular Cowork Desktop build accepts the upload.
cowork_plugin = Path(sys.argv[1]).resolve() if len(sys.argv) >= 2 else None
# For repository plugins, copy verified native binaries into an isolated
# installer's default location. Never use the user's installed executable.
installed_binaries = Path(sys.argv[2]).resolve() if len(sys.argv) == 3 else None
plugin_name = (json.loads((cowork_plugin / ".claude-plugin/plugin.json").read_text())["name"]
               if cowork_plugin else "zapier-read-only")
requests = []


class Fixture(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        payload = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))))
        if self.path.startswith("/v1/messages"):
            requests.append(payload)
        if "count_tokens" in self.path:
            result = {"input_tokens": 100}
        else:
            # Response retries must not consume a one-shot tool response.
            # Advance only when the request contains the tool's actual result.
            tool_result_present = any(
                isinstance(block, dict) and block.get("type") == "tool_result"
                for message in payload.get("messages", [])
                for block in message.get("content", [])
            )
            if not tool_result_present:
                content = [{"type": "tool_use", "id": "tool_skill_fixture",
                            "name": "Skill", "input": {"skill": plugin_name + ":zapier"}}]
                stop_reason = "tool_use"
            else:
                content = [{"type": "text", "text": "Discovery fixture complete."}]
                stop_reason = "end_turn"
            result = {"id": "msg_fixture", "type": "message", "role": "assistant",
                      "model": payload.get("model", "fixture"),
                      "content": content,
                      "stop_reason": stop_reason, "stop_sequence": None,
                      "usage": {"input_tokens": 100, "output_tokens": 5}}
        raw = json.dumps(result).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)


claude = shutil.which("claude")
if not claude:
    raise SystemExit("Claude Code is required for the native discovery test")
with tempfile.TemporaryDirectory(prefix="zapier-plugin-discovery-") as temporary:
    temp = Path(temporary)
    outside = temp / "unrelated-folder"
    outside.mkdir()
    settings = temp / "claude"
    settings.mkdir()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    env = {"PATH": os.environ["PATH"], "HOME": str(temp),
           "TMPDIR": str(temp), "CLAUDE_CONFIG_DIR": str(settings),
           "ANTHROPIC_API_KEY": "synthetic-fixture-only",
           "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{server.server_port}",
           "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
           "DISABLE_AUTOUPDATER": "1"}
    env.update({"XDG_CONFIG_HOME": str(temp / "config"),
                "XDG_DATA_HOME": str(temp / "data"),
                "XDG_CACHE_HOME": str(temp / "cache"),
                "ZAPIER_CONFIG": str(temp / "missing-config.json"),
                "ZAPIER_BASE_URL": f"http://127.0.0.1:{server.server_port}"})
    if installed_binaries:
        assert plugin_name == "zapier-cowork-macos", "native fixture currently covers macOS only"
        destination = temp / ".local/bin"
        destination.mkdir(parents=True)
        for binary in ("zapier-pp-cli", "zapier-pp-mcp"):
            shutil.copy2(installed_binaries / binary, destination / binary)

    def run(*args):
        result = subprocess.run([claude, *args], cwd=outside, env=env,
                                capture_output=True, text=True, timeout=45)
        if result.returncode:
            raise AssertionError(result.stdout + result.stderr)
        return result.stdout

    try:
        plugin_args = []
        if cowork_plugin:
            run("plugin", "validate", str(cowork_plugin))
            if installed_binaries:
                run("plugin", "marketplace", "add", str(ROOT), "--scope", "user")
                run("plugin", "install", plugin_name + "@vas-zapier-cli", "--scope", "user")
            else:
                plugin_args = ["--plugin-dir", str(cowork_plugin)]
        else:
            run("plugin", "marketplace", "add", str(ROOT), "--scope", "user")
            run("plugin", "install", "zapier-read-only@vas-zapier-cli", "--scope", "user")
        # No skill invocation or repository hint appears in the user prompt.
        # Strict MCP mode would disable the Cowork plugin's own MCP config.
        strict_args = [] if cowork_plugin else ["--strict-mcp-config"]
        run(*plugin_args, "-p", "Go check if this Zap has any issues.", "--tools", "Skill", *strict_args,
            "--no-session-persistence", "--permission-mode", "dontAsk",
            "--output-format", "json")
        captured = json.dumps(requests)
        skill_name = plugin_name + ":zapier"
        assert skill_name in captured, "fresh chat did not discover the installed skill"
        tool_results = [block for request in requests for message in request.get("messages", [])
                        for block in message.get("content", []) if isinstance(block, dict) and block.get("type") == "tool_result"]
        assert "resolve the supplied Zap name or URL" in captured, ("invoking Skill did not load inspection instructions", tool_results)
        assert "session --agent --no-learn" in captured, "invoked skill omitted session-first instructions"
        if cowork_plugin:
            assert "mcp__plugin_" + plugin_name + "_zapier__session_check" in captured, (
                "native MCP tools did not connect", [tool.get("name") for request in requests
                                                      for tool in request.get("tools", [])])
        else:
            assert "zapier-pp-cli" in captured, "skill description was missing from model context"
        assert requests, "no model request was captured"
        print("PASS: native Claude fresh chat outside the repo advertises " + skill_name)
        print("PASS: synthetic Skill invocation loads the full inspection instructions")
        if cowork_plugin:
            print("PASS: MCP tools connected through the native plugin loader; Cowork UI still needs manual acceptance")
        print("Model response was synthetic; no Zapier requests or credentials were used.")
    finally:
        server.shutdown()
