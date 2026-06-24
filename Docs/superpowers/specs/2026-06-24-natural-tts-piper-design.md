# Natural TTS via Bundled Piper Neural Engine — Design

**Date:** 2026-06-24
**Status:** Approved (design); pending implementation plan
**Author:** Tanaaz Cyrus + Claude

## Problem

MDViewer's text-to-speech sounds robotic. It relies entirely on the operating
system's built-in voices:

- **Windows:** SAPI desktop voices (David/Zira) — classic robotic quality. A
  prior attempt added OneCore voices (`speakLineWindowsOneCore`), which are
  better but require the user to have installed the voice packs and still are
  not truly natural.
- **macOS:** `say` + `afplay` — acceptable but dated unless premium/Siri voices
  are downloaded.

The robotic quality is inherent to these engines, not a defect in MDViewer's
code. Fixing it for real requires a better voice engine.

> Note: OpenRouter (originally floated as the AI provider) is a **text-LLM**
> gateway and does **not** offer text-to-speech audio generation, so it is not a
> viable option for this feature.

## Goal

Make the spoken voice sound natural while keeping the feature **free, offline,
and private**, with zero ongoing cost.

## Chosen approach

Bundle **[Piper](https://github.com/rhasspy/piper)**, an open-source neural TTS
engine, inside the app as the new default voice engine. Piper produces natural,
modern neural speech, runs fully offline on CPU faster than real-time, requires
no API key, and is free.

### Decisions locked during brainstorming

| Decision | Choice |
|----------|--------|
| Engine | Local neural TTS (Piper) |
| Packaging | Bundle into the app (works offline from first launch) |
| Platforms | Windows **and** macOS (Intel + Apple Silicon) |
| Old native voices | Keep as a fallback tier in the dropdown |
| Default bundled voices | Two: `en_US-amy-medium` (female) + `en_US-ryan-medium` (male) (~145 MB/build). Reducible to one (~85 MB) if size matters. |

## Architecture

`SpeakLine(text, voice, rate)` in `app.go` already dispatches by an
engine-prefixed voice selector (`sapi\t…`, `onecore\t…`). We extend the same
pattern with a new `piper\t…` engine. **TTS remains line-by-line** — the
frontend's `speakLinesSequentially` loop, highlighting, and Stop behavior are
unchanged; only the engine behind the selected voice changes.

```
SpeakLine(text, voice, rate)
   └─ parseVoice(voice) → (engine, id)
        ├─ "piper"   → speakLinePiper(text, id, rate)      ← NEW (natural, default)
        ├─ "onecore" → speakLineWindowsOneCore(...)         (Windows fallback)
        ├─ "sapi"    → speakLineWindowsSAPI(...)            (Windows fallback)
        └─ (default) → speakLineMacOS(...)                  (macOS fallback)
```

### Component: `speakLinePiper(text, voiceID, rate)`

Reuses the proven **file-based** flow (the same design that fixed the historical
audio-cutoff bug — generate a complete audio file, then play it):

1. Resolve the Piper engine path and the voice `.onnx` model path (see
   *Bundling*).
2. Run the Piper binary with the line text on **stdin**, writing a complete WAV
   to a unique temp file:
   `piper --model <voice>.onnx --length_scale <ls> --output_file <tmp>.wav`
   (text piped to stdin).
3. Play the WAV to completion:
   - **macOS:** `afplay <tmp>.wav`
   - **Windows:** reuse `runHiddenPowerShell` with
     `(New-Object System.Media.SoundPlayer '<tmp>.wav').PlaySync()` — hidden, no
     console flash, consistent with existing Windows playback.
4. Delete the temp WAV (matches existing AIFF cleanup).

The `ttsProcess` field tracks the active Piper/playback process so `Stop()` can
kill it.

### Rate control

Piper has no WPM parameter; it uses `--length_scale` (1.0 = normal pace, higher
= slower, lower = faster). Map the existing 100–250 WPM slider with:

```
length_scale = clamp(175.0 / rate, 0.5, 2.0)
```

So 175 WPM → 1.0 (natural). **No UI change** to the slider. Implemented as a
small pure function `wpmToPiperLengthScale(rate int) float64` (unit-tested).

## Bundling & build

- New repo directory `resources/piper/`:
  - `engine/windows/`, `engine/darwin-amd64/`, `engine/darwin-arm64/` — the
    Piper binary plus its required runtime files (onnxruntime shared library and
    `espeak-ng-data/`), kept in the original release layout Piper expects.
  - `voices/` — shared, platform-independent `.onnx` + `.onnx.json` voice model
    pairs (the same voice files work on every platform).
- Binaries and voice models are **large and not committed**. A fetch script
  downloads pinned versions from the official sources and `.gitignore` excludes
  them:
  - **Engine:** rhasspy/piper GitHub releases (per-OS archives:
    `piper_windows_amd64.zip`, `piper_macos_x64.tar.gz`,
    `piper_macos_aarch64.tar.gz`). Exact release tag pinned during
    implementation.
  - **Voices:** `rhasspy/piper-voices` on HuggingFace
    (`.../en/en_US/amy/medium/en_US-amy-medium.onnx` + `.onnx.json`, and the
    `ryan/medium` equivalents).
  - Scripts: `scripts/fetch-piper.sh` (macOS/Linux) and `scripts/fetch-piper.ps1`
    (Windows).
- `build.sh` / `build.bat` run the fetch (if files are absent), then place the
  correct-OS engine + the shared voices into the build output:
  - **Windows:** next to `MDViewer.exe` in a `piper/` subfolder.
  - **macOS:** inside the `.app` bundle's `Contents/Resources/piper/`.
- **Runtime path resolution:** Go locates the engine and voices relative to
  `os.Executable()` (e.g. `<exeDir>/piper/` on Windows,
  `<app>/Contents/Resources/piper/` on macOS), with a per-user app-data fallback
  directory. A `piperAvailable()` check gates whether Piper voices are offered.

## Voice list & UI

- `GetAvailableVoices()` enumerates the bundled `.onnx` voices (reading each
  paired `.onnx.json` for display name + language), emits them first tagged
  `piper\t<voiceId>\t<label>`, then appends the existing native voices
  (`sapi`/`onecore`/macOS) unchanged.
- Frontend (`frontend/src/main.js`) groups the dropdown with `<optgroup>`:
  - **Natural** (Piper voices) on top — the first Piper voice is the default
    selection.
  - **System voices** (existing native voices) below.
- This is a small change to the voice-population code only; the rest of the TTS
  frontend (`speakLinesSequentially`, highlighting, Stop, context menu) is
  untouched.

## Error handling & fallback

- If Piper is missing or fails for a line, `SpeakLine` logs the error and falls
  back to the platform-native engine for that line, so audio never silently
  stops.
- `Stop()` is extended to also terminate the `piper` process and its WAV
  playback on both platforms (Windows `taskkill` / process kill; macOS `pkill`
  for `piper` and `afplay`).
- **macOS Gatekeeper:** a bundled, unsigned Piper binary inside the `.app` is
  quarantined and blocked unless signed/notarized with the app. For distribution
  the binary must be included in code-signing/notarization; for local dev the
  build script strips the quarantine attribute
  (`xattr -dr com.apple.quarantine`). Documented in build docs.

## Testing

- **Go unit tests** (pure logic, no audio):
  - voice-selector parsing including the new `piper\t…` engine,
  - `wpmToPiperLengthScale` mapping (boundaries 100/175/250 + clamping),
  - voice-list assembly ordering (Piper first, then native).
- **Manual verification** with `test_sample.md`:
  - natural playback quality vs. old voices,
  - historical problem words ("MicroPython", "changes", "programming") — confirm
    no tail cutoff,
  - Stop mid-line,
  - native-fallback path (simulate missing Piper).

## Out of scope (YAGNI)

- In-app "download more voices" UI — only the two bundled voices for v1.
- Multi-speaker voice selection, SSML, pitch/emphasis controls.
- Whole-document (non-line-by-line) synthesis.
- Linux packaging (engine layout supports it later, not bundled now).

## Files affected

- `app.go` — add `speakLinePiper`, `wpmToPiperLengthScale`, extend
  `parseVoice`/dispatch, `GetAvailableVoices`, and `Stop`; add Piper path
  resolution.
- `app_test.go` *(new)* — unit tests for the pure logic above.
- `frontend/src/main.js` — `<optgroup>` voice grouping + default selection.
- `resources/piper/` *(new, gitignored binaries)* — engine + voices layout.
- `scripts/fetch-piper.sh`, `scripts/fetch-piper.ps1` *(new)* — fetch pinned
  engine + voices.
- `build.sh`, `build.bat` — fetch + place Piper into build output.
- `.gitignore` — exclude downloaded Piper binaries/voices.
- Docs — note Piper engine, sizes, and macOS signing requirement.
