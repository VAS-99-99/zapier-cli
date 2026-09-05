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
import tempfile
import threading

ROOT = Path(__file__).resolve().parents[2]
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
            result = {"id": "msg_fixture", "type": "message", "role": "assistant",
                      "model": payload.get("model", "fixture"),
                      "content": [{"type": "text", "text": "Discovery fixture complete."}],
                      "stop_reason": "end_turn", "stop_sequence": None,
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

    def run(*args):
        result = subprocess.run([claude, *args], cwd=outside, env=env,
                                capture_output=True, text=True, timeout=45)
        if result.returncode:
            raise AssertionError(result.stdout + result.stderr)
        return result.stdout

    try:
        run("plugin", "marketplace", "add", str(ROOT), "--scope", "user")
        run("plugin", "install", "zapier-read-only@vas-zapier-cli", "--scope", "user")
        # No plugin path, skill invocation, repository path, or command hint
        # appears in the fresh chat. Only user-scoped registration can supply it.
        run("-p", "Check my Zapier runs.", "--tools", "Skill", "--strict-mcp-config",
            "--no-session-persistence", "--permission-mode", "dontAsk",
            "--output-format", "json")
        captured = json.dumps(requests)
        assert "zapier-read-only:zapier" in captured, "fresh chat did not discover the installed skill"
        assert "zapier-pp-cli" in captured, "skill description was missing from model context"
        assert requests, "no model request was captured"
        print("PASS: native Claude fresh chat outside the repo advertises the installed Zapier skill")
        print("Model response was synthetic; no Zapier requests or credentials were used.")
    finally:
        server.shutdown()
