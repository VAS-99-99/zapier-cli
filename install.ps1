[CmdletBinding()]
param(
    [Parameter()]
    [string]$Tag,

    [Parameter()]
    [string]$InstallDir = (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Microsoft\WindowsApps'),

    [Parameter()]
    [switch]$VerifyOnly,

    [Parameter()]
    [switch]$NoPathUpdate
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repository = 'VAS-99-99/zapier-cli'
$AssetName = 'zapier-cli_windows_x86_64.zip'
$TempRoot = $null

function Stop-Install {
    param([Parameter(Mandatory = $true)][string]$Message)
    throw "Install failed: $Message"
}

function Test-GitHubCLIAuthentication {
    $GitHubCLI = Get-Command gh -ErrorAction SilentlyContinue
    if (-not $GitHubCLI) {
        return $false
    }
    & gh auth status --hostname github.com *> $null
    return $LASTEXITCODE -eq 0
}

function Get-GitHubCLIFallbackHint {
    return 'Run "gh auth login" and retry, or download the release archive and SHA256SUMS manually from GitHub.'
}

$RunningOnWindows = $env:OS -eq 'Windows_NT'
if (-not $RunningOnWindows -and (Get-Variable -Name IsWindows -ErrorAction SilentlyContinue)) {
    $RunningOnWindows = $IsWindows
}
if (-not $RunningOnWindows) {
    Stop-Install 'install.ps1 supports Windows x64 only. Use install.sh on macOS or Linux.'
}

$RawArchitecture = if ($env:PROCESSOR_ARCHITEW6432) {
    $env:PROCESSOR_ARCHITEW6432
} else {
    $env:PROCESSOR_ARCHITECTURE
}
if ($RawArchitecture -notin @('AMD64', 'x86_64')) {
    Stop-Install "unsupported Windows architecture: $RawArchitecture. This release supports Windows x64 only."
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    Stop-Install 'the install directory cannot be empty'
}

if ([string]::IsNullOrWhiteSpace($Tag)) {
    try {
        $Headers = @{ Accept = 'application/vnd.github+json'; 'User-Agent' = 'zapier-read-only-cli-installer' }
        # Windows PowerShell 5.1 can return GitHub's JSON root array as one
        # Object[] pipeline result. Enumerate the response explicitly so the
        # draft filter always receives release objects.
        $ReleaseResponse = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases?per_page=20" -Headers $Headers
        $Releases = @(
            foreach ($Release in $ReleaseResponse) {
                $Release
            }
        )
        $PublishedRelease = $Releases | Where-Object { -not $_.draft } | Select-Object -First 1
    } catch {
        if (-not (Test-GitHubCLIAuthentication)) {
            Stop-Install "could not resolve the newest release in $Repository anonymously. $(Get-GitHubCLIFallbackHint)"
        }
        try {
            $GitHubCLIResponse = @(& gh api "repos/$Repository/releases?per_page=20" 2>$null)
            if ($LASTEXITCODE -ne 0) {
                throw 'GitHub CLI could not read releases'
            }
            $ReleaseResponse = ($GitHubCLIResponse -join "`n") | ConvertFrom-Json
            $Releases = @(
                foreach ($Release in $ReleaseResponse) {
                    $Release
                }
            )
            $PublishedRelease = $Releases | Where-Object { -not $_.draft } | Select-Object -First 1
        } catch {
            Stop-Install "could not resolve the newest release in $Repository with GitHub CLI. $(Get-GitHubCLIFallbackHint)"
        }
    }
    if (-not $PublishedRelease -or [string]::IsNullOrWhiteSpace($PublishedRelease.tag_name)) {
        Stop-Install "the repository $Repository does not have a published release"
    }
    $Tag = $PublishedRelease.tag_name.Trim()
}

if ($Tag -notmatch '^v[0-9A-Za-z][0-9A-Za-z._-]*$') {
    Stop-Install "invalid release tag: $Tag"
}

$TempRoot = Join-Path ([IO.Path]::GetTempPath()) ("zapier-cli-install-{0}" -f [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $TempRoot | Out-Null

try {
    Write-Host "Downloading $AssetName ($Tag)..."
    $ReleaseRoot = "https://github.com/$Repository/releases/download/$Tag"
    $DownloadHeaders = @{ 'User-Agent' = 'zapier-read-only-cli-installer' }
    $SavedProgressPreference = $ProgressPreference
    try {
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseRoot/$AssetName" -Headers $DownloadHeaders -OutFile (Join-Path $TempRoot $AssetName)
        Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseRoot/SHA256SUMS" -Headers $DownloadHeaders -OutFile (Join-Path $TempRoot 'SHA256SUMS')
    } catch {
        if (-not (Test-GitHubCLIAuthentication)) {
            Stop-Install "could not download release $Tag from $Repository anonymously. $(Get-GitHubCLIFallbackHint)"
        }
        & gh release download $Tag --repo $Repository --pattern $AssetName --pattern SHA256SUMS --dir $TempRoot --clobber *> $null
        if ($LASTEXITCODE -ne 0) {
            Stop-Install "could not download release $Tag from $Repository with GitHub CLI. $(Get-GitHubCLIFallbackHint)"
        }
    } finally {
        $ProgressPreference = $SavedProgressPreference
    }

    $ArchivePath = Join-Path $TempRoot $AssetName
    $SumsPath = Join-Path $TempRoot 'SHA256SUMS'
    if (-not (Test-Path -LiteralPath $ArchivePath -PathType Leaf)) {
        Stop-Install "release $Tag does not contain $AssetName"
    }
    if (-not (Test-Path -LiteralPath $SumsPath -PathType Leaf)) {
        Stop-Install "release $Tag does not contain SHA256SUMS"
    }

    $ExpectedHash = $null
    foreach ($Line in Get-Content -LiteralPath $SumsPath) {
        if ($Line -match '^(?<Hash>[A-Fa-f0-9]{64})\s+\*?(?<File>.+)$' -and $Matches.File -eq $AssetName) {
            $ExpectedHash = $Matches.Hash.ToLowerInvariant()
            break
        }
    }
    if (-not $ExpectedHash) {
        Stop-Install "SHA256SUMS does not contain a valid checksum for $AssetName"
    }

    $ActualHash = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        Stop-Install "checksum verification failed for $AssetName; nothing was installed"
    }
    Write-Host "Checksum verified for $AssetName."

    $ExtractDir = Join-Path $TempRoot 'extracted'
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractDir
    $CliSource = Join-Path $ExtractDir 'zapier-pp-cli.exe'
    $McpSource = Join-Path $ExtractDir 'zapier-pp-mcp.exe'
    foreach ($Source in @($CliSource, $McpSource)) {
        if (-not (Test-Path -LiteralPath $Source -PathType Leaf) -or (Get-Item -LiteralPath $Source).Length -eq 0) {
            Stop-Install "$AssetName is missing a non-empty $(Split-Path -Leaf $Source); nothing was installed"
        }
    }

    $VersionOutput = & $CliSource version 2>&1
    if ($LASTEXITCODE -ne 0) {
        Stop-Install "$AssetName contains a zapier-pp-cli.exe that cannot report its release version; nothing was installed"
    }
    $VersionLine = @($VersionOutput | ForEach-Object { $_.ToString().Trim() } | Where-Object { $_ -match '^zapier-pp-cli\s+\S+$' } | Select-Object -Last 1)
    if ($VersionLine.Count -ne 1) {
        Stop-Install "$AssetName contains a zapier-pp-cli.exe that cannot report its release version; nothing was installed"
    }
    $ReleaseVersion = ($VersionLine[0] -split '\s+', 2)[1]
    if ($ReleaseVersion -like '0.0.0-dev*') {
        Stop-Install "$AssetName contains development version $ReleaseVersion; nothing was installed. Use a properly stamped GitHub Release."
    }
    if ($ReleaseVersion -ne $Tag) {
        Stop-Install "$AssetName reports version $ReleaseVersion but release tag is $Tag; nothing was installed"
    }

    if ($VerifyOnly) {
        Write-Host "Verified release $Tag (CLI version $ReleaseVersion); no files were installed."
        return
    }

    $ResolvedInstallDir = [IO.Path]::GetFullPath($InstallDir)
    New-Item -ItemType Directory -Path $ResolvedInstallDir -Force | Out-Null
    $CliTarget = Join-Path $ResolvedInstallDir 'zapier-pp-cli.exe'
    $McpTarget = Join-Path $ResolvedInstallDir 'zapier-pp-mcp.exe'
    $TransactionId = [Guid]::NewGuid().ToString('N')
    $CliPending = Join-Path $ResolvedInstallDir ".zapier-pp-cli.install.$TransactionId"
    $McpPending = Join-Path $ResolvedInstallDir ".zapier-pp-mcp.install.$TransactionId"
    $CliBackup = Join-Path $ResolvedInstallDir ".zapier-pp-cli.backup.$TransactionId"
    $McpBackup = Join-Path $ResolvedInstallDir ".zapier-pp-mcp.backup.$TransactionId"
    $CliExisted = Test-Path -LiteralPath $CliTarget -PathType Leaf
    $McpExisted = Test-Path -LiteralPath $McpTarget -PathType Leaf
    $CliInstalled = $false
    $McpInstalled = $false

    try {
        Copy-Item -LiteralPath $CliSource -Destination $CliPending
        Copy-Item -LiteralPath $McpSource -Destination $McpPending
        if ($CliExisted) { Copy-Item -LiteralPath $CliTarget -Destination $CliBackup }
        if ($McpExisted) { Copy-Item -LiteralPath $McpTarget -Destination $McpBackup }

        Move-Item -LiteralPath $CliPending -Destination $CliTarget -Force
        $CliInstalled = $true
        Move-Item -LiteralPath $McpPending -Destination $McpTarget -Force
        $McpInstalled = $true

        $InstalledVersionOutput = & $CliTarget version 2>&1
        $InstalledVersionLine = @($InstalledVersionOutput | ForEach-Object { $_.ToString().Trim() } | Where-Object { $_ -match '^zapier-pp-cli\s+\S+$' } | Select-Object -Last 1)
        if ($LASTEXITCODE -ne 0 -or $InstalledVersionLine.Count -ne 1 -or (($InstalledVersionLine[0] -split '\s+', 2)[1] -ne $ReleaseVersion)) {
            throw "installed zapier-pp-cli.exe did not report expected version $ReleaseVersion"
        }
    } catch {
        if ($CliInstalled) {
            if ($CliExisted) {
                Move-Item -LiteralPath $CliBackup -Destination $CliTarget -Force
            } else {
                Remove-Item -LiteralPath $CliTarget -Force -ErrorAction SilentlyContinue
            }
        }
        if ($McpInstalled) {
            if ($McpExisted) {
                Move-Item -LiteralPath $McpBackup -Destination $McpTarget -Force
            } else {
                Remove-Item -LiteralPath $McpTarget -Force -ErrorAction SilentlyContinue
            }
        }
        Stop-Install "could not replace both executables; the previous installation was restored. $($_.Exception.Message)"
    } finally {
        foreach ($TransactionFile in @($CliPending, $McpPending, $CliBackup, $McpBackup)) {
            Remove-Item -LiteralPath $TransactionFile -Force -ErrorAction SilentlyContinue
        }
    }

    $PathChanged = $false
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $UserPathParts = @($UserPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $AlreadyOnUserPath = $UserPathParts | Where-Object { $_.TrimEnd('\') -ieq $ResolvedInstallDir.TrimEnd('\') }
    if (-not $AlreadyOnUserPath -and -not $NoPathUpdate) {
        $NewUserPath = (@($ResolvedInstallDir) + $UserPathParts) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $NewUserPath, 'User')
        $PathChanged = $true
    }
    if (($env:Path -split ';') -notcontains $ResolvedInstallDir) {
        $env:Path = "$ResolvedInstallDir;$env:Path"
    }

    Write-Host ''
    Write-Host "Installed Zapier read-only CLI version $ReleaseVersion from release $Tag`:"
    Write-Host "  $CliTarget"
    Write-Host "  $McpTarget"
    Write-Host 'The command is available as:'
    Write-Host '  zapier-pp-cli'
    Write-Host ''
    Write-Host 'Register the MCP server with your agent (choose one):'
    Write-Host "  Claude: claude mcp add --scope user zapier -- `"$McpTarget`""
    Write-Host "  Codex:  codex mcp add zapier -- `"$McpTarget`""
    Write-Host ''
    Write-Host 'Then connect Zapier in a visible browser:'
    Write-Host '  zapier-pp-cli auth browser'
    Write-Host 'After login, the first and only account check must be:'
    Write-Host '  zapier-pp-cli session --agent --no-learn'
} finally {
    if ($TempRoot -and (Test-Path -LiteralPath $TempRoot)) {
        Remove-Item -LiteralPath $TempRoot -Recurse -Force
    }
}
