#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <v-tag> [output-directory]" >&2
  exit 2
}

[[ $# -ge 1 && $# -le 2 ]] || usage

release_tag=$1
output_dir=${2:-dist}

if [[ ! "$release_tag" =~ ^v[0-9A-Za-z][0-9A-Za-z._-]*$ ]]; then
  echo "release tag must start with v and contain only letters, numbers, dots, underscores, or hyphens" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)

if [[ "$output_dir" != /* ]]; then
  output_dir="$repo_root/$output_dir"
fi

for required_file in README.md SKILL.md LICENSE go.mod; do
  if [[ ! -f "$repo_root/$required_file" ]]; then
    echo "required release file is missing: $required_file" >&2
    exit 1
  fi
done

command -v go >/dev/null 2>&1 || {
  echo "Go is required to build release artifacts" >&2
  exit 1
}
command -v python3 >/dev/null 2>&1 || {
  echo "python3 is required to create deterministic release archives" >&2
  exit 1
}

source_date_epoch=${SOURCE_DATE_EPOCH:-}
if [[ -z "$source_date_epoch" ]]; then
  source_date_epoch=$(git -C "$repo_root" log -1 --format=%ct 2>/dev/null || true)
fi
if [[ ! "$source_date_epoch" =~ ^[0-9]+$ ]]; then
  echo "SOURCE_DATE_EPOCH must be an integer Unix timestamp" >&2
  exit 2
fi

mkdir -p "$output_dir"

# Remove only assets this script owns. Never clear the caller's output directory.
owned_assets=(
  zapier-cli_windows_x86_64.zip
  zapier-cli_darwin_arm64.tar.gz
  zapier-cli_darwin_x86_64.tar.gz
  zapier-cli_linux_x86_64.tar.gz
  SHA256SUMS
)
for asset in "${owned_assets[@]}"; do
  rm -f "$output_dir/$asset"
done

build_root=$(mktemp -d "${TMPDIR:-/tmp}/zapier-release.XXXXXX")
cleanup() {
  rm -rf "$build_root"
}
trap cleanup EXIT

module_path=$(cd "$repo_root" && go list -m)
base_ldflags="-s -w -buildid="
cli_ldflags="$base_ldflags -X ${module_path}/internal/cli.version=${release_tag}"
mcp_ldflags="$base_ldflags -X ${module_path}/internal/cli.version=${release_tag} -X main.version=${release_tag}"

package_archive() {
  local stage_dir=$1
  local archive_path=$2
  local archive_kind=$3

  python3 - "$stage_dir" "$archive_path" "$archive_kind" "$source_date_epoch" <<'PY'
import gzip
from pathlib import Path
import sys
import tarfile
import time
import zipfile

stage = Path(sys.argv[1])
archive = Path(sys.argv[2])
kind = sys.argv[3]
epoch = int(sys.argv[4])
names = sorted(path.name for path in stage.iterdir() if path.is_file())

if kind == "tar.gz":
    with archive.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=epoch, compresslevel=9) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as bundle:
                for name in names:
                    path = stage / name
                    info = bundle.gettarinfo(str(path), arcname=name)
                    info.uid = 0
                    info.gid = 0
                    info.uname = "root"
                    info.gname = "root"
                    info.mtime = epoch
                    info.mode = 0o755 if name.startswith("zapier-pp-") else 0o644
                    with path.open("rb") as source:
                        bundle.addfile(info, source)
elif kind == "zip":
    # Store entries rather than deflating them so bytes do not depend on the
    # host zlib implementation. Releases remain deterministic across runners.
    zip_epoch = max(epoch, 315532800)  # ZIP timestamps cannot predate 1980.
    date_time = time.gmtime(zip_epoch)[:6]
    with zipfile.ZipFile(archive, mode="w", compression=zipfile.ZIP_STORED) as bundle:
        for name in names:
            mode = 0o755 if name.startswith("zapier-pp-") else 0o644
            info = zipfile.ZipInfo(name, date_time=date_time)
            info.create_system = 3
            info.external_attr = mode << 16
            with (stage / name).open("rb") as source:
                bundle.writestr(info, source.read())
else:
    raise SystemExit(f"unsupported archive kind: {kind}")
PY
}

build_target() {
  local target_os=$1
  local target_arch=$2
  local archive_name=$3
  local archive_kind=$4
  local suffix=""
  local stage_dir="$build_root/${target_os}_${target_arch}"

  if [[ "$target_os" == "windows" ]]; then
    suffix=".exe"
  fi

  mkdir -p "$stage_dir"
  (
    cd "$repo_root"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
      go build -mod=readonly -trimpath -buildvcs=false -ldflags "$cli_ldflags" \
      -o "$stage_dir/zapier-pp-cli$suffix" ./cmd/zapier-pp-cli
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
      go build -mod=readonly -trimpath -buildvcs=false -ldflags "$mcp_ldflags" \
      -o "$stage_dir/zapier-pp-mcp$suffix" ./cmd/zapier-pp-mcp
  )

  cp "$repo_root/README.md" "$repo_root/SKILL.md" "$repo_root/LICENSE" "$stage_dir/"
  chmod 0755 "$stage_dir/zapier-pp-cli$suffix" "$stage_dir/zapier-pp-mcp$suffix"
  chmod 0644 "$stage_dir/README.md" "$stage_dir/SKILL.md" "$stage_dir/LICENSE"
  package_archive "$stage_dir" "$output_dir/$archive_name" "$archive_kind"
}

build_target windows amd64 zapier-cli_windows_x86_64.zip zip
build_target darwin arm64 zapier-cli_darwin_arm64.tar.gz tar.gz
build_target darwin amd64 zapier-cli_darwin_x86_64.tar.gz tar.gz
build_target linux amd64 zapier-cli_linux_x86_64.tar.gz tar.gz

python3 - "$output_dir" "${owned_assets[@]:0:4}" <<'PY'
import hashlib
from pathlib import Path
import sys

output = Path(sys.argv[1])
assets = sorted(sys.argv[2:])
lines = []
for name in assets:
    digest = hashlib.sha256((output / name).read_bytes()).hexdigest()
    lines.append(f"{digest}  {name}\n")
(output / "SHA256SUMS").write_text("".join(lines), encoding="utf-8", newline="\n")
PY

echo "release artifacts written to $output_dir"
