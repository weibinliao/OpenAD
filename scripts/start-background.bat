@echo off
REM Starts OpenAD without opening a browser or waiting for user input.
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%.") do set "SCRIPT_HOME=%%~fI"
set "PROJECT_ROOT=%SCRIPT_HOME%"
if not exist "%PROJECT_ROOT%\OpenAD.Server.exe" set "PROJECT_ROOT=%SCRIPT_HOME%\.."
for %%I in ("%PROJECT_ROOT%") do set "PROJECT_ROOT=%%~fI"

set "START_BACKGROUND_SCRIPT=%PROJECT_ROOT%\scripts\start-background.ps1"
if not "%API_HOST%"=="" if /I not "%API_HOST%"=="127.0.0.1" echo [SECURITY] Ignoring API_HOST=%API_HOST%; desktop LAN binding is disabled.
set "API_HOST=127.0.0.1"
if "%API_PORT%"=="" set "API_PORT=18080"
if "%PORT%"=="" set "PORT=%API_PORT%"
if "%GIN_MODE%"=="" set "GIN_MODE=release"
if "%WEB_PORT%"=="" set "WEB_PORT=3010"
if not "%WEB_HOST%"=="" if /I not "%WEB_HOST%"=="127.0.0.1" echo [SECURITY] Ignoring WEB_HOST=%WEB_HOST%; desktop LAN binding is disabled.
set "WEB_HOST=127.0.0.1"
set "PUBLIC_HOST=localhost"
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
