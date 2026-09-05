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

RELEASE_DIR="$TEST_ROOT/release"
mkdir -p "$RELEASE_DIR/payload"

PS_WRAPPER="$TEST_ROOT/mock-install.ps1"
cat >"$PS_WRAPPER" <<'EOF'
param(
    [Parameter(Mandatory = $true)][string]$Installer,
    [string]$Tag,
    [string]$InstallDir,
    [switch]$VerifyOnly,
    [switch]$NoPathUpdate
)

function Invoke-WebRequest {
    param(
        [Parameter(Mandatory = $true)]$Uri,
        [Parameter(Mandatory = $true)][string]$OutFile,
        $Headers,
        [switch]$UseBasicParsing
    )
    if ($env:MOCK_IWR_FAIL -eq 'true') { throw 'anonymous download denied' }
    $Name = [IO.Path]::GetFileName(([Uri]$Uri).AbsolutePath)
    Copy-Item -LiteralPath (Join-Path $env:MOCK_RELEASE_DIR $Name) -Destination $OutFile
}

function Invoke-RestMethod {
    param(
        [Parameter(Mandatory = $true)]$Uri,
        $Headers
    )
    if ($env:MOCK_IRM_FAIL -eq 'true') { throw 'anonymous metadata denied' }
    # Windows PowerShell 5.1 returns a JSON array as one Object[] result from
    # Invoke-RestMethod. Preserve that shape instead of streaming its members.
    Write-Output -NoEnumerate ([object[]]@(
        [pscustomobject]@{ draft = $true; tag_name = 'v999-draft' }
        [pscustomobject]@{ draft = $false; tag_name = 'v1-prerelease' }
    ))
}

function gh {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    if ($Arguments[0] -eq 'auth') {
        $global:LASTEXITCODE = if ($env:MOCK_GH_AUTH -eq 'true') { 0 } else { 1 }
        return
    }
    if ($env:MOCK_GH_AUTH -ne 'true') {
        $global:LASTEXITCODE = 1
        return
    }
    if ($Arguments[0] -eq 'api') {
        Write-Output '[{"draft":true,"tag_name":"v999-draft"},{"draft":false,"tag_name":"v1-prerelease"}]'
        $global:LASTEXITCODE = 0
        return
    }
    if ($Arguments[0] -eq 'release' -and $Arguments[1] -eq 'download') {
        $directory = $Arguments[($Arguments.IndexOf('--dir') + 1)]
        foreach ($patternIndex in 0..($Arguments.Count - 1)) {
            if ($Arguments[$patternIndex] -eq '--pattern') {
                $name = $Arguments[$patternIndex + 1]
                Copy-Item -LiteralPath (Join-Path $env:MOCK_RELEASE_DIR $name) -Destination (Join-Path $directory $name)
            }
        }
        $global:LASTEXITCODE = 0
        return
    }
    $global:LASTEXITCODE = 1
}

$InstallerArgs = @{}
if (-not [string]::IsNullOrWhiteSpace($Tag)) { $InstallerArgs.Tag = $Tag }
if ($VerifyOnly) { $InstallerArgs.VerifyOnly = $true }
if ($NoPathUpdate) { $InstallerArgs.NoPathUpdate = $true }
if (-not [string]::IsNullOrWhiteSpace($InstallDir)) { $InstallerArgs.InstallDir = $InstallDir }
. $Installer @InstallerArgs
EOF

printf '#!/bin/sh\nprintf "zapier-pp-cli v1-prerelease\\n"\n' >"$RELEASE_DIR/payload/zapier-pp-cli.exe"
printf 'new windows mcp\n' >"$RELEASE_DIR/payload/zapier-pp-mcp.exe"
chmod +x "$RELEASE_DIR/payload/zapier-pp-cli.exe"
(cd "$RELEASE_DIR/payload" && zip -q "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" zapier-pp-cli.exe zapier-pp-mcp.exe)
if command -v shasum >/dev/null 2>&1; then
  RELEASE_HASH=$(shasum -a 256 "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
else
  RELEASE_HASH=$(sha256sum "$RELEASE_DIR/zapier-cli_windows_x86_64.zip" | awk '{print $1}')
fi
printf '%s  %s\n' "$RELEASE_HASH" zapier-cli_windows_x86_64.zip >"$RELEASE_DIR/SHA256SUMS"

PS_ENV="OS=Windows_NT"

public_verify_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -VerifyOnly 2>&1)
assert_contains "$public_verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

