#!/bin/sh

set -eu

REPOSITORY="VAS-99-99/zapier-cli"
TAG=""
VERIFY_ONLY=false
UPDATE_PATH=true
INSTALL_DIR="${XDG_BIN_HOME:-${HOME:?HOME is not set}/.local/bin}"
TEMP_DIR=""

usage() {
  cat <<'EOF'
Install the prebuilt Zapier read-only CLI from its GitHub repository.

Usage: ./install.sh [options]

Options:
  --tag TAG          Install a specific release tag (including a prerelease).
  --install-dir DIR  Install into DIR (default: ~/.local/bin).
  --verify-only      Download and verify the archive without installing it.
  --no-path-update   Do not add the install directory to a shell profile.
  -h, --help         Show this help.

Uses `curl` or `wget` for anonymous downloads. For a private release, install
GitHub CLI and sign in yourself with `gh auth login`.
EOF
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
    rm -rf -- "$TEMP_DIR"
  fi
}

download() {
  source_url=$1
  destination=$2
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error --retry 2 --output "$destination" "$source_url"
  elif command -v wget >/dev/null 2>&1; then
    wget --quiet --output-document="$destination" "$source_url"
  else
    fail "curl or wget is required to download the public release"
  fi
}

github_cli_authenticated() {
  command -v gh >/dev/null 2>&1 && gh auth status --hostname github.com >/dev/null 2>&1
}

github_cli_hint() {
  printf '%s' 'Run "gh auth login" and retry, or download the release archive and SHA256SUMS manually from GitHub.'
}

read_cli_version() {
  version_output=$("$1" version 2>&1) || return 1
  version_line=$(printf '%s\n' "$version_output" | awk '/^zapier-pp-cli [^[:space:]]+$/ { line = $0 } END { print line }')
  [ -n "$version_line" ] || return 1
  printf '%s\n' "$version_line" | awk '{ print $2 }'
}

quote_for_shell() {
  escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
  printf "'%s'" "$escaped"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag)
      [ "$#" -ge 2 ] || fail "--tag requires a value"
      TAG=$2
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || fail "--install-dir requires a value"
      INSTALL_DIR=$2
      shift 2
      ;;
    --verify-only)
      VERIFY_ONLY=true
      shift
      ;;
    --no-path-update)
      UPDATE_PATH=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1 (run ./install.sh --help)"
      ;;
  esac
done

[ -n "$INSTALL_DIR" ] || fail "the install directory cannot be empty"
case "$INSTALL_DIR" in
  *"
"*) fail "the install directory cannot contain a newline" ;;
esac

case "$(uname -s)" in
  Darwin) PLATFORM=darwin ;;
  Linux) PLATFORM=linux ;;
  *) fail "unsupported operating system: $(uname -s). Supported: macOS and Linux x64." ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=x86_64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) fail "unsupported architecture: $(uname -m). Supported: macOS arm64/x86_64 and Linux x86_64." ;;
esac

if [ "$PLATFORM" = linux ] && [ "$ARCH" != x86_64 ]; then
  fail "unsupported platform: Linux $ARCH. This release currently supports Linux x86_64 only."
fi

ASSET_NAME="zapier-cli_${PLATFORM}_${ARCH}.tar.gz"

TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/zapier-cli-install.XXXXXX") || fail "could not create a temporary directory"
trap cleanup 0
trap 'exit 130' HUP INT TERM

if [ -z "$TAG" ]; then
  RELEASES_JSON="$TEMP_DIR/releases.json"
  if ! download "https://api.github.com/repos/$REPOSITORY/releases?per_page=20" "$RELEASES_JSON"; then
    if github_cli_authenticated; then
      TAG=$(gh api "repos/$REPOSITORY/releases?per_page=20" --jq '[.[] | select(.draft == false)][0].tag_name // empty' 2>/dev/null) ||
        fail "could not resolve the newest release in $REPOSITORY with GitHub CLI. $(github_cli_hint)"
      [ -n "$TAG" ] || fail "the repository $REPOSITORY does not have a published release"
    else
      fail "could not resolve the newest release in $REPOSITORY anonymously. $(github_cli_hint)"
    fi
  fi
  if [ -z "$TAG" ]; then
    TAG=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$RELEASES_JSON" | head -n 1)
    [ -n "$TAG" ] || fail "the repository $REPOSITORY does not have a published release"
  fi
fi

case "$TAG" in
  v[0-9A-Za-z]* ) ;;
  *) fail "invalid release tag: $TAG" ;;
esac
case "$TAG" in
  *[!0-9A-Za-z._-]*) fail "invalid release tag: $TAG" ;;
esac

