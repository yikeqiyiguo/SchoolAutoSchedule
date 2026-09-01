@echo off
cd /d "%~dp0"
echo Killing old scheduler.exe processes...
taskkill /F /IM scheduler.exe >nul 2>&1
echo Starting scheduler on port 8899...
set SAS_PORT=8899
start "" "build\scheduler.exe"
echo.
echo Done. Browser will open http://127.0.0.1:8899 automatically.
timeout /t 5 >nul
