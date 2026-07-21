@echo off
REM Enables LAN browser access for the packaged static web server and API ports.
setlocal

if "%WEB_PORT%"=="" set "WEB_PORT=3010"
if "%API_PORT%"=="" set "API_PORT=18080"

net session >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Please run this script as Administrator.
    exit /b 1
)

echo [INFO] Adding OpenAD firewall rules for ports %WEB_PORT%,%API_PORT%...
netsh advfirewall firewall delete rule name="PermissionProtector Web" >nul 2>&1
netsh advfirewall firewall delete rule name="PermissionProtector API" >nul 2>&1
netsh advfirewall firewall add rule name="OpenAD Web" dir=in action=allow protocol=TCP localport=%WEB_PORT% >nul
if errorlevel 1 exit /b 1
netsh advfirewall firewall add rule name="OpenAD API" dir=in action=allow protocol=TCP localport=%API_PORT% >nul
if errorlevel 1 exit /b 1

echo.
echo [OK] LAN access prerequisites applied.
echo [INFO] start-background.bat now binds the Web UI to 0.0.0.0 by default.
echo [INFO] If you need to force a specific public host, run:
echo     set PUBLIC_HOST=your-host-ip
echo     start-background.bat
echo.
echo [INFO] LAN users can then open:
echo     http://your-host-ip:%WEB_PORT%
