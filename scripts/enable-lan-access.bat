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
echo [INFO] Default launches remain bound to 127.0.0.1. No persistent LAN binding was enabled.
echo [SECURITY WARNING] OpenAD has no product login or RBAC. Use LAN mode only on a trusted administration network.
echo [ACTION] Replace your-host-ip below, then run this one-time command in Command Prompt:
echo     set "API_HOST=0.0.0.0" ^&^& set "WEB_HOST=0.0.0.0" ^&^& set "PUBLIC_HOST=your-host-ip" ^&^& set "ALLOW_ORIGINS=http://your-host-ip:%WEB_PORT%" ^&^& set "WEBSOCKET_ALLOWED_ORIGINS=http://your-host-ip:%WEB_PORT%" ^&^& call "%~dp0start-background.bat"
echo.
echo [INFO] LAN users can then open:
echo     http://your-host-ip:%WEB_PORT%
