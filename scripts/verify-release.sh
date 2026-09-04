#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <release-directory> <v-tag>" >&2
  exit 2
fi

release_dir=$1
release_tag=$2

if [[ ! "$release_tag" =~ ^v[0-9A-Za-z][0-9A-Za-z._-]*$ ]]; then
  echo "release tag must start with v and contain only letters, numbers, dots, underscores, or hyphens" >&2
  exit 2
fi

command -v go >/dev/null 2>&1 || {
  echo "Go is required to inspect embedded release build metadata" >&2
  exit 1
}
command -v python3 >/dev/null 2>&1 || {
  echo "python3 is required to verify release archives" >&2
  exit 1
}

python3 - "$release_dir" "$release_tag" <<'PY'
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import sys
import tarfile
import tempfile
import zipfile

release_dir = Path(sys.argv[1]).resolve()
release_tag = sys.argv[2]
assets = {
    "zapier-cli_windows_x86_64.zip": ("windows", "amd64", "zip"),
    "zapier-cli_darwin_arm64.tar.gz": ("darwin", "arm64", "tar"),
    "zapier-cli_darwin_x86_64.tar.gz": ("darwin", "amd64", "tar"),
    "zapier-cli_linux_x86_64.tar.gz": ("linux", "amd64", "tar"),
}
docs = {"README.md", "SKILL.md", "LICENSE"}

checksums_path = release_dir / "SHA256SUMS"
if not checksums_path.is_file():
    raise SystemExit("missing SHA256SUMS")

recorded = {}
for line in checksums_path.read_text(encoding="utf-8").splitlines():
    parts = line.split("  ", 1)
    if len(parts) != 2 or len(parts[0]) != 64:
        raise SystemExit(f"invalid SHA256SUMS line: {line!r}")
    digest, name = parts
    if name in recorded:
        raise SystemExit(f"duplicate checksum entry: {name}")
    recorded[name] = digest

if set(recorded) != set(assets):
    raise SystemExit(
        f"checksum asset set mismatch: got {sorted(recorded)}, want {sorted(assets)}"
    )

def read_archive(path, kind):
    contents = {}
    modes = {}
    if kind == "zip":
        with zipfile.ZipFile(path) as bundle:
            infos = bundle.infolist()
            names = [info.filename for info in infos]
            if names != sorted(names) or len(names) != len(set(names)):
                raise SystemExit(f"{path.name}: entries are not sorted and unique")
            for info in infos:
                if info.is_dir() or "/" in info.filename or "\\" in info.filename:
                    raise SystemExit(f"{path.name}: unsafe or unexpected entry {info.filename!r}")
                contents[info.filename] = bundle.read(info)
                modes[info.filename] = (info.external_attr >> 16) & 0o777
    else:
        with tarfile.open(path, mode="r:gz") as bundle:
            members = bundle.getmembers()
            names = [member.name for member in members]
            if names != sorted(names) or len(names) != len(set(names)):
                raise SystemExit(f"{path.name}: entries are not sorted and unique")
            for member in members:
                if not member.isfile() or "/" in member.name or "\\" in member.name:
                    raise SystemExit(f"{path.name}: unsafe or unexpected entry {member.name!r}")
                source = bundle.extractfile(member)
                if source is None:
                    raise SystemExit(f"{path.name}: could not read {member.name}")
                contents[member.name] = source.read()
                modes[member.name] = member.mode & 0o777
    return contents, modes

