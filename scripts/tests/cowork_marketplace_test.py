#!/usr/bin/env python3
"""Repository plugins must survive cache copying without a source checkout."""
import json
from pathlib import Path
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]


class CoworkMarketplace(unittest.TestCase):
    def test_generated_plugins_are_current(self):
        subprocess.run([sys.executable, str(ROOT / "scripts/sync-cowork-marketplace.py"),
                        "--check"], check=True)

    def test_catalog_plugins_are_self_contained_and_use_installed_binaries(self):
        catalog = json.loads((ROOT / ".claude-plugin/marketplace.json").read_text())
        entries = {entry["name"]: entry for entry in catalog["plugins"]}
        self.assertEqual(len(entries), len(catalog["plugins"]))
        for platform, command in (
            ("macos", "${HOME}/.local/bin/zapier-pp-mcp"),
            ("windows", "${LOCALAPPDATA}/Microsoft/WindowsApps/zapier-pp-mcp.exe"),
        ):
            name = "zapier-cowork-" + platform
            source = (ROOT / entries[name]["source"]).resolve()
            self.assertTrue(source.is_relative_to(ROOT / "plugins"))
            manifest = json.loads((source / ".claude-plugin/plugin.json").read_text())
            self.assertEqual(manifest["name"], name)
            config = json.loads((source / ".mcp.json").read_text())["mcpServers"]
            self.assertEqual(set(config), {"zapier"})
            self.assertEqual(config["zapier"]["command"], command)
            self.assertEqual(config["zapier"]["args"], ["--transport", "stdio"])
            self.assertEqual(config["zapier"]["env"], {"ZAPIER_NO_LEARN": "true"})
            self.assertTrue((source / "skills/zapier/SKILL.md").is_file())
            for path in source.rglob("*"):
                self.assertFalse(path.is_symlink(), f"cache-external symlink: {path}")
                if path.is_file():
                    self.assertIn(path.suffix, {".md", ".json"})


if __name__ == "__main__":
    unittest.main()
