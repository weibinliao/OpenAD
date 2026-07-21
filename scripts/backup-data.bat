@echo off
REM Backs up the local OpenAD SQLite database.
setlocal enabledelayedexpansion

if "%PERMISSION_PROTECTOR_DATA_DIR%"=="" set "PERMISSION_PROTECTOR_DATA_DIR=%APPDATA%\PermissionProtector"
set "DB_FILE=%PERMISSION_PROTECTOR_DATA_DIR%\permission-protector.db"
set "BACKUP_DIR=%~1"
if "%BACKUP_DIR%"=="" set "BACKUP_DIR=%PERMISSION_PROTECTOR_DATA_DIR%\backups"
if not exist "%BACKUP_DIR%" mkdir "%BACKUP_DIR%"

if not exist "%DB_FILE%" (
    echo [ERROR] Database file not found: %DB_FILE%
    echo Start OpenAD once before running backup.
    exit /b 1
)

for /f %%I in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd-HHmmss"') do set "STAMP=%%I"
set "BACKUP_STEM=%BACKUP_DIR%\permission-protector-%STAMP%"
set "BACKUP_FILE=%BACKUP_STEM%.db"

copy /Y "%DB_FILE%" "%BACKUP_FILE%" >nul
if errorlevel 1 (
    echo [ERROR] Backup failed.
    exit /b 1
)

if exist "%DB_FILE%-wal" copy /Y "%DB_FILE%-wal" "%BACKUP_STEM%.db-wal" >nul
if exist "%DB_FILE%-shm" copy /Y "%DB_FILE%-shm" "%BACKUP_STEM%.db-shm" >nul

echo [OK] Backup created: %BACKUP_FILE%
echo [INFO] If OpenAD was running, matching SQLite WAL/SHM files were copied when present.
