[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
    [switch]$IncludeDependencies,
    [switch]$IncludeReleases,
    [switch]$IncludeLocalData
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path -LiteralPath (Join-Path $scriptDir '..')).Path.TrimEnd('\')

function Assert-WorkspacePath([string]$Path) {
    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $prefix = $rootDir + [System.IO.Path]::DirectorySeparatorChar
    if (-not $fullPath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "拒绝删除 OpenAD 工作区之外的路径 / Refusing to delete outside the OpenAD workspace: $fullPath"
    }
    return $fullPath
}

function Remove-WorkspacePath([string]$RelativePath) {
    $target = Assert-WorkspacePath (Join-Path $rootDir $RelativePath)
    if (-not (Test-Path -LiteralPath $target)) {
        return
    }

    if ($PSCmdlet.ShouldProcess($target, '删除生成的工作区内容 / Remove generated workspace content')) {
        Remove-Item -LiteralPath $target -Recurse -Force
    }
}

$temporaryBackupDir = Join-Path $rootDir '.codex-tmp\backups'
if (Test-Path -LiteralPath $temporaryBackupDir) {
    $temporaryBackups = Get-ChildItem -LiteralPath $temporaryBackupDir -Force -File -ErrorAction SilentlyContinue
    if ($temporaryBackups) {
        throw '清理前请先把 .codex-tmp\backups 中的文件移动到 backups\source-archives。 / Move files from .codex-tmp\backups to backups\source-archives before cleanup.'
    }
}

$generatedPaths = @(
    '.codex-tmp',
    '.gocache',
    '.gomodcache',
    '.gotelemetry',
    '.playwright-mcp',
    '.screenshots',
    '.superpowers',
    '.trash-legacy',
    'apps\backend\.cache',
    'apps\backend\api.exe',
    'apps\backend\cli.exe',
    'apps\desktop-win\bin',
    'apps\desktop-win\obj',
    'apps\desktop-win.tests\bin',
    'apps\desktop-win.tests\obj',
    'apps\web\.local',
    'apps\web\.next',
    'apps\web\.next-dev',
    'apps\web\.next-static',
    'apps\web\out',
    'apps\web\tsconfig.tsbuildinfo'
)

foreach ($path in $generatedPaths) {
    Remove-WorkspacePath $path
}

foreach ($directory in @('apps\backend', 'apps\web')) {
    $absoluteDirectory = Assert-WorkspacePath (Join-Path $rootDir $directory)
    Get-ChildItem -LiteralPath $absoluteDirectory -File -Filter '*.log' -ErrorAction SilentlyContinue |
        ForEach-Object {
            if ($PSCmdlet.ShouldProcess($_.FullName, '删除生成日志 / Remove generated log')) {
                Remove-Item -LiteralPath $_.FullName -Force
            }
        }
}

if ($IncludeDependencies) {
    Remove-WorkspacePath 'apps\web\node_modules'
}

if ($IncludeReleases) {
    Remove-WorkspacePath 'dist'
}

if ($IncludeLocalData) {
    Remove-WorkspacePath '.local'
}

Write-Host 'OpenAD 工作区清理完成。 / OpenAD workspace cleanup completed.'