printf 'Downloading %s (%s)...\n' "$ASSET_NAME" "$TAG"
RELEASE_ROOT="https://github.com/$REPOSITORY/releases/download/$TAG"
if ! download "$RELEASE_ROOT/$ASSET_NAME" "$TEMP_DIR/$ASSET_NAME" ||
   ! download "$RELEASE_ROOT/SHA256SUMS" "$TEMP_DIR/SHA256SUMS"; then
  if github_cli_authenticated; then
    gh release download "$TAG" --repo "$REPOSITORY" --pattern "$ASSET_NAME" --pattern SHA256SUMS --dir "$TEMP_DIR" --clobber >/dev/null 2>&1 ||
      fail "could not download release $TAG from $REPOSITORY with GitHub CLI. $(github_cli_hint)"
  else
    fail "could not download release $TAG from $REPOSITORY anonymously. $(github_cli_hint)"
  fi
fi

ARCHIVE_PATH="$TEMP_DIR/$ASSET_NAME"
SUMS_PATH="$TEMP_DIR/SHA256SUMS"
[ -f "$ARCHIVE_PATH" ] || fail "release $TAG does not contain $ASSET_NAME"
[ -f "$SUMS_PATH" ] || fail "release $TAG does not contain SHA256SUMS"

EXPECTED_HASH=$(awk -v name="$ASSET_NAME" '
  {
    file = $2
    sub(/^\*/, "", file)
    if (file == name) {
      print tolower($1)
      exit
    }
  }
' "$SUMS_PATH")

[ "${#EXPECTED_HASH}" -eq 64 ] || fail "SHA256SUMS does not contain a valid checksum for $ASSET_NAME"
case "$EXPECTED_HASH" in
  *[!0-9a-f]*) fail "SHA256SUMS contains an invalid checksum for $ASSET_NAME" ;;
esac

if command -v shasum >/dev/null 2>&1; then
  ACTUAL_HASH=$(shasum -a 256 "$ARCHIVE_PATH" | awk '{print tolower($1)}')
elif command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_HASH=$(sha256sum "$ARCHIVE_PATH" | awk '{print tolower($1)}')
else
  fail "no SHA-256 utility found; install shasum or sha256sum and retry"
fi

[ "$ACTUAL_HASH" = "$EXPECTED_HASH" ] || fail "checksum verification failed for $ASSET_NAME; nothing was installed"
printf 'Checksum verified for %s.\n' "$ASSET_NAME"

command -v tar >/dev/null 2>&1 || fail "tar is required to extract $ASSET_NAME"
EXTRACT_DIR="$TEMP_DIR/extracted"
mkdir -p "$EXTRACT_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR" || fail "could not extract $ASSET_NAME"

for binary in zapier-pp-cli zapier-pp-mcp; do
  [ -f "$EXTRACT_DIR/$binary" ] || fail "$ASSET_NAME is missing $binary; nothing was installed"
  [ -s "$EXTRACT_DIR/$binary" ] || fail "$ASSET_NAME contains an empty $binary; nothing was installed"
  chmod 755 "$EXTRACT_DIR/$binary"
done

RELEASE_VERSION=$(read_cli_version "$EXTRACT_DIR/zapier-pp-cli") ||
  fail "$ASSET_NAME contains a zapier-pp-cli binary that cannot report its release version; nothing was installed"
case "$RELEASE_VERSION" in
  0.0.0-dev*) fail "$ASSET_NAME contains development version $RELEASE_VERSION; nothing was installed. Use a properly stamped GitHub Release." ;;
esac
[ "$RELEASE_VERSION" = "$TAG" ] ||
  fail "$ASSET_NAME reports version $RELEASE_VERSION but release tag is $TAG; nothing was installed"

if [ "$VERIFY_ONLY" = true ]; then
  printf 'Verified release %s (CLI version %s); no files were installed.\n' "$TAG" "$RELEASE_VERSION"
  exit 0
fi

mkdir -p "$INSTALL_DIR" || fail "could not create install directory: $INSTALL_DIR"
INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd -P) || fail "could not resolve install directory: $INSTALL_DIR"

CLI_TARGET="$INSTALL_DIR/zapier-pp-cli"
MCP_TARGET="$INSTALL_DIR/zapier-pp-mcp"
CLI_PENDING="$INSTALL_DIR/.zapier-pp-cli.install.$$"
MCP_PENDING="$INSTALL_DIR/.zapier-pp-mcp.install.$$"
CLI_BACKUP="$INSTALL_DIR/.zapier-pp-cli.backup.$$"
MCP_BACKUP="$INSTALL_DIR/.zapier-pp-mcp.backup.$$"

if ! cp "$EXTRACT_DIR/zapier-pp-cli" "$CLI_PENDING" ||
   ! cp "$EXTRACT_DIR/zapier-pp-mcp" "$MCP_PENDING"; then
  rm -f -- "$CLI_PENDING" "$MCP_PENDING"
  fail "could not stage both executables in $INSTALL_DIR; the existing installation was not changed"
