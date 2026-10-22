# MDViewer

A modern, dual-pane markdown editor and viewer with native text-to-speech capabilities for macOS and Windows.

## Native macOS app

The `Mac/` folder contains a native Swift/SwiftUI build of MDViewer with the
same file-then-play TTS pipeline: line N+1 is synthesized while line N plays,
and Stop kills playback immediately. It builds with Swift Package Manager and
does not require Wails or Go:

```bash
cd Mac
bash build.sh
open build/MDViewer.app
```

See [Mac/README.md](Mac/README.md) for build details, Piper voice setup, and
keyboard shortcuts.

## Features

### 📝 MD Editor
- Raw markdown editing with syntax highlighting
- Line numbers for easy navigation
- Word count tracking
- Clean, distraction-free interface

### 👁️ MD Viewer
- Real-time markdown rendering
- Support for:
  - Headers (H1-H6)
  - Bold, italic, and inline code
  - Lists (ordered and unordered)
  - Blockquotes
  - Code blocks
  - Links
  - Horizontal rules
- Click any word to select reading start position

### 🔊 Text-to-Speech (TTS)
- Natural neural voices via bundled **Piper** engine — offline, no API key required
  - **macOS**: `AVSpeechSynthesizer` with downloaded Enhanced/Premium voices;
    bundled Piper neural TTS as fallback
  - **Windows**: Piper neural TTS (primary); Windows Speech API (SAPI) as fallback
- Multiple voice options (two bundled Piper voices: Amy and Ryan)
- Adjustable speech rate (100-250 words per minute)
- Click-to-select reading position
- Visual line highlighting during speech
- Stop/Start controls

### 📕 Convert to PDF
- One-click **Convert to PDF** button in both the Windows and macOS apps
- Converts the whole current document, including unsaved edits, to a PDF
- Opens a native Save dialog so the PDF is written to a separate file
- **Windows** renders through the installed Microsoft Edge or Google Chrome
- **macOS** renders natively with AppKit, entirely offline

## Requirements

### macOS
- macOS 10.13 or later
- Bundled Piper neural TTS engine (included in the build; ~180 MB fetch at build time)
- Native macOS speech synthesis (pre-installed; Enhanced/Premium voices downloadable)
- **Distribution builds**: Apple Developer ID certificate for code-signing/notarizing the bundled `piper` binary (Gatekeeper blocks unsigned third-party binaries)

### Windows
- Windows 10 or later
- Bundled Piper neural TTS engine (included in the build; ~180 MB fetch at build time)
- Windows Speech API (SAPI) — pre-installed with Windows (used as fallback)
- PowerShell 5.0 or later (pre-installed with Windows 10+)
- Microsoft Edge or Google Chrome (pre-installed on most systems; used for PDF export)

## Installation

### From Source

1. Install Go 1.21 or later
2. Install Wails CLI:
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
3. Clone and build:
   
   **On macOS/Linux:**
   ```bash
   git clone <repository-url>
   cd MDViewer
   ./build.sh
   ```
   
   **On Windows (PowerShell):**
   ```powershell
   git clone <repository-url>
   cd MDViewer
   build.bat
   ```

   > **Note:** The build scripts (`build.sh` / `build.bat`) automatically run `scripts/fetch-piper.sh` or `scripts/fetch-piper.ps1` on the first run to download the Piper binary and voice models (~180 MB). An internet connection is required for this one-time step.

4. The built app will be in:
   - **macOS**: `build/bin/MDViewer.app`
   - **Windows**: `build/bin/MDViewer.exe`

### Running in Development Mode

```bash
wails dev
```

### Quick Scripts

**macOS/Linux:**

Two convenient shell scripts are provided:
- **`./build.sh`** - Builds the production application
- **`./run.sh`** - Launches the built application

```bash
# Build and run
./build.sh
./run.sh
```

**Windows:**

Two convenient batch files are provided:
- **`build.bat`** - Builds the production application
- **`run.bat`** - Launches the built application

```batch
REM Build and run
build.bat
run.bat
```

Or use Wails commands directly:

```powershell
# Build
wails build

# Run in development mode
wails dev
```

## Usage

### Opening Files
1. Click the **Open** button in the toolbar
2. Select a `.md`, `.markdown`, or `.txt` file
3. The file content will load in both Editor and Viewer tabs

### Editing
1. Switch to the **MD Editor** tab
2. Type or edit your markdown content
3. Use the **Save** button to save changes

### Viewing and Reading
1. Switch to the **MD Viewer** tab to see rendered markdown
2. Click on any word in the viewer to set the reading position
3. Select a voice from the dropdown (optional)
4. Adjust the speech rate slider (optional)
5. Click **Speak** to start text-to-speech
6. Click **Stop** to halt reading

