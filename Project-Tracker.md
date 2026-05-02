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

---
*Last Updated: 2026-04-18*