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
if [ "${MOCK_CURL_FAIL:-false}" = true; then
  exit 22
fi
case "$url" in
  *api.github.com*/releases*)
    if [ "${MOCK_PUBLIC_MULTI:-false}" = true ]; then
      printf '[\n{"tag_name":"v1-prerelease","draft":false},\n{"tag_name":"v0-older","draft":false}\n]\n' >"$destination"
    else
      printf '[{"tag_name":"%s","draft":false}]\n' "${MOCK_RELEASE_TAG:-v1-prerelease}" >"$destination"
    fi
    ;;
  *) cp "$MOCK_RELEASE_DIR/${url##*/}" "$destination" ;;
esac
EOF

cat >"$MOCK_BIN/gh" <<'EOF'
#!/bin/sh
set -eu
case "$1" in
  auth)
    [ "${MOCK_GH_AUTH:-false}" = true ]
    ;;
  api)
    [ "${MOCK_GH_AUTH:-false}" = true ]
    [ "$2" = 'repos/VAS-99-99/zapier-cli/releases?per_page=20' ]
    if [ "${3:-}" = --jq ]; then
      [ "${4:-}" = '[.[] | select(.draft == false)][0].tag_name // empty' ]
      printf '%s\n' "${MOCK_RELEASE_TAG:-v1-prerelease}"
    else
      printf '[{"tag_name":"v999-draft","draft":true},{"tag_name":"%s","draft":false},{"tag_name":"v0-older","draft":false}]' "${MOCK_RELEASE_TAG:-v1-prerelease}"
    fi
    ;;
  release)
    shift
    [ "$1" = download ]
    shift
    tag=$1
    shift
    dir=""
    patterns=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --repo) shift 2 ;;
        --pattern) patterns="$patterns $2"; shift 2 ;;
        --dir) dir=$2; shift 2 ;;
        --clobber) shift ;;
        *) exit 2 ;;
      esac
    done
    [ "$tag" = "${MOCK_RELEASE_TAG:-v1-prerelease}" ]
    for pattern in $patterns; do
      cp "$MOCK_RELEASE_DIR/$pattern" "$dir/$pattern"
    done
    ;;
  *) exit 2 ;;
esac
EOF

cat >"$MOCK_BIN/claude" <<'EOF'
#!/bin/sh
set -eu
printf 'claude %s\n' "$*" >>"$MOCK_HOST_LOG"
[ "${MOCK_HOST_FAIL:-}" != "claude:$*" ]
EOF

cat >"$MOCK_BIN/codex" <<'EOF'
#!/bin/sh
set -eu
printf 'codex %s\n' "$*" >>"$MOCK_HOST_LOG"
[ "${MOCK_HOST_FAIL:-}" != "codex:$*" ]
EOF
chmod +x "$MOCK_BIN/uname" "$MOCK_BIN/curl" "$MOCK_BIN/gh" "$MOCK_BIN/claude" "$MOCK_BIN/codex"

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

public_multiline_output=$(PATH="$TEST_PATH" MOCK_PUBLIC_MULTI=true MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1)
assert_contains "$public_multiline_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

