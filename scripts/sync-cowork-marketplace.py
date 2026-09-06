#!/usr/bin/env python3
"""Maintain cache-contained Cowork repository plugins from the ZIP skill source.

Normal users install prebuilt CLI binaries; this is a contributor-only helper.
"""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TARGETS = {
    "macos": "${HOME}/.local/bin/zapier-pp-mcp",
    "windows": "${LOCALAPPDATA}/Microsoft/WindowsApps/zapier-pp-mcp.exe",
}


def expected_files():
    template = ROOT / "cowork/zapier-cowork"
    manifest = json.loads((template / ".claude-plugin/plugin.json").read_text())
    skill = (template / "skills/zapier/SKILL.md").read_text()
    files = {}
    for platform, command in TARGETS.items():
        name = "zapier-cowork-" + platform
        base = ROOT / "plugins" / name
        metadata = dict(manifest, name=name)
        metadata["description"] = (
            f"Local Cowork Zapier inspection on {platform}. Requires the prebuilt "
            "CLI installed at its default location; read-only, saved login only."
        )
        config = {"mcpServers": {"zapier": {
            "command": command, "args": ["--transport", "stdio"],
            "env": {"ZAPIER_NO_LEARN": "true"},
        }}}
        files[base / ".claude-plugin/plugin.json"] = json.dumps(metadata, indent=2) + "\n"
        files[base / ".mcp.json"] = json.dumps(config, indent=2) + "\n"
        files[base / "skills/zapier/SKILL.md"] = skill.replace("`zapier-cowork`", f"`{name}`")
    return files


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="fail on stale generated files")
    args = parser.parse_args()
    stale = []
    for path, expected in expected_files().items():
        if path.is_file() and path.read_text() == expected:
            continue
        if args.check:
            stale.append(str(path.relative_to(ROOT)))
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(expected)
    if stale:
        parser.exit(1, "Run scripts/sync-cowork-marketplace.py; stale files: " + ", ".join(stale) + "\n")


if __name__ == "__main__":
    main()
