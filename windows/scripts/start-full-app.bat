@echo off
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..\..") do set "PROJECT_ROOT=%%~fI"
set "WEB_APP_DIR=%PROJECT_ROOT%\apps\web"
set "API_APP_DIR=%PROJECT_ROOT%\apps\backend"
set "GO_EXE=%PROJECT_ROOT%\tools\go\bin\go.exe"
set "NODE_BOOTSTRAP_SCRIPT=%PROJECT_ROOT%\scripts\setup-portable-node.ps1"
set "NODE_DIR=%PROJECT_ROOT%\tools\node"
set "NODE_EXE=%NODE_DIR%\node.exe"
set "NPM_CMD=%NODE_DIR%\npm.cmd"
set "NPM_PREFIX_JS=%NODE_DIR%\node_modules\npm\bin\npm-prefix.js"
set "NPM_CLI_JS=%NODE_DIR%\node_modules\npm\bin\npm-cli.js"
set "PORT_HELPER=%SCRIPT_DIR%ensure-dev-port.ps1"
set "GOCACHE=%PROJECT_ROOT%\.gocache"
set "GOMODCACHE=%PROJECT_ROOT%\.gomodcache"
set "GOPROXY=https://goproxy.cn,direct"
set "GOSUMDB=sum.golang.google.cn"
set "API_HOST=0.0.0.0"
set "WEB_HOST=0.0.0.0"
if "%NETWORK_ACL%"=="" set "NETWORK_ACL=loopback,private"

rem Clear known broken local proxy placeholders that block dependency downloads.
if /I "%HTTP_PROXY%"=="http://127.0.0.1:9" set "HTTP_PROXY="
if /I "%HTTPS_PROXY%"=="http://127.0.0.1:9" set "HTTPS_PROXY="
if /I "%ALL_PROXY%"=="http://127.0.0.1:9" set "ALL_PROXY="
if /I "%GIT_HTTP_PROXY%"=="http://127.0.0.1:9" set "GIT_HTTP_PROXY="
if /I "%GIT_HTTPS_PROXY%"=="http://127.0.0.1:9" set "GIT_HTTPS_PROXY="

echo === PermissionProtector ===
echo Launching Go API + Next.js Web UI...
echo Frontend source: %WEB_APP_DIR%
if exist "%PROJECT_ROOT%\apps\frontend\package.json" (
    echo NOTE: apps\frontend is a legacy placeholder. Do not start Next.js from that directory.
)
echo.

if not exist "%NODE_EXE%" (
    echo ERROR: Portable Node.js not found: %NODE_EXE%
    echo Run START.bat so the local Node.js runtime can be prepared automatically.
    pause
    exit /b 1
)

if not exist "%NPM_CMD%" (
    echo ERROR: Portable npm launcher not found: %NPM_CMD%
    echo Run START.bat so the local Node.js runtime can be prepared automatically.
    pause
    exit /b 1
)

if not exist "%PORT_HELPER%" (
    echo ERROR: Port helper script not found: %PORT_HELPER%
    pause
    exit /b 1
)

if not exist "%GO_EXE%" (
    echo ERROR: Go toolchain not found: %GO_EXE%
    echo Run scripts\setup-portable-go.ps1 first.
    pause
    exit /b 1
)

if not exist "%WEB_APP_DIR%\package.json" (
    echo ERROR: Web app not found: %WEB_APP_DIR%
    pause
    exit /b 1
)

if not exist "%API_APP_DIR%\go.mod" (
    echo ERROR: API app not found: %API_APP_DIR%
    pause
    exit /b 1
)

if not exist "%NPM_PREFIX_JS%" goto repair_node
if not exist "%NPM_CLI_JS%" goto repair_node
call "%NPM_CMD%" --version >nul 2>&1
if errorlevel 1 goto repair_node
goto node_ready

