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
pushd "%~dp0frontend"

REM node_modules may be absent (fresh clone) or installed for another OS
REM (e.g. via WSL on this same folder), which lacks the Windows vite.cmd shim
REM and the win32 esbuild/rollup binaries. Detect that and (re)install.
if exist "node_modules" if not exist "node_modules\.bin\vite.cmd" (
    echo Detected non-Windows node_modules; removing it for a clean Windows install...
    rmdir /s /q node_modules
)
if not exist "node_modules\.bin\vite.cmd" (
    echo Installing frontend dependencies...
    call npm.cmd install
    if errorlevel 1 (
        popd
        echo [ERROR] npm install failed.
        exit /b 1
    )
)

call npm.cmd run build
if errorlevel 1 (
    popd
    echo [ERROR] Frontend build failed.
    echo If this persists, delete frontend\node_modules and re-run this script.
    exit /b 1
)
popd

REM Wails invokes npm.exe internally, which can fail on some Windows setups; frontend is already built.
wails build -s
if errorlevel 1 (
    echo [ERROR] wails build failed.
    exit /b 1
)

echo Bundling Piper TTS engine + voices...
powershell -ExecutionPolicy Bypass -File "%~dp0scripts\fetch-piper.ps1"
if errorlevel 1 (
    echo [ERROR] Piper fetch failed.
    exit /b 1
)
set "PIPER_OUT=%~dp0build\bin\piper"
if not exist "%PIPER_OUT%\voices" mkdir "%PIPER_OUT%\voices"
xcopy /E /I /Y "%~dp0resources\piper\engine\windows\piper" "%PIPER_OUT%" >nul
copy /Y "%~dp0resources\piper\voices\*.onnx" "%PIPER_OUT%\voices\" >nul
copy /Y "%~dp0resources\piper\voices\*.onnx.json" "%PIPER_OUT%\voices\" >nul
echo Piper bundled at %PIPER_OUT%

echo.
echo OK:  %~dp0build\bin\MDViewer.exe
endlocal
exit /b 0
