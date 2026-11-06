# MDViewer - Developer Documentation

## Architecture Overview

MDViewer is built using the Wails framework, which combines a Go backend with a modern web-based frontend. The application follows a clean separation of concerns:

```
┌─────────────────────────────────────────────┐
│              Frontend (JS/HTML/CSS)          │
│  ┌─────────────┐          ┌──────────────┐  │
│  │  MD Editor  │          │  MD Viewer   │  │
│  │   (Tab 1)   │          │   (Tab 2)    │  │
│  └─────────────┘          └──────────────┘  │
│         │                        │           │
│         └────────────┬───────────┘           │
│                      │                       │
│              ┌───────▼────────┐              │
│              │  Wails Bridge  │              │
│              └───────┬────────┘              │
└──────────────────────┼──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│           Go Backend (internal/mdviewer)               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │   File   │  │   TTS    │  │ Runtime  │  │
│  │   I/O    │  │ Engine   │  │ Dialogs  │  │
│  └──────────┘  └──────────┘  └──────────┘  │
└─────────────────────────────────────────────┘
```

## Backend (Go)

### Main Components

#### `internal/mdviewer/app.go`
The main application file containing:

**Type Definitions:**
```go
type App struct {
    ctx           context.Context  // Wails context
    currentFile   string           // Current file path
    content       string           // Current file content
    ttsProcess    *exec.Cmd        // TTS process handle
    ttsMutex      sync.Mutex       // Thread-safe TTS operations
    isSpeaking    bool             // TTS status flag
}
```

**Key Methods:**

1. **File Operations**
   - `OpenFileDialog()` - Native file picker for opening .md files
   - `SaveFileDialog(content)` - Native save dialog
   - `ReadFile(filepath)` - Read file contents
   - `SaveFile(filepath, content)` - Write file contents

2. **TTS Operations**
   - `Speak(text, startLine)` - Start TTS from a specific line
   - `SpeakWithVoice(text, startLine, voice, rate)` - TTS with custom settings
   - `Stop()` - Halt TTS immediately
   - `IsSpeaking()` - Check TTS status
   - `GetAvailableVoices()` - List system voices

### TTS Implementation

The TTS system uses macOS's native `say` command:

```go
cmd := exec.Command("say", "-v", voice, "-r", fmt.Sprintf("%d", rate), text)
```

**Thread Safety:**
- Uses `sync.Mutex` to prevent concurrent TTS operations
- Stops existing speech before starting new
- Goroutine monitors completion status

**Process Management:**
- Stores `*exec.Cmd` reference for process control
- Uses `pkill -f say` for reliable stopping
- Updates `isSpeaking` flag on completion

## Frontend (JavaScript)

### Main Components

#### `main.js`
The main frontend application logic.

**State Management:**
```javascript
let currentFile = null;        // Current file path
let content = '';              // Current content
let isSpeaking = false;        // TTS status
let selectedLineIndex = 0;     // Reading start position
let speechCheckInterval = null; // TTS status polling
```

**Key Functions:**

1. **File Operations**
   - `openBtn` event handler - Opens native dialog
   - `saveBtn` event handler - Saves via native dialog
   - Uses Wails runtime APIs (not browser file APIs)

2. **Editor Features**
   - `updateLineNumbers()` - Syncs line numbers with editor
   - Real-time word count
   - Synchronized scrolling

3. **Markdown Rendering**
   - `renderMarkdown(text)` - Parses and renders markdown
   - `processInline(text)` - Handles inline formatting
   - `makeClickable(text, lineNum)` - Wraps words for TTS selection

4. **TTS Control**
   - `startSpeechMonitoring()` - Polls speech status every 500ms
   - `highlightLine(lineNum)` - Visual feedback during reading
   - `stopSpeaking()` - Cleanup function

### Markdown Parser

Custom lightweight parser supporting:
- Headers (H1-H6)
- Lists (ordered/unordered)
- Code blocks with language tags
- Blockquotes
- Horizontal rules
- Inline formatting (bold, italic, code)
- Links

**Implementation:**
- Line-by-line parsing
- State machine for code blocks and lists
- Data attributes for line tracking (`data-line`)

### Styling (`app.css`)

**Design System:**
- Dark theme following VS Code color palette
- Monospace fonts for editor (SF Mono)
- System fonts for UI
- CSS transitions for smooth interactions
- Custom scrollbars matching theme

