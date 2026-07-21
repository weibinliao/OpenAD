param(
    [string]$Configuration = 'Release',
    [string]$RuntimeIdentifier = 'win-x64',
    [string]$Version = '0.1.0',
    [switch]$SelfContained
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir '..')).Path
$distDir = Join-Path $rootDir 'dist'
$releaseDir = Join-Path $distDir ("OpenAD-Windows-Desktop-v{0}" -f $Version)
$goExe = Join-Path $rootDir 'tools\go\bin\go.exe'
$npmCmd = Join-Path $rootDir 'tools\node\npm.cmd'
$desktopProject = Join-Path $rootDir 'apps\desktop-win\PermissionProtector.Desktop.csproj'
$desktopTestProject = Join-Path $rootDir 'apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj'

function Invoke-Step([string]$Label, [scriptblock]$Action) {
    Write-Host "[INFO] $Label"
    & $Action
}

function Assert-ChildPath([string]$Parent, [string]$Child) {
    $parentFull = [System.IO.Path]::GetFullPath($Parent).TrimEnd('\') + '\'
    $childFull = [System.IO.Path]::GetFullPath($Child)
    if (-not $childFull.StartsWith($parentFull, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to operate outside $parentFull`: $childFull"
    }
}
if (-not (Test-Path -LiteralPath $goExe -PathType Leaf)) {
    Invoke-Step 'Preparing portable Go toolchain...' {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $rootDir 'scripts\setup-portable-go.ps1')
    }
}

if (-not (Test-Path -LiteralPath $npmCmd -PathType Leaf)) {
    Invoke-Step 'Preparing portable Node.js toolchain...' {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $rootDir 'scripts\setup-portable-node.ps1')
    }
}

New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Assert-ChildPath -Parent $distDir -Child $releaseDir
if (Test-Path -LiteralPath $releaseDir) {
    Remove-Item -LiteralPath $releaseDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $releaseDir | Out-Null

$env:GOTELEMETRY = 'off'
$env:GOCACHE = Join-Path $rootDir '.gocache'
$env:GOMODCACHE = Join-Path $rootDir '.gomodcache'
Invoke-Step 'Building Go backend services...' {
    Push-Location (Join-Path $rootDir 'apps\backend')
    try {
        & $goExe test ./...
        if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
        & $goExe build -o (Join-Path $releaseDir 'permission-protector-server.exe') .\cmd\api
        if ($LASTEXITCODE -ne 0) { throw 'go api build failed' }
        & $goExe build -o (Join-Path $releaseDir 'permission-protector-cli.exe') .\cmd\cli
        if ($LASTEXITCODE -ne 0) { throw 'go cli build failed' }
        & $goExe build -o (Join-Path $releaseDir 'permission-protector-web.exe') .\cmd\webserver
        if ($LASTEXITCODE -ne 0) { throw 'go webserver build failed' }
    }
    finally {
        Pop-Location
    }
}

Invoke-Step 'Building static web console...' {
    Push-Location (Join-Path $rootDir 'apps\web')
    try {
        & $npmCmd run typecheck
        if ($LASTEXITCODE -ne 0) { throw 'web typecheck failed' }
        & $npmCmd test
        if ($LASTEXITCODE -ne 0) { throw 'web tests failed' }
        & $npmCmd run build:static
        if ($LASTEXITCODE -ne 0) { throw 'web static build failed' }
    }
    finally {
        Pop-Location
    }
}
Invoke-Step 'Copying static web files...' {
    Copy-Item -LiteralPath (Join-Path $rootDir 'apps\web\out') -Destination (Join-Path $releaseDir 'web') -Recurse -Force
}

Invoke-Step 'Testing Windows desktop shell...' {
    & dotnet test $desktopTestProject -c $Configuration '/p:RestoreIgnoreFailedSources=true'
    if ($LASTEXITCODE -ne 0) { throw 'desktop test failed' }
}

Invoke-Step 'Publishing Windows desktop shell...' {
    $publishArgs = @(
        'publish',
        $desktopProject,
        '-c', $Configuration,
        '-r', $RuntimeIdentifier,
        '-o', $releaseDir,
        '--self-contained', ($(if ($SelfContained) { 'true' } else { 'false' })),
        '/p:RestoreIgnoreFailedSources=true'
    )
    & dotnet @publishArgs
    if ($LASTEXITCODE -ne 0) { throw 'dotnet publish failed' }
}

Invoke-Step 'Copying fallback launch scripts and desktop notes...' {
    foreach ($file in @('start-windows.bat', 'stop-windows.bat', 'verify-install.bat')) {
        $source = Join-Path $rootDir "scripts\$file"
        if (Test-Path -LiteralPath $source -PathType Leaf) {
            Copy-Item -LiteralPath $source -Destination $releaseDir -Force
        }
    }

    $readme = @(
        'OpenAD Windows Desktop',
        '',
        'Double-click PermissionProtector.exe (compatibility filename) to start OpenAD.',
        'OpenAD starts the bundled Go API and static web service locally, then opens the product UI inside WebView2.',
        '',
        'For compatibility, logs and local data currently remain under %APPDATA%\PermissionProtector.',
        'Fallback browser launcher: start-windows.bat'
    )
    Set-Content -LiteralPath (Join-Path $releaseDir 'README_DESKTOP.txt') -Value $readme -Encoding UTF8
}

Write-Host '[OK] Desktop package created:'
Write-Host $releaseDir
