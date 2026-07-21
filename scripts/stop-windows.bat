@echo off
REM Stops OpenAD processes started by release scripts.
setlocal enabledelayedexpansion

if "%PERMISSION_PROTECTOR_DATA_DIR%"=="" set "PERMISSION_PROTECTOR_DATA_DIR=%APPDATA%\PermissionProtector"
set "RUN_DIR=%PERMISSION_PROTECTOR_DATA_DIR%\run"
set "API_SERVER_PID_FILE=%RUN_DIR%\api-server.pid"
set "STATIC_SERVER_PID_FILE=%RUN_DIR%\static-server.pid"
set "LEGACY_STATIC_SERVER_PID_FILE=%TEMP%\permission-protector-static-server.pid"

echo [INFO] Stopping OpenAD API...
if exist "%API_SERVER_PID_FILE%" (
    set /p API_SERVER_PID=<"%API_SERVER_PID_FILE%"
    if defined API_SERVER_PID taskkill /T /F /PID !API_SERVER_PID! >nul 2>nul
    del /q "%API_SERVER_PID_FILE%" 2>nul
)
taskkill /F /IM permission-protector-server.exe >nul 2>nul

echo [INFO] Stopping OpenAD web server...
if exist "%STATIC_SERVER_PID_FILE%" (
    set /p STATIC_SERVER_PID=<"%STATIC_SERVER_PID_FILE%"
    if defined STATIC_SERVER_PID taskkill /T /F /PID !STATIC_SERVER_PID! >nul 2>nul
    del /q "%STATIC_SERVER_PID_FILE%" 2>nul
)
if exist "%LEGACY_STATIC_SERVER_PID_FILE%" (
    set /p STATIC_SERVER_PID=<"%LEGACY_STATIC_SERVER_PID_FILE%"
    if defined STATIC_SERVER_PID taskkill /T /F /PID !STATIC_SERVER_PID! >nul 2>nul
    del /q "%LEGACY_STATIC_SERVER_PID_FILE%" 2>nul
)

REM Fallback cleanup for static web PowerShell process if the PID file was missing or stale.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$currentPid = $PID; Get-CimInstance Win32_Process | Where-Object { $_.ProcessId -ne $currentPid -and $_.Name -match '^(powershell|pwsh)(\.exe)?$' -and $_.CommandLine -like '*serve-static.ps1*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }" 2>nul
taskkill /F /IM permission-protector-web.exe >nul 2>nul

echo [OK] Stop command completed.