def expected_magic(target_os, data):
    if target_os == "windows":
        return data.startswith(b"MZ")
    if target_os == "linux":
        return data.startswith(b"\x7fELF")
    return data.startswith((b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf"))

native_system = {"Darwin": "darwin", "Linux": "linux", "Windows": "windows"}.get(platform.system())
native_machine = platform.machine().lower()
native_arch = "arm64" if native_machine in {"arm64", "aarch64"} else "amd64" if native_machine in {"x86_64", "amd64"} else None
native_files = None

with tempfile.TemporaryDirectory(prefix="zapier-release-verify.") as temporary:
    temporary_path = Path(temporary)
    for asset_name, (target_os, target_arch, kind) in assets.items():
        asset_path = release_dir / asset_name
        if not asset_path.is_file():
            raise SystemExit(f"missing release asset: {asset_name}")
        actual_digest = hashlib.sha256(asset_path.read_bytes()).hexdigest()
        if actual_digest != recorded[asset_name]:
            raise SystemExit(f"checksum mismatch: {asset_name}")

        contents, modes = read_archive(asset_path, kind)
        suffix = ".exe" if target_os == "windows" else ""
        binaries = {f"zapier-pp-cli{suffix}", f"zapier-pp-mcp{suffix}"}
        expected_names = docs | binaries
        if set(contents) != expected_names:
            raise SystemExit(
                f"{asset_name}: content mismatch: got {sorted(contents)}, want {sorted(expected_names)}"
            )
        for doc in docs:
            if modes[doc] != 0o644:
                raise SystemExit(f"{asset_name}: {doc} mode is {oct(modes[doc])}, want 0o644")
        for binary in binaries:
            if modes[binary] != 0o755:
                raise SystemExit(f"{asset_name}: {binary} mode is {oct(modes[binary])}, want 0o755")
            if not expected_magic(target_os, contents[binary]):
                raise SystemExit(f"{asset_name}: {binary} has the wrong executable format")

            extracted = temporary_path / f"{target_os}-{target_arch}-{binary}"
            extracted.write_bytes(contents[binary])
            extracted.chmod(0o755)
            metadata = subprocess.run(
                ["go", "version", "-m", str(extracted)],
                check=True,
                text=True,
                capture_output=True,
            ).stdout
            if f"build\tGOOS={target_os}" not in metadata or f"build\tGOARCH={target_arch}" not in metadata:
                raise SystemExit(f"{asset_name}: {binary} build metadata has the wrong target")

            # Go does not expose -X settings through `go version -m`. The CLI
            # receives one version variable and the MCP receives both its
            # server version and the linked CLI package version, so the binary
            # must contain at least one or two exact stamp strings respectively.
            stamp_count = contents[binary].count(release_tag.encode("utf-8"))
            minimum_stamps = 2 if binary.startswith("zapier-pp-mcp") else 1
            if stamp_count < minimum_stamps:
                raise SystemExit(
                    f"{asset_name}: {binary} contains {stamp_count} version stamps, "
                    f"want at least {minimum_stamps}"
                )

        if target_os == native_system and target_arch == native_arch:
            native_files = (contents, suffix, temporary_path)

    if native_files is not None:
        contents, suffix, temporary_path = native_files
        cli_path = temporary_path / f"native-zapier-pp-cli{suffix}"
        cli_path.write_bytes(contents[f"zapier-pp-cli{suffix}"])
        cli_path.chmod(0o755)
        cli_version = subprocess.run(
            [str(cli_path), "version"], check=True, text=True, capture_output=True, timeout=15
        ).stdout.strip()
        if cli_version != f"zapier-pp-cli {release_tag}":
            raise SystemExit(f"native CLI reported {cli_version!r}, want 'zapier-pp-cli {release_tag}'")

        mcp_path = temporary_path / f"native-zapier-pp-mcp{suffix}"
        mcp_path.write_bytes(contents[f"zapier-pp-mcp{suffix}"])
        mcp_path.chmod(0o755)
        request = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2025-03-26",
                "capabilities": {},
                "clientInfo": {"name": "release-verifier", "version": "1"},
            },
        }
        result = subprocess.run(
            [str(mcp_path)],
            input=json.dumps(request) + "\n",
            check=True,
            text=True,
            capture_output=True,
            timeout=15,
        )
        responses = []
        for line in result.stdout.splitlines():
            try:
                responses.append(json.loads(line))
            except json.JSONDecodeError:
                continue
        reported = next(
            (
                response.get("result", {}).get("serverInfo", {}).get("version")
                for response in responses
                if response.get("id") == 1
            ),
            None,
        )
        if reported != release_tag:
            raise SystemExit(f"native MCP reported {reported!r}, want {release_tag!r}")

print(f"verified {len(assets)} release archives and native runtime version {release_tag}")
PY
