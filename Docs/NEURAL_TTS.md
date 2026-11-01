# Neural TTS (Paradee and Kokoro)

MDViewer can read documents with offline neural voices in addition to the OS
voices and Piper.

| Family | Model | Size | Voices | Default |
|---|---|---|---|---|
| Paradee 1.2 | `paradee_int8.onnx` | 8.5 MB | 4 | listed |
| Kokoro-82M | `kokoro-v1.0.int8.onnx` + `voices-v1.0.bin` | 121 MB | 28 English | opt-in |

## How it works

```
text -> espeak-ng IPA -> misaki normalization -> token ids
     -> ONNX Runtime 1.20 -> 24 kHz WAV -> existing playback
```

MDViewer starts one `neural-tts.exe --serve` helper per family and keeps it
alive for the reading session. Lines are sent as newline-delimited JSON on
stdin; each carries text, voice, speed, language, and an output WAV path. The
helper replies with `{"ok":true}` or an error, so the model is loaded once
instead of once per line. The app still pre-renders line N+1 while line N
plays, and Stop kills the helper process tree.

**Why persistent:** loading the Kokoro session per spawn cost several seconds
on its own. Preloading removes that, but Kokoro's inference is still slow on
modest CPUs (~6–21 s per line on a 4-core i5-11300H). Paradee renders in about
0.1–0.2 s per line after a ~2 s warmup, so Paradee is the default neural
option and Kokoro is hidden unless explicitly enabled.

## Build

1. `scripts/fetch-piper.ps1` (espeak-ng DLL + data)
2. `scripts/fetch-neural.ps1` (Paradee model + ONNX Runtime 1.20)
3. `build.bat`

MinGW (`gcc`) is required for cgo. Without the neural assets the neural voices
simply do not appear in the dropdown.

To include Kokoro as well, run `scripts/fetch-neural.ps1 -All` (or set
`MDVIEWER_NEURAL_KOKORO=1`) before `build.bat`. Then set the same environment
variable when launching MDViewer so its voices are listed.

## Manual test

Select a "Paradee (4)" voice in the dropdown, press Speak, and verify prefetch,
Stop, and fallback. To test fallback, rename `build\bin\neural\neural-tts.exe`,
restart, and confirm the neural group disappears while native voices still
speak.

## Licenses

- Paradee: Apache-2.0 (model and code, sahilmahendrakar/paradee).
- Kokoro-82M: Apache-2.0 (hexgrad/Kokoro-82M).
- kokoro-onnx export and voices: Apache-2.0 (thewh1teagle/kokoro-onnx).
- misaki mapping rules: Apache-2.0; ported from `web/misaki.js`.
- espeak-ng: GPL, bundled by the existing Piper engine.
- ONNX Runtime 1.20.0: MIT.
