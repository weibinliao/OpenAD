@echo off
setlocal

net session >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Please run this script as Administrator.
    exit /b 1
)

echo [INFO] Removing OpenAD LAN firewall rules...
for %%R in ("OpenAD Web" "OpenAD API" "PermissionProtector Web" "PermissionProtector API") do (
    netsh advfirewall firewall delete rule name=%%R >nul 2>&1
)

echo [OK] OpenAD LAN firewall rules removed. The desktop launcher now uses loopback only.