**Key Classes:**
- `.tab` - Tab navigation
- `.editor` - Markdown editor textarea
- `.viewer-content` - Rendered markdown container
- `.word` - Clickable word spans
- `.line-reading` - Highlighted line during TTS

## Wails Integration

### Bindings

Wails automatically generates JavaScript bindings from Go methods:

```
frontend/wailsjs/go/main/App.js
frontend/wailsjs/go/main/App.d.ts
```

**Usage in frontend:**
```javascript
import { OpenFileDialog, Speak, Stop } from '../wailsjs/go/main/App';

// Call Go functions from JS
const path = await OpenFileDialog();
await Speak(content, lineNumber);
```

### Runtime APIs

The application uses Wails runtime for native dialogs:
- `runtime.OpenFileDialog()` - File picker
- `runtime.SaveFileDialog()` - Save dialog

## Build Process

### Development
```bash
wails dev
```
- Runs Go backend
- Starts Vite dev server for frontend
- Hot reload for frontend changes
- Automatic rebuild for Go changes

### Production Build
```bash
wails build
```
1. Generates bindings
2. Installs frontend dependencies (`npm install`)
3. Compiles frontend (`vite build`)
4. Compiles Go binary
5. Packages as macOS `.app` bundle

Output: `build/bin/MDViewer.app`

## Development Workflow

### Adding a New Go Method

1. Add method to `App` struct in `internal/mdviewer/app.go`
2. Ensure method is exported (capitalized name)
3. Run `wails build` (or `wails dev`) to refresh bindings
4. Import in `main.js`
5. Call from frontend code

### Adding a New Feature

1. **Backend changes** (if needed):
   - Modify `internal/mdviewer/`
   - Run `wails build` (or `wails dev`) to refresh bindings

2. **Frontend changes**:
   - Update `main.js` for logic
   - Update `app.css` for styling
   - Update `index.html` for structure

3. **Test**:
   - Run `wails dev`
   - Test in development window
   - Check browser console for errors

4. **Build**:
   - Run `wails build`
   - Test built `.app` file

## Common Issues

### Bindings Out of Sync
**Problem:** Frontend can't find Go methods  
**Solution:** Run `wails generate module`

### TTS Not Stopping
**Problem:** Multiple TTS processes running  
**Solution:** Check `ttsMutex` implementation, ensure `pkill` is called

### Styling Not Applied
**Problem:** CSS not loading  
**Solution:** Check import order in `index.html`, rebuild frontend

### File Dialog Not Opening
**Problem:** Native dialogs fail  
**Solution:** Ensure `runtime` package is imported, check macOS permissions

## Performance Considerations

1. **Markdown Rendering**
   - Renders on demand, not continuously
   - Lightweight parser for speed
   - Consider memoization for large documents

2. **TTS Polling**
   - 500ms interval is a balance
   - Consider WebSocket for real-time status

3. **Editor Scrolling**
   - Synchronized line numbers can lag on very large files
   - Consider virtualization for 10,000+ lines

## Security Notes

1. **File System Access**
   - Goes through native dialogs (user consent required)
   - No direct file system access from frontend

2. **Command Execution**
   - Only `say` command is executed
   - Input is properly escaped
   - Process is managed with mutex

3. **XSS Prevention**
   - HTML is escaped in code blocks
   - Inline content processed through safe functions
   - Links open in `target="_blank"`

## Testing

Currently, the application has no automated tests. Consider adding:

1. **Unit Tests (Go)**
   - Test file operations
   - Test TTS command generation
   - Test voice parsing

2. **Integration Tests**
   - Test Wails bindings
   - Test file dialog flows

3. **E2E Tests**
   - Test full user workflows
   - Use Wails testing utilities

## Future Technical Improvements

1. **Markdown Parser**
   - Use `github.com/gomarkdown/markdown` library
   - Support tables, footnotes, task lists

2. **TTS Enhancement**
   - Word-by-word highlighting
   - Pause/resume functionality
   - Progress indicator

3. **State Management**
   - Consider Redux/Vuex-like pattern
   - Undo/redo support
   - Auto-save functionality

4. **Performance**
   - Virtual scrolling for large files
   - Lazy rendering for markdown
   - Worker threads for parsing

## Resources

- [Wails Documentation](https://wails.io/docs/introduction)
- [Go Documentation](https://golang.org/doc/)
- [Vite Documentation](https://vitejs.dev/)
- [macOS say Command](https://ss64.com/osx/say.html)
