#!/bin/sh

set -eu

REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd -P)
command -v pwsh >/dev/null 2>&1 || {
  printf 'SKIP: pwsh is not installed; install.ps1 runtime tests were not run\n'
  exit 0
}
command -v zip >/dev/null 2>&1 || {
  printf 'SKIP: zip is not installed; install.ps1 runtime tests were not run\n'
  exit 0
}

TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/zapier-install-ps1-test.XXXXXX")
cleanup() {
  if [ -n "$TEST_ROOT" ] && [ -d "$TEST_ROOT" ]; then
    rm -rf -- "$TEST_ROOT"
  fi
}
trap cleanup 0 HUP INT TERM

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  haystack=$1
  needle=$2
  printf '%s' "$haystack" | grep -F "$needle" >/dev/null || fail "output did not contain: $needle"
}

MOCK_BIN="$TEST_ROOT/bin"
RELEASE_DIR="$TEST_ROOT/release"
mkdir -p "$MOCK_BIN" "$RELEASE_DIR/payload"

cat >"$MOCK_BIN/gh" <<'EOF'
#!/bin/sh
set -eu
if [ "$1 $2" = "auth status" ]; then
  [ "${MOCK_GH_AUTH:-ok}" = ok ]
  exit
fi
if [ "$1 $2" = "release view" ]; then
  printf '%s\n' "${MOCK_RELEASE_TAG:-v-test}"
  exit
fi
if [ "$1 $2" = "release download" ]; then
  shift 2
  destination=""
  patterns=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dir) destination=$2; shift 2 ;;
      --pattern) patterns="$patterns $2"; shift 2 ;;
      --repo) shift 2 ;;
      --clobber) shift ;;
      *) shift ;;
    esac
  done
  for pattern in $patterns; do
    cp "$MOCK_RELEASE_DIR/$pattern" "$destination/$pattern"
  done
  exit
fi
exit 3
EOF
chmod +x "$MOCK_BIN/gh"

printf '#!/bin/sh\nprintf "zapier-pp-cli v-prerelease\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli.exe"
printf 'new windows mcp\n' >"$RELEASE_DIR/payload/zapier-pp-mcp.exe"
chmod +x "$RELEASE_DIR/payload/zapier-pp-cli.exe"
(cd "$RELEASE_DIR/payload" && zip -q "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" zapier-pp-cli.exe zapier-pp-mcp.exe)
if command -v shasum >/dev/null 2>&1; then
  RELEASE_HASH=$(shasum -a 256 "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
else
  RELEASE_HASH=$(sha256sum "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
fi
printf '%s  %s\n' "$RELEASE_HASH" zapier-cli_windows_x86_64.zip >"$RELEASE_DIR/SHA256SUMS"

TEST_PATH="$MOCK_BIN:$PATH"
PS_ENV="OS=Windows_NT"

missing_auth_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_GH_AUTH=missing MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-prerelease -VerifyOnly 2>&1) && fail "missing authentication unexpectedly succeeded"
assert_contains "$missing_auth_output" "GitHub CLI is not signed in"
assert_contains "$missing_auth_output" "gh auth"

cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '%064d  %s\n' 0 zapier-cli_windows_x86_64.zip >"$RELEASE_DIR/SHA256SUMS"
checksum_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-prerelease -VerifyOnly 2>&1) && fail "bad checksum unexpectedly succeeded"
assert_contains "$checksum_output" "checksum verification failed"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

verify_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-prerelease -VerifyOnly 2>&1)
assert_contains "$verify_output" "Verified release v-prerelease (CLI version v-prerelease); no files were installed."

version_mismatch_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-other -VerifyOnly 2>&1) && fail "mismatched release version unexpectedly succeeded"
assert_contains "$version_mismatch_output" "reports version"
assert_contains "$version_mismatch_output" "release tag is v-other"

cp "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" "$RELEASE_DIR/zapier-cli_windows_x86_64.zip.good"
cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '#!/bin/sh\nprintf "zapier-pp-cli 0.0.0-dev\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli.exe"
chmod +x "$RELEASE_DIR/payload/zapier-pp-cli.exe"
(cd "$RELEASE_DIR/payload" && zip -q -FS "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" zapier-pp-cli.exe zapier-pp-mcp.exe)
if command -v shasum >/dev/null 2>&1; then
  DEV_HASH=$(shasum -a 256 "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
else
  DEV_HASH=$(sha256sum "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
fi
printf '%s  %s\n' "$DEV_HASH" zapier-cli_windows_x86_64.zip >"$RELEASE_DIR/SHA256SUMS"
dev_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-dev -VerifyOnly 2>&1) && fail "development binary unexpectedly passed verification"
assert_contains "$dev_output" "contains development"
assert_contains "$dev_output" "version 0.0.0-dev"
mv "$RELEASE_DIR/zapier-cli_windows_x86_64.zip.good" "$RELEASE_DIR/zapier-cli_windows_x86_64.zip"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

INSTALL_DIR="$TEST_ROOT/install target"
mkdir -p "$INSTALL_DIR"
printf 'old windows cli\n' >"$INSTALL_DIR/zapier-pp-cli.exe"
printf 'old windows mcp\n' >"$INSTALL_DIR/zapier-pp-mcp.exe"
install_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-prerelease -InstallDir "$INSTALL_DIR" -NoPathUpdate 2>&1)

[ "$("$INSTALL_DIR/zapier-pp-cli.exe" version)" = "zapier-pp-cli v-prerelease" ] || fail "zapier-pp-cli.exe was not replaced with verified payload"
[ "$(cat "$INSTALL_DIR/zapier-pp-mcp.exe")" = "new windows mcp" ] || fail "zapier-pp-mcp.exe was not replaced with verified payload"
assert_contains "$install_output" "claude mcp add --scope user zapier --"
assert_contains "$install_output" "codex mcp add zapier --"
assert_contains "$install_output" "zapier-pp-cli session --agent --no-learn"
assert_contains "$install_output" "CLI version v-prerelease"

unsupported_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=ARM64 PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$REPO_ROOT/install.ps1" -Tag v-prerelease -VerifyOnly 2>&1) && fail "unsupported architecture unexpectedly succeeded"
assert_contains "$unsupported_output" "Windows architecture: ARM64"

printf 'PASS: install.ps1 authentication, architecture, checksum, verification, replacement, and guidance tests\n'
