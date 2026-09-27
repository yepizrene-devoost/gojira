$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$installer = Join-Path $root 'scripts/install.ps1'
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("gojira-installer-test-" + [Guid]::NewGuid().ToString('N'))
$fixture = Join-Path $temp 'fixture'
$install = Join-Path $temp 'install'
$assetName = 'gojira_Windows_x86_64.zip'
$global:GojiraInstallerFixtureUseBasicParsingCalls = @()

function Invoke-WebRequest {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [string]$OutFile,
        [int]$MaximumRedirection,
        [switch]$UseBasicParsing
    )
    $global:GojiraInstallerFixtureUseBasicParsingCalls += $UseBasicParsing.IsPresent
    if ([string]::IsNullOrWhiteSpace($OutFile)) {
        throw 'The fixture only permits pinned-version file downloads.'
    }
    $name = ($Uri -split '/')[-1]
    $source = Join-Path $script:fixture $name
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "Unexpected fixture download: $Uri"
    }
    Copy-Item -LiteralPath $source -Destination $OutFile
}

function New-FixtureArchive {
    param([Parameter(Mandatory = $true)][string]$Content)
    $payload = Join-Path $temp 'payload'
    Remove-Item -LiteralPath $payload -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $payload | Out-Null
    Set-Content -LiteralPath (Join-Path $payload 'gojira.exe') -Value $Content -NoNewline
    $archive = Join-Path $fixture $assetName
    Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
    Compress-Archive -LiteralPath (Join-Path $payload 'gojira.exe') -DestinationPath $archive
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $fixture 'checksums.txt') -Value "$hash  $assetName"
}

function Invoke-InstallerExpectFailure {
    param([Parameter(Mandatory = $true)][string]$Reason)
    try {
        & $installer *> $null
        throw "Installer unexpectedly accepted $Reason."
    }
    catch {
        if ($_.Exception.Message -eq "Installer unexpectedly accepted $Reason.") { throw }
    }
}

try {
    New-Item -ItemType Directory -Path $fixture, $install -Force | Out-Null
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    $env:LOCALAPPDATA = Join-Path $temp 'local-app-data'
    $env:GOJIRA_INSTALL_DIR = $install
    $env:GOJIRA_VERSION = 'v1.2.3'
    $env:GOJIRA_SKIP_PATH_UPDATE = '1'

    New-FixtureArchive -Content 'new binary'
    Set-Content -LiteralPath (Join-Path $install 'gojira.exe') -Value 'old binary' -NoNewline
    & $installer *> $null
    if ((Get-Content -LiteralPath (Join-Path $install 'gojira.exe') -Raw) -ne 'new binary') {
        throw 'Installed binary did not safely replace the old binary.'
    }
    if ($global:GojiraInstallerFixtureUseBasicParsingCalls.Count -ne 2) {
        throw "Expected two fixture downloads, observed $($global:GojiraInstallerFixtureUseBasicParsingCalls.Count)."
    }
    $expectBasicParsing = $PSVersionTable.PSVersion.Major -lt 6
    $unexpectedBasicParsing = @($global:GojiraInstallerFixtureUseBasicParsingCalls | Where-Object { $_ -ne $expectBasicParsing })
    if ($unexpectedBasicParsing.Count -ne 0) {
        throw "Invoke-WebRequest UseBasicParsing did not match PowerShell $($PSVersionTable.PSVersion.Major) requirements."
    }

    Set-Content -LiteralPath (Join-Path $install 'gojira.exe') -Value 'old binary' -NoNewline
    $checksumPath = Join-Path $fixture 'checksums.txt'
    $line = Get-Content -LiteralPath $checksumPath -Raw
    Set-Content -LiteralPath $checksumPath -Value "$line`n$line"
    Invoke-InstallerExpectFailure -Reason 'duplicate checksum entries'
    if ((Get-Content -LiteralPath (Join-Path $install 'gojira.exe') -Raw) -ne 'old binary') {
        throw 'Failed checksum validation changed the installed binary.'
    }

    Set-Content -LiteralPath $checksumPath -Value "$line`nnot-a-hash  $assetName  unexpected-field"
    Invoke-InstallerExpectFailure -Reason 'a malformed duplicate checksum reference'
    if ((Get-Content -LiteralPath (Join-Path $install 'gojira.exe') -Raw) -ne 'old binary') {
        throw 'Malformed checksum manifest changed the installed binary.'
    }

    Set-Content -LiteralPath $checksumPath -Value "$('0' * 64)  $assetName"
    Invoke-InstallerExpectFailure -Reason 'a checksum mismatch'
    if ((Get-Content -LiteralPath (Join-Path $install 'gojira.exe') -Raw) -ne 'old binary') {
        throw 'Checksum mismatch changed the installed binary.'
    }

    $payload = Join-Path $temp 'missing-payload'
    New-Item -ItemType Directory -Path $payload -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $payload 'README.txt') -Value 'not a binary'
    $archive = Join-Path $fixture $assetName
    Remove-Item -LiteralPath $archive -Force
    Compress-Archive -LiteralPath (Join-Path $payload 'README.txt') -DestinationPath $archive
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath $checksumPath -Value "$hash  $assetName"
    Invoke-InstallerExpectFailure -Reason 'an archive without gojira.exe'

    New-FixtureArchive -Content 'new binary'
    $target = Join-Path $install 'gojira.exe'
    Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $target | Out-Null
    Invoke-InstallerExpectFailure -Reason 'a destination directory'
    if (@(Get-ChildItem -LiteralPath $target -Force).Count -ne 0) {
        throw 'Installer staged content inside the destination directory.'
    }
    Remove-Item -LiteralPath $target -Recurse -Force

    $env:GOJIRA_VERSION = '../bad'
    Invoke-InstallerExpectFailure -Reason 'a malformed version'

    $config = Get-Content -LiteralPath (Join-Path $root '.goreleaser.yaml') -Raw
    if ($config -notmatch [regex]::Escape('scripts/install.ps1')) {
        throw 'GoReleaser does not publish scripts/install.ps1.'
    }

    Write-Host 'PowerShell installer contract passed'
}
finally {
    Remove-Variable -Name GojiraInstallerFixtureUseBasicParsingCalls -Scope Global -ErrorAction SilentlyContinue
    Remove-Item Env:GOJIRA_INSTALL_DIR -ErrorAction SilentlyContinue
    Remove-Item Env:GOJIRA_VERSION -ErrorAction SilentlyContinue
    Remove-Item Env:GOJIRA_SKIP_PATH_UPDATE -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
