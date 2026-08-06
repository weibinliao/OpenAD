@echo off
REM Starts OpenAD without opening a browser or waiting for user input.
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%.") do set "SCRIPT_HOME=%%~fI"
set "PROJECT_ROOT=%SCRIPT_HOME%"
if not exist "%PROJECT_ROOT%\OpenAD.Server.exe" set "PROJECT_ROOT=%SCRIPT_HOME%\.."
for %%I in ("%PROJECT_ROOT%") do set "PROJECT_ROOT=%%~fI"

set "START_BACKGROUND_SCRIPT=%PROJECT_ROOT%\scripts\start-background.ps1"
if "%API_HOST%"=="" set "API_HOST=127.0.0.1"
if "%API_PORT%"=="" set "API_PORT=18080"
if "%PORT%"=="" set "PORT=%API_PORT%"
if "%GIN_MODE%"=="" set "GIN_MODE=release"
if "%WEB_PORT%"=="" set "WEB_PORT=3010"
if "%WEB_HOST%"=="" if not "%PP_WEB_HOST%"=="" set "WEB_HOST=%PP_WEB_HOST%"
if "%WEB_HOST%"=="" set "WEB_HOST=127.0.0.1"
if "%PUBLIC_HOST%"=="" set "PUBLIC_HOST=localhost"
set "LAN_WEB_BIND=0"
if /I "%WEB_HOST%"=="+" set "LAN_WEB_BIND=1"
if /I "%WEB_HOST%"=="*" set "LAN_WEB_BIND=1"
if /I "%WEB_HOST%"=="0.0.0.0" set "LAN_WEB_BIND=1"
if "%LAN_WEB_BIND%"=="1" if /I "%PUBLIC_HOST%"=="localhost" (
    for /f "usebackq delims=" %%H in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "$primary = Get-NetIPConfiguration -ErrorAction SilentlyContinue | Where-Object { $_.IPv4DefaultGateway -and $_.IPv4Address } | Sort-Object InterfaceMetric | Select-Object -First 1; if ($primary) { $primary.IPv4Address[0].IPAddress } else { $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -ne '127.0.0.1' -and $_.IPAddress -notlike '169.254*' -and $_.PrefixOrigin -ne 'WellKnown' } | Sort-Object InterfaceMetric | Select-Object -First 1 -ExpandProperty IPAddress; if ($ip) { $ip } else { 'localhost' } }"`) do set "PUBLIC_HOST=%%H"
)
if "%PERMISSION_PROTECTOR_DATA_DIR%"=="" set "PERMISSION_PROTECTOR_DATA_DIR=%APPDATA%\OpenAD"
if not exist "%PERMISSION_PROTECTOR_DATA_DIR%" mkdir "%PERMISSION_PROTECTOR_DATA_DIR%"
if "%DATABASE_URL%"=="" set "DATABASE_URL=sqlite:///%PERMISSION_PROTECTOR_DATA_DIR%\OpenAD.db"
set "RUN_DIR=%PERMISSION_PROTECTOR_DATA_DIR%\run"
if not exist "%RUN_DIR%" mkdir "%RUN_DIR%"

if not exist "%PROJECT_ROOT%\OpenAD.Server.exe" (
    echo [ERROR] OpenAD API service (OpenAD.Server.exe) not found in %PROJECT_ROOT%
    exit /b 1
)

if not exist "%PROJECT_ROOT%\web\index.html" (
    echo [ERROR] Web files not found in %PROJECT_ROOT%\web
    exit /b 1
)

if not exist "%START_BACKGROUND_SCRIPT%" (
    echo [ERROR] Background launcher script not found: %START_BACKGROUND_SCRIPT%
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%START_BACKGROUND_SCRIPT%" -ProjectRoot "%PROJECT_ROOT%" -DataDir "%PERMISSION_PROTECTOR_DATA_DIR%" -ApiHost "%API_HOST%" -ApiPort "%API_PORT%" -GinMode "%GIN_MODE%" -DatabaseUrl "%DATABASE_URL%" -WebHost "%WEB_HOST%" -WebPort "%WEB_PORT%" -PublicHost "%PUBLIC_HOST%"
exit /b %ERRORLEVEL%
