# MDViewer for macOS

A native macOS version of the MDViewer dual-pane markdown editor, written in
Swift with SwiftUI and AppKit. It mirrors the Windows app: live markdown
rendering, document tabs, recent files, find, file watching, and line-by-line
text-to-speech with the same prefetch pipeline and instant Stop.

> The Windows app lives in the repository root. This folder is a self-contained
> Swift package plus a small packaging script that produces `MDViewer.app`.

## Requirements

- macOS 13 (Ventura) or newer
- Xcode Command Line Tools

```bash
xcode-select --install
```

No Xcode project is required; the app builds with Swift Package Manager.
On a machine with only the Command Line Tools (no full Xcode), `build.sh`
automatically picks an SDK whose SwiftUI still exposes `@State`/`@Binding` as
property wrappers, because the macro plugin used by the newer SDKs ships only
with Xcode.

## Quick start

```bash
cd Mac
bash build.sh
open build/MDViewer.app
```

Or use the helper script, which builds on first run:

```bash
cd Mac
bash run.sh
```

## Install and make MDViewer the default Markdown app

```bash
cd Mac
bash register.sh
```

`register.sh` builds, installs `/Applications/MDViewer.app`, registers it with
Launch Services, and asks macOS to make it the handler for
`net.daringfireball.markdown` (`.md` and `.markdown`). macOS 15+ refuses silent
association changes from a shell script, so the app performs the request
itself. You can repeat it at any time with **File → Set as Default Markdown
App**.

To undo, select another app in Finder: right-click a `.md` file → **Get Info**
→ **Open with** → choose an app → **Change All…**.

## Natural voices

There are two ways to get natural-sounding speech on macOS.

### 1. Apple's Enhanced / Premium voices (native, best option)

macOS ships only compact voices by default, which is why Samantha and friends
sound robotic. Apple offers free **Enhanced** and **Premium** voices
(~100 MB–1 GB each) that you download once:

1. System Settings → **Accessibility** → **Read & Speak**.
2. Click **ⓘ** next to **System voice** → **Manage Voices…**.
3. Open **English**, then download e.g. **Ava (Premium)**, **Zoe (Premium)**,
   **Evan (Enhanced)** or **Samantha (Enhanced)**.
4. In MDViewer press **Speech → Refresh Voices**.

The app enumerates voices with `AVSpeechSynthesizer`, so downloaded voices are
listed as `Ava (Premium) — en_US`, sorted above the compact ones, and picked
automatically (a downloaded natural voice takes priority over everything else).
The same pane is one click away via **Speech → Download Natural Voices…**.

Siri voices are intentionally not exposed to third-party apps, and macOS has no
supported way to download Enhanced/Premium voices from the command line.

### 2. Bundled Piper voices (offline neural fallback)

The app also bundles **Piper**, an offline neural engine with two English
voices (Amy, Ryan). They are listed at the bottom of the picker and used as the
fallback while no natural macOS voice is installed. To fetch/re-fetch them:

```bash
bash scripts/fetch-piper.sh
```

Then rebuild:

```bash
cd Mac
bash build.sh
```

`build.sh` detects Apple Silicon vs. Intel and bundles the matching Piper
engine (`darwin-arm64` or `darwin-amd64`) plus the voices into
`MDViewer.app/Contents/Resources/piper`. If the assets are missing the app falls
back to the system voices.

You can also point the app at an existing Piper folder at runtime:

```bash
MDVIEWER_PIPER_DIR=/path/to/piper open build/MDViewer.app
```

The folder must contain the `piper` executable and a `voices/` subdirectory.

## Features

- **MD Editor** — raw markdown editing with a live line-number gutter, word
  count, undo/redo, and a monospaced dark theme.
- **MD Viewer** — headings, paragraphs, bold/italic, inline code, fenced code
  blocks, bullet and numbered lists, blockquotes, tables, horizontal rules, and
  links. Click any block to set the reading start line.
- **Line-by-line TTS** — while one line is playing, the next line is already
  being synthesized in the background, so there is no silent gap between
  paragraphs.
- **Instant Stop** — kills the active player and any in-flight synthesizer
  immediately, and discards queued audio files.
- **Native voices** — macOS `AVSpeechSynthesizer`/`afplay` (WAV), including
  downloaded Enhanced/Premium voices, plus bundled Piper (WAV).
