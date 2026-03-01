# Hidden Window Fix for Windows TTS

## Issue
When TTS was running on Windows, a Command Prompt window would flash/appear for every line being spoken, creating a distracting and unprofessional user experience.

## Root Cause
The PowerShell commands were being executed without any window hiding flags, causing Windows to create visible console windows for each TTS operation.

## Solution
Implemented multiple layers of window hiding to ensure PowerShell runs completely in the background:

### 1. PowerShell Command Line Flag
Added `-WindowStyle Hidden` to the PowerShell command:
```go
cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
```

### 2. Windows System Call Attributes
Used Go's `syscall.SysProcAttr` to set window creation flags at the Win32 API level:
```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow:    true,
    CreationFlags: 0x08000000, // CREATE_NO_WINDOW
}
```

**Flags explained:**
- `HideWindow: true` - Tells Windows to hide the window at creation
- `CreationFlags: 0x08000000` - Uses the `CREATE_NO_WINDOW` flag which prevents console window allocation entirely

### 3. Import Addition
Added `syscall` package to imports:
```go
import (
    // ... other imports
    "syscall"
)
```

## Changes Made

### Files Modified

#### `app.go`
1. **Imports**: Added `"syscall"` package
2. **`speakLineWindows()` function**: 
   - Added `-WindowStyle Hidden` flag to PowerShell command
   - Added `cmd.SysProcAttr` with window hiding flags
3. **`getAvailableVoicesWindows()` function**:
   - Added same window hiding configuration for voice discovery

## Technical Details

### Why Both Methods?
Using both PowerShell flags AND syscall attributes provides defense-in-depth:
- **PowerShell flag** (-WindowStyle Hidden): Tells PowerShell itself to hide its window
- **syscall attributes** (CREATE_NO_WINDOW): Tells Windows not to allocate a console at all

This ensures compatibility across different Windows versions and configurations.

### Platform-Specific Code
This fix is Windows-only. The `syscall.SysProcAttr` struct and its fields are Windows-specific and won't affect the macOS build.

## Testing

### Before Fix
- ❌ Command Prompt window flashes on screen for each spoken line
- ❌ Distracting visual interruption
- ❌ Unprofessional appearance

### After Fix
- ✅ No visible windows during TTS
- ✅ Silent background execution
- ✅ Smooth, professional user experience
- ✅ Voice discovery also runs silently

### Test Commands
```powershell
# Build and test
wails build
.\build\bin\MDViewer.exe

# Open test_sample.md and use TTS - no console windows should appear
```

## Benefits

1. **Clean UX**: No distracting window flashing during text-to-speech
2. **Professional**: Application behaves like native software
3. **Performance**: No overhead from unnecessary window creation/destruction
4. **Consistency**: Matches the macOS behavior (which also runs silently)

## Compatibility

- ✅ Windows 10
- ✅ Windows 11
- ✅ All Windows versions with PowerShell 5.0+
- ✅ Does not affect macOS/Linux builds

## References

- Windows CREATE_NO_WINDOW flag: `0x08000000`
- Go syscall package: https://pkg.go.dev/syscall
- PowerShell -WindowStyle parameter: https://docs.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_powershell_exe

---

**Fixed**: February 28, 2026
**Issue**: Command Prompt windows appearing during TTS
**Status**: ✅ Resolved
