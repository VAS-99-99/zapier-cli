#!/usr/bin/env bash
set -euo pipefail
destination=${1:?destination directory required}
mkdir -p "$destination"
filename=agent-browser-win32-x64.exe
url="https://github.com/VAS-99-99/zapier-cli/releases/download/v0.1.0-rc.7/$filename"
if ! curl --fail --location --silent --show-error "$url" -o "$destination/$filename"; then
  # Bootstrap rc.7 from the successful pinned-source build. After publication,
  # the immutable release URL above is the durable source for future gates.
  rm -f "$destination/$filename"
  gh run download 34118315767 --repo VAS-99-99/zapier-cli --name windows-browser-helper --dir "$destination"
fi
printf '%s  %s\n' 3d26b5541213d7d7ecce5908e4908990d51ca55fb524ef0af12ae3ac9f7f4a66 "$destination/$filename" | sha256sum --check
