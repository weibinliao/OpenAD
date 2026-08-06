[CmdletBinding()]
param(
    [string]$Configuration = 'Release',
    [string]$RuntimeIdentifier = 'win-x64',
    [string]$Version = '1.0.0',
    [string]$InnoCompiler,
    [switch]$SkipDesktopBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir '..')).Path
$distDir = Join-Path $rootDir 'dist'
$releaseDir = Join-Path $distDir ("OpenAD-Windows-Desktop-v{0}" -f $Version)
$installerScript = Join-Path $rootDir 'installer\OpenAD.iss'
$auditScript = Join-Path $scriptDir 'audit-release-package.ps1'

if ($RuntimeIdentifier -ne 'win-x64') {
    throw "The installer currently supports only win-x64, got '$RuntimeIdentifier'."
}
if ($Version -notmatch '^\d+\.\d+\.\d+$') {
    throw "Installer version must use numeric major.minor.patch format, got '$Version'."
}

if (-not $SkipDesktopBuild) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $scriptDir 'build-desktop-windows.ps1') `
        -Configuration $Configuration `
        -RuntimeIdentifier $RuntimeIdentifier `
        -Version $Version `
        -SelfContained
    if ($LASTEXITCODE -ne 0) { throw 'Desktop package build failed.' }
}

if (-not (Test-Path -LiteralPath $releaseDir -PathType Container)) {
    throw "Desktop release directory not found: $releaseDir"
}

& powershell -NoProfile -ExecutionPolicy Bypass -File $auditScript `
    -PackagePath $releaseDir `
    -ExpectedVersion $Version
if ($LASTEXITCODE -ne 0) { throw 'Release package audit failed.' }

if ([string]::IsNullOrWhiteSpace($InnoCompiler)) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $scriptDir 'setup-inno-setup.ps1')
    if ($LASTEXITCODE -ne 0) { throw 'Inno Setup preparation failed.' }
    $InnoCompiler = Join-Path $rootDir 'tools\inno-setup\ISCC.exe'
}

$outputBaseName = 'OpenAD'
$installerPath = Join-Path $distDir "$outputBaseName.exe"
$checksumPath = "$installerPath.sha256"
$portableBaseName = "OpenAD-v$Version-win-x64"
$portablePath = Join-Path $distDir "$portableBaseName.zip"
$portableChecksumPath = "$portablePath.sha256"
foreach ($path in @($installerPath, $checksumPath, $portablePath, $portableChecksumPath)) {
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        Remove-Item -LiteralPath $path -Force
    }
}

& $InnoCompiler `
    "/DMyAppVersion=$Version" `
    "/DSourceDir=$releaseDir" `
    "/DOutputDir=$distDir" `
    $installerScript
if ($LASTEXITCODE -ne 0) { throw 'Inno Setup compilation failed.' }
if (-not (Test-Path -LiteralPath $installerPath -PathType Leaf)) {
    throw "Installer was not created: $installerPath"
}

& powershell -NoProfile -ExecutionPolicy Bypass -File $auditScript `
    -PackagePath $releaseDir `
    -ExpectedVersion $Version `
    -AdditionalArtifactPath $installerPath
if ($LASTEXITCODE -ne 0) { throw 'Final installer privacy audit failed.' }

$hash = (Get-FileHash -LiteralPath $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -LiteralPath $checksumPath -Encoding ASCII -Value "$hash *$outputBaseName.exe"

Compress-Archive -Path (Join-Path $releaseDir '*') -DestinationPath $portablePath -CompressionLevel Optimal
$portableHash = (Get-FileHash -LiteralPath $portablePath -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -LiteralPath $portableChecksumPath -Encoding ASCII -Value "$portableHash *$portableBaseName.zip"

Write-Host '[OK] Windows installer created:'
Write-Host $installerPath
Write-Host "SHA256: $hash"
Write-Host '[OK] Portable Windows package created:'
Write-Host $portablePath
Write-Host "SHA256: $portableHash"