private_verify_output=$(PATH="$TEST_PATH" MOCK_CURL_FAIL=true MOCK_GH_AUTH=true MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1)
assert_contains "$private_verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '%064d  %s\n' 0 zapier-cli_linux_x86_64.tar.gz >"$RELEASE_DIR/SHA256SUMS"
checksum_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "bad checksum unexpectedly succeeded"
assert_contains "$checksum_output" "checksum verification failed"
FALLBACK_INSTALL_DIR="$TEST_ROOT/fallback install"
mkdir -p "$FALLBACK_INSTALL_DIR"
printf 'old cli\n' >"$FALLBACK_INSTALL_DIR/zapier-pp-cli"
fallback_checksum_output=$(PATH="$TEST_PATH" MOCK_CURL_FAIL=true MOCK_GH_AUTH=true MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$FALLBACK_INSTALL_DIR" --no-path-update 2>&1) && fail "bad fallback checksum unexpectedly succeeded"
assert_contains "$fallback_checksum_output" "checksum verification failed"
[ "$(cat "$FALLBACK_INSTALL_DIR/zapier-pp-cli")" = "old cli" ] || fail "bad fallback checksum changed the existing installation"
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

# A normal install remains CLI-only: it must not invoke either host command.
[ ! -e "$TEST_ROOT/default-host.log" ] || fail "default install registered a host plugin"

# Verify-only is deliberately host-write-free even when an agent was selected.
agent_verify_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" MOCK_HOST_LOG="$TEST_ROOT/verify-host.log" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --verify-only --agent claude 2>&1)
assert_contains "$agent_verify_output" "no files were installed"
[ ! -e "$TEST_ROOT/verify-host.log" ] || fail "verify-only invoked a host command"

CLAUDE_LOG="$TEST_ROOT/claude-host.log"
claude_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" MOCK_HOST_LOG="$CLAUDE_LOG" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$TEST_ROOT/claude install" --no-path-update --agent claude 2>&1)
assert_contains "$claude_output" "Installed or updated zapier-read-only@vas-zapier-cli for claude."
assert_contains "$(cat "$CLAUDE_LOG")" "claude plugin marketplace add VAS-99-99/zapier-cli --scope user"
assert_contains "$(cat "$CLAUDE_LOG")" "claude plugin marketplace update vas-zapier-cli"
assert_contains "$(cat "$CLAUDE_LOG")" "claude plugin install zapier-read-only@vas-zapier-cli --scope user"
assert_contains "$(cat "$CLAUDE_LOG")" "claude plugin update zapier-read-only@vas-zapier-cli --scope user"

# The native sequence is idempotent: a repeated selected-host run succeeds.
PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" MOCK_HOST_LOG="$CLAUDE_LOG" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$TEST_ROOT/claude install" --no-path-update --agent claude >/dev/null 2>&1 || fail "repeated Claude install failed"

CODEX_LOG="$TEST_ROOT/codex-host.log"
codex_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" MOCK_HOST_LOG="$CODEX_LOG" \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$TEST_ROOT/codex install" --no-path-update --agent codex 2>&1)
assert_contains "$codex_output" "Installed or updated zapier-read-only@vas-zapier-cli for codex."
assert_contains "$(cat "$CODEX_LOG")" "codex plugin marketplace add VAS-99-99/zapier-cli"
assert_contains "$(cat "$CODEX_LOG")" "codex plugin marketplace upgrade vas-zapier-cli"
assert_contains "$(cat "$CODEX_LOG")" "codex plugin add zapier-read-only@vas-zapier-cli"

plugin_failure_output=$(PATH="$TEST_PATH" MOCK_RELEASE_DIR="$RELEASE_DIR" MOCK_HOST_LOG="$TEST_ROOT/failure-host.log" MOCK_HOST_FAIL='codex:plugin add zapier-read-only@vas-zapier-cli' \
  sh "$REPO_ROOT/install.sh" --tag v1-prerelease --install-dir "$TEST_ROOT/failure install" --no-path-update --agent codex 2>&1) && fail "plugin failure unexpectedly succeeded"
assert_contains "$plugin_failure_output" "CLI version v1-prerelease was installed, but codex plugin setup failed"
[ -x "$TEST_ROOT/failure install/zapier-pp-cli" ] || fail "plugin failure rolled back the installed CLI"

invalid_agent_output=$(PATH="$TEST_PATH" sh "$REPO_ROOT/install.sh" --agent nope --verify-only 2>&1) && fail "invalid agent unexpectedly succeeded"
assert_contains "$invalid_agent_output" "unsupported agent: nope"

unsupported_output=$(PATH="$TEST_PATH" MOCK_UNAME_S=Linux MOCK_UNAME_M=arm64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  sh "$REPO_ROOT/install.sh" --verify-only 2>&1) && fail "unsupported platform unexpectedly succeeded"
assert_contains "$unsupported_output" "Linux arm64"

printf 'PASS: install.sh public download, platform, checksum, verification, CLI-only default, and opt-in plugin tests\n'
