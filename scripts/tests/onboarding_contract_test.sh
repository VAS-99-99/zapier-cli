#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

python3 - "$repo_root" <<'PY'
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
readme = (root / "README.md").read_text(encoding="utf-8")
skill = (root / "SKILL.md").read_text(encoding="utf-8")
agents = (root / "AGENTS.md").read_text(encoding="utf-8")
prompt_file = (root / "CLAUDE_INSTALL_PROMPT.txt").read_text(encoding="utf-8")
installers = "\n".join(
    (root / name).read_text(encoding="utf-8") for name in ("install.sh", "install.ps1")
)

match = re.search(
    r"## Give this to Claude or Codex\s+.*?```text\s*(.*?)\s*```",
    readme,
    flags=re.DOTALL,
)
if not match:
    raise SystemExit("README is missing the copyable agent-install prompt")
prompt = match.group(1)
if prompt.strip() != prompt_file.strip():
    raise SystemExit("README copyable prompt differs from CLAUDE_INSTALL_PROMPT.txt")

for forbidden in (
    "0.0.0-dev",
    "auth set-token",
    "copy the full value",
    "copy the complete",
    "cookie request header",
    "cookie request-header",
    "devtools",
):
    if forbidden in (readme + "\n" + skill).lower():
        raise SystemExit(f"legacy onboarding instruction returned: {forbidden!r}")

for forbidden in ("go install", "verify go", "go build", "otherwise clone"):
    if forbidden in prompt.lower():
        raise SystemExit(f"agent prompt can still trigger a source build: {forbidden!r}")

for forbidden in (
    "audit the source",
    "release workflow",
    "published sha256sums",
    "open a new terminal",
):
    if forbidden in prompt.lower():
        raise SystemExit(f"agent prompt can still trigger slow or broken onboarding: {forbidden!r}")

for required in (
    "https://github.com/VAS-99-99/zapier-cli",
    "quick install",
    "prebuilt GitHub Release",
    "installer performs the checksum and version checks",
    "confirm `zapier-pp-cli version` works in the current terminal",
    "tell me to run `zapier-pp-cli auth browser` myself",
    "only after I explicitly reply `connected`",
    "zapier-pp-cli session --agent --no-learn",
):
    if required.lower() not in prompt.lower():
        raise SystemExit(f"agent prompt is missing required step: {required!r}")

session_at = prompt.lower().find("zapier-pp-cli session --agent --no-learn")
doctor_at = prompt.lower().find("doctor")
if doctor_at == -1 or session_at == -1 or session_at >= doctor_at:
    raise SystemExit("agent prompt does not put the session-only checkpoint before doctor")

if "go " in installers.lower() or "go\t" in installers.lower():
    raise SystemExit("a normal-user installer invokes or instructs Go")

if "github cli (gh) is required" in installers.lower():
    raise SystemExit("a public-repository installer still requires GitHub CLI authentication")

if "microsoft\\windowsapps" not in (root / "install.ps1").read_text(encoding="utf-8").lower():
    raise SystemExit("Windows installer does not use the existing per-user command directory")

if "authentication is a user-owned boundary" not in agents.lower():
    raise SystemExit("AGENTS.md does not preserve the user-owned authentication boundary")
if "no authorized remote mutation" not in agents.lower():
    raise SystemExit("AGENTS.md does not preserve fail-closed remote safety")

print("PASS: no-Go, no-cookie-copy, session-first onboarding contract")
PY
