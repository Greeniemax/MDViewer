@echo off
echo ========================================
echo MDViewer - Launching Application
echo ========================================
echo.

REM Check if the executable exists
if not exist "build\bin\MDViewer.exe" (
    echo ERROR: MDViewer.exe not found!
    echo.
    echo Please build the application first using 'build.bat'
    echo.
    pause
    exit /b 1
)

echo Starting MDViewer...
echo.

REM Launch the application
start "" "build\bin\MDViewer.exe"

echo Application launched successfully!
echo.
echo Note: This window can be closed. The application is running separately.
echo.

timeout /t 3 >nul
