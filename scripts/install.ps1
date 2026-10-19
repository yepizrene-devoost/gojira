# Install GoJira on Windows without administrator rights.
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$RepositoryUrl = 'https://github.com/yepizrene-devoost/gojira'
$Project = 'gojira'
$Binary = 'gojira.exe'

function Get-BasicParsingSwitch {
    if ($PSVersionTable.PSVersion.Major -ge 6) { return @{} }
    return @{ UseBasicParsing = $true }
}

function Get-RemoteFile {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$Destination
    )
    $basicParsing = Get-BasicParsingSwitch
    Invoke-WebRequest -Uri $Uri -OutFile $Destination -ErrorAction Stop @basicParsing
}

function Resolve-LatestTag {
    $latestUrl = "$RepositoryUrl/releases/latest"
    $location = $null
    $basicParsing = Get-BasicParsingSwitch
    try {
        $response = Invoke-WebRequest -Uri $latestUrl -MaximumRedirection 0 -ErrorAction Stop @basicParsing
        if ($null -ne $response -and $null -ne $response.Headers['Location']) {
            $location = [string]$response.Headers['Location']
        }
    }
    catch {
        try {
            $response = $_.Exception.Response
            if ($null -ne $response -and $null -ne $response.Headers['Location']) {
                $location = [string]$response.Headers['Location']
            }
        }
        catch {}
    }
    if ([string]::IsNullOrWhiteSpace($location)) {
        throw 'Could not resolve the latest release; set GOJIRA_VERSION.'
    }
    return ($location.TrimEnd('/') -split '/')[-1]
}

function Test-ReleaseTag {
    param([Parameter(Mandatory = $true)][string]$Tag)
    return $Tag -cmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'
}

function Get-ExpectedHash {
    param(
        [Parameter(Mandatory = $true)][string]$ChecksumPath,
        [Parameter(Mandatory = $true)][string]$AssetName
    )
    $references = 0
    $hashes = @()
    foreach ($line in Get-Content -LiteralPath $ChecksumPath) {
        $fields = @($line.Trim() -split '\s+' | Where-Object { $_ -ne '' })
        if ($fields.Count -ge 2 -and $fields[1].TrimStart('*') -ceq $AssetName) {
            $references++
            if ($fields.Count -eq 2 -and $fields[0] -cmatch '^[0-9A-Fa-f]{64}$') {
                $hashes += $fields[0].ToLowerInvariant()
            }
        }
    }
    if ($references -ne 1 -or $hashes.Count -ne 1) {
        throw "checksums.txt must contain one valid SHA-256 entry for $AssetName."
    }
    return $hashes[0]
}

function Install-StagedBinary {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Target
    )
    if (Test-Path -LiteralPath $Target) {
        $targetItem = Get-Item -LiteralPath $Target -Force
        if (($targetItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Refusing to replace reparse-point destination: $Target"
        }
        if ($targetItem.PSIsContainer) {
            throw "Destination is a directory: $Target"
        }
    }

    $nonce = [Guid]::NewGuid().ToString('N')
    $staged = "$Target.new.$nonce"
    $backup = "$Target.old.$nonce"
    try {
        Copy-Item -LiteralPath $Source -Destination $staged
        if (Test-Path -LiteralPath $Target -PathType Leaf) {
            [System.IO.File]::Replace($staged, $Target, $backup, $true)
            Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        }
        else {
            Move-Item -LiteralPath $staged -Destination $Target
        }
    }
    finally {
        Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
    }
}

function Add-UserPath {
    param([Parameter(Mandatory = $true)][string]$Directory)
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $userPath) { $userPath = '' }
    $entries = @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    foreach ($entry in $entries) {
        if ($entry.TrimEnd('\') -ieq $Directory.TrimEnd('\')) {
            Write-Host "==> $Directory is already on your user PATH."
            return
        }
    }
    $newPath = if ($entries.Count -eq 0) { $Directory } else { "$userPath;$Directory" }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "==> Added $Directory to your user PATH. Open a new terminal to use it."
}

switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'x86_64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "Unsupported processor architecture '$env:PROCESSOR_ARCHITECTURE'." }
}

$requestedInstallDir = [Environment]::GetEnvironmentVariable('GOJIRA_INSTALL_DIR')
$localAppData = [Environment]::GetEnvironmentVariable('LOCALAPPDATA')
if (-not [string]::IsNullOrWhiteSpace($requestedInstallDir)) {
    $installDir = [System.IO.Path]::GetFullPath($requestedInstallDir)
}
elseif (-not [string]::IsNullOrWhiteSpace($localAppData)) {
    $installDir = Join-Path $localAppData 'Programs\gojira'
}
else {
    throw 'LOCALAPPDATA is unset; set GOJIRA_INSTALL_DIR.'
}

$requestedVersion = [Environment]::GetEnvironmentVariable('GOJIRA_VERSION')
if (-not [string]::IsNullOrWhiteSpace($requestedVersion)) {
    $requested = $requestedVersion.Trim()
    $tag = if ($requested.StartsWith('v')) { $requested } else { "v$requested" }
}
else {
    Write-Host '==> Resolving the latest GoJira release...'
    $tag = Resolve-LatestTag
}
if (-not (Test-ReleaseTag -Tag $tag)) { throw "Invalid release version: $tag" }
$version = $tag.Substring(1)

$assetName = "${Project}_Windows_$arch.zip"
$checksumName = 'checksums.txt'
$baseUrl = "$RepositoryUrl/releases/download/$tag"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gojira-install-" + [Guid]::NewGuid().ToString('N'))
$progressWas = $ProgressPreference

try {
    $ProgressPreference = 'SilentlyContinue'
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    $archivePath = Join-Path $tempDir $assetName
    $checksumPath = Join-Path $tempDir $checksumName
    Get-RemoteFile -Uri "$baseUrl/$assetName" -Destination $archivePath
    Get-RemoteFile -Uri "$baseUrl/$checksumName" -Destination $checksumPath

    $expectedHash = Get-ExpectedHash -ChecksumPath $checksumPath -AssetName $assetName
    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -cne $expectedHash) { throw "Checksum mismatch for $assetName." }
    Write-Host '==> Verified SHA-256 checksum.'

    $extractDir = Join-Path $tempDir 'extract'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir
    $expectedBinary = Join-Path $extractDir $Binary
    $binaryMatches = @(Get-ChildItem -LiteralPath $extractDir -Recurse -File | Where-Object { $_.Name -ieq $Binary })
    if ($binaryMatches.Count -ne 1 -or $binaryMatches[0].FullName -cne $expectedBinary) {
        throw "Archive must contain exactly one root $Binary binary."
    }
    if (Test-Path -LiteralPath (Join-Path $extractDir 'gojira') -PathType Leaf) {
        throw 'Archive contained an unexpected non-Windows binary.'
    }

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $installDirItem = Get-Item -LiteralPath $installDir -Force
    if (-not $installDirItem.PSIsContainer -or
        ($installDirItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Install destination is not a safe directory: $installDir"
    }
    $targetBinary = Join-Path $installDir $Binary
    Install-StagedBinary -Source $expectedBinary -Target $targetBinary
    Write-Host "==> Installed GoJira $version to $targetBinary"
}
finally {
    $ProgressPreference = $progressWas
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

if ([Environment]::GetEnvironmentVariable('GOJIRA_SKIP_PATH_UPDATE') -eq '1') {
    Write-Host "==> PATH update skipped; add $installDir to your user PATH."
}
else {
    Add-UserPath -Directory $installDir
}
Write-Host "Run 'gojira --help' to get started."
