@echo off
REM Windows release launcher for packaged backend + static web build
setlocal enabledelayedexpansion

echo [INFO] Starting OpenAD Windows release...

REM Get script directory
set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%.") do set "SCRIPT_HOME=%%~fI"
set "PROJECT_ROOT=%SCRIPT_HOME%"
if not exist "%PROJECT_ROOT%\OpenAD.Server.exe" set "PROJECT_ROOT=%SCRIPT_HOME%\.."
for %%I in ("%PROJECT_ROOT%") do set "PROJECT_ROOT=%%~fI"
set "WEB_SERVER_EXE=%PROJECT_ROOT%\OpenAD.Web.exe"
set "STATIC_SERVER_PID_FILE=%TEMP%\OpenAD-static-server.pid"
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

REM Check if binaries exist
if not exist "%PROJECT_ROOT%\OpenAD.Server.exe" (
    echo [ERROR] OpenAD API service (OpenAD.Server.exe) not found
    echo Expected OpenAD API service location: %PROJECT_ROOT%\OpenAD.Server.exe
    echo.
    echo Please ensure you have:
    echo 1. Pre-compiled binaries in project root, OR
    echo 2. Run build script first
    echo.
    pause
    exit /b 1
)

REM Start backend server
echo [INFO] Starting backend server...
pushd "%PROJECT_ROOT%"
start /B OpenAD.Server.exe
popd

REM Wait for server
timeout /t 3 /nobreak >nul

REM Check web files
if not exist "%PROJECT_ROOT%\web\index.html" (
    echo [ERROR] Web files not found
    echo Expected: %PROJECT_ROOT%\web\index.html
    echo.
    pause
    exit /b 1
)

if not exist "%WEB_SERVER_EXE%" (
    echo [ERROR] Web server executable not found
    echo Expected: %WEB_SERVER_EXE%
    echo.
    pause
    exit /b 1
)

REM Start web server
echo [INFO] Starting web interface...
if exist "%STATIC_SERVER_PID_FILE%" del /q "%STATIC_SERVER_PID_FILE%"
start "OpenAD Web" /B "%WEB_SERVER_EXE%" -root "%PROJECT_ROOT%\web" -port "%WEB_PORT%" -host "%WEB_HOST%"
if errorlevel 1 (
    echo [ERROR] Failed to start the bundled static web server
    pause
    exit /b 1
)

echo.
echo [OK] Application started successfully!
echo [INFO] Web interface: http://%PUBLIC_HOST%:%WEB_PORT%
echo [INFO] API server: http://%PUBLIC_HOST%:%API_PORT%
echo [INFO] Web bind host: %WEB_HOST%
echo [INFO] API bind host: %API_HOST%
echo [INFO] Data directory: %PERMISSION_PROTECTOR_DATA_DIR%
start "" "http://%PUBLIC_HOST%:%WEB_PORT%"
echo.
echo Press any key to stop...
pause >nul

REM Cleanup
taskkill /F /IM OpenAD.Server.exe 2>nul
taskkill /F /IM OpenAD.Web.exe 2>nul
if exist "%STATIC_SERVER_PID_FILE%" (
    set /p STATIC_SERVER_PID=<"%STATIC_SERVER_PID_FILE%"
    if defined STATIC_SERVER_PID taskkill /F /PID !STATIC_SERVER_PID! >nul 2>nul
    del /q "%STATIC_SERVER_PID_FILE%" 2>nul
)

echo [INFO] Application stopped.
pause
