param(
    [Parameter(Mandatory = $true)][string]$ProjectRoot,
    [Parameter(Mandatory = $true)][string]$DataDir,
    [string]$ApiHost = '127.0.0.1',
    [int]$ApiPort = 18080,
    [string]$GinMode = 'release',
    [Parameter(Mandatory = $true)][string]$DatabaseUrl,
    [string]$WebHost = '127.0.0.1',
    [int]$WebPort = 3010,
    [string]$PublicHost = 'localhost'
)

$ErrorActionPreference = 'Stop'

$allInterfaceHosts = @('0.0.0.0', '*', '+')
$lanBinding = $allInterfaceHosts -contains $ApiHost.Trim() -or $allInterfaceHosts -contains $WebHost.Trim()
if ($lanBinding) {
    Write-Warning 'OpenAD is listening on all network interfaces without product login or RBAC. Use only on a trusted administration network.'
}

function Assert-File([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Label not found: $Path"
    }
}

function Assert-Directory([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        throw "$Label not found: $Path"
    }
}

function Escape-BatchValue([string]$Value) {
    return $Value.Replace('"', '""')
}

function New-DetachedProcess([string]$CommandLine, [string]$WorkingDirectory) {
    $startup = ([wmiclass]'Win32_ProcessStartup').CreateInstance()
    $startup.ShowWindow = 0

    $result = ([wmiclass]'Win32_Process').Create($CommandLine, $WorkingDirectory, $startup)

    if ($result.ReturnValue -ne 0) {
        throw "Failed to start process. Win32_Process.Create returned $($result.ReturnValue)."
    }

    return [int]$result.ProcessId
}

$resolvedProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
$serverExe = Join-Path $resolvedProjectRoot 'OpenAD.Server.exe'
$webServerExe = Join-Path $resolvedProjectRoot 'OpenAD.Web.exe'
$webRoot = Join-Path $resolvedProjectRoot 'web'
$webIndex = Join-Path $webRoot 'index.html'

Assert-File $serverExe 'OpenAD API service (OpenAD.Server.exe)'
Assert-File $webServerExe 'OpenAD web service (OpenAD.Web.exe)'
Assert-Directory $webRoot 'Web directory'
Assert-File $webIndex 'Web index.html'

$resolvedDataDir = [System.IO.Path]::GetFullPath($DataDir)
$runDir = Join-Path $resolvedDataDir 'run'
New-Item -ItemType Directory -Force -Path $resolvedDataDir, $runDir | Out-Null

$apiLogFile = Join-Path $runDir 'api-server.log'
$apiRunner = Join-Path $runDir 'run-api-server.cmd'
$webLogFile = Join-Path $runDir 'static-server.log'
$webRunner = Join-Path $runDir 'run-static-server.cmd'
$apiPidFile = Join-Path $runDir 'api-server.pid'
$staticPidFile = Join-Path $runDir 'static-server.pid'

Remove-Item -LiteralPath $apiPidFile, $staticPidFile -Force -ErrorAction SilentlyContinue

$apiRunnerLines = @(
    '@echo off',
    ('set "API_HOST={0}"' -f (Escape-BatchValue $ApiHost)),
    ('set "PORT={0}"' -f $ApiPort),
    ('set "GIN_MODE={0}"' -f (Escape-BatchValue $GinMode)),
    ('set "DATABASE_URL={0}"' -f (Escape-BatchValue $DatabaseUrl)),
    ('cd /d "{0}"' -f $resolvedProjectRoot),
    ('"{0}" 1>>"{1}" 2>&1' -f $serverExe, $apiLogFile)
)
Set-Content -LiteralPath $apiRunner -Value $apiRunnerLines -Encoding ascii

Write-Host "[INFO] Starting OpenAD API on $ApiHost`:$ApiPort..."
$apiCommandLine = 'cmd.exe /d /c "{0}"' -f $apiRunner
$apiPid = New-DetachedProcess -CommandLine $apiCommandLine -WorkingDirectory $resolvedProjectRoot
Set-Content -LiteralPath $apiPidFile -Value $apiPid -Encoding ascii

Write-Host "[INFO] Starting OpenAD web on $WebHost`:$WebPort..."
$webRunnerLines = @(
    '@echo off',
    ('"{0}" -root "{1}" -port {2} -host "{3}" 1>>"{4}" 2>&1' -f $webServerExe, $webRoot, $WebPort, $WebHost, $webLogFile)
)
Set-Content -LiteralPath $webRunner -Value $webRunnerLines -Encoding ascii

$webCommandLine = 'cmd.exe /d /c "{0}"' -f $webRunner
$webPid = New-DetachedProcess -CommandLine $webCommandLine -WorkingDirectory $resolvedProjectRoot
Set-Content -LiteralPath $staticPidFile -Value $webPid -Encoding ascii

Write-Host '[OK] OpenAD started in background.'
$displayHost = if ([string]::IsNullOrWhiteSpace($PublicHost)) { 'localhost' } else { $PublicHost }
Write-Host "[INFO] Web: http://$displayHost`:$WebPort"
Write-Host "[INFO] API: http://$displayHost`:$ApiPort"
Write-Host "[INFO] API log: $apiLogFile"
Write-Host "[INFO] Web log: $webLogFile"
Write-Host "[INFO] Data: $resolvedDataDir"
