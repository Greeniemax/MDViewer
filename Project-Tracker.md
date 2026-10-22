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
- Convert to PDF — save the whole document as a separate PDF; Windows uses Edge/Chrome headless printing, macOS uses native AppKit printing

## Tech Stack
| Component | Technology |
|-----------|------------|
| Backend | Go 1.23 |
| Framework | Wails v2.11.0 |
| Frontend | Vanilla JavaScript, HTML5, CSS3 |
| Build Tool | Vite |
| TTS Backend | macOS: `say`/`afplay`, Windows: SAPI via PowerShell |
| Native macOS app | Swift 5.9, SwiftUI + AppKit, Swift Package Manager |
| PDF export | Windows: Edge/Chrome headless `--print-to-pdf`; macOS: AppKit `NSPrintOperation` |

## Current State
**Status:** Active development
- Extensive `Docs/` folder with 20 documentation files
- Recent activity (Feb 28 - Oct 22, 2026)
- Native macOS Swift port added under `Mac/` (build/run on macOS only)
- Known issues fixed: AUDIO_CUTOFF_FIX, DEADLOCK_FIX, HIDDEN_WINDOW_FIX, TTS_AND_TABLE_FIXES, LINE_BY_LINE_FIX, PKILL_BUG_FIX, WINDOWS_TTS_IMPLEMENTATION

