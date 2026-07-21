param(
    [string]$ProjectRoot,
    [string]$DataDir,
    [string]$ApiHost = '0.0.0.0',
    [int]$ApiPort = 18080,
    [string]$WebHost = '0.0.0.0',
    [int]$WebPort = 3010,
    [string]$PublicHost = '',
    [string]$NetworkAcl = $env:NETWORK_ACL,
    [string]$WebMode = 'production',
    [bool]$SkipWebBuild = ($env:PP_SKIP_WEB_BUILD -eq '1')
)

$ErrorActionPreference = 'Stop'

function Escape-BatchValue([string]$Value) {
    return $Value.Replace('"', '""')
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

function Write-Log([string]$Message) {
    $line = '[{0}] {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $Message
    Add-Content -LiteralPath $script:SetupLog -Value $line -Encoding utf8
    Write-Host $line
}

function Invoke-LoggedExternal([string]$Label, [scriptblock]$Command) {
    Write-Log $Label
    $global:LASTEXITCODE = 0
    & $Command *>> $script:SetupLog
    if ($global:LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code $global:LASTEXITCODE. See $script:SetupLog"
    }
}

function New-HiddenRunnerProcess([string]$RunnerPath, [string]$WorkingDirectory) {
    $commandLine = 'cmd.exe /d /c "{0}"' -f $RunnerPath

    try {
        $startup = ([wmiclass]'Win32_ProcessStartup').CreateInstance()
        $startup.ShowWindow = 0

        $result = ([wmiclass]'Win32_Process').Create($commandLine, $WorkingDirectory, $startup)
        if ($result.ReturnValue -ne 0) {
            throw "Win32_Process.Create returned $($result.ReturnValue)."
        }

        return [int]$result.ProcessId
    }
    catch {
        Write-Log "WMI hidden launch unavailable. Using .NET no-window fallback: $($_.Exception.Message)"

        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = if ($env:ComSpec) { $env:ComSpec } else { 'cmd.exe' }
        $psi.Arguments = '/d /c "{0}"' -f $RunnerPath
        $psi.WorkingDirectory = $WorkingDirectory
        $psi.UseShellExecute = $false
        $psi.CreateNoWindow = $true
        $psi.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden

        $process = [System.Diagnostics.Process]::Start($psi)
        Start-Sleep -Milliseconds 750
        if ($process.HasExited) {
            throw "Hidden runner exited immediately with code $($process.ExitCode)."
        }

        return [int]$process.Id
    }
}

function Get-DefaultPublicHost {
    try {
        $primary = Get-NetIPConfiguration -ErrorAction SilentlyContinue |
            Where-Object { $_.IPv4DefaultGateway -and $_.IPv4Address } |
            Sort-Object InterfaceMetric |
            Select-Object -First 1

        if ($primary -and $primary.IPv4Address.Count -gt 0) {
            return $primary.IPv4Address[0].IPAddress
        }

        $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
            Where-Object {
                $_.IPAddress -ne '127.0.0.1' -and
                $_.IPAddress -notlike '169.254*' -and
                $_.PrefixOrigin -ne 'WellKnown'
            } |
            Sort-Object InterfaceMetric |
            Select-Object -First 1 -ExpandProperty IPAddress

        if ($ip) {
            return $ip
        }
    }
    catch {
    }

    return 'localhost'
}

function Ensure-FreePort([string]$Role, [int]$Port) {
    $state = & $script:PortHelper -Role $Role -Port $Port -RepoRoot $script:ResolvedProjectRoot -StopProjectOwned
    if ($LASTEXITCODE -ne 0) {
        throw "Port helper failed for $Role on $Port."
    }

    $stateText = (($state | Select-Object -First 1) -as [string]).Trim()
    if ($stateText -ne 'FREE') {
        throw "Port $Port is not available for $Role. State: $stateText"
    }
}

if ([string]::IsNullOrWhiteSpace($ProjectRoot)) {
    $ProjectRoot = Join-Path $PSScriptRoot '..'
}

$dataDirWasProvided = -not [string]::IsNullOrWhiteSpace($DataDir)
$script:ResolvedProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
$webAppDir = Join-Path $script:ResolvedProjectRoot 'apps\web'
$apiAppDir = Join-Path $script:ResolvedProjectRoot 'apps\backend'
$scriptsDir = Join-Path $script:ResolvedProjectRoot 'scripts'
$windowsScriptsDir = Join-Path $script:ResolvedProjectRoot 'windows\scripts'
$goBootstrap = Join-Path $scriptsDir 'setup-portable-go.ps1'
$nodeBootstrap = Join-Path $scriptsDir 'setup-portable-node.ps1'
$script:PortHelper = Join-Path $windowsScriptsDir 'ensure-dev-port.ps1'
$goExe = Join-Path $script:ResolvedProjectRoot 'tools\go\bin\go.exe'
$nodeDir = Join-Path $script:ResolvedProjectRoot 'tools\node'
$nodeExe = Join-Path $nodeDir 'node.exe'
$npmCmd = Join-Path $nodeDir 'npm.cmd'
$npmPrefixJs = Join-Path $nodeDir 'node_modules\npm\bin\npm-prefix.js'
$npmCliJs = Join-Path $nodeDir 'node_modules\npm\bin\npm-cli.js'
$nextBin = Join-Path $webAppDir 'node_modules\next\dist\bin\next'
$powerShellExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'

Assert-Directory $webAppDir 'Web app'
Assert-Directory $apiAppDir 'API app'
Assert-File $goBootstrap 'Go bootstrap script'
Assert-File $nodeBootstrap 'Node bootstrap script'
Assert-File $script:PortHelper 'Port helper script'
Assert-File $powerShellExe 'Windows PowerShell'

if ([string]::IsNullOrWhiteSpace($DataDir)) {
    $DataDir = Join-Path $env:APPDATA 'PermissionProtector'
}

function Initialize-RunDirectory([string]$CandidateDataDir) {
    $candidateResolvedDataDir = [System.IO.Path]::GetFullPath($CandidateDataDir)
    $candidateRunDir = Join-Path $candidateResolvedDataDir 'run'
    New-Item -ItemType Directory -Force -Path $candidateResolvedDataDir, $candidateRunDir | Out-Null

    $writeTest = Join-Path $candidateRunDir '.write_test'
    Set-Content -LiteralPath $writeTest -Value 'ok' -Encoding ascii
    Remove-Item -LiteralPath $writeTest -Force

    return @{
        DataDir = $candidateResolvedDataDir
        RunDir = $candidateRunDir
    }
}

try {
    $runState = Initialize-RunDirectory -CandidateDataDir $DataDir
}
catch {
    if ($dataDirWasProvided) {
        throw
    }

    $fallbackDataDir = Join-Path $script:ResolvedProjectRoot '.local\PermissionProtector'
    Write-Warning "Default data directory is not writable. Falling back to $fallbackDataDir"
    $runState = Initialize-RunDirectory -CandidateDataDir $fallbackDataDir
}

$resolvedDataDir = $runState.DataDir
$runDir = $runState.RunDir

$script:SetupLog = Join-Path $runDir 'source-background-launch.log'
$apiLog = Join-Path $runDir 'api-source.log'
$webLog = Join-Path $runDir 'web-source.log'
$apiRunner = Join-Path $runDir 'run-source-api.cmd'
$webRunner = Join-Path $runDir 'run-source-web.cmd'
$apiPidFile = Join-Path $runDir 'api-server.pid'
$webPidFile = Join-Path $runDir 'static-server.pid'

Set-Content -LiteralPath $script:SetupLog -Value ('[{0}] Starting source background launcher.' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss')) -Encoding utf8
Remove-Item -LiteralPath `
    $apiPidFile, `
    $webPidFile, `
    $apiLog, `
    $webLog, `
    (Join-Path $runDir 'run-source-api.ps1'), `
    (Join-Path $runDir 'run-source-web.ps1'), `
    (Join-Path $runDir 'run-source-api.stdout.log'), `
    (Join-Path $runDir 'run-source-api.stderr.log'), `
    (Join-Path $runDir 'run-source-web.stdout.log'), `
    (Join-Path $runDir 'run-source-web.stderr.log') `
    -Force `
    -ErrorAction SilentlyContinue

if ([string]::IsNullOrWhiteSpace($NetworkAcl)) {
    $NetworkAcl = 'loopback,private'
}

$envWebMode = [Environment]::GetEnvironmentVariable('PP_WEB_MODE', 'Process')
if (-not [string]::IsNullOrWhiteSpace($envWebMode)) {
    $WebMode = $envWebMode
}

$WebMode = $WebMode.Trim().ToLowerInvariant()
if ($WebMode -ne 'production' -and $WebMode -ne 'dev') {
    throw "Unsupported WebMode: $WebMode. Use production or dev."
}

if ([string]::IsNullOrWhiteSpace($PublicHost)) {
    $PublicHost = Get-DefaultPublicHost
}

if ([string]::IsNullOrWhiteSpace($PublicHost)) {
    $PublicHost = 'localhost'
}

foreach ($proxyName in @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'GIT_HTTP_PROXY', 'GIT_HTTPS_PROXY')) {
    $proxyValue = [Environment]::GetEnvironmentVariable($proxyName, 'Process')
    if ($proxyValue -and $proxyValue.Equals('http://127.0.0.1:9', [System.StringComparison]::OrdinalIgnoreCase)) {
        [Environment]::SetEnvironmentVariable($proxyName, $null, 'Process')
    }
}

if (-not (Test-Path -LiteralPath $goExe -PathType Leaf)) {
    Invoke-LoggedExternal 'Preparing portable Go toolchain' {
        & $powerShellExe -NoProfile -ExecutionPolicy Bypass -File $goBootstrap
    }
}

if (-not (Test-Path -LiteralPath $nodeExe -PathType Leaf) -or
    -not (Test-Path -LiteralPath $npmCmd -PathType Leaf) -or
    -not (Test-Path -LiteralPath $npmPrefixJs -PathType Leaf) -or
    -not (Test-Path -LiteralPath $npmCliJs -PathType Leaf)) {
    Invoke-LoggedExternal 'Preparing portable Node.js toolchain' {
        & $powerShellExe -NoProfile -ExecutionPolicy Bypass -File $nodeBootstrap
    }
}

Assert-File $goExe 'Go executable'
Assert-File $nodeExe 'Node executable'
Assert-File $npmCmd 'npm launcher'

$npmHealthy = $false
try {
    & $npmCmd --version *> $null
    $npmHealthy = $LASTEXITCODE -eq 0
}
catch {
    $npmHealthy = $false
}

if (-not $npmHealthy) {
    Invoke-LoggedExternal 'Repairing portable Node.js toolchain' {
        & $powerShellExe -NoProfile -ExecutionPolicy Bypass -File $nodeBootstrap -Force
    }
}

if (-not (Test-Path -LiteralPath (Join-Path $webAppDir 'node_modules') -PathType Container) -or
    -not (Test-Path -LiteralPath $nextBin -PathType Leaf)) {
    Invoke-LoggedExternal 'Installing web dependencies' {
        & $npmCmd --prefix $webAppDir install
    }
}

Assert-File $nextBin 'Next.js launcher'

$goCache = Join-Path $script:ResolvedProjectRoot '.gocache'
$goModCache = Join-Path $script:ResolvedProjectRoot '.gomodcache'
New-Item -ItemType Directory -Force -Path $goCache, $goModCache | Out-Null

try {
    $cacheWriteTest = Join-Path $goModCache '.write_test'
    Set-Content -LiteralPath $cacheWriteTest -Value 'ok' -Encoding ascii
    Remove-Item -LiteralPath $cacheWriteTest -Force
}
catch {
    $cacheRoot = Join-Path $env:TEMP 'permission-protector'
    $goCache = Join-Path $cacheRoot '.gocache'
    $goModCache = Join-Path $cacheRoot '.gomodcache'
    New-Item -ItemType Directory -Force -Path $goCache, $goModCache | Out-Null
}

$databaseUrl = $env:DATABASE_URL
if ([string]::IsNullOrWhiteSpace($databaseUrl)) {
    $databaseUrl = 'sqlite:///{0}' -f (Join-Path $resolvedDataDir 'permission-protector-dev.db')
}

Invoke-LoggedExternal 'Downloading Go dependencies' {
    $env:GOCACHE = $goCache
    $env:GOMODCACHE = $goModCache
    $env:GOPROXY = 'https://goproxy.cn,direct'
    $env:GOSUMDB = 'sum.golang.google.cn'
    & $goExe -C $apiAppDir mod download
}

Write-Log "Checking backend port $ApiPort"
Ensure-FreePort -Role backend -Port $ApiPort

Write-Log "Checking frontend port $WebPort"
Ensure-FreePort -Role frontend -Port $WebPort

$apiBaseUrl = 'http://{0}:{1}' -f $PublicHost, $ApiPort
$webBuildId = Join-Path $webAppDir '.next\BUILD_ID'
if ($WebMode -eq 'production') {
    if ((-not $SkipWebBuild) -or (-not (Test-Path -LiteralPath $webBuildId -PathType Leaf))) {
        Invoke-LoggedExternal 'Building web production bundle' {
            $env:NEXT_TELEMETRY_DISABLED = '1'
            Remove-Item Env:NEXT_PUBLIC_API_BASE_URL -ErrorAction SilentlyContinue
            & $npmCmd --prefix $webAppDir run build
        }
    }

    Assert-File $webBuildId 'Next.js production build'
}

$apiRunnerLines = @(
    '@echo off',
    'chcp 65001 >nul',
    ('set "GOCACHE={0}"' -f (Escape-BatchValue $goCache)),
    ('set "GOMODCACHE={0}"' -f (Escape-BatchValue $goModCache)),
    'set "GOPROXY=https://goproxy.cn,direct"',
    'set "GOSUMDB=sum.golang.google.cn"',
    ('set "API_HOST={0}"' -f (Escape-BatchValue $ApiHost)),
    ('set "API_PORT={0}"' -f $ApiPort),
    ('set "PORT={0}"' -f $ApiPort),
    ('set "NETWORK_ACL={0}"' -f (Escape-BatchValue $NetworkAcl)),
    ('set "PERMISSION_PROTECTOR_DATA_DIR={0}"' -f (Escape-BatchValue $resolvedDataDir)),
    ('set "DATABASE_URL={0}"' -f (Escape-BatchValue $databaseUrl)),
    ('echo [%DATE% %TIME%] API runner starting>>"{0}"' -f $apiLog),
    ('cd /d "{0}"' -f $apiAppDir),
    ('"{0}" run ./cmd/api 1>>"{1}" 2>&1' -f $goExe, $apiLog),
    ('set "EXIT_CODE=%ERRORLEVEL%"'),
    ('echo [%DATE% %TIME%] API runner exited with code %EXIT_CODE%>>"{0}"' -f $apiLog),
    'exit /b %EXIT_CODE%'
)
Set-Content -LiteralPath $apiRunner -Value $apiRunnerLines -Encoding ascii

if ($WebMode -eq 'production') {
    $webCommand = ('"{0}" "{1}" start --hostname "{2}" --port {3} 1>>"{4}" 2>&1' -f $nodeExe, $nextBin, $WebHost, $WebPort, $webLog)
    $webRunnerLines = @(
        '@echo off',
        'chcp 65001 >nul',
        'set "NODE_ENV=production"',
        'set "NEXT_TELEMETRY_DISABLED=1"',
        ('echo [%DATE% %TIME%] Web runner starting in production mode>>"{0}"' -f $webLog),
        ('cd /d "{0}"' -f $webAppDir),
        $webCommand,
        ('set "EXIT_CODE=%ERRORLEVEL%"'),
        ('echo [%DATE% %TIME%] Web runner exited with code %EXIT_CODE%>>"{0}"' -f $webLog),
        'exit /b %EXIT_CODE%'
    )
}
else {
    $webCommand = ('"{0}" "{1}" dev --hostname "{2}" --port {3} 1>>"{4}" 2>&1' -f $nodeExe, $nextBin, $WebHost, $WebPort, $webLog)
    $webRunnerLines = @(
        '@echo off',
        'chcp 65001 >nul',
        'set "NEXT_TELEMETRY_DISABLED=1"',
        ('set "NEXT_PUBLIC_API_BASE_URL={0}"' -f (Escape-BatchValue $apiBaseUrl)),
        ('echo [%DATE% %TIME%] Web runner starting in dev mode>>"{0}"' -f $webLog),
        ('cd /d "{0}"' -f $webAppDir),
        $webCommand,
        ('set "EXIT_CODE=%ERRORLEVEL%"'),
        ('echo [%DATE% %TIME%] Web runner exited with code %EXIT_CODE%>>"{0}"' -f $webLog),
        'exit /b %EXIT_CODE%'
    )
}
Set-Content -LiteralPath $webRunner -Value $webRunnerLines -Encoding ascii

Write-Log "Starting hidden API runner on $ApiHost`:$ApiPort"
$apiPid = New-HiddenRunnerProcess -RunnerPath $apiRunner -WorkingDirectory $apiAppDir
Set-Content -LiteralPath $apiPidFile -Value $apiPid -Encoding ascii

Write-Log "Starting hidden web runner on $WebHost`:$WebPort"
$webPid = New-HiddenRunnerProcess -RunnerPath $webRunner -WorkingDirectory $webAppDir
Set-Content -LiteralPath $webPidFile -Value $webPid -Encoding ascii

Write-Log 'OpenAD source checkout started in background.'
Write-Host "[OK] OpenAD started in background."
Write-Host "[INFO] Web: http://$PublicHost`:$WebPort"
Write-Host "[INFO] API: http://$PublicHost`:$ApiPort"
Write-Host "[INFO] Launcher log: $script:SetupLog"
Write-Host "[INFO] API log: $apiLog"
Write-Host "[INFO] Web log: $webLog"
