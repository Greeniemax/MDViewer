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

MDViewer starts one `neural-tts --serve` helper per family and keeps it
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

### Windows

1. `scripts/fetch-piper.ps1` (espeak-ng DLL + data)
2. `scripts/fetch-neural.ps1` (Paradee model + ONNX Runtime 1.20)
3. `build.bat`

MinGW (`gcc`) is required for cgo. Without the neural assets the neural voices
simply do not appear in the dropdown.

To include Kokoro as well, run `scripts/fetch-neural.ps1 -All` (or set
`MDVIEWER_NEURAL_KOKORO=1`) before `build.bat`. Then set the same environment
variable when launching MDViewer so its voices are listed.

### macOS

1. `bash scripts/fetch-piper.sh` (Piper engine, espeak-ng dylib + data)
2. `bash scripts/fetch-neural.sh` (Paradee model + ONNX Runtime 1.20)
3. `bash Mac/build.sh`

`Mac/build.sh` compiles the helper with cgo and bundles it, the model, ONNX
Runtime, and the espeak dylibs into `MDViewer.app/Contents/Resources/neural`.
The 18 MB espeak-ng data set is shared with the Piper bundle through a symlink
instead of being copied twice.

**Architecture:** the upstream Piper macOS release contains x86_64 binaries even
in its `aarch64` tarball, and the helper loads the same `libespeak-ng.dylib`, so
on Apple Silicon the helper is built for x86_64 and runs under Rosetta. This
matches how the app already runs Piper; on Intel Macs everything is native.
`fetch-neural.sh` picks the matching ONNX Runtime build automatically, so a
future native Piper bundle switches the helper to arm64 with no code change.

To include Kokoro, run `bash scripts/fetch-neural.sh --all` and launch the app
with `MDVIEWER_NEURAL_KOKORO=1`.

## Manual test

**Windows:** select a "Paradee (4)" voice in the dropdown, press Speak, and
verify prefetch, Stop, and fallback. To test fallback, rename
`build\bin\neural\neural-tts.exe`, restart, and confirm the neural group
disappears while native voices still speak.

**macOS:** select a voice from the "Neural voices (4)" section, press Speak, and
check that exactly one `neural-tts --serve` process stays alive while reading
(`pgrep -fl neural-tts`) and disappears immediately on Stop. The helper can be
smoke-tested on its own:

```bash
cd Mac/build/MDViewer.app/Contents/Resources/neural
./neural-tts --list-voices --family paradee --config paradee-config.json
printf '%s\n' '{"text":"Hello.","voice":"af_heart","speed":1,"lang":"en-us","out":"/tmp/p.wav"}' \
  | ./neural-tts --serve --family paradee --model paradee_int8.onnx \
      --config paradee-config.json --ort libonnxruntime.dylib \
      --espeak-dll libespeak-ng.1.dylib --espeak-data .
afplay /tmp/p.wav
```

Run the helper's unit tests with
`cd tools/neural-tts && CGO_ENABLED=1 GOARCH=amd64 go test ./...`
(use the architecture of the bundled espeak dylib; on Intel, `GOARCH=amd64` is
the host default).

## Licenses

- Paradee: Apache-2.0 (model and code, sahilmahendrakar/paradee).
- Kokoro-82M: Apache-2.0 (hexgrad/Kokoro-82M).
- kokoro-onnx export and voices: Apache-2.0 (thewh1teagle/kokoro-onnx).
- misaki mapping rules: Apache-2.0; ported from `web/misaki.js`.
- espeak-ng: GPL, bundled by the existing Piper engine.
- ONNX Runtime 1.20.0: MIT.
