@echo off
REM OpenAD - Windows Release Build Script
REM Version: 0.1.0

setlocal
set "ROOT_DIR=%~dp0.."
set "GO_EXE=%ROOT_DIR%\tools\go\bin\go.exe"
set "NPM_CMD=%ROOT_DIR%\tools\node\npm.cmd"
set "RELEASE_VERSION=0.1.0"
set "RELEASE_DIR=%ROOT_DIR%\dist\OpenAD-Windows-v%RELEASE_VERSION%"

if not exist "%GO_EXE%" (
    echo Preparing portable Go...
    powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%\scripts\setup-portable-go.ps1"
    if errorlevel 1 exit /b 1
)

if not exist "%NPM_CMD%" (
    echo Preparing portable Node.js...
    powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%\scripts\setup-portable-node.ps1"
    if errorlevel 1 exit /b 1
)

echo [INFO] Starting production build...

if not exist "%ROOT_DIR%\dist" mkdir "%ROOT_DIR%\dist"

REM Build backend
echo [INFO] Building backend...
pushd "%ROOT_DIR%\apps\backend"
"%GO_EXE%" mod download
if errorlevel 1 exit /b 1
"%GO_EXE%" test .\...
if errorlevel 1 exit /b 1
"%GO_EXE%" build -o "%ROOT_DIR%\dist\permission-protector-server.exe" .\cmd\api
if errorlevel 1 exit /b 1
"%GO_EXE%" build -o "%ROOT_DIR%\dist\permission-protector-cli.exe" .\cmd\cli
if errorlevel 1 exit /b 1
"%GO_EXE%" build -o "%ROOT_DIR%\dist\permission-protector-web.exe" .\cmd\webserver
if errorlevel 1 exit /b 1
popd

REM Build frontend
echo [INFO] Building frontend...
pushd "%ROOT_DIR%\apps\web"
call "%NPM_CMD%" ci
if errorlevel 1 exit /b 1
call "%NPM_CMD%" run build:static
if errorlevel 1 exit /b 1
if exist "%ROOT_DIR%\dist\web" rmdir /s /q "%ROOT_DIR%\dist\web"
xcopy /E /I /Y out "%ROOT_DIR%\dist\web"
if errorlevel 1 exit /b 1
popd

REM Create release package
echo [INFO] Creating release package...
if exist "%RELEASE_DIR%" rmdir /s /q "%RELEASE_DIR%"
mkdir "%RELEASE_DIR%"
if errorlevel 1 exit /b 1
mkdir "%RELEASE_DIR%\scripts"
if errorlevel 1 exit /b 1

REM Copy binaries
copy "%ROOT_DIR%\dist\permission-protector-*.exe" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
xcopy /E /I /Y "%ROOT_DIR%\dist\web" "%RELEASE_DIR%\web"
if errorlevel 1 exit /b 1

REM Copy documentation
copy "%ROOT_DIR%\docs\USER_MANUAL.md" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\README.md" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\RELEASE_NOTES.md" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\WINDOWS_DEPLOYMENT.md" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\INSTALL_WINDOWS.md" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\START_HERE_INTERNAL.md" "%RELEASE_DIR%\START_HERE_INTERNAL.md"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\PROJECT_STRUCTURE.md" "%RELEASE_DIR%\PROJECT_STRUCTURE.md"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\RELEASE_MANIFEST.md" "%RELEASE_DIR%\RELEASE_MANIFEST.md"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\PERMISSION_EXPOSURE_ENGINE.md" "%RELEASE_DIR%\PERMISSION_EXPOSURE_ENGINE.md"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\docs\FILE_ACCESS_ACTIVITY.md" "%RELEASE_DIR%\FILE_ACCESS_ACTIVITY.md"
if errorlevel 1 exit /b 1

REM Copy release scripts
copy "%ROOT_DIR%\scripts\start-windows.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\start-background.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\stop-windows.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\backup-data.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\install-startup-task.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\uninstall-startup-task.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\enable-lan-access.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\verify-install.bat" "%RELEASE_DIR%\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\serve-static.ps1" "%RELEASE_DIR%\scripts\"
if errorlevel 1 exit /b 1
copy "%ROOT_DIR%\scripts\start-background.ps1" "%RELEASE_DIR%\scripts\"
if errorlevel 1 exit /b 1

echo [OK] Build completed successfully!
echo [INFO] Release package: %RELEASE_DIR%\
