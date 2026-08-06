[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$PackagePath,
    [string]$ExpectedVersion,
    [string]$AdditionalArtifactPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir '..')).Path
$distDir = (Resolve-Path (Join-Path $rootDir 'dist')).Path
$resolvedPackage = (Resolve-Path -LiteralPath $PackagePath).Path
$distPrefix = $distDir.TrimEnd('\') + '\'
if (-not ($resolvedPackage + '\').StartsWith($distPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Release audit only accepts packages under $distDir`: $resolvedPackage"
}

function Get-PackageRelativePath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BasePath,
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $baseUri = [Uri]([IO.Path]::GetFullPath($BasePath).TrimEnd('\') + '\')
    $pathUri = [Uri][IO.Path]::GetFullPath($Path)
    return [Uri]::UnescapeDataString($baseUri.MakeRelativeUri($pathUri).ToString()).Replace('/', '\')
}

$findings = [System.Collections.Generic.List[string]]::new()
$requiredFiles = @(
    'OpenAD.exe',
    'OpenAD.Server.exe',
    'OpenAD.CLI.exe',
    'OpenAD.Web.exe',
    'web\index.html',
    'OpenAD-README.txt',
    'LICENSE',
    'LICENSING.md',
    'NOTICE'
)
foreach ($relativePath in $requiredFiles) {
    if (-not (Test-Path -LiteralPath (Join-Path $resolvedPackage $relativePath) -PathType Leaf)) {
        $findings.Add("missing required file: $relativePath")
    }
}

$forbiddenFilePatterns = @(
    '*.pdb',
    '*.db',
    '*.db-shm',
    '*.db-wal',
    '*.sqlite',
    '*.sqlite3',
    '*.log',
    '*.pid',
    '*.p12',
    '*.pfx',
    '*.pem',
    '*.key',
    '.env',
    '.env.*'
)
foreach ($pattern in $forbiddenFilePatterns) {
    foreach ($file in @(Get-ChildItem -LiteralPath $resolvedPackage -File -Recurse -Filter $pattern)) {
        $relative = Get-PackageRelativePath -BasePath $resolvedPackage -Path $file.FullName
        $findings.Add("forbidden release file: $relative")
    }
}

foreach ($file in @(Get-ChildItem -LiteralPath $resolvedPackage -File -Recurse)) {
    if ($file.Name -match '(?i)permission[-_.]?protector') {
        $relative = Get-PackageRelativePath -BasePath $resolvedPackage -Path $file.FullName
        $findings.Add("legacy product name in release filename: $relative")
    }
}

$contentRules = @(
    @{ Name = 'QQ email address'; Pattern = '(?i)\b[0-9]{5,}@qq\.com\b' }
    @{ Name = 'Windows user profile path'; Pattern = '(?i)\b[A-Z]:\\Users\\[^\\\r\n\x00]+' }
    @{ Name = 'local OpenAD repository path'; Pattern = '(?i)\b[A-Z]:\\[^\r\n\x00]{0,300}\\OpenAD(?:[\\/\x00]|$)' }
    @{ Name = 'private AD environment identity'; Pattern = '(?i)\bGENEW\b|\bgenew\.(?:com|cn)\b' }
    # Self-contained .NET carries assembly versions that overlap the private IPv4 range.
    @{ Name = 'RFC 1918 address literal'; Pattern = '(?<![\d.])(?!(?:10\.(?:0|1)\.0\.0)(?![\d.]))(?:10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2})(?![\d.])' }
)

$filesToScan = [System.Collections.Generic.List[IO.FileInfo]]::new()
foreach ($file in @(Get-ChildItem -LiteralPath $resolvedPackage -File -Recurse)) {
    $filesToScan.Add($file)
}
if (-not [string]::IsNullOrWhiteSpace($AdditionalArtifactPath)) {
    $resolvedArtifact = (Resolve-Path -LiteralPath $AdditionalArtifactPath).Path
    if (-not $resolvedArtifact.StartsWith($distPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Additional release audit artifact must be under $distDir`: $resolvedArtifact"
    }
    if (-not (Test-Path -LiteralPath $resolvedArtifact -PathType Leaf)) {
        throw "Additional release audit artifact is not a file: $resolvedArtifact"
    }
    $filesToScan.Add((Get-Item -LiteralPath $resolvedArtifact))
}

$latin1 = [Text.Encoding]::GetEncoding(28591)
foreach ($file in $filesToScan) {
    if ($file.FullName.StartsWith($resolvedPackage.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) {
        $relative = Get-PackageRelativePath -BasePath $resolvedPackage -Path $file.FullName
    } else {
        $relative = $file.Name
    }
    $bytes = [IO.File]::ReadAllBytes($file.FullName)
    $asciiText = [regex]::Replace($latin1.GetString($bytes), '[^\x09\x0A\x0D\x20-\x7E]', "`n")
    $unicodeText = [regex]::Replace([Text.Encoding]::Unicode.GetString($bytes), '[^\x09\x0A\x0D\x20-\x7E]', "`n")
    foreach ($rule in $contentRules) {
        if ([regex]::IsMatch($asciiText, $rule.Pattern) -or [regex]::IsMatch($unicodeText, $rule.Pattern)) {
            $findings.Add("$relative [$($rule.Name)]")
        }
    }
}

if (-not [string]::IsNullOrWhiteSpace($ExpectedVersion)) {
    $executablePath = Join-Path $resolvedPackage 'OpenAD.exe'
    if (Test-Path -LiteralPath $executablePath -PathType Leaf) {
        $productVersion = [Diagnostics.FileVersionInfo]::GetVersionInfo($executablePath).ProductVersion
        if ([string]::IsNullOrWhiteSpace($productVersion) -or
            -not $productVersion.StartsWith($ExpectedVersion, [StringComparison]::OrdinalIgnoreCase)) {
            $findings.Add("OpenAD.exe product version '$productVersion' does not start with '$ExpectedVersion'")
        }
    }
}

if ($findings.Count -gt 0) {
    Write-Host 'Release package audit failed:'
    $findings | Sort-Object -Unique | ForEach-Object { Write-Host " - $_" }
    exit 1
}

$totalBytes = ($filesToScan | Measure-Object -Property Length -Sum).Sum
Write-Host "[OK] Release package audit passed ($($filesToScan.Count) files, $totalBytes bytes)."