:repair_node
echo Portable npm installation is incomplete. Rebuilding portable Node.js...
if not exist "%NODE_BOOTSTRAP_SCRIPT%" (
    echo ERROR: Node bootstrap script not found: %NODE_BOOTSTRAP_SCRIPT%
    pause
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%NODE_BOOTSTRAP_SCRIPT%" -Force
if errorlevel 1 (
    echo ERROR: failed to rebuild portable Node.js.
    pause
    exit /b 1
)

if not exist "%NPM_PREFIX_JS%" (
    echo ERROR: Portable npm runtime file missing after rebuild: %NPM_PREFIX_JS%
    pause
    exit /b 1
)
if not exist "%NPM_CLI_JS%" (
    echo ERROR: Portable npm runtime file missing after rebuild: %NPM_CLI_JS%
    pause
    exit /b 1
)
call "%NPM_CMD%" --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Portable npm is still unhealthy after rebuild.
    pause
    exit /b 1
)

:node_ready

set "API_PORT=18080"
set "WEB_PORT=3010"

if "%PERMISSION_PROTECTOR_DATA_DIR%"=="" set "PERMISSION_PROTECTOR_DATA_DIR=%APPDATA%\PermissionProtector"
if not exist "%PERMISSION_PROTECTOR_DATA_DIR%" mkdir "%PERMISSION_PROTECTOR_DATA_DIR%"
if "%DATABASE_URL%"=="" set "DATABASE_URL=sqlite:///%PERMISSION_PROTECTOR_DATA_DIR%\permission-protector-dev.db"

