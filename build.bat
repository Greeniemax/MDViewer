@echo off
echo ========================================
echo MDViewer - Building Application
echo ========================================
echo.

REM Check if Wails is installed
where wails >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Wails CLI is not installed or not in PATH
    echo.
    echo Please install Wails CLI first:
    echo   go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo.
    pause
    exit /b 1
)

echo Building MDViewer for Windows...
echo.

wails build

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Build completed successfully!
    echo ========================================
    echo.
    echo Output: build\bin\MDViewer.exe
    echo.
    echo Run './run.bat' to launch the application
    echo.
) else (
    echo.
    echo ========================================
    echo Build failed!
    echo ========================================
    echo.
    echo Check the error messages above for details.
    echo.
)

pause