fi
chmod 755 "$CLI_PENDING" "$MCP_PENDING"

CLI_EXISTED=false
MCP_EXISTED=false
if [ -e "$CLI_TARGET" ]; then
  cp -p "$CLI_TARGET" "$CLI_BACKUP" || {
    rm -f -- "$CLI_PENDING" "$MCP_PENDING"
    fail "could not preserve the existing zapier-pp-cli; the existing installation was not changed"
  }
  CLI_EXISTED=true
fi
if [ -e "$MCP_TARGET" ]; then
  cp -p "$MCP_TARGET" "$MCP_BACKUP" || {
    rm -f -- "$CLI_PENDING" "$MCP_PENDING" "$CLI_BACKUP"
    fail "could not preserve the existing zapier-pp-mcp; the existing installation was not changed"
  }
  MCP_EXISTED=true
fi

if ! mv -f "$CLI_PENDING" "$CLI_TARGET"; then
  rm -f -- "$CLI_PENDING" "$MCP_PENDING" "$CLI_BACKUP" "$MCP_BACKUP"
  fail "could not install zapier-pp-cli; the existing installation was not changed"
fi

if ! mv -f "$MCP_PENDING" "$MCP_TARGET"; then
  rm -f -- "$MCP_PENDING"
  if [ "$CLI_EXISTED" = true ]; then
    mv -f "$CLI_BACKUP" "$CLI_TARGET" || true
  else
    rm -f -- "$CLI_TARGET"
  fi
  rm -f -- "$MCP_BACKUP"
  fail "could not install zapier-pp-mcp; the previous installation was restored"
fi

INSTALLED_VERSION=$(read_cli_version "$CLI_TARGET" || true)
if [ "$INSTALLED_VERSION" != "$RELEASE_VERSION" ]; then
  if [ "$CLI_EXISTED" = true ]; then mv -f "$CLI_BACKUP" "$CLI_TARGET" || true; else rm -f -- "$CLI_TARGET"; fi
  if [ "$MCP_EXISTED" = true ]; then mv -f "$MCP_BACKUP" "$MCP_TARGET" || true; else rm -f -- "$MCP_TARGET"; fi
  fail "the installed zapier-pp-cli did not report expected version $RELEASE_VERSION; the previous installation was restored"
fi
rm -f -- "$CLI_BACKUP" "$MCP_BACKUP"

PATH_LINE="export PATH=$(quote_for_shell "$INSTALL_DIR"):\"\$PATH\""
PATH_UPDATED=false
case ":${PATH:-}:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    if [ "$UPDATE_PATH" = true ]; then
      case "${SHELL:-}" in
        */zsh)
          if [ "$PLATFORM" = darwin ]; then PROFILE="${ZDOTDIR:-$HOME}/.zprofile"; else PROFILE="${ZDOTDIR:-$HOME}/.zshrc"; fi
          ;;
        */bash)
          if [ "$PLATFORM" = darwin ]; then PROFILE="$HOME/.bash_profile"; else PROFILE="$HOME/.bashrc"; fi
          ;;
        *) PROFILE="$HOME/.profile" ;;
      esac
      if [ ! -f "$PROFILE" ] || ! grep -F "$PATH_LINE" "$PROFILE" >/dev/null 2>&1; then
        if printf '\n# Added by the Zapier read-only CLI installer\n%s\n' "$PATH_LINE" >>"$PROFILE"; then
          PATH_UPDATED=true
        else
          printf 'Warning: could not update %s. Add the PATH command below manually.\n' "$PROFILE" >&2
        fi
      fi
    fi
    ;;
esac

QUOTED_MCP=$(quote_for_shell "$MCP_TARGET")
printf '\nInstalled Zapier read-only CLI version %s from release %s:\n' "$RELEASE_VERSION" "$TAG"
printf '  %s\n' "$CLI_TARGET" "$MCP_TARGET"
if [ "$PATH_UPDATED" = true ]; then
  printf 'PATH was added to %s. Open a new terminal or run:\n  %s\n' "$PROFILE" "$PATH_LINE"
else
  printf 'If the command is not on PATH, run:\n  %s\n' "$PATH_LINE"
fi
printf '\nRegister the MCP server with your agent (choose one):\n'
printf '  Claude: claude mcp add --scope user zapier -- %s\n' "$QUOTED_MCP"
printf '  Codex:  codex mcp add zapier -- %s\n' "$QUOTED_MCP"
printf '\nThen connect Zapier in a visible browser:\n  zapier-pp-cli auth browser\n'
printf 'After login, the first and only account check must be:\n  zapier-pp-cli session --agent --no-learn\n'
