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

echo Building neural TTS helper...
where gcc >nul 2>&1
if errorlevel 1 (
    echo [ERROR] gcc is required to build the neural TTS helper ^(MinGW^).
    exit /b 1
)
powershell -ExecutionPolicy Bypass -File "%~dp0scripts\fetch-neural.ps1"
if errorlevel 1 (
    echo [ERROR] Neural asset fetch failed.
    exit /b 1
)
set "NEURAL_OUT=%~dp0build\bin\neural"
if not exist "%NEURAL_OUT%" mkdir "%NEURAL_OUT%"
pushd "%~dp0tools\neural-tts"
set CGO_ENABLED=1
go build -o "%NEURAL_OUT%\neural-tts.exe" .
if errorlevel 1 (
    popd
    echo [ERROR] neural-tts build failed.
    exit /b 1
)
popd
copy /Y "%~dp0resources\neural\paradee_int8.onnx" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\paradee-config.json" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\onnxruntime.dll" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\espeak-ng.dll" "%NEURAL_OUT%\" >nul
if "%MDVIEWER_NEURAL_KOKORO%"=="1" (
    copy /Y "%~dp0resources\neural\kokoro-v1.0.int8.onnx" "%NEURAL_OUT%\" >nul
    copy /Y "%~dp0resources\neural\voices-v1.0.bin" "%NEURAL_OUT%\" >nul
)
copy /Y "%~dp0resources\neural\kokoro-config.json" "%NEURAL_OUT%\" >nul
echo Neural TTS bundled at %NEURAL_OUT%

echo.
echo OK:  %~dp0build\bin\MDViewer.exe
endlocal
exit /b 0
