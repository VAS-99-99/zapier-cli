#!/bin/sh

set -eu

REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd -P)
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/zapier-install-test.XXXXXX")

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

cat >"$MOCK_BIN/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf '%s\n' "${MOCK_UNAME_S:-Linux}" ;;
  -m) printf '%s\n' "${MOCK_UNAME_M:-x86_64}" ;;
  *) exit 2 ;;
esac
EOF

cat >"$MOCK_BIN/curl" <<'EOF'
#!/bin/sh
set -eu
destination=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) destination=$2; shift 2 ;;
    --retry) shift 2 ;;
    --fail|--location|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
case "$url" in
  *api.github.com*/releases*) printf '[{"tag_name":"%s","draft":false}]\n' "${MOCK_RELEASE_TAG:-v1-prerelease}" >"$destination" ;;
  *) cp "$MOCK_RELEASE_DIR/${url##*/}" "$destination" ;;
esac
EOF
chmod +x "$MOCK_BIN/uname" "$MOCK_BIN/curl"

printf '#!/bin/sh\nprintf "zapier-pp-cli v1-prerelease\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli"
printf '#!/bin/sh\nprintf "new mcp\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-mcp"
chmod +x "$RELEASE_DIR/payload/zapier-pp-cli" "$RELEASE_DIR/payload/zapier-pp-mcp"
tar -czf "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" -C "$RELEASE_DIR/payload" zapier-pp-cli zapier-pp-mcp
if command -v shasum >/dev/null 2>&1; then
  RELEASE_HASH=$(shasum -a 256 "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" | awk '{print $1}')
else
  RELEASE_HASH=$(sha256sum "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" | awk '{print $1}')
fi
printf '%s  %s\n' "$RELEASE_HASH" zapier-cli_linux_x86_64.tar.gz >"$RELEASE_DIR/SHA256SUMS"

TEST_PATH="$MOCK_BIN:$PATH"

public_verify_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1)
assert_contains "$public_verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '%064d  %s\n' 0 zapier-cli_linux_x86_64.tar.gz >"$RELEASE_DIR/SHA256SUMS"
checksum_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "bad checksum unexpectedly succeeded"
assert_contains "$checksum_output" "checksum verification failed"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

verify_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --verify-only 2>&1)
assert_contains "$verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

version_mismatch_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v2-other --verify-only 2>&1) && fail "mismatched release version unexpectedly succeeded"
assert_contains "$version_mismatch_output" "reports version v1-prerelease but release tag is v2-other"

cp "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz.good"
cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '#!/bin/sh\nprintf "zapier-pp-cli 0.0.0-dev\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli"
chmod +x "$RELEASE_DIR/payload/zapier-pp-cli"
tar -czf "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" -C "$RELEASE_DIR/payload" zapier-pp-cli zapier-pp-mcp
if command -v shasum >/dev/null 2>&1; then
  DEV_HASH=$(shasum -a 256 "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" | awk '{print $1}')
else
  DEV_HASH=$(sha256sum "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz" | awk '{print $1}')
fi
printf '%s  %s\n' "$DEV_HASH" zapier-cli_linux_x86_64.tar.gz >"$RELEASE_DIR/SHA256SUMS"
dev_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v3-dev --verify-only 2>&1) && fail "development binary unexpectedly passed verification"
assert_contains "$dev_output" "development version 0.0.0-dev"
mv "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz.good" "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

INSTALL_DIR="$TEST_ROOT/install target"
mkdir -p "$INSTALL_DIR"
printf 'old cli\n' >"$INSTALL_DIR/zapier-pp-cli"
printf 'old mcp\n' >"$INSTALL_DIR/zapier-pp-mcp"
install_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$INSTALL_DIR" --no-path-update 2>&1)

installed_cli=$("$INSTALL_DIR/zapier-pp-cli" version)
installed_mcp=$("$INSTALL_DIR/zapier-pp-mcp")
[ "$installed_cli" = "zapier-pp-cli v1-prerelease" ] || fail "zapier-pp-cli was not replaced with verified payload"
[ "$installed_mcp" = "new mcp" ] || fail "zapier-pp-mcp was not replaced with verified payload"
assert_contains "$install_output" "claude mcp add --scope user zapier --"
assert_contains "$install_output" "codex mcp add zapier --"
assert_contains "$install_output" "zapier-pp-cli session --agent --no-learn"
assert_contains "$install_output" "CLI version v1-prerelease"

unsupported_output=$(PATH="$TEST_PATH" MOCK_UNAME_S=Linux MOCK_UNAME_M=arm64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "unsupported platform unexpectedly succeeded"
assert_contains "$unsupported_output" "Linux arm64"

printf 'PASS: install.sh public download, platform, checksum, verification, replacement, and guidance tests\n'