private_verify_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_IRM_FAIL=true MOCK_IWR_FAIL=true MOCK_GH_AUTH=true MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -VerifyOnly 2>&1)
assert_contains "$private_verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

cp "$RELEASE_DIR/SHA256SUMS" "$RELEASE_DIR/SHA256SUMS.good"
printf '%064d  %s\n' 0 zapier-cli_windows_x86_64.zip >"$RELEASE_DIR/SHA256SUMS"
checksum_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v1-prerelease -VerifyOnly 2>&1) && fail "bad checksum unexpectedly succeeded"
assert_contains "$checksum_output" "checksum verification failed"
FALLBACK_INSTALL_DIR="$TEST_ROOT/fallback install"
mkdir -p "$FALLBACK_INSTALL_DIR"
printf 'old windows cli\n' >"$FALLBACK_INSTALL_DIR/zapier-pp-cli.exe"
fallback_checksum_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_IWR_FAIL=true MOCK_GH_AUTH=true MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v1-prerelease -InstallDir "$FALLBACK_INSTALL_DIR" -NoPathUpdate 2>&1) && fail "bad fallback checksum unexpectedly succeeded"
assert_contains "$fallback_checksum_output" "checksum verification failed"
[ "$(cat "$FALLBACK_INSTALL_DIR/zapier-pp-cli.exe")" = "old windows cli" ] || fail "bad fallback checksum changed the existing installation"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

verify_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v1-prerelease -VerifyOnly 2>&1)
assert_contains "$verify_output" "Verified release v1-prerelease (CLI version v1-prerelease); no files were installed."

version_mismatch_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v2-other -VerifyOnly 2>&1) && fail "mismatched release version unexpectedly succeeded"
assert_contains "$version_mismatch_output" "reports version"
assert_contains "$version_mismatch_output" "release tag is v2-other"

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
dev_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v3-dev -VerifyOnly 2>&1) && fail "development binary unexpectedly passed verification"
assert_contains "$dev_output" "contains development"
assert_contains "$dev_output" "version 0.0.0-dev"
mv "$RELEASE_DIR/zapier-cli_windows_x86_64.zip.good" "$RELEASE_DIR/zapier-cli_windows_x86_64.zip"
mv "$RELEASE_DIR/SHA256SUMS.good" "$RELEASE_DIR/SHA256SUMS"

INSTALL_DIR="$TEST_ROOT/install target"
mkdir -p "$INSTALL_DIR"
printf 'old windows cli\n' >"$INSTALL_DIR/zapier-pp-cli.exe"
printf 'old windows mcp\n' >"$INSTALL_DIR/zapier-pp-mcp.exe"
install_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=AMD64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v1-prerelease -InstallDir "$INSTALL_DIR" -NoPathUpdate 2>&1)

[ "$("$INSTALL_DIR/zapier-pp-cli.exe" version)" = "zapier-pp-cli v1-prerelease" ] || fail "zapier-pp-cli.exe was not replaced with verified payload"
[ "$(cat "$INSTALL_DIR/zapier-pp-mcp.exe")" = "new windows mcp" ] || fail "zapier-pp-mcp.exe was not replaced with verified payload"
assert_contains "$install_output" "claude mcp add --scope user zapier --"
assert_contains "$install_output" "codex mcp add zapier --"
assert_contains "$install_output" "zapier-pp-cli session --agent --no-learn"
assert_contains "$install_output" "CLI version v1-prerelease"

unsupported_output=$(env "$PS_ENV" PROCESSOR_ARCHITECTURE=ARM64 MOCK_RELEASE_DIR="$RELEASE_DIR" \
  pwsh -NoLogo -NoProfile -File "$PS_WRAPPER" -Installer "$REPO_ROOT/install.ps1" -Tag v1-prerelease -VerifyOnly 2>&1) && fail "unsupported architecture unexpectedly succeeded"
assert_contains "$unsupported_output" "Windows architecture: ARM64"

printf 'PASS: install.ps1 public download, architecture, checksum, verification, replacement, and guidance tests\n'
