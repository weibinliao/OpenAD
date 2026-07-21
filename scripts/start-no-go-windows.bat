@echo off
setlocal

set "PROJECT_ROOT=%~dp0.."
echo Bootstrapping the source checkout via START.bat...
echo For packaged Windows releases, use start-windows.bat from the extracted release folder.
call "%PROJECT_ROOT%\START.bat"
