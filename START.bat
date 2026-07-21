@echo off
setlocal

set "ROOT_DIR=%~dp0"
set "BOOTSTRAP_SCRIPT=%ROOT_DIR%scripts\setup-portable-go.ps1"
set "NODE_BOOTSTRAP_SCRIPT=%ROOT_DIR%scripts\setup-portable-node.ps1"

echo Starting PermissionProtector source checkout mode (Go API + Next.js web).
echo This entrypoint bootstraps local tools for the repository; packaged Windows releases use start-windows.bat.
echo.

if not exist "%BOOTSTRAP_SCRIPT%" (
    echo ERROR: bootstrap script not found: %BOOTSTRAP_SCRIPT%
    pause
    exit /b 1
)

if not exist "%NODE_BOOTSTRAP_SCRIPT%" (
    echo ERROR: node bootstrap script not found: %NODE_BOOTSTRAP_SCRIPT%
    pause
    exit /b 1
)

echo [INFO] Preparing portable Go toolchain...
powershell -NoProfile -ExecutionPolicy Bypass -File "%BOOTSTRAP_SCRIPT%"
if errorlevel 1 (
    echo ERROR: failed to prepare portable Go.
    pause
    exit /b 1
)

echo [INFO] Preparing portable Node.js toolchain...
powershell -NoProfile -ExecutionPolicy Bypass -File "%NODE_BOOTSTRAP_SCRIPT%"
if errorlevel 1 (
    echo ERROR: failed to prepare portable Node.js.
    pause
    exit /b 1
)

call "%ROOT_DIR%windows\scripts\start-full-app.bat"
