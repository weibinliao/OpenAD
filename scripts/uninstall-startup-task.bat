@echo off
REM Removes the OpenAD Windows logon task and its legacy compatibility task.
setlocal

set "TASK_NAME=OpenAD"
set "LEGACY_TASK_NAME=PermissionProtector"
set "REMOVED=0"

schtasks /Delete /TN "%TASK_NAME%" /F >nul 2>nul
if not errorlevel 1 set "REMOVED=1"
schtasks /Delete /TN "%LEGACY_TASK_NAME%" /F >nul 2>nul
if not errorlevel 1 set "REMOVED=1"

if "%REMOVED%"=="0" (
    echo [WARN] Startup task was not removed. It may not exist, or you may need Administrator rights.
    exit /b 1
)

echo [OK] OpenAD startup task removed.