## Keyboard Shortcuts

Currently, all actions are toolbar-based. Future versions may include keyboard shortcuts.

## Technical Stack

- **Backend**: Go 1.23
- **Framework**: Wails v2.11.0
- **Frontend**: Vanilla JavaScript, HTML5, CSS3
- **Build Tool**: Vite
- **TTS**:
  - Primary: Piper neural TTS engine (bundled, offline, no API key)
  - Fallback — macOS: Native `AVSpeechSynthesizer` voices
  - Fallback — Windows: Windows Speech API (SAPI) via PowerShell
- **PDF export**: Edge or Chrome headless printing on Windows; native AppKit printing on macOS

## Project Structure

```
MDViewer/
├── app.go                  # Main Go application and TTS logic
├── pdf.go                  # HTML-to-PDF export via Edge/Chrome headless
├── frontend/
│   ├── src/
│   │   ├── main.js         # Frontend application logic
│   │   ├── app.css         # Main styles
│   │   └── style.css       # Base styles
│   ├── index.html          # HTML structure
│   └── package.json        # Frontend dependencies
├── build/                  # Build output directory
├── wails.json              # Wails configuration
└── go.mod                  # Go dependencies
```

## Design Specifications

### Color Palette
- **Primary Background**: `#1E1E1E` (Dark editor background)
- **Secondary Background**: `#252526` (Panels and toolbars)
- **Accent Color**: `#007ACC` (Buttons and highlights)
- **Text Primary**: `#D4D4D4`
- **Text Secondary**: `#808080`
- **Highlight Line**: `#2D4F1E` (Reading highlight)
- **Success Button**: `#4EC9B0` (Speak button)
- **Danger Button**: `#F14C4C` (Stop button)

### Typography
- **Editor Font**: SF Mono, Menlo, Monaco (monospace)
- **UI Font**: -apple-system, system default
- **Font Size**: 14px (editor), 13px (UI)

## Known Limitations

1. Markdown rendering is custom (not using a full parser library)
2. No collaborative editing features
3. No syntax highlighting for code blocks (planned for future)

## Troubleshooting

### TTS Not Working

**Piper (primary, both platforms):**
- Ensure the build fetch step completed successfully (run `scripts/fetch-piper.sh` / `scripts\fetch-piper.ps1` manually if needed)
- Verify that `piper/piper` (macOS) or `piper\piper.exe` (Windows) exists next to the application
- On macOS, if you see a Gatekeeper security error, re-run `build.sh` to strip the quarantine attribute, or code-sign the binary

**macOS (fallback):**
- Ensure your Mac has text-to-speech enabled in System Preferences
- Check that macOS speech works in Terminal: `say "Hello"` (or download
  Enhanced/Premium voices in Accessibility → Read & Speak)
- Try selecting a different voice from the dropdown

**Windows (fallback):**
- Ensure PowerShell is available (pre-installed on Windows 10+)
- Test SAPI in PowerShell:
  ```powershell
  Add-Type -AssemblyName System.Speech
  $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
  $synth.Speak("Hello")
  ```
- Check that you have at least one voice installed (Windows Settings > Time & Language > Speech)
- Try running the app as Administrator if TTS fails to initialize

### File Won't Open
- Ensure the file has a `.md`, `.markdown`, or `.txt` extension
- Check file permissions

### Build Errors
- Ensure Wails CLI is installed: `wails doctor`
- Check Go version: `go version` (must be 1.21+)
- Clean and rebuild: `wails build -clean`

## Future Enhancements

- [ ] Syntax highlighting for code blocks in viewer
- [ ] Export to HTML/PDF
- [ ] Recent files list
- [ ] Keyboard shortcuts
- [ ] Themes (light mode)
- [ ] Advanced markdown features (tables, footnotes)
- [ ] Word-by-word highlighting during TTS
- [ ] Pause/Resume TTS functionality
- [ ] Bookmark/annotation support

## Contributing

This is a personal/educational project. Feel free to fork and modify for your needs.

## License

[Specify your license here]

## Credits

- Built with [Wails](https://wails.io)
- Neural TTS powered by [Piper](https://github.com/rhasspy/piper) (open-source, offline)
- Voice models from [rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices) on Hugging Face
- System voice fallback: macOS `AVSpeechSynthesizer`, Windows Speech API (SAPI)
- Inspired by the need for accessible markdown reading tools

## Screenshot

![MDViewer Application](screenshot.png) 

*MDViewer showing the MD Viewer tab with text-to-speech controls and rendered markdown content*

## Contact

For questions or suggestions, contact: tazzu.far@gmail.com
