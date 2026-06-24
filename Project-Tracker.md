# MDViewer — Project Tracker

## Project Overview
**Purpose:** Modern dual-pane markdown editor and viewer with native text-to-speech (TTS) capabilities.

**Features:**
- MD Editor — Raw markdown editing with syntax highlighting, line numbers, word count
- MD Viewer — Real-time markdown rendering (headers, bold, italic, code, lists, blockquotes, tables, links)
- Text-to-Speech — Native TTS:
  - **macOS:** `say` command via temp AIFF files and `afplay`
  - **Windows:** Windows Speech API (SAPI) via PowerShell
- Click-to-Select — Click word in viewer to set reading position
- Voice Controls — Multiple voices with adjustable rate (100-250 WPM)
- Visual Highlighting — Line highlighting during TTS with animated pulse

## Tech Stack
| Component | Technology |
|-----------|------------|
| Backend | Go 1.23 |
| Framework | Wails v2.11.0 |
| Frontend | Vanilla JavaScript, HTML5, CSS3 |
| Build Tool | Vite |
| TTS Backend | macOS: `say`/`afplay`, Windows: SAPI via PowerShell |

## Current State
**Status:** Active development
- Extensive `Docs/` folder with 20 documentation files
- Recent activity (Feb 28 - Apr 18, 2026)
- Known issues fixed: AUDIO_CUTOFF_FIX, DEADLOCK_FIX, HIDDEN_WINDOW_FIX, TTS_AND_TABLE_FIXES, LINE_BY_LINE_FIX, PKILL_BUG_FIX, WINDOWS_TTS_IMPLEMENTATION

## Project Structure
```
MDViewer/
├── app.go                    # Main Go application (TTS logic)
├── go.mod / go.sum           # Dependencies
├── wails.json                # Wails configuration
├── frontend/
│   ├── src/
│   │   ├── main.js           # Frontend logic (565 lines)
│   │   ├── app.css          # Main styles
│   │   └── style.css        # Base styles
│   ├── index.html
│   ├── dist/                 # Built frontend assets
│   └── package.json
├── Docs/                     # 20 development documentation files
├── build/                    # Build output
└── README.md
```

## Key Documentation
| File | Description |
|------|-------------|
| `Docs/CHANGELOG.md` | Version history |
| `Docs/DEVELOPMENT.md` | Development guide |
| `Docs/SPEC.md` | Specifications |
| `Docs/HOW_IT_WORKS.md` | Technical overview |
| `Docs/WINDOWS_TTS_IMPLEMENTATION.md` | Windows TTS details |

## Next Steps
1. Add syntax highlighting for code blocks
2. Export to HTML/PDF
3. Add themes
4. Keyboard shortcuts
5. macOS TTS optimization

## Activity Log

### 2026-06-24
- **Natural TTS (Piper) — design approved.** The robotic voice is inherent to
  the OS engines (Windows SAPI/OneCore, macOS `say`). Decided to bundle the
  open-source **Piper** neural TTS engine (free, offline, no API key) as the new
  default voice for both Windows and macOS, keeping the native voices as a
  fallback tier. (OpenRouter was ruled out — it's a text-LLM gateway with no
  TTS.) Design doc: `Docs/superpowers/specs/2026-06-24-natural-tts-piper-design.md`.
- **Natural TTS (Piper) — implemented.** Bundled the Piper neural TTS engine for
  both Windows (`piper\piper.exe`) and macOS (`Contents/Resources/piper/piper`).
  Default voices: `en_US-amy-medium` (female) and `en_US-ryan-medium` (male).
  Rate slider (100–250 WPM) maps to Piper `--length_scale`. Synthesis uses
  file-then-play (full WAV rendered before playback) to preserve complete word
  endings. OS-native voices (SAPI/OneCore on Windows, `say` on macOS) kept as
  automatic fallback. Line-by-line TTS architecture preserved unchanged.
  Plan: `Docs/superpowers/plans/2026-06-24-natural-tts-piper.md`.
- **build.bat — auto-install frontend deps.** Windows build failed with
  "'vite' is not recognized" because `node_modules` had been installed under
  WSL/Linux (same folder) and lacked the Windows `vite.cmd` shim + win32
  esbuild/rollup binaries. `build.bat` now detects a non-Windows/missing
  `node_modules` and runs `npm install` (removing the foreign install first)
  before `npm run build`.
- **Fix: "Open" loaded an empty document.** The Open-file handler set the path
  but the editor stayed blank. Root cause: `openBtn` fetched content via a
  separate `GetContent()` call (backend-state) that returned empty, while the
  path came from `OpenFileDialog()`'s return — so the tab/status bar populated
  but content didn't. Fixed by reading content directly with
  `ReadFile(filePath)` (the same robust pattern the Recent-files path already
  uses). Frontend-only change in `frontend/src/main.js`.

---
*Last Updated: 2026-06-24*