@echo off
REM Starts the source checkout in background without keeping API/Web console windows open.
setlocal

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..") do set "PROJECT_ROOT=%%~fI"
set "START_SCRIPT=%PROJECT_ROOT%\scripts\start-source-background.ps1"

if not exist "%START_SCRIPT%" (
    echo [ERROR] Background source launcher not found: %START_SCRIPT%
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%START_SCRIPT%" -ProjectRoot "%PROJECT_ROOT%"
exit /b %ERRORLEVEL%
