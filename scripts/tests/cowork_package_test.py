#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import warnings
import zipfile


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "package-cowork.py"
TARGETS = (
    ("darwin_arm64", "zapier-cli_darwin_arm64.tar.gz", ""),
    ("darwin_x86_64", "zapier-cli_darwin_x86_64.tar.gz", ""),
    ("windows_x86_64", "zapier-cli_windows_x86_64.zip", ".exe"),
)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def make_release(path):
    path.mkdir()
    for target, name, suffix in TARGETS:
        files = {
            "zapier-pp-cli" + suffix: ("cli-" + target).encode(),
            "zapier-pp-mcp" + suffix: ("mcp-" + target).encode(),
            "README.md": b"input documentation must not ship",
            ".env": b"never copied",
        }
        archive = path / name
        if suffix:
            with zipfile.ZipFile(archive, "w") as bundle:
                for entry, data in files.items():
                    bundle.writestr(entry, data)
        else:
            with tarfile.open(archive, "w:gz") as bundle:
                for entry, data in files.items():
                    info = tarfile.TarInfo(entry)
                    info.size = len(data)
                    bundle.addfile(info, __import__("io").BytesIO(data))
    write_sums(path)


def write_sums(path):
    lines = [f"{digest(path / name)}  {name}\n" for _, name, _ in TARGETS]
    (path / "SHA256SUMS").write_text("".join(lines), encoding="utf-8")


class CoworkPackageTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="cowork package ")
        self.base = Path(self.tmp.name)
        self.release = self.base / "release assets"
        self.output = self.base / "output assets"
        make_release(self.release)

    def tearDown(self):
        self.tmp.cleanup()

    def run_packager(self, expect=0):
        result = subprocess.run(
            ["python3", str(SCRIPT), "--release-dir", str(self.release), "--tag", "v2.4.6", "--output-dir", str(self.output)],
            text=True, capture_output=True,
        )
        self.assertEqual(result.returncode, expect, result.stderr)
        return result

    def test_packages_all_platforms_deterministically_without_copying_input_docs(self):
        before = {path.name: digest(path) for path in self.release.iterdir() if path.is_file()}
        self.run_packager()
        expected = {"COWORK_SHA256SUMS"} | {f"zapier-cowork_{target}.zip" for target, _, _ in TARGETS}
        self.assertEqual({path.name for path in self.output.iterdir()}, expected)
        first = {path.name: digest(path) for path in self.output.iterdir()}
        self.run_packager()
        self.assertEqual(first, {path.name: digest(path) for path in self.output.iterdir()})
        self.assertEqual(before, {path.name: digest(path) for path in self.release.iterdir() if path.is_file()})
        sums = (self.output / "COWORK_SHA256SUMS").read_text().splitlines()
        self.assertEqual(3, len(sums))
        for line in sums:
            expected_hash, filename = line.split("  ")
            self.assertEqual(expected_hash, digest(self.output / filename))
        for target, _, suffix in TARGETS:
            with zipfile.ZipFile(self.output / f"zapier-cowork_{target}.zip") as bundle:
                names = bundle.namelist()
                self.assertEqual(names, sorted(names))
                self.assertNotIn("README.md", names)
                self.assertNotIn(".env", names)
                self.assertEqual({
                    ".claude-plugin/plugin.json", ".codex-plugin/plugin.json", ".mcp.json", "LICENSE",
                    "skills/zapier/SKILL.md", "server/zapier-pp-cli" + suffix,
                    "server/zapier-pp-mcp" + suffix,
                }, set(names))
                mcp = json.loads(bundle.read(".mcp.json"))
                self.assertEqual("${CLAUDE_PLUGIN_ROOT}/server/zapier-pp-mcp" + suffix, mcp["mcpServers"]["zapier"]["command"])
                for manifest in (".claude-plugin/plugin.json", ".codex-plugin/plugin.json"):
                    data = json.loads(bundle.read(manifest))
                    self.assertEqual("zapier-cowork", data["name"])
                    self.assertEqual("2.4.6", data["version"])
                self.assertEqual(0o755, (bundle.getinfo("server/zapier-pp-cli" + suffix).external_attr >> 16) & 0o777)

    def test_rejects_checksum_mismatch(self):
        archive = self.release / TARGETS[0][1]
        archive.write_bytes(archive.read_bytes() + b"tampered")
        self.run_packager(expect=1)
        self.assertIn("checksum mismatch", self.run_packager(expect=1).stderr)

    def test_rejects_traversal_symlink_and_duplicate_entries(self):
        for kind in ("traversal", "symlink", "duplicate"):
            with self.subTest(kind=kind):
                archive = self.release / TARGETS[0][1]
                with tarfile.open(archive, "w:gz") as bundle:
                    for name in ("zapier-pp-cli", "zapier-pp-mcp"):
                        data = b"binary"
                        info = tarfile.TarInfo(name)
                        info.size = len(data)
                        bundle.addfile(info, __import__("io").BytesIO(data))
                    if kind == "traversal":
                        info = tarfile.TarInfo("../outside")
                        info.size = 1
                        bundle.addfile(info, __import__("io").BytesIO(b"x"))
                    elif kind == "symlink":
                        info = tarfile.TarInfo("linked")
                        info.type = tarfile.SYMTYPE
                        info.linkname = "zapier-pp-cli"
                        bundle.addfile(info)
                    else:
                        for _ in range(2):
                            info = tarfile.TarInfo("same")
                            info.size = 1
                            bundle.addfile(info, __import__("io").BytesIO(b"x"))
                write_sums(self.release)
                self.run_packager(expect=1)
                shutil.rmtree(self.release)
                make_release(self.release)

    def test_rejects_unsafe_zip_entries(self):
        archive = self.release / TARGETS[2][1]
        for kind in ("traversal", "symlink", "duplicate"):
            with self.subTest(kind=kind):
                with zipfile.ZipFile(archive, "w") as bundle:
                    bundle.writestr("zapier-pp-cli.exe", b"cli")
                    bundle.writestr("zapier-pp-mcp.exe", b"mcp")
                    if kind == "traversal":
                        bundle.writestr("../outside", b"x")
                    elif kind == "symlink":
                        link = zipfile.ZipInfo("linked")
                        link.create_system = 3
                        link.external_attr = (0o120777 << 16)
                        bundle.writestr(link, b"zapier-pp-cli.exe")
                    else:
                        with warnings.catch_warnings():
                            warnings.simplefilter("ignore", UserWarning)
                            bundle.writestr("same", b"one")
                            bundle.writestr("same", b"two")
                write_sums(self.release)
                self.run_packager(expect=1)
                shutil.rmtree(self.release)
                make_release(self.release)


if __name__ == "__main__":
    unittest.main()
