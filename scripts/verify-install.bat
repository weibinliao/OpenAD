@echo off
REM Verifies that the packaged OpenAD release can start and serve Web/API/CLI.
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%.") do set "PROJECT_ROOT=%%~fI"
if "%API_PORT%"=="" set "API_PORT=18080"
if "%WEB_PORT%"=="" set "WEB_PORT=3010"
set "API_HEALTH_URL=http://localhost:%API_PORT%/health"
set "WEB_URL=http://localhost:%WEB_PORT%"
set "STARTED_BY_VERIFY=0"

echo [INFO] Verifying OpenAD package in:
echo        %PROJECT_ROOT%
echo.

set "MISSING_REQUIRED_FILE=0"
for %%F in (
    "%PROJECT_ROOT%\permission-protector-server.exe"
    "%PROJECT_ROOT%\permission-protector-cli.exe"
    "%PROJECT_ROOT%\permission-protector-web.exe"
    "%PROJECT_ROOT%\web\index.html"
    "%PROJECT_ROOT%\scripts\start-background.ps1"
    "%PROJECT_ROOT%\start-background.bat"
    "%PROJECT_ROOT%\stop-windows.bat"
    "%PROJECT_ROOT%\backup-data.bat"
    "%PROJECT_ROOT%\INSTALL_WINDOWS.md"
) do (
    if not exist "%%~fF" (
        echo [ERROR] Missing required file: %%~fF
        set "MISSING_REQUIRED_FILE=1"
    )
)
if "%MISSING_REQUIRED_FILE%"=="1" exit /b 1

echo [OK] Required package files exist.

powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $r = Invoke-WebRequest -Uri '%API_HEALTH_URL%' -UseBasicParsing -TimeoutSec 2; if ($r.StatusCode -eq 200) { exit 0 } else { exit 1 } } catch { exit 1 }"
if errorlevel 1 (
    echo [INFO] API is not running. Starting package in background for verification...
    call "%PROJECT_ROOT%\start-background.bat"
    if errorlevel 1 exit /b 1
    set "STARTED_BY_VERIFY=1"
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Sleep -Seconds 5"
) else (
    echo [INFO] API already running; verification will not stop existing services.
)

echo [INFO] Checking API health: %API_HEALTH_URL%
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $r = Invoke-WebRequest -Uri '%API_HEALTH_URL%' -UseBasicParsing -TimeoutSec 8; $json = $r.Content | ConvertFrom-Json; if ($r.StatusCode -eq 200 -and $json.service -eq 'openad' -and $json.database -eq $true -and $json.status -eq 'healthy') { Write-Host $r.Content; exit 0 } Write-Host $r.Content; exit 1 } catch { Write-Host $_.Exception.Message; exit 1 }"
if errorlevel 1 goto :fail

echo [INFO] Checking Web UI: %WEB_URL%
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $r = Invoke-WebRequest -Uri '%WEB_URL%' -UseBasicParsing -TimeoutSec 8; if ($r.StatusCode -eq 200 -and $r.Content -match 'OpenAD') { exit 0 } exit 1 } catch { Write-Host $_.Exception.Message; exit 1 }"
if errorlevel 1 goto :fail

echo [INFO] Checking CLI...
"%PROJECT_ROOT%\permission-protector-cli.exe" --help >nul
if errorlevel 1 goto :fail

if "%STARTED_BY_VERIFY%"=="1" (
    call "%PROJECT_ROOT%\stop-windows.bat" >nul
)

echo.
echo [OK] OpenAD package verification passed.
echo [INFO] Start with start-windows.bat or start-background.bat.
exit /b 0

:fail
if "%STARTED_BY_VERIFY%"=="1" (
    call "%PROJECT_ROOT%\stop-windows.bat" >nul
)
echo.
echo [ERROR] OpenAD package verification failed.
exit /b 1
