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
installers = "\n".join(
    (root / name).read_text(encoding="utf-8") for name in ("install.sh", "install.ps1")
)

match = re.search(
    r"## Give this to your agent\s+.*?```text\s*(.*?)\s*```",
    readme,
    flags=re.DOTALL,
)
if not match:
    raise SystemExit("README is missing the copyable agent-install prompt")
prompt = match.group(1)

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

for required in (
    "./install.sh",
    "install.ps1",
    "zapier-pp-cli auth browser",
    "zapier-pp-cli session --agent --no-learn",
):
    if required.lower() not in prompt.lower():
        raise SystemExit(f"agent prompt is missing required step: {required!r}")

session_at = prompt.lower().find("zapier-pp-cli session --agent --no-learn")
doctor_at = prompt.lower().find("zapier-pp-cli doctor")
if doctor_at == -1 or session_at == -1 or session_at >= doctor_at:
    raise SystemExit("agent prompt does not put the session-only checkpoint before doctor")

if "go " in installers.lower() or "go\t" in installers.lower():
    raise SystemExit("a normal-user installer invokes or instructs Go")

if "auth browser" not in agents or "no authorized remote mutation" not in agents.lower():
    raise SystemExit("AGENTS.md does not preserve browser onboarding and fail-closed remote safety")

print("PASS: no-Go, no-cookie-copy, session-first onboarding contract")
PY
