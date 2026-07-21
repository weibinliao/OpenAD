param(
    [string]$GoVersion = "1.25.8",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$toolsDir = Join-Path $projectRoot "tools"
$goDir = Join-Path $toolsDir "go"
$goExe = Join-Path $goDir "bin\go.exe"

Write-Host "== OpenAD: Portable Go Bootstrap =="
Write-Host "Project root: $projectRoot"

if ((Test-Path $goExe) -and -not $Force) {
    $version = & $goExe version
    Write-Host "Portable Go already exists: $version"
    exit 0
}

if ($Force -and (Test-Path $goDir)) {
    Write-Host "Force mode enabled. Removing existing Go directory..."
    Remove-Item -Recurse -Force $goDir
}

if (-not (Test-Path $toolsDir)) {
    New-Item -ItemType Directory -Path $toolsDir | Out-Null
}

$preferredZip = Join-Path $toolsDir ("go{0}.windows-amd64.zip" -f $GoVersion)
$zipPath = $null
$downloadedZip = $false

if (Test-Path $preferredZip) {
    $zipPath = $preferredZip
    Write-Host "Using bundled Go archive: $zipPath"
} else {
    $bundledArchives = Get-ChildItem -Path $toolsDir -Filter "go*.windows-amd64.zip" -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending

    if ($bundledArchives.Count -gt 0) {
        $zipPath = $bundledArchives[0].FullName
        Write-Host "Requested version archive not found. Using bundled archive: $zipPath"
    } else {
        $zipPath = Join-Path $toolsDir "_go_download.windows-amd64.zip"
        $downloadUrl = "https://go.dev/dl/go{0}.windows-amd64.zip" -f $GoVersion
        Write-Host "No bundled Go archive found. Downloading from: $downloadUrl"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
        $downloadedZip = $true
    }
}

if (Test-Path $goDir) {
    Remove-Item -Recurse -Force $goDir
}

Write-Host "Extracting Go archive..."
Expand-Archive -Path $zipPath -DestinationPath $toolsDir -Force

if (-not (Test-Path $goExe)) {
    throw "Portable Go installation failed. Missing file: $goExe"
}

$installedVersion = & $goExe version
Write-Host "Portable Go ready: $installedVersion"

if ($downloadedZip -and (Test-Path $zipPath)) {
    Remove-Item -Force $zipPath
}

exit 0
