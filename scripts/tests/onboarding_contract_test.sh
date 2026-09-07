#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

python3 - "$repo_root" <<'PY'
from pathlib import Path
import sys

root = Path(sys.argv[1])
readme = (root / "README.md").read_text(encoding="utf-8")
installers = "\n".join((root / name).read_text(encoding="utf-8") for name in ("install.sh", "install.ps1"))

for required in (
    "Run this yourself in Terminal",
    "./install.sh --agent claude",
    'export PATH="$HOME/.local/bin:$PATH"',
    "zapier-pp-cli auth browser",
    "zapier-cowork_darwin_arm64.zip",
    "Customize → Plugins → Add → Upload a plugin",
    "Do not use **Add marketplace**",
):
    if required not in readme:
        raise SystemExit(f"README is missing manual setup detail: {required!r}")

if "```text" in readme or "prompt" in readme.lower():
    raise SystemExit("README still contains an AI copy-paste prompt")

if "go " in installers.lower() or "go\t" in installers.lower():
    raise SystemExit("a normal-user installer invokes or instructs Go")

print("PASS: minimal manual CLI and Cowork ZIP onboarding")
PY
