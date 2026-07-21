@echo off
REM Installs a Windows logon task that starts OpenAD in background.
setlocal

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%.") do set "SCRIPT_HOME=%%~fI"
set "START_SCRIPT=%SCRIPT_HOME%start-background.bat"
set "TASK_NAME=OpenAD"
set "LEGACY_TASK_NAME=PermissionProtector"

if not exist "%START_SCRIPT%" (
    echo [ERROR] start-background.bat not found: %START_SCRIPT%
    exit /b 1
)

schtasks /Delete /TN "%LEGACY_TASK_NAME%" /F >nul 2>nul
schtasks /Create /TN "%TASK_NAME%" /TR "\"%START_SCRIPT%\"" /SC ONLOGON /RL HIGHEST /F
if errorlevel 1 (
    echo [ERROR] Failed to install startup task. Try running this script as Administrator.
    exit /b 1
)

echo [OK] Startup task installed: %TASK_NAME%
echo [INFO] It will run at next user logon. To start now, run start-background.bat.
