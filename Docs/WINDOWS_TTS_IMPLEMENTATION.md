# Windows TTS Support - Implementation Summary

## Overview
Added complete Windows support for Text-to-Speech functionality to MDViewer, making it a truly cross-platform markdown reader with native TTS on both macOS and Windows.

## Changes Made

### 1. Core Application (`app.go`)

#### Added OS Detection
- Imported Go's `runtime` package (aliased as `runtimePkg`) for OS detection
- Implemented cross-platform branching logic based on `runtime.GOOS`

#### New TTS Architecture
- **`SpeakLine()`** - Main entry point that detects OS and delegates to platform-specific functions
- **`speakLineMacOS()`** - Original macOS implementation using `say` and `afplay` commands
- **`speakLineWindows()`** - New Windows implementation using PowerShell and SAPI

#### Windows TTS Implementation Details
```go
func speakLineWindows(text string, voice string, rate int) error
```
- Uses Windows Speech API (SAPI) via PowerShell
- **Runs completely in the background with no visible windows**
- Uses `syscall.SysProcAttr` with `HideWindow` and `CREATE_NO_WINDOW` flags
- PowerShell invoked with `-WindowStyle Hidden` flag
- Escapes special characters for PowerShell (single quotes and backticks)
- Converts rate from WPM (100-250) to SAPI rate scale (-10 to 10)
  - 100 WPM → -5 (slower)
  - 175 WPM → 0 (normal)
  - 250 WPM → 5 (faster)
- Creates PowerShell script that:
  - Loads System.Speech assembly
  - Creates SpeechSynthesizer object
  - Sets rate and optionally selects voice
  - Speaks the text synchronously
  - Properly disposes the synthesizer

#### Enhanced Stop Functionality
- **Windows**: Uses `taskkill` to terminate PowerShell processes, plus process kill
- **macOS**: Uses `pkill` for `say` and `afplay` commands

#### Voice Discovery
- **`getAvailableVoicesWindows()`** - Queries SAPI for installed voices
- **`getAvailableVoicesMacOS()`** - Original implementation using `say -v ?`
- **`GetAvailableVoices()`** - Dispatches to appropriate platform function

### 2. Documentation Updates (`README.md`)

#### Updated Title and Description
- Changed from "macOS-only" to "macOS and Windows"

#### Enhanced Requirements Section
- Split into macOS and Windows subsections
- Added Windows 10+ requirement
- Noted SAPI and PowerShell 5.0+ (pre-installed)

#### Improved Installation Instructions
- Separate build commands for macOS/Linux vs Windows
- Windows users guided to use `wails build` directly
- Updated output paths for both platforms

#### Comprehensive Troubleshooting
- Added Windows-specific TTS troubleshooting steps
- Included PowerShell test command for SAPI verification
- Guidance on checking installed voices and running as Administrator

#### Technical Stack Updates
- Listed both TTS implementations in tech stack section
- Removed "macOS-only" from known limitations

## Technical Highlights

### Why PowerShell + SAPI?
1. **Native Integration**: SAPI is built into Windows, no external dependencies
2. **High Quality**: Uses Microsoft's native speech synthesis engine
3. **Voice Variety**: Access to all installed Windows voices (David, Zira, etc.)
4. **Synchronous Operation**: Matches the macOS implementation's blocking behavior
5. **Silent Execution**: Runs completely in the background with no visible windows or console flashing

### Rate Conversion Logic
The implementation intelligently maps the unified 100-250 WPM rate to each platform:
- **macOS**: Direct WPM value to `say -r`
- **Windows**: Converts to SAPI's -10 to 10 scale with proper clamping

### Thread Safety
- Maintained all mutex locks for thread-safe TTS operations
- Process state properly managed across platform boundaries

### Hidden Window Implementation
The Windows implementation uses multiple techniques to ensure no console windows appear:
1. **PowerShell Flags**: `-WindowStyle Hidden` prevents PowerShell window from showing
2. **SysProcAttr Settings**: 
   - `HideWindow: true` - Hides the window at the Win32 level
   - `CreationFlags: 0x08000000` - Uses `CREATE_NO_WINDOW` flag to prevent console allocation
3. **Result**: TTS runs silently in the background with no visual interruption

## Testing Performed

1. **Build Test**: Successfully built Windows executable
2. **Voice Discovery**: Confirmed voice listing works (found David and Zira)
3. **TTS Playback**: Verified audio playback through test script
4. **Application Launch**: Built application launches correctly on Windows

## Files Modified

1. `app.go` - Core TTS implementation
2. `README.md` - Documentation updates

## Files Created for Testing

1. `test_sample.md` - Sample markdown file for TTS testing
   (Kept for user testing)

## Next Steps for Users

1. Build the application: `wails build`
2. Run the executable: `build\bin\MDViewer.exe`
3. Open the test file: `test_sample.md`
4. Test TTS functionality with Windows voices
5. Try different voices from the dropdown
6. Adjust speech rate slider

## Compatibility

- ✅ Windows 10 and later
- ✅ Windows 11
- ✅ macOS 10.13 and later (original functionality preserved)
- ✅ Cross-platform voice selection
- ✅ Cross-platform rate adjustment
- ✅ Unified user experience across platforms

## Benefits

1. **Accessibility**: Windows users with dyslexia or visual impairments can now use MDViewer
2. **Cross-Platform**: Single codebase supports both major desktop platforms
3. **Native Quality**: Uses platform-native TTS engines for best quality
4. **No Dependencies**: No external TTS libraries required
5. **Maintainable**: Clean separation of platform-specific code

---

Implementation completed: February 28, 2026
