# Windows Batch Files Guide

## Overview

For Windows users, MDViewer now includes two convenient batch files that simplify building and running the application.

## Batch Files

### `build.bat`

Builds the MDViewer application for Windows.

**Features:**
- Checks if Wails CLI is installed
- Builds the production application using `wails build`
- Shows clear success/failure messages
- Displays the output path
- Provides helpful error messages if Wails is not installed

**Usage:**
```batch
build.bat
```

**What it does:**
1. Verifies Wails CLI is in your PATH
2. Runs `wails build` command
3. Reports build status
4. Shows location of built executable: `build\bin\MDViewer.exe`

**First-time setup:**
If you see an error about Wails not being installed, run:
```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### `run.bat`

Launches the built MDViewer application.

**Features:**
- Checks if the executable exists
- Launches the application in a separate window
- Provides helpful error if app hasn't been built yet
- Auto-closes the launcher window after 3 seconds

**Usage:**
```batch
run.bat
```

**What it does:**
1. Checks if `build\bin\MDViewer.exe` exists
2. Launches the application using `start` command
3. Displays success message
4. Auto-closes after 3 seconds

**Note:** If the executable doesn't exist, you'll see a message telling you to run `build.bat` first.

## Typical Workflow

### First Time Setup
```batch
REM 1. Clone the repository
git clone <repository-url>
cd MDViewer

REM 2. Build the application
build.bat

REM 3. Run the application
run.bat
```

### Regular Usage
```batch
REM Just run the application
run.bat
```

### After Making Changes
```batch
REM Rebuild and run
build.bat
run.bat
```

## Comparison with Shell Scripts

| Platform | Build Script | Run Script |
|----------|-------------|------------|
| **Windows** | `build.bat` | `run.bat` |
| **macOS/Linux** | `./build.sh` | `./run.sh` |

The batch files provide the same convenience for Windows users that the shell scripts provide for macOS/Linux users.

## Troubleshooting

### "Wails is not installed" Error
**Problem:** The `build.bat` script can't find the Wails CLI.

**Solution:**
1. Install Wails CLI:
   ```powershell
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
2. Ensure your Go bin directory is in your PATH
3. Restart your terminal and try again

### "MDViewer.exe not found" Error
**Problem:** The `run.bat` script can't find the executable.

**Solution:**
1. Run `build.bat` first to build the application
2. Check that `build\bin\MDViewer.exe` exists

### Build Fails
**Problem:** The build process encounters errors.

**Solution:**
1. Ensure Go 1.21+ is installed: `go version`
2. Check Wails installation: `wails doctor`
3. Try cleaning and rebuilding: `wails build -clean`
4. Check the error messages in the build output

## Advanced Usage

### Development Mode
For development with hot reload, use Wails directly:
```powershell
wails dev
```

### Clean Build
To force a clean rebuild:
```powershell
wails build -clean
```

### Custom Build Options
You can modify `build.bat` to add custom build flags. For example:
```batch
REM Add after "wails build"
wails build -clean -upx -upxflags "-9"
```

## Tips

1. **Create Desktop Shortcut:** Right-click `run.bat` → "Send to" → "Desktop (create shortcut)"
2. **Pin to Start:** Create a shortcut to `run.bat` and pin it to Start Menu
3. **Quick Access:** Add the MDViewer folder to your Quick Access in File Explorer

## Integration with README

These batch files are documented in the main README.md under the "Quick Scripts" section, providing parity with the macOS/Linux shell scripts.

---

Created: February 28, 2026
