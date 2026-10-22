# MDViewer — Project Tracker

## Project Overview
**Purpose:** Modern dual-pane markdown editor and viewer with native text-to-speech (TTS) capabilities.

**Features:**
- MD Editor — Raw markdown editing with syntax highlighting, line numbers, word count
- MD Viewer — Real-time markdown rendering (headers, bold, italic, code, lists, blockquotes, tables, links)
- Text-to-Speech — Native TTS:
  - **macOS:** `AVSpeechSynthesizer` (WAV) + `afplay`; Enhanced/Premium voices supported
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
| TTS Backend | macOS: AVSpeechSynthesizer/`afplay`, Windows: SAPI/OneCore via PowerShell |
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
│   ├── register.sh           # installs to /Applications, default .md handler
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
6. ~~Verify the native Mac app build on macOS~~ — verified: builds and runs on
   macOS 26.6 with Command Line Tools only

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
- **Native Mac app built and run on a real Mac (macOS 26.6).** First local
  build exposed environment issues rather than code issues:
  - **CLT-only SwiftUI macros.** With only the Command Line Tools installed,
    the macOS 27 SDK declares `@State`/`@Binding`/`@ObservedObject` as macros
    whose `SwiftUIMacros` plugin ships *with Xcode only*, so compilation failed
    with "external macro implementation type ... could not be found".
    `Mac/build.sh` now detects the missing plugin and builds against the newest
    installed SDK that still exposes SwiftUI property wrappers as structs
    (26.5 here), so `xcode-select --install` remains sufficient.
  - **Bundled Piper could not start in the app.** The release `piper` binary
    links `@rpath/libespeak-ng`/`libonnxruntime` but carries no `LC_RPATH`.
    `build.sh` now adds `@executable_path` with `install_name_tool`, so the
    dylibs next to the binary resolve inside the bundle (verified: Piper renders
    a valid WAV, RTF ~0.09).
  - **Piper fetch copied debug dSYM stubs over the real dylibs.**
    `scripts/fetch-piper.sh` copied every `*.dylib` found under the
    piper-phonemize tarball, including the DWARF companion named
    `libonnxruntime.1.14.1.dylib`; dyld then rejected it as "unloadable mach-o
    file type 10". The find now prunes `*.dSYM` bundles.
  - **Default Markdown app.** Set `LSHandlerRank` to `Owner`, dropped `.txt`
    and `public.plain-text` so MDViewer no longer claims plain-text files, and
    added `Mac/register.sh` (build → install `/Applications/MDViewer.app` →
    `lsregister` → request the association). macOS 15+ ignores
    `LSSetDefaultRoleHandlerForContentType` from a shell tool, so the app
    handles `--set-default-markdown` via
    `NSWorkspace.setDefaultApplication(at:toOpen:)`; **File → Set as Default
    Markdown App** exposes the same request. Verified with
    `NSWorkspace.urlForApplication(toOpen:)`: `net.daringfireball.markdown`
    now resolves to `com.mdviewer.mac`, and double-clicking a `.md` file opens
    it in MDViewer.
- **Voice list now really reflects the machine (and is short).** The Mac
  picker showed a handful of names, which looked like a hardcoded list:
  - **Parser bug fixed.** `say -v ?` lines like
    `Eddy (German (Germany)) de_DE  # ...` were skipped because the old regex
    required two spaces before the locale, and the fallback split on the first
    space. Only 83 of 184 installed voices survived (and labels were wrong).
    The parser now strips the `#` sample and takes the right-most token as the
    locale: all 184 voices parse. The Wails macOS path had the same bug and was
    fixed (`Name|locale`).
  - **English-only shortlist.** The native list is now filtered to English and
    narrowed to the standard, useful voices actually installed
    (`preferredEnglishNames`); novelty voices (Zarvox, Bells, Bad News...) and
    the multi-language variants are hidden. Currently: Samantha, Kathy,
    Daniel, Karen, Moira, Tessa, Rishi, Aman, Tara.
  - **Ordering.** Native macOS voices are listed first and selected by default
    (Samantha, falling back to the first installed English voice); the two
    bundled Piper voices (Amy, Ryan) sit at the bottom as a fallback. Sections
    show their counts (“macOS voices (9)”, “Bundled voices (2)”). The stored
    voice key was bumped to `selectedVoiceID.v2` so the previous bundled-voice
    selection does not stick.
  - **Windows.** The PowerShell scan now also merges a
    `golang.org/x/sys/windows/registry` fallback (SAPI 64/32-bit + OneCore,
    HKLM + HKCU), so the list still contains the machine's installed voices if
    the WinRT/SAPI projection fails. It is filtered to English, de-duplicated
    across SAPI/OneCore, capped at 12, listed before the bundled voices, and the
    first native voice is selected by default.
  - **On-demand refresh.** Added **Speech → Refresh Voices** on macOS; the
    cached list is still refreshed automatically 800 ms after launch. Verified
    the dropdown shows only macOS voices with Samantha selected and Amy/Ryan at
    the bottom. (Native rendering later moved to AVSpeechSynthesizer.)
- **Research: how to get natural speech on macOS.** Investigated the options
  beyond the `say` CLI:
  - Apple ships only **compact** voices by default; every voice installed here
    reports `AVSpeechSynthesisVoiceQuality.default` (180/180). The natural ones
    are free **Enhanced** (~100 MB) / **Premium** (~1 GB) downloads in System
    Settings → Accessibility → Read & Speak → ⓘ System voice → Manage Voices.
    There is no supported CLI/automation to download them, and Siri voices are
    not exposed to third-party apps.
  - The modern API that sees those voices (and their quality tier) is
    **AVSpeechSynthesizer**; `say -v ?` only exposes names and can lag behind.
    `say` and AVSpeech both resolve the same voice identifiers, but AVSpeech is
    the supported path for Enhanced/Premium.
  - Offline neural alternatives researched: Piper (already bundled), Kokoro-82M
    (Apache-2.0, ~82M params, very good and fast on Apple Silicon, but needs a
    Python/MLX/CoreML runtime and a ~300 MB model), XTTS/F5/Chatterbox (heavier,
    Python). Kokoro is the best future upgrade; not bundled yet.
- **Native rendering switched to AVSpeechSynthesizer.** `SpeechEngine` now
  synthesizes system voices with `AVSpeechSynthesizer.write(_:toBufferCallback:)`,
  assembles the PCM buffers into a WAV via `AVAudioFile`, and plays it with
  `afplay` — same prefetch/stop pipeline as Piper. The voice identifier from
  `AVSpeechSynthesisVoice` is passed through, so a downloaded `Ava (Premium)`
  is used exactly, not a same-named compact fallback. Stop calls
  `stopSpeaking(at: .immediate)` on inline and prefetched synthesizers (verified:
  cancellation returns in ~0.6 s and playback dies instantly). Rate 100–250 WPM
  maps to `AVSpeechUtterance.rate` 0.3–0.9.
- **Natural-voice discovery.** `VoiceCatalog` now enumerates
  `AVSpeechSynthesisVoice.speechVoices()`, labels the tier
  (`Ava (Premium) — en_US`), sorts Enhanced/Premium above compact voices,
  auto-selects the best native voice when one is downloaded, and exposes
  **Speech → Download Natural Voices…** (opens the Read & Speak pane).
  The bundled Amy/Ryan remain at the bottom as fallback; the current default is
  Amy until a natural macOS voice is installed, then the app upgrades itself.
  Verified the AVSpeech path end-to-end: WAV rendered, `afplay` played it, stop
  was immediate.

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