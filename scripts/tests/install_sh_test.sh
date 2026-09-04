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
chmod +x "$MOCK_BIN/uname" "$MOCK_BIN/gh"

printf '#!/bin/sh\nprintf "zapier-pp-cli v-prerelease\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli"
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

missing_auth_output=$(PATH="$TEST_PATH" MOCK_GH_AUTH=missing MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "missing authentication unexpectedly succeeded"
assert_contains "$missing_auth_output" "gh auth login"

cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '%064d  %s\n' 0 zapier-cli_linux_x86_64.tar.gz >"$RELEASE_DIR/SHA256SUMS"
checksum_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "bad checksum unexpectedly succeeded"
assert_contains "$checksum_output" "checksum verification failed"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

verify_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v-prerelease --verify-only 2>&1)
assert_contains "$verify_output" "Verified release v-prerelease (CLI version v-prerelease); no files were installed."

version_mismatch_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v-other --verify-only 2>&1) && fail "mismatched release version unexpectedly succeeded"
assert_contains "$version_mismatch_output" "reports version v-prerelease but release tag is v-other"

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
  sh "$REPO_ROOT/install.sh" --tag v-dev --verify-only 2>&1) && fail "development binary unexpectedly passed verification"
assert_contains "$dev_output" "development version 0.0.0-dev"
mv "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz.good" "$RELEASE_DIR/zapier-cli_linux_x86_64.tar.gz"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

INSTALL_DIR="$TEST_ROOT/install target"
mkdir -p "$INSTALL_DIR"
printf 'old cli\n' >"$INSTALL_DIR/zapier-pp-cli"
printf 'old mcp\n' >"$INSTALL_DIR/zapier-pp-mcp"
install_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v-prerelease --install-dir "$INSTALL_DIR" --no-path-update 2>&1)

installed_cli=$("$INSTALL_DIR/zapier-pp-cli" version)
installed_mcp=$("$INSTALL_DIR/zapier-pp-mcp")
[ "$installed_cli" = "zapier-pp-cli v-prerelease" ] || fail "zapier-pp-cli was not replaced with verified payload"
[ "$installed_mcp" = "new mcp" ] || fail "zapier-pp-mcp was not replaced with verified payload"
assert_contains "$install_output" "claude mcp add --scope user zapier --"
assert_contains "$install_output" "codex mcp add zapier --"
assert_contains "$install_output" "zapier-pp-cli session --agent --no-learn"
assert_contains "$install_output" "CLI version v-prerelease"

unsupported_output=$(PATH="$TEST_PATH" MOCK_UNAME_S=Linux MOCK_UNAME_M=arm64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "unsupported platform unexpectedly succeeded"
assert_contains "$unsupported_output" "Linux arm64"

printf 'PASS: install.sh authentication, platform, checksum, verification, replacement, and guidance tests\n'