- **Voice controls** — the picker lists a short English-only set of the voices
  actually installed on the Mac (via `AVSpeechSynthesizer`): Samantha, Daniel, Karen, Moira,
  Tessa, Rishi and friends. Novelty voices and non-English locales are hidden.
  The default is **Samantha**; the two bundled Piper voices (Amy, Ryan) are
  kept at the bottom as a fallback. **Speech → Refresh Voices** re-scans the
  system (e.g. after installing a voice in System Settings). The rate slider
  (100–250 WPM) maps to Piper `length_scale`.
- **Document tabs** — open several files, switch between them, and close them
  with a save prompt for unsaved changes.
- **Recent files** — the ten most recent files, persisted across launches.
- **Find** — live search with match-case and whole-word options, next/previous,
  a match counter, and selection/scroll in both editor and viewer.
- **Convert to PDF** — renders the whole current document (including unsaved
  edits) to a paginated A4 PDF through a native Save dialog, with headings,
  lists, quotes, code blocks, and tables. Runs entirely offline with AppKit.
- **External change watching** — polling watcher reloads changed files without
  clobbering unsaved edits.
- **Drag and drop** — drop `.md`, `.markdown`, or `.txt` files onto the window.
- **Fast launch** — native voice enumeration is deferred until after launch and
  cached in `~/Library/Caches/MDViewer/voices.json`.

## Keyboard shortcuts

| Action | Shortcut |
|---|---|
| New tab | `⌘N` |
| Open | `⌘O` |
| Save | `⌘S` |
| Save As | `⇧⌘S` |
| Convert to PDF | `⇧⌘E` |
| Close tab | `⌘W` |
| Find | `⌘F` |
| Speak / resume from selected line | `⌘↩` |
| Stop speaking | `⌘.` |
| Editor pane | `⌘1` |
| Viewer pane | `⌘2` |

## Project layout

```
Mac/
├── Package.swift              # SwiftPM executable target (macOS 13+)
├── Info.plist                 # app bundle metadata / markdown document types
├── build.sh                   # release build + .app packaging + ad-hoc sign
├── run.sh                     # build if needed, then open the app
├── register.sh                # install to /Applications + default .md handler
└── Sources/MDViewer/
    ├── MDViewerApp.swift      # @main app, app delegate, menu commands
    ├── AppModel.swift         # documents, tabs, find, voices, speech control
    ├── Models.swift           # DocumentTab, VoiceOption, PiperLocator
    ├── MarkdownParser.swift   # block-level markdown parser
    ├── EditorView.swift       # NSTextView wrapper + line-number ruler
    ├── ViewerView.swift       # SwiftUI markdown renderer
    ├── ContentView.swift      # window chrome, toolbar, find bar, status bar
    ├── SpeechEngine.swift     # prefetch + file-then-play TTS pipeline
    ├── VoiceCatalog.swift     # Piper + AVSpeech voice discovery/quality + cache
    ├── PDFExport.swift        # markdown -> attributed string -> PDF
    ├── FileWatcher.swift      # polling external-change watcher
    └── Theme.swift            # colors and button styles
```

## How the speech pipeline works

1. The document is split into cleaned, non-empty lines.
2. The first line is synthesized to a temporary audio file.
3. While line N plays with `afplay`, line N+1 is rendered by a concurrent
   worker (Piper → WAV, AVSpeechSynthesizer → WAV).
4. When playback finishes, the prefetched file is already complete, so playback
   starts immediately.
5. Stop increments the speech session, kills the player and any synthesizer,
   clears the prefetch cache, and deletes queued files. The UI state is reset
   synchronously, so the button feels instantaneous.

## Notes

- The build is ad-hoc signed, not notarized. On first launch macOS may require
  right-click → Open, or you can clear the quarantine flag:
  `xattr -dr com.apple.quarantine build/MDViewer.app`.
- Without Piper assets, first-line latency is lower but the voices are the
  built-in macOS ones. With Piper, each process loads a ~63 MB model; the
  prefetch hides that cost between paragraphs, though the very first line can
  still take a moment.
- The renderer intentionally supports a practical subset of Markdown (the same
  set the Windows app supports) rather than every CommonMark extension.
