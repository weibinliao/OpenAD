@echo off
setlocal

title FSA Frontend

set "WEB_APP_DIR=%~1"
set "NPM_CMD=%~2"
set "API_PORT=%~3"
set "WEB_PORT=%~4"
set "WEB_HOST=%~5"
set "PUBLIC_HOST=%~6"
set "PORT_HELPER=%~dp0ensure-dev-port.ps1"
for %%I in ("%~dp0..\..") do set "PROJECT_ROOT=%%~fI"

if "%WEB_APP_DIR%"=="" exit /b 1
if "%NPM_CMD%"=="" exit /b 1
if "%API_PORT%"=="" exit /b 1
if "%WEB_PORT%"=="" set "WEB_PORT=3010"
if "%WEB_HOST%"=="" set "WEB_HOST=127.0.0.1"
if "%PUBLIC_HOST%"=="" set "PUBLIC_HOST=localhost"
if not exist "%PORT_HELPER%" (
    echo ERROR: port helper script not found: %PORT_HELPER%
    exit /b 1
)

call "%NPM_CMD%" --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: portable npm is not healthy: %NPM_CMD%
    exit /b 1
)

for /f "usebackq delims=" %%S in (`powershell -NoProfile -ExecutionPolicy Bypass -File "%PORT_HELPER%" -Role frontend -Port %WEB_PORT% -RepoRoot "%PROJECT_ROOT%" -StopProjectOwned`) do set "WEB_PORT_STATE=%%S"

if /I not "%WEB_PORT_STATE%"=="FREE" (
    echo ERROR: Port %WEB_PORT% is already in use and cannot be reclaimed for this project.
    echo Details: %WEB_PORT_STATE%
    exit /b 1
)

cd /d "%WEB_APP_DIR%"
set "NEXT_PUBLIC_API_BASE_URL=http://%PUBLIC_HOST%:%API_PORT%"
call "%NPM_CMD%" run dev -- --hostname %WEB_HOST% --port %WEB_PORT%
