@echo off
setlocal EnableExtensions
cd /d "%~dp0"

if not exist "wails.json" (
    echo [ERROR] Run this from the MDViewer project folder ^(wails.json not found^).
    exit /b 1
)

where wails >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Wails CLI is not on your PATH.
    echo Install it, then open a new Command Prompt:
    echo   go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo Ensure Go's bin directory is on PATH ^(often %%USERPROFILE%%\go\bin^).
    exit /b 1
)

where node >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Node.js is not on your PATH. Wails needs it to build the frontend.
    exit /b 1
)

echo Building MDViewer ^(frontend + bindings + Go embed^)...
wails build
if errorlevel 1 (
    echo [ERROR] wails build failed.
    exit /b 1
)

echo.
echo OK:  %~dp0build\bin\MDViewer.exe
endlocal
exit /b 0
