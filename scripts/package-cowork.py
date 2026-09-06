#!/usr/bin/env python3
"""Build offline Cowork plugin archives from already-published CLI assets."""

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import shutil
import stat
import sys
import tarfile
import tempfile
import time
import zipfile


TARGETS = (
    ("darwin_arm64", "zapier-cli_darwin_arm64.tar.gz", "", "tar"),
    ("darwin_x86_64", "zapier-cli_darwin_x86_64.tar.gz", "", "tar"),
    ("windows_x86_64", "zapier-cli_windows_x86_64.zip", ".exe", "zip"),
)
FIXED_EPOCH = 315532800  # 1980-01-01, the earliest ZIP timestamp.


def fail(message):
    raise ValueError(message)


def safe_name(name):
    path = PurePosixPath(name)
    if not name or "\\" in name or path.is_absolute() or ".." in path.parts:
        fail(f"unsafe archive entry: {name!r}")
    if any(part in ("", ".") for part in path.parts):
        fail(f"unsafe archive entry: {name!r}")
    return str(path)


def parse_checksums(path):
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as error:
        fail(f"cannot read SHA256SUMS: {error}")
    sums = {}
    for line in lines:
        fields = line.split()
        if len(fields) != 2 or len(fields[0]) != 64:
            fail(f"invalid SHA256SUMS line: {line!r}")
        digest, name = fields
        if any(char not in "0123456789abcdefABCDEF" for char in digest):
            fail(f"invalid SHA256SUMS digest: {digest!r}")
        if name in sums:
            fail(f"duplicate SHA256SUMS entry: {name}")
        sums[name] = digest.lower()
    return sums


def checked_archive(release_dir, archive_name, sums):
    path = release_dir / archive_name
    if not path.is_file():
        fail(f"missing release archive: {archive_name}")
    expected = sums.get(archive_name)
    if expected is None:
        fail(f"missing SHA256SUMS entry: {archive_name}")
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    if actual != expected:
        fail(f"checksum mismatch: {archive_name}")
    return path


def archive_files(path, kind):
    files = {}
    if kind == "tar":
        try:
            with tarfile.open(path, mode="r:gz") as archive:
                for member in archive.getmembers():
                    name = safe_name(member.name)
                    if name in files:
                        fail(f"duplicate archive entry: {name}")
                    if not member.isfile() or member.issym() or member.islnk():
                        fail(f"unsafe archive entry type: {name}")
                    source = archive.extractfile(member)
                    if source is None:
                        fail(f"cannot read archive entry: {name}")
                    files[name] = source.read()
        except (tarfile.TarError, OSError) as error:
            fail(f"invalid tar archive {path.name}: {error}")
    else:
        try:
            with zipfile.ZipFile(path) as archive:
                for member in archive.infolist():
                    name = safe_name(member.filename)
                    if name in files:
                        fail(f"duplicate archive entry: {name}")
                    mode = member.external_attr >> 16
                    # ZIP creators commonly omit the POSIX file-type bits; a
                    # non-directory entry is acceptable unless it is a link.
                    if member.is_dir() or stat.S_ISLNK(mode):
                        fail(f"unsafe archive entry type: {name}")
                    files[name] = archive.read(member)
        except (zipfile.BadZipFile, OSError) as error:
            fail(f"invalid zip archive {path.name}: {error}")
    return files


def template_files(repo_root):
    root = repo_root / "cowork" / "zapier-cowork"
    wanted = (
        ".claude-plugin/plugin.json",
        ".codex-plugin/plugin.json",
        "skills/zapier/SKILL.md",
    )
    files = {}
    for name in wanted:
        path = root / name
        if not path.is_file() or path.is_symlink():
            fail(f"missing required Cowork template: {path}")
        files[name] = path.read_bytes()
    license_path = repo_root / "LICENSE"
    if not license_path.is_file() or license_path.is_symlink():
        fail("missing required LICENSE")
    files["LICENSE"] = license_path.read_bytes()
    return files


def mcp_config(binary):
    return (json.dumps({"mcpServers": {"zapier": {
        "command": "${CLAUDE_PLUGIN_ROOT}/server/" + binary,
        "args": ["--transport", "stdio"],
        "env": {"ZAPIER_NO_LEARN": "true"},
    }}}, indent=2, sort_keys=True) + "\n").encode("utf-8")


def write_zip(path, files):
    stamp = time.gmtime(FIXED_EPOCH)[:6]
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_STORED) as archive:
        for name in sorted(files):
            info = zipfile.ZipInfo(name, stamp)
            info.create_system = 3
            info.external_attr = ((0o755 if name.startswith("server/") else 0o644) << 16)
            info.compress_type = zipfile.ZIP_STORED
            archive.writestr(info, files[name])


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--release-dir", required=True, type=Path)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args(argv)
    if not args.tag.startswith("v") or len(args.tag) == 1:
        fail("tag must start with v")
    version = args.tag[1:]
    if any(char not in "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._-" for char in version):
        fail("tag contains unsupported characters")
    release_dir = args.release_dir.resolve()
    output_dir = args.output_dir.resolve()
    if not release_dir.is_dir():
        fail(f"release directory does not exist: {release_dir}")
    repo_root = Path(__file__).resolve().parent.parent
    templates = template_files(repo_root)
    sums = parse_checksums(release_dir / "SHA256SUMS")

    output_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="zapier-cowork-", dir=output_dir) as temporary:
        staged = Path(temporary)
        checksums = []
        for target, archive_name, suffix, kind in TARGETS:
            source = checked_archive(release_dir, archive_name, sums)
            contents = archive_files(source, kind)
            cli = "zapier-pp-cli" + suffix
            mcp = "zapier-pp-mcp" + suffix
            if cli not in contents or mcp not in contents:
                fail(f"release archive missing required binary: {archive_name}")
            files = dict(templates)
            files["server/" + cli] = contents[cli]
            files["server/" + mcp] = contents[mcp]
            files[".mcp.json"] = mcp_config(mcp)
            # Keep a traceable version in both manifests without trusting their old value.
            for manifest in (".claude-plugin/plugin.json", ".codex-plugin/plugin.json"):
                data = json.loads(files[manifest].decode("utf-8"))
                if data.get("name") != "zapier-cowork":
                    fail(f"unexpected plugin name in {manifest}")
                data["version"] = version
                files[manifest] = (json.dumps(data, indent=2, sort_keys=True) + "\n").encode("utf-8")
            output_name = "zapier-cowork_" + target + ".zip"
            staged_path = staged / output_name
            write_zip(staged_path, files)
            checksums.append((output_name, hashlib.sha256(staged_path.read_bytes()).hexdigest()))
        checksum_path = staged / "COWORK_SHA256SUMS"
        checksum_path.write_text("".join(f"{digest}  {name}\n" for name, digest in sorted(checksums)), encoding="utf-8", newline="\n")
        for path in staged.iterdir():
            shutil.move(str(path), output_dir / path.name)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except ValueError as error:
        print(f"package-cowork: {error}", file=sys.stderr)
        sys.exit(1)