if "%PUBLIC_HOST%"=="" (
    for /f "usebackq delims=" %%H in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "$primary = Get-NetIPConfiguration -ErrorAction SilentlyContinue | Where-Object { $_.IPv4DefaultGateway -and $_.IPv4Address } | Sort-Object InterfaceMetric | Select-Object -First 1; if ($primary) { $primary.IPv4Address[0].IPAddress } else { $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -ne '127.0.0.1' -and $_.IPAddress -notlike '169.254*' -and $_.PrefixOrigin -ne 'WellKnown' } | Sort-Object InterfaceMetric | Select-Object -First 1 -ExpandProperty IPAddress; if ($ip) { $ip } else { 'localhost' } }"`) do set "PUBLIC_HOST=%%H"
)

for /f "usebackq delims=" %%S in (`powershell -NoProfile -ExecutionPolicy Bypass -File "%PORT_HELPER%" -Role backend -Port %API_PORT% -RepoRoot "%PROJECT_ROOT%" -StopProjectOwned`) do set "API_PORT_STATE=%%S"
if /I not "%API_PORT_STATE%"=="FREE" (
    echo ERROR: Port %API_PORT% is already in use and cannot be reclaimed for this project.
    echo Details: %API_PORT_STATE%
    pause
    exit /b 1
)

for /f "usebackq delims=" %%S in (`powershell -NoProfile -ExecutionPolicy Bypass -File "%PORT_HELPER%" -Role frontend -Port %WEB_PORT% -RepoRoot "%PROJECT_ROOT%" -StopProjectOwned`) do set "WEB_PORT_STATE=%%S"
if /I not "%WEB_PORT_STATE%"=="FREE" (
    echo ERROR: Port %WEB_PORT% is already in use and cannot be reclaimed for this project.
    echo Details: %WEB_PORT_STATE%
    pause
    exit /b 1
)

if not exist "%WEB_APP_DIR%\node_modules" (
    echo Installing web dependencies...
    call "%NPM_CMD%" --prefix "%WEB_APP_DIR%" install
    if errorlevel 1 (
        echo ERROR: Failed to install web dependencies.
        pause
        exit /b 1
    )
)

if exist "%WEB_APP_DIR%\.next" (
    echo Clearing stale Next.js build cache...
    rmdir /s /q "%WEB_APP_DIR%\.next"
)

if not exist "%GOCACHE%" mkdir "%GOCACHE%"
if not exist "%GOMODCACHE%" mkdir "%GOMODCACHE%"

set "CACHE_WRITE_TEST=%GOMODCACHE%\.write_test"
> "%CACHE_WRITE_TEST%" echo ok 2>nul
if errorlevel 1 (
    echo WARNING: Project cache directory is not writable. Falling back to %%TEMP%% cache.
    set "GOCACHE=%TEMP%\permission-protector\.gocache"
    set "GOMODCACHE=%TEMP%\permission-protector\.gomodcache"
    if not exist "%GOCACHE%" mkdir "%GOCACHE%"
    if not exist "%GOMODCACHE%" mkdir "%GOMODCACHE%"
) else (
    del /q "%CACHE_WRITE_TEST%" >nul 2>&1
)

echo Downloading Go dependencies...
call "%GO_EXE%" -C "%API_APP_DIR%" mod download
if errorlevel 1 (
    echo ERROR: Failed to download Go dependencies.
    pause
    exit /b 1
)

echo API port: %API_PORT%
echo Web port: %WEB_PORT%
echo Bind host: %WEB_HOST%
echo Public URL host: %PUBLIC_HOST%
echo Network ACL: %NETWORK_ACL%
echo Data directory: %PERMISSION_PROTECTOR_DATA_DIR%
echo.

start "PermissionProtector API" cmd /k "cd /d ""%API_APP_DIR%"" && set ""GOCACHE=%GOCACHE%"" && set ""GOMODCACHE=%GOMODCACHE%"" && set ""GOPROXY=%GOPROXY%"" && set ""GOSUMDB=%GOSUMDB%"" && set ""API_HOST=%API_HOST%"" && set ""API_PORT=%API_PORT%"" && set ""PORT=%API_PORT%"" && set ""NETWORK_ACL=%NETWORK_ACL%"" && set ""PERMISSION_PROTECTOR_DATA_DIR=%PERMISSION_PROTECTOR_DATA_DIR%"" && set ""DATABASE_URL=%DATABASE_URL%"" && ""%GO_EXE%"" run ./cmd/api"
start "PermissionProtector Web" cmd /k ""%SCRIPT_DIR%run-web-dev.bat" "%WEB_APP_DIR%" "%NPM_CMD%" "%API_PORT%" "%WEB_PORT%" "%WEB_HOST%" "%PUBLIC_HOST%""

echo Waiting for API startup...
for /l %%I in (1,1,30) do (
    powershell -NoProfile -Command "try { Invoke-WebRequest -Uri 'http://localhost:%API_PORT%/health' -UseBasicParsing -TimeoutSec 2 ^| Out-Null; exit 0 } catch { exit 1 }" >nul 2>&1
    if !errorlevel! equ 0 goto api_ready
    timeout /t 1 /nobreak >nul
)

echo WARNING: API health endpoint did not respond within 30 seconds.
echo The API/Web windows are still open so you can inspect startup logs directly.
echo.
goto startup_summary

:api_ready
echo Waiting for web startup...
for /l %%I in (1,1,45) do (
    powershell -NoProfile -Command "try { Invoke-WebRequest -Uri 'http://localhost:%WEB_PORT%' -UseBasicParsing -TimeoutSec 2 ^| Out-Null; exit 0 } catch { exit 1 }" >nul 2>&1
    if !errorlevel! equ 0 goto web_ready
    timeout /t 1 /nobreak >nul
)

echo WARNING: Web UI did not respond within 45 seconds.
echo The API/Web windows are still open so you can inspect startup logs directly.
echo.
goto startup_summary

:web_ready
echo Opening browser...
start "" "http://localhost:%WEB_PORT%"

:startup_summary
echo.
echo Application windows opened.
echo - Web UI: http://localhost:%WEB_PORT%
echo - Web UI on host IP: http://%PUBLIC_HOST%:%WEB_PORT%
echo - API:    http://localhost:%API_PORT%
echo - API on host IP:    http://%PUBLIC_HOST%:%API_PORT%
echo.
echo Leave the two command windows open while using the app.
echo Close them when you want to stop the application.
echo.
pause
