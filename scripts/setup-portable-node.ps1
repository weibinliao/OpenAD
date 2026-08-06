param(
    [string]$NodeVersion = "22.23.2",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$toolsDir = Join-Path $projectRoot "tools"
$nodeDir = Join-Path $toolsDir "node"
$nodeExe = Join-Path $nodeDir "node.exe"
$npmCmd = Join-Path $nodeDir "npm.cmd"
$npmPrefixJs = Join-Path $nodeDir "node_modules\npm\bin\npm-prefix.js"
$npmCliJs = Join-Path $nodeDir "node_modules\npm\bin\npm-cli.js"
$downloadPath = Join-Path $toolsDir "_node_download.win-x64.zip"
$downloadUrl = "https://nodejs.org/dist/v{0}/node-v{0}-win-x64.zip" -f $NodeVersion

function Test-PortableNodeReady {
    param(
        [string]$NodeExePath,
        [string]$NpmCmdPath,
        [string]$NpmPrefixJsPath,
        [string]$NpmCliJsPath
    )

    if (-not (Test-Path $NodeExePath) -or -not (Test-Path $NpmCmdPath) -or -not (Test-Path $NpmPrefixJsPath) -or -not (Test-Path $NpmCliJsPath)) {
        return $false
    }

    try {
        & $NodeExePath --version | Out-Null
        & $NpmCmdPath --version | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

function Expand-PortableNodeArchive {
    param(
        [string]$ArchivePath,
        [string]$DestinationRoot,
        [string]$ExtractDestination
    )

    if (Test-Path $ExtractDestination) {
        Remove-Item -Recurse -Force $ExtractDestination
    }
    if (Test-Path $DestinationRoot) {
        Remove-Item -Recurse -Force $DestinationRoot
    }

    Write-Host "Extracting Node archive..."
    Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDestination -Force

    $extractedNodeDir = Get-ChildItem -Path $ExtractDestination -Directory | Select-Object -First 1
    if (-not $extractedNodeDir) {
        throw "Portable Node extraction failed. No extracted directory found."
    }

    Move-Item -Path $extractedNodeDir.FullName -Destination $DestinationRoot
    Remove-Item -Recurse -Force $ExtractDestination
}

function Download-OfficialNodeArchive {
    param(
        [string]$ArchivePath,
        [string]$Url
    )

    if (Test-Path $ArchivePath) {
        Remove-Item -Force $ArchivePath
    }

    Write-Host "Downloading official Node archive: $Url"
    Invoke-WebRequest -Uri $Url -OutFile $ArchivePath -UseBasicParsing
}

Write-Host "== OpenAD: Portable Node Bootstrap =="
Write-Host "Project root: $projectRoot"

if ((Test-PortableNodeReady -NodeExePath $nodeExe -NpmCmdPath $npmCmd -NpmPrefixJsPath $npmPrefixJs -NpmCliJsPath $npmCliJs) -and -not $Force) {
    $version = (& $nodeExe --version).Trim()
    if ($version -eq "v$NodeVersion") {
        Write-Host "Portable Node already exists: $version"
        exit 0
    }
    Write-Host "Portable Node $version does not match requested v$NodeVersion. Upgrading..."
    Remove-Item -Recurse -Force $nodeDir
}

if ($Force -and (Test-Path $nodeDir)) {
    Write-Host "Force mode enabled. Removing existing Node directory..."
    Remove-Item -Recurse -Force $nodeDir
}

if (-not (Test-Path $toolsDir)) {
    New-Item -ItemType Directory -Path $toolsDir | Out-Null
}

$preferredZip = Join-Path $toolsDir ("node-v{0}-win-x64.zip" -f $NodeVersion)
$zipPath = $null
$downloadedZip = $false

if (Test-Path $preferredZip) {
    $zipPath = $preferredZip
    Write-Host "Using bundled Node archive: $zipPath"
} else {
    $zipPath = $downloadPath
    Write-Host "Requested Node archive not bundled. Downloading from: $downloadUrl"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
    $downloadedZip = $true
}

$extractRoot = Join-Path $toolsDir "_node_extract"
Expand-PortableNodeArchive -ArchivePath $zipPath -DestinationRoot $nodeDir -ExtractDestination $extractRoot

if (-not (Test-Path $nodeExe)) {
    throw "Portable Node installation failed. Missing file: $nodeExe"
}
if (-not (Test-Path $npmCmd)) {
    throw "Portable Node installation failed. Missing file: $npmCmd"
}

if (-not (Test-PortableNodeReady -NodeExePath $nodeExe -NpmCmdPath $npmCmd -NpmPrefixJsPath $npmPrefixJs -NpmCliJsPath $npmCliJs)) {
    if (-not $downloadedZip -or $zipPath -ne $downloadPath) {
        Write-Warning "Portable Node installation is incomplete. Retrying with a fresh official download."
        Download-OfficialNodeArchive -ArchivePath $downloadPath -Url $downloadUrl
        $zipPath = $downloadPath
        $downloadedZip = $true
        Expand-PortableNodeArchive -ArchivePath $zipPath -DestinationRoot $nodeDir -ExtractDestination $extractRoot
    }
}

if (-not (Test-PortableNodeReady -NodeExePath $nodeExe -NpmCmdPath $npmCmd -NpmPrefixJsPath $npmPrefixJs -NpmCliJsPath $npmCliJs)) {
    throw "Portable Node installation failed. npm runtime files are missing under $nodeDir\node_modules\npm\bin."
}

$installedVersion = & $nodeExe --version
Write-Host "Portable Node ready: $installedVersion"

if ($downloadedZip -and (Test-Path $zipPath)) {
    Remove-Item -Force $zipPath
}

exit 0