## Project Structure
```
MDViewer/
├── app.go                    # Main Go application (TTS logic)
├── pdf.go                    # HTML -> PDF export via Edge/Chrome headless
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
├── Mac/                      # Native macOS Swift port (SwiftPM + AppKit/SwiftUI)
│   ├── Sources/MDViewer/     # app, editor, viewer, markdown parser, TTS engine
│   ├── Sources/MDViewer/PDFExport.swift  # markdown -> attributed string -> PDF
│   ├── build.sh              # builds MDViewer.app, bundles Piper, ad-hoc signs
│   └── README.md             # macOS build/run instructions
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
2. Export to HTML
3. Add themes
4. Keyboard shortcuts
5. macOS TTS optimization
6. Verify the native Mac app build on macOS (or via the macOS CI workflow)

## Activity Log

### 2026-10-22
- **TTS pipeline — next line renders while the current one plays.** Added
  `PrefetchLine` + a backend prefetch cache (`tts_pipeline.go`). The frontend
  now pre-renders the upcoming line as soon as the current one starts playing,
  so there is no silent synthesis gap between paragraphs. All engines use the
  same file-then-play pipeline: Piper writes WAV, SAPI/OneCore write WAV, and
  macOS `say` writes AIFF. Piper remains the default with native fallback.
- **Stop is now immediate.** `Stop()` no longer holds the TTS mutex while
  killing processes. It kills the active player via `taskkill /F /T /PID` on
  Windows (tracked process tree) or `Process.Kill` on macOS, kills any
  in-flight prefetch render, deletes prefetched temp files, and returns at
  once. The frontend resets the Speak/Stop UI before awaiting the backend, so
  the button responds instantly. Native SAPI/OneCore synthesis was switched to
  `SetOutputToWaveFile` / stream-to-file so playback still has a single
  trackable player process to kill.
- **Faster startup / no background voice scan at launch.** Native voice
  enumeration (PowerShell SAPI + OneCore, ~1.5s) used to start during page
  load. It is now deferred until after `window.load` + 800ms, and its result is
  cached at `%LOCALAPPDATA%\MDViewer\native_voices.json`. Cached launches return
  the dropdown instantly and refresh the cache in the background
  (`voices-updated` event updates the list without resetting the user's
  selection). This removes the PowerShell burst that competed with WebView2
  startup.
- **Verified:** real run showed Piper synthesizing line N+1 while line N's
  player was active; Stop removed the player and synth processes within one
  poll (<1s). SAPI and OneCore file-render scripts each produced valid RIFF/WAV
  output.
- **Native macOS port added (`Mac/`).** New Swift Package Manager executable
  (SwiftUI + AppKit, macOS 13+) that mirrors the Windows app: tabbed document
  editor with line-number gutter, live markdown viewer, recent files, find with
  match-case/whole-word, drag-and-drop, polling file watcher, and voice/rate
  controls. TTS uses the same file-then-play design: Piper renders WAV (or
  `say` renders AIFF) to a temp file, `afplay` plays it, and a concurrent
  render queue synthesizes line N+1 while line N plays. Stop increments the
  speech session and SIGKILLs the player/synthesizer immediately, then clears
  queued temp files. Voice enumeration is cached in
  `~/Library/Caches/MDViewer/voices.json` and refreshed 800ms after launch so
  startup stays fast.
- **Mac packaging:** `Mac/build.sh` builds a release binary, wraps it in
  `MDViewer.app`, bundles the arch-matched Piper engine
  (`darwin-arm64`/`darwin-amd64`) plus voices, and ad-hoc signs the bundle.
  Falls back to native `say` when Piper assets are absent.
  `Mac/run.sh` builds on first run and opens the app. Build/run instructions
  are in `Mac/README.md`.
- **Not compiled on this machine:** the Windows development host has no Swift
  toolchain, so the macOS target still needs a first build/run verification on
  an actual Mac (Xcode Command Line Tools). The Swift sources were written
  conservatively for Swift 5 mode to avoid strict-concurrency breakage around
  `Process`/`DispatchQueue`.
- **First-build verification path:** added
  `.github/workflows/macos.yml` so a push touching `Mac/**` compiles the Swift
  package on a macOS runner, packages `MDViewer.app`, and uploads a tarball.
  Also hardened the Mac sources for first run: CRLF files are normalized on
  open, the reused editor text view resets its undo stack/selection per tab,
  the voice picker includes the "Default Voice" option, and `build.sh` now uses
  an `ls` glob check instead of bash-4-only `compgen -G` (macOS ships bash 3.2).
- **CI compile fixes:** the first macOS run failed on two Swift errors:
  `NSRulerView.init(coder:)` is non-failable (so `EditorView`'s required
  initializer must not be `init?`), and `NSSize(width: .greatestFiniteMagnitude,
  ...)` was ambiguous between `CGFloat` and `Double`. Both are fixed.
- **Convert to PDF (Windows + macOS).** Added a "Convert to PDF" button to both
  apps. It converts the whole active document (including unsaved edits) and
  opens a native Save dialog so the PDF is written to a separate file:
  - **Windows:** new `pdf.go` binding `ExportPDF(html, suggestedName)`. The
    frontend builds a standalone print HTML document from the current markdown
    render (A4 `@page`, light print theme), and Go prints it with headless
    Edge/Chrome (`--print-to-pdf`), trying Chrome then Edge plus both headless
    modes so policy-disabled Edge installs still work. PDFs are validated by
    the `%PDF-` header before returning the path. The button disables itself
    while converting and tracks whether the document has content.
  - **macOS:** new `PDFExport.swift` renders parsed markdown blocks into a
    styled `NSAttributedString` (headings, inline bold/italic/code/links,
    lists, quotes, fenced code, padded monospaced tables, rules) and uses
    `NSPrintOperation.pdfOperation` into A4 PDF data. `AppModel.exportPDF()`
    drives `NSSavePanel` (default `<name>.pdf`, current file's folder) and
    offers Show in Finder. The button lives in the document-tabs row; the File
    menu and `⇧⌘E` also invoke it.
  - Windows: `go vet ./...` and `go test ./...` pass; `build.bat` produced
    `build/bin/MDViewer.exe`. A real Chrome headless smoke test produced a
    valid PDF. Mac compile verification is via the macOS CI workflow.
- **macOS CI is green.** The second run compiled the Swift package, packaged
  `MDViewer.app`, and uploaded the artifact. The only remaining packaging
  failure was `build.sh` treating the UTF-8 ellipsis after `$APP` as part of
  the variable name under bash 3.2's `set -u`; the echo strings now use ASCII
  `...`.
- **Windows PDF pipeline is covered by a real integration test.** Refactored
  `pdf.go` so the save-dialog flow calls a testable `printHTMLToPDF`, and added
  `pdf_test.go` (`TestValidPDF` + `TestPrintHTMLToPDF`). The integration test
  invokes the discovered Edge/Chrome headless binary, writes a PDF, and checks
  the `%PDF-` header; it passes on this machine via Chrome in ~5s. `wails build
  -s` rebuilt `build/bin/MDViewer.exe` with the final pipeline.

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
- **Fix: Open into the initial empty tab wiped the content.** Even after the
  ReadFile fix, the first Open showed nothing (DEBUG proved content read =
  55 KB but editor = 0). Root cause: `openOrFocusDocument` reuses the active
  empty "Untitled" tab and sets `t.value = content`, then `switchToDocTab(t.id)`
  calls `flushActiveTabEditor()` first — which copies the still-empty editor
  back over `t.value`, erasing it. Fix: sync `editor.value = fileContent` before
  the switch so the flush preserves it. (Recent-files dodged this by pushing a
  new tab instead of reusing the active one.)

---
*Last Updated: 2026-10-22*