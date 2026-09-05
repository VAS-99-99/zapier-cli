#!/usr/bin/env python3
"""Validate the two host catalogs and their shared, self-contained skill."""
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
PLUGIN = ROOT / "plugins/zapier-read-only"


class PluginContract(unittest.TestCase):
    def test_update_prompt_matches_readme_and_preserves_connection(self):
        prompt = (ROOT / 'UPDATE_PROMPT.txt').read_text().strip()
        self.assertIn(prompt, (ROOT / 'README.md').read_text())
        for expected in ('--agent claude', '--agent codex', '-Agent Claude',
                         '-Agent Codex', 'Preserve existing credentials',
                         'session --agent --no-learn', 'Never change anything in Zapier'):
            self.assertIn(expected, prompt)

    def test_catalogs_resolve_the_same_plugin(self):
        claude = json.loads((ROOT / ".claude-plugin/marketplace.json").read_text())
        codex = json.loads((ROOT / ".agents/plugins/marketplace.json").read_text())
        self.assertEqual(claude["name"], "vas-zapier-cli")
        self.assertEqual(codex["name"], claude["name"])
        for catalog in (claude, codex):
            self.assertEqual(len(catalog["plugins"]), 1)
            entry = catalog["plugins"][0]
            self.assertEqual(entry["name"], PLUGIN.name)
            source = entry["source"]
            path = source["path"] if isinstance(source, dict) else source
            self.assertEqual((ROOT / path).resolve(), PLUGIN)

    def test_hosts_ship_matching_versioned_skill_only_plugins(self):
        manifests = [json.loads((PLUGIN / host / "plugin.json").read_text())
                     for host in (".claude-plugin", ".codex-plugin")]
        self.assertEqual(manifests[0]["version"], manifests[1]["version"])
        for manifest in manifests:
            self.assertEqual(manifest["name"], PLUGIN.name)
            self.assertRegex(manifest["version"], r"^\d+\.\d+\.\d+$")
            self.assertTrue(manifest["description"].strip())
            self.assertFalse(set(manifest) & {"mcpServers", "hooks", "apps"})
        self.assertEqual(manifests[1]["skills"], "./skills/")
        skills = list((PLUGIN / "skills").glob("*/SKILL.md"))
        self.assertEqual(len(skills), 1)
        text = skills[0].read_text()
        self.assertTrue(text.startswith("---\nname: zapier\n"))
        self.assertIn("description:", text.split("---", 2)[1])
        self.assertNotIn("disable-model-invocation: true", text)
        self.assertNotIn("[TODO", text)
        # Relative Markdown dependencies must survive a native plugin cache copy.
        for link in re.findall(r"\]\(([^)]+)\)", text):
            if "://" not in link and not link.startswith("#"):
                target = (skills[0].parent / link.split("#")[0]).resolve()
                self.assertTrue(target.is_relative_to(PLUGIN))
                self.assertTrue(target.exists())
        for path in PLUGIN.rglob("*"):
            self.assertFalse(path.is_symlink(), f"cache-external link: {path}")
            if path.is_file():
                self.assertIn(path.suffix, {".json", ".md"}, f"unexpected executable/resource: {path}")


if __name__ == "__main__":
    unittest.main()
