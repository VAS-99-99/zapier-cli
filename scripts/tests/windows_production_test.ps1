[CmdletBinding()]
param(
    [Parameter()]
    [string]$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Test {
    param([Parameter(Mandatory = $true)][string]$Message)
    throw "windows production test failed: $Message"
}

function Assert-Contains {
    param(
        [Parameter(Mandatory = $true)][string]$Text,
        [Parameter(Mandatory = $true)][string]$Expected
    )
    if (-not $Text.Contains($Expected)) {
        Stop-Test "expected output to contain $Expected, got: $Text"
    }
}

if ($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5) {
    Stop-Test 'this test must run under native Windows PowerShell 5.1'
}

$installer = Join-Path $RepositoryRoot 'install.ps1'
if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
    Stop-Test "install.ps1 was not found under $RepositoryRoot"
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("zapier-windows-production-{0}" -f [Guid]::NewGuid().ToString('N'))
$releaseDir = Join-Path $testRoot 'release'
$payloadDir = Join-Path $releaseDir 'payload'
$installDir = Join-Path $testRoot 'install'
$releaseTag = 'v9-windows-test'
$originalPath = $env:Path

try {
    New-Item -ItemType Directory -Path $payloadDir -Force | Out-Null
    Push-Location $RepositoryRoot
    try {
        $modulePath = (& go list -m).Trim()
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($modulePath)) {
            Stop-Test 'could not resolve the Go module path'
        }
        $cliFlags = "-X $modulePath/internal/cli.version=$releaseTag"
        $mcpFlags = "$cliFlags -X main.version=$releaseTag"
        & go build -mod=readonly -trimpath -buildvcs=false -ldflags $cliFlags -o (Join-Path $payloadDir 'zapier-pp-cli.exe') ./cmd/zapier-pp-cli
        if ($LASTEXITCODE -ne 0) { Stop-Test 'could not build zapier-pp-cli.exe' }
        & go build -mod=readonly -trimpath -buildvcs=false -ldflags $mcpFlags -o (Join-Path $payloadDir 'zapier-pp-mcp.exe') ./cmd/zapier-pp-mcp
        if ($LASTEXITCODE -ne 0) { Stop-Test 'could not build zapier-pp-mcp.exe' }
    } finally {
        Pop-Location
    }

    $archive = Join-Path $releaseDir 'zapier-cli_windows_x86_64.zip'
    Compress-Archive -Path (Join-Path $payloadDir '*') -DestinationPath $archive -Force
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $releaseDir 'SHA256SUMS') -Value "$hash  zapier-cli_windows_x86_64.zip" -NoNewline

    function Invoke-WebRequest {
        param(
            [Parameter(Mandatory = $true)]$Uri,
            [Parameter(Mandatory = $true)][string]$OutFile,
            $Headers,
            [switch]$UseBasicParsing
        )
        $name = [IO.Path]::GetFileName(([Uri]$Uri).AbsolutePath)
        Copy-Item -LiteralPath (Join-Path $releaseDir $name) -Destination $OutFile
    }

    function Invoke-RestMethod {
        param(
            [Parameter(Mandatory = $true)]$Uri,
            $Headers
        )
        # Match Windows PowerShell 5.1's root-array response shape.
        Write-Output -NoEnumerate ([object[]]@(
            [pscustomobject]@{ draft = $true; tag_name = 'v999-draft' }
            [pscustomobject]@{ draft = $false; tag_name = $releaseTag }
        ))
    }

    $noTagOutput = & {
        . $installer -VerifyOnly
    } *>&1 | Out-String
    Assert-Contains $noTagOutput "Verified release $releaseTag (CLI version $releaseTag); no files were installed."

    $explicitTagOutput = & {
        . $installer -Tag $releaseTag -VerifyOnly
    } *>&1 | Out-String
    Assert-Contains $explicitTagOutput "Verified release $releaseTag (CLI version $releaseTag); no files were installed."

    Set-Content -LiteralPath (Join-Path $releaseDir 'SHA256SUMS') -Value (('0' * 64) + '  zapier-cli_windows_x86_64.zip') -NoNewline
    try {
        & { . $installer -Tag $releaseTag -VerifyOnly } *> $null
        Stop-Test 'bad checksum unexpectedly verified'
    } catch {
        Assert-Contains $_.Exception.Message 'checksum verification failed'
    }
    Set-Content -LiteralPath (Join-Path $releaseDir 'SHA256SUMS') -Value "$hash  zapier-cli_windows_x86_64.zip" -NoNewline

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $installDir 'zapier-pp-cli.exe') -Value 'old cli'
    Set-Content -LiteralPath (Join-Path $installDir 'zapier-pp-mcp.exe') -Value 'old mcp'
    & { . $installer -Tag $releaseTag -InstallDir $installDir -NoPathUpdate } *> $null
    if ((& (Join-Path $installDir 'zapier-pp-cli.exe') version) -ne "zapier-pp-cli $releaseTag") {
        Stop-Test 'installer did not replace the existing CLI executable'
    }
    if ((Get-Item -LiteralPath (Join-Path $installDir 'zapier-pp-mcp.exe')).Length -eq 0) {
        Stop-Test 'installer did not replace the existing MCP executable'
    }
    $resolvedCli = Get-Command zapier-pp-cli -CommandType Application -ErrorAction Stop
    if ($resolvedCli.Path -ne (Join-Path $installDir 'zapier-pp-cli.exe')) {
        Stop-Test "PATH resolved zapier-pp-cli to $($resolvedCli.Path), want $installDir"
    }
    if ((& zapier-pp-cli version) -ne "zapier-pp-cli $releaseTag") {
        Stop-Test 'PATH-resolved zapier-pp-cli did not run the installed executable'
    }

    # Fixture host commands keep this native Windows PowerShell 5.1 test
    # isolated from actual Claude/Codex configuration.
    $pluginLog = Join-Path $testRoot 'plugin-host.log'
    function claude {
        param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
        Add-Content -LiteralPath $pluginLog -Value ('claude ' + ($Arguments -join ' '))
        $global:LASTEXITCODE = 0
    }
    function codex {
        param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
        Add-Content -LiteralPath $pluginLog -Value ('codex ' + ($Arguments -join ' '))
        $global:LASTEXITCODE = 0
    }
    & { . $installer -Tag $releaseTag -InstallDir $installDir -NoPathUpdate -Agent Claude } *> $null
    & { . $installer -Tag $releaseTag -InstallDir $installDir -NoPathUpdate -Agent Codex } *> $null
    $pluginCommands = Get-Content -LiteralPath $pluginLog -Raw
    Assert-Contains $pluginCommands 'claude plugin marketplace add VAS-99-99/zapier-cli --scope user'
    Assert-Contains $pluginCommands 'claude plugin install zapier-read-only@vas-zapier-cli --scope user'
    Assert-Contains $pluginCommands 'codex plugin marketplace add VAS-99-99/zapier-cli'
    Assert-Contains $pluginCommands 'codex plugin add zapier-read-only@vas-zapier-cli'

    $aclTests = @(
        'TestWindowsCredentialTrusteeAliasRequiresExactSID',
        'TestAtomicWritePrivateFileRefusesUnsafeEmptyTempBeforeWriting',
        'TestLoadCredentials_RefusesOverPermissiveFileOnRead',
        'TestSaveCredentialsVerifiesWrittenFile'
    )
    Push-Location $RepositoryRoot
    try {
        $goTestOutput = @(& go test -json ./internal/cliutil -run ('^(' + ($aclTests -join '|') + ')$') 2>&1)
        if ($LASTEXITCODE -ne 0) {
            Stop-Test "credential ACL tests failed:`n$($goTestOutput -join [Environment]::NewLine)"
        }
    } finally {
        Pop-Location
    }
    $events = @(
        foreach ($line in $goTestOutput) {
            $json = [string]$line
            if (-not $json.TrimStart().StartsWith('{')) {
                continue
            }
            try {
                ConvertFrom-Json -InputObject $json
            } catch {
                # Go may write non-JSON warnings through the merged stream.
            }
        }
    )
    foreach ($testName in $aclTests) {
        $testEvents = @($events | Where-Object {
            $testProperty = $_.PSObject.Properties['Test']
            $testProperty -and $testProperty.Value -eq $testName
        })
        $actions = @($testEvents | ForEach-Object {
            $actionProperty = $_.PSObject.Properties['Action']
            if ($actionProperty) { $actionProperty.Value }
        })
        if ($actions -contains 'skip') {
            Stop-Test "credential ACL test was skipped: $testName"
        }
        if (-not ($actions -contains 'pass')) {
            Stop-Test "credential ACL test did not pass: $testName"
        }
    }

    Push-Location $RepositoryRoot
    try {
        & go test ./internal/config -run '^TestSaveCredentialRollsBackWhenConfigWriteFails$'
        if ($LASTEXITCODE -ne 0) {
            Stop-Test 'credential rollback tests failed'
        }
        & go test ./internal/cli -run '^(TestAuthBrowser|TestAgentBrowser|TestBrowser|TestAuthLogout)'
        if ($LASTEXITCODE -ne 0) {
            Stop-Test 'fixture-based browser authentication and logout tests failed'
        }
    } finally {
        Pop-Location
    }

    Push-Location $RepositoryRoot
    try {
        & go test ./cmd/zapier-pp-mcp -run '^TestMCPProductionStdioDiscovery$'
        if ($LASTEXITCODE -ne 0) {
            Stop-Test 'MCP production stdio discovery test failed'
        }
    } finally {
        Pop-Location
    }

    Write-Host 'PASS: Windows PowerShell 5.1 installer, opt-in plugin fixtures, credential ACL, browser authentication, logout, and MCP stdio tests'
} finally {
    $env:Path = $originalPath
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}
