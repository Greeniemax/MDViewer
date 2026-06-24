# Piper Neural TTS — Implementation Guide

## Overview

MDViewer bundles the open-source **Piper** neural text-to-speech engine to provide natural, human-sounding voices on both Windows and macOS. Piper runs entirely offline and requires no API key. The OS-native voices (Windows SAPI/OneCore, macOS `say`) are kept as a fallback tier in case the bundled engine is unavailable.

Piper project: https://github.com/rhasspy/piper

---

## Why Piper?

| Property | OS-native (SAPI / `say`) | Piper |
|----------|--------------------------|-------|
| Voice quality | Robotic / synthesized | Natural neural voices |
| Offline | Yes | Yes |
| API key required | No | No |
| CPU speed | Fast | ~10× faster than real-time |
| Bundled in app | No (OS-provided) | Yes |
| Customisable voices | OS-dependent | Add `.onnx` files |

---

## Voice Selector Wire Format

When the frontend sends a voice selection to the backend the selector string encodes the TTS tier and voice identity:

| Tier | Format | Example |
|------|--------|---------|
| Piper | `piper\t<modelKey>` | `piper\ten_US-amy-medium` |
| Windows SAPI | `sapi\t<id>` | `sapi\tMicrosoft David Desktop` |
| Windows OneCore | `onecore\t<id>` | `onecore\tMicrosoft Zira` |
| macOS native | `<name>` (bare) | `Samantha` |

The tab character (`\t`) acts as the field separator between tier and identity. The backend `SpeakLine` dispatcher splits on `\t` and routes accordingly.

---

## Bundled Voices

Two default voices are included in every build:

| Model key | Description | Approximate size |
|-----------|-------------|-----------------|
| `en_US-amy-medium` | American English, female | ~63 MB |
| `en_US-ryan-medium` | American English, male | ~63 MB |

These are fetched from [huggingface.co/rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices) during the build fetch step (see below).

---

## Speech Rate Mapping

The UI exposes a 100–250 WPM slider. Piper controls pace via a `--length_scale` argument (higher = slower):

```
length_scale = clamp(175 / rate, 0.5, 2.0)
```

At 175 WPM (the midpoint/natural pace) `length_scale` = 1.0. The clamp keeps the value within Piper's supported range at the slider extremes.

---

## Synthesis Strategy: File-then-Play

Piper synthesis is file-based:

1. Piper renders the text to a temporary WAV file (typically ~10× faster than real-time on CPU).
2. The WAV is played by the OS audio player (`afplay` on macOS, `mplayer`/`MediaPlayer` on Windows).

This approach guarantees complete word endings are rendered before playback begins, eliminating the tail-cutoff problem that affected the old streaming approach.

---

## Bundle Layout at Runtime

### Windows
```
<exeDir>/
└── piper/
    ├── piper.exe
    └── voices/
        ├── en_US-amy-medium.onnx
        ├── en_US-amy-medium.onnx.json
        ├── en_US-ryan-medium.onnx
        └── en_US-ryan-medium.onnx.json
```

### macOS
```
MDViewer.app/
└── Contents/
    └── Resources/
        └── piper/
            ├── piper          (binary, must be code-signed for distribution)
            └── voices/
                ├── en_US-amy-medium.onnx
                ├── en_US-amy-medium.onnx.json
                ├── en_US-ryan-medium.onnx
                └── en_US-ryan-medium.onnx.json
```

---

## Runtime Path Resolution (`resolvePiper`)

The `resolvePiper` function locates the Piper binary and voices directory at startup:

1. **`MDVIEWER_PIPER_DIR` environment override** — if set, the function looks for the binary and a `voices/` subdirectory inside that path. Useful for development and testing with a custom Piper build.
2. **Bundled path** — falls back to the platform-specific bundle path described above (relative to the executable/app-bundle).

If neither location yields a valid binary + voices directory, `resolvePiper` returns an error and the backend falls back to the OS-native voice tier automatically.

---

## Building from Source

### Fetch step (required once per machine)

The build scripts invoke the fetch step automatically. You can also run it manually:

**macOS / Linux:**
```bash
scripts/fetch-piper.sh
```

**Windows (PowerShell):**
```powershell
scripts\fetch-piper.ps1
```

The fetch script downloads the Piper binary and default voice models (~180 MB total) from:
- **Binary**: GitHub releases — `github.com/rhasspy/piper` release `2023.11.14-2`
- **Voices**: `huggingface.co/rhasspy/piper-voices`

An internet connection is required for the first run. Subsequent builds skip the download if the files are already present.

### Full build

**macOS / Linux:**
```bash
./build.sh    # runs fetch-piper.sh internally, then wails build
```

**Windows:**
```batch
build.bat     :: runs fetch-piper.ps1 internally, then wails build
```

---

## macOS Distribution: Code-Signing Requirement

macOS Gatekeeper blocks unsigned binaries downloaded from the internet. The bundled `piper` binary **must be code-signed and notarized** before shipping a distribution build, otherwise users will see a security error when the app tries to launch it.

For **local development**, `build.sh` automatically strips the quarantine attribute from the downloaded binary:
```bash
xattr -dr com.apple.quarantine resources/piper/piper
```

For **distribution builds**, you must:
1. Sign the `piper` binary with your Apple Developer ID certificate.
2. Notarize the full `.app` bundle with Apple's notarization service.

Refer to Apple's documentation on [notarizing macOS software](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution) for the full workflow.

---

## Adding a New Voice

1. Download the voice files from [huggingface.co/rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices):
   - `<key>.onnx`
   - `<key>.onnx.json`
2. Place both files in `resources/piper/voices/`.
3. Extend `scripts/fetch-piper.sh` (macOS/Linux) and `scripts/fetch-piper.ps1` (Windows) to download the new files automatically, so CI and fresh clones pick them up.
4. The voice will appear in the voice selector under the **Piper** option group on next build.

---

## Fallback Behaviour

If Piper is unavailable (binary missing, quarantine blocking execution, unsupported platform), MDViewer logs the error and silently falls back to:

- **Windows**: SAPI / OneCore voices
- **macOS**: `say` command

Users see the native voices in the selector rather than Piper voices when this occurs.

---

## Related Documentation

| File | Description |
|------|-------------|
| `Docs/WINDOWS_TTS_IMPLEMENTATION.md` | Original Windows SAPI implementation |
| `Docs/AUDIO_CUTOFF_FIX.md` | Audio tail-cutoff fix (superseded by file-then-play) |
| `Docs/LINE_BY_LINE_FIX.md` | Line-by-line TTS architecture |
| `Docs/superpowers/plans/2026-06-24-natural-tts-piper.md` | Implementation plan |

---

*Added: 2026-06-24*
