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

## Natural voices (Piper)

The macOS `say` voices work out of the box but sound robotic. To use the same
offline neural voices as the Windows app, fetch the Piper engine and voice
models once from the repository root:

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
`MDViewer.app/Contents/Resources/piper`. If the assets are missing the app still
runs and automatically falls back to `say`.

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
- **Native voices** — macOS `say`/`afplay` (AIFF) and bundled Piper (WAV).
- **Voice controls** — voice picker grouped into Natural and System voices,
  with a 100–250 WPM rate slider mapped to Piper `length_scale`.
- **Document tabs** — open several files, switch between them, and close them
  with a save prompt for unsaved changes.
- **Recent files** — the ten most recent files, persisted across launches.
- **Find** — live search with match-case and whole-word options, next/previous,
  a match counter, and selection/scroll in both editor and viewer.
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
└── Sources/MDViewer/
    ├── MDViewerApp.swift      # @main app, app delegate, menu commands
    ├── AppModel.swift         # documents, tabs, find, voices, speech control
    ├── Models.swift           # DocumentTab, VoiceOption, PiperLocator
    ├── MarkdownParser.swift   # block-level markdown parser
    ├── EditorView.swift       # NSTextView wrapper + line-number ruler
    ├── ViewerView.swift       # SwiftUI markdown renderer
    ├── ContentView.swift      # window chrome, toolbar, find bar, status bar
    ├── SpeechEngine.swift     # prefetch + file-then-play TTS pipeline
    ├── VoiceCatalog.swift     # Piper + `say -v ?` discovery and cache
    ├── FileWatcher.swift      # polling external-change watcher
    └── Theme.swift            # colors and button styles
```

## How the speech pipeline works

1. The document is split into cleaned, non-empty lines.
2. The first line is synthesized to a temporary audio file.
3. While line N plays with `afplay`, line N+1 is rendered by a concurrent
   worker (Piper → WAV, `say` → AIFF).
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
