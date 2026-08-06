[CmdletBinding()]
param(
    [string]$Version = '6.7.3',
    [string]$ExpectedSha256 = '9C73C3BAE7ED48D44112A0F48E66742C00090BDB5BEF71D9D3C056C66E97B732',
    [string]$ChineseLanguageSha256 = '3C2C27DB8E346EE824F058ABDB56C4AC2F599D8315B4C7089D5D5615C8D2CF54'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir '..')).Path
$toolsDir = Join-Path $rootDir 'tools'
$installDir = Join-Path $toolsDir 'inno-setup'
$compiler = Join-Path $installDir 'ISCC.exe'
$downloadDir = Join-Path $toolsDir 'downloads'
$installer = Join-Path $downloadDir ("innosetup-{0}.exe" -f $Version)
$downloadUrl = "https://github.com/jrsoftware/issrc/releases/download/is-{0}/innosetup-{1}.exe" -f ($Version -replace '\.', '_'), $Version
New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

if (-not (Test-Path -LiteralPath $compiler -PathType Leaf)) {
    if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
        Write-Host "[INFO] Downloading Inno Setup $Version..."
        Invoke-WebRequest -UseBasicParsing -Uri $downloadUrl -OutFile $installer
    }

    $actualHash = (Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash
    if (-not [string]::Equals($actualHash, $ExpectedSha256, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Inno Setup checksum mismatch. Expected $ExpectedSha256, got $actualHash."
    }

    $signature = Get-AuthenticodeSignature -LiteralPath $installer
    if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid -or
        $signature.SignerCertificate.Subject -notmatch 'O=Pyrsys B\.V\.') {
        throw "Inno Setup signature validation failed: $($signature.Status) $($signature.StatusMessage)"
    }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $arguments = @(
        '/VERYSILENT',
        '/SUPPRESSMSGBOXES',
        '/NORESTART',
        '/CURRENTUSER',
        '/NOICONS',
        "/DIR=$installDir"
    )
    $process = Start-Process -FilePath $installer -ArgumentList $arguments -Wait -PassThru
    if ($process.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $compiler -PathType Leaf)) {
        throw "Inno Setup installation failed with exit code $($process.ExitCode)."
    }
}

$languageDir = Join-Path $installDir 'Languages'
$languageFile = Join-Path $languageDir 'ChineseSimplified.isl'
$languageDownload = Join-Path $downloadDir 'ChineseSimplified.isl'
$languageUrl = 'https://raw.githubusercontent.com/jrsoftware/issrc/main/Files/Languages/ChineseSimplified.isl'
if (-not (Test-Path -LiteralPath $languageFile -PathType Leaf)) {
    if (-not (Test-Path -LiteralPath $languageDownload -PathType Leaf)) {
        Write-Host '[INFO] Downloading the Inno Setup Simplified Chinese translation...'
        Invoke-WebRequest -UseBasicParsing -Uri $languageUrl -OutFile $languageDownload
    }

    $languageHash = (Get-FileHash -LiteralPath $languageDownload -Algorithm SHA256).Hash
    if (-not [string]::Equals($languageHash, $ChineseLanguageSha256, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Inno Setup language checksum mismatch. Expected $ChineseLanguageSha256, got $languageHash."
    }

    New-Item -ItemType Directory -Force -Path $languageDir | Out-Null
    Copy-Item -LiteralPath $languageDownload -Destination $languageFile
}

$installedLanguageHash = (Get-FileHash -LiteralPath $languageFile -Algorithm SHA256).Hash
if (-not [string]::Equals($installedLanguageHash, $ChineseLanguageSha256, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Installed Inno Setup language checksum mismatch. Expected $ChineseLanguageSha256, got $installedLanguageHash."
}

Write-Host "[OK] Inno Setup compiler ready: $compiler"
