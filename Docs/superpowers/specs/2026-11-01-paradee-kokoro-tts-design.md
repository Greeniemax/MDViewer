# Paradee + Kokoro Neural TTS — Design

**Date:** 2026-11-01
**Status:** Approved design (implementation not started)
**Scope:** Windows/Wails first; native macOS app reuses the same helper in a later phase.

## 1. Summary

Add two offline neural TTS families to MDViewer's existing voice dropdown:

| Family | Model | Size | Voices | License |
|--------|-------|------|--------|---------|
| Paradee 1.2 | `paradee_int8.onnx` | 8.5 MB | 4 (`af_heart`, `am_puck`, `bf_isabella`, `bm_fable`) | Apache-2.0 |
| Kokoro-82M v1.0 | `kokoro-v1.0.int8.onnx` + `voices-v1.0.bin` | 92.4 MB + 28.2 MB | 28 English (`af_*`, `am_*`, `bf_*`, `bm_*`) | Apache-2.0 |

Both are **options**, not defaults. The current selection behavior is unchanged
(the first installed native voice is selected; Piper remains the bundled
fallback). Neither model replaces the existing pipeline: they plug into the
same file-then-play, prefetch-next-line, instant-Stop architecture used by
Piper and the OS voices.

## 2. Why one pipeline serves both

Paradee is a distilled Kokoro, so the two models share:

- the same 178-token IPA vocabulary (`n_token: 178`);
- 24 kHz mono float32 waveform output;
- ONNX Runtime CPU inference.

They differ only in the voice-conditioning input:

- **Paradee:** `voice` — int64 `[1]`, index into `config.json`'s `voices` list.
- **Kokoro:** `style` — float32 `[1,256]`, a row selected from `voices-v1.0.bin`
  by token count.

Everything else is identical:

```
text
  → espeak-ng IPA            (bundled espeak-ng.dll / espeak-ng binary)
  → misaki normalization     (port of the author's web/misaki.js rules)
  → phoneme IDs              (config.json vocab, 0-padded, ≤510 tokens)
  → ONNX Runtime (CPU)       (Paradee or Kokoro session)
  → float32 waveform @ 24 kHz
  → 16-bit PCM WAV
  → existing playAudioFile() + prefetch + Stop
```

### Research findings (validated on this machine)

- `resources/piper/engine/windows/piper/espeak-ng.dll` exports
  `espeak_Initialize`, `espeak_SetVoiceByName`, `espeak_TextToPhonemes`,
  `espeak_Synchronize`, `espeak_Terminate`.
- A C probe initialized it against the bundled `espeak-ng-data` and produced
  IPA: `Hello world` → `həlˈoʊ wˈɜːld`, `Paradee` → `pɚɹˈeɪdiː`.
- A cgo probe linked the DLL under MinGW (`-l:espeak-ng.dll`) successfully.
  The Go-side pointer loop needs the corrected `*C.char` form; the C behavior
  is confirmed.
- `espeak_TextToPhonemes` returns one clause per call and drops the clause's
  punctuation, so punctuation must be re-attached by splitting the input first
  (see §5.3).
- The bundled Piper ONNX Runtime is 1.14.1. Every current
  `github.com/yalue/onnxruntime_go` release targets a newer C API (1.15.1+),
  so Paradee/Kokoro require a bundled ORT 1.20.0 (~12 MB Windows DLL).

Sources:
- <https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2>
- <https://github.com/sahilmahendrakar/paradee>
- <https://arxiv.org/html/2610.06817v1>
- <https://github.com/thewh1teagle/kokoro-onnx>
- <https://huggingface.co/hexgrad/Kokoro-82M>

## 3. Non-goals (this design)

- Non-English Kokoro voices (`zf_*`, `jm_*`, `ef_*`, `ff_*`, `hf_*`, `if_*`, `pf_*`).
  The shared vocabulary would accept them, but they need language-specific G2P
  that espeak alone does not approximate well. Separate follow-up.
- Changing the default voice or the Piper pipeline.
- Runtime model downloads. Assets are fetched at build time.
- Persistent `--serve` helper process. Deferred behind a measurement gate (§8).

## 4. Architecture

```
MDViewer.exe (Go/Wails, pure Go)
  GetAvailableVoices()
    ├─ getPiperVoices()                 existing
    ├─ getNeuralVoices()                NEW → runs neural-tts --list-voices
    └─ native voices                    existing

  SpeakLine / PrefetchLine              existing
    └─ synthesizeToFile()
         ├─ piper\t…      → synthesizePiperFile()      existing
         ├─ paradee\t…    → synthesizeNeuralFile("paradee", …)   NEW
         ├─ kokoro\t…     → synthesizeNeuralFile("kokoro", …)    NEW
         └─ native        → SAPI / OneCore / say                 existing

  neural-tts.exe (tools/neural-tts, cgo)
    ├─ espeak-ng             text → IPA clauses
    ├─ misaki mapping        eSpeak IPA → Paradee/Kokoro alphabet
    ├─ tokenizer             vocab lookup, 0 pads, 510-token cap
    ├─ ONNX Runtime 1.20     Paradee or Kokoro session
    └─ WAV writer            24 kHz mono 16-bit PCM
```

## 5. Components

### 5.1 `tools/neural-tts/` — new Go module

Its own `go.mod` keeps cgo and ORT out of the Wails module, so the app remains
pure Go and `go test ./...` at the repo root stays compiler-free.

```
tools/neural-tts/
├── go.mod / go.sum          module + onnxruntime_go v1.14.0
├── main.go                  CLI, flags, stdin text, WAV output
├── espeak_windows.go        cgo binding to espeak-ng.dll
├── espeak_darwin.go         macOS binding (phase 3)
├── internal/misaki/         eSpeak→misaki table + tests (pure)
├── internal/phonemes/       clause split, punctuation, token ids, tests (pure)
├── internal/npz/            minimal NPY/NPZ reader for Kokoro voices (pure)
├── internal/wav/            16-bit PCM WAV writer + tests (pure)
└── internal/model/          ORT session helpers (cgo)
```

CLI:

```
neural-tts --family paradee|kokoro
           --model <onnx>
           --config <config.json>
           [--voices <voices-v1.0.bin>]     kokoro only
           --voice <name>
           --speed <float>                 1.0 = normal
           --lang en-us|en-gb
           --espeak-data <dir>
           --out <wav>
           [--list-voices]

# text is read from stdin
```

- `--list-voices` prints `family<TAB>voice<TAB>label` lines and exits without
  loading the model or the ORT environment. Kokoro voices come from the NPZ
  keys (English prefixes only); Paradee voices from `config.json`.
- Exit code non-zero + message on stderr for any failure. The app treats that
  as "engine failed" and falls back.

### 5.2 `internal/tts` (existing package)

Add pure helpers:

- `NeuralVoiceID(selector) (family, voice string, ok bool)` for the prefixes
  `paradee\t` and `kokoro\t`.
- `NeuralSpeed(rate int) float64` = `rate / 175`, clamped to `[0.5, 2.0]`
  (Kokoro's own accepted range, and the same 175 WPM reference Piper uses).

### 5.3 Phoneme pipeline (pure package)

1. **Clause split:** split on `, . ; : ! ? — …` keeping the delimiter; drop
   empty clauses.
2. **espeak:** for each clause, call `espeak_SetVoiceByName(lang)` once, then
   loop `espeak_TextToPhonemes(&ptr, textmode=1 /*UTF-8*/,
   phonememode=0x02 /*IPA*/)` until it returns NULL.
3. **Punctuation re-attachment:** append the delimiter that ended each clause
   to its phoneme string. This restores the pause tokens the model was trained
   with (`; : , . ! ? — …` are all in the vocab).
4. **misaki mapping:** apply the ordered `E2M` replacement table ported from
   `web/misaki.js`, plus its British/American branches. Attribution comment
   retained (Apache-2.0).
5. **Token IDs:** `ids = [0] + [vocab[r] for r in phonemes if r in vocab] +
   [0]`, truncated to 510 phonemes before the trailing `0`. Unknown symbols
   are dropped, matching the reference implementation.
6. **Chunking:** if a clause exceeds 510 phonemes, cut at the last space that
   fits (hard cut if none), matching Paradee's `_chunks`.

### 5.4 Model adapters

| | Paradee | Kokoro |
|---|---|---|
| Token input | `input_ids` int64 `[1,T]` | `input_ids` (or `tokens`) int64 `[1,T]` |
| Speed | `speed` float32 `[1]` | `speed` float32 `[1]` |
| Voice | `voice` int64 `[1]` = index in config `voices` | `style` float32 `[1,256]` from NPZ row |
| Style row | n/a | `voice[min(len(ids), rows) - 1]` |
| Output | `waveform` float32 `[1,S]` | `waveform` float32 `[1,S]` |

Session input names are read from the loaded graph, so an export that names the
token input `tokens` still works. Exact padding for Kokoro (0 pads vs none)
and the resulting style row are confirmed by listening test in the spike;
Paradee padding is fixed by its reference implementation.

Sample rate is 24000 for both. Waveform samples are clipped to `[-1, 1]`,
scaled by 32767, and written as little-endian int16 mono PCM.

### 5.5 `tts_neural.go` — MDViewer integration

- `resolveNeural()` locates the helper + assets:
  1. `MDVIEWER_NEURAL_DIR` env override;
  2. `<exeDir>/neural`;
  3. macOS later: `<exeDir>/../Resources/neural`.

  Always required: `neural-tts.exe`, `onnxruntime.dll`, `espeak-ng.dll`, and
  an espeak data directory. Each family is independent:
  - Paradee: `paradee_int8.onnx`, `paradee-config.json`.
  - Kokoro: `kokoro-v1.0.int8.onnx`, `voices-v1.0.bin`, `kokoro-config.json`.

  A family whose files are missing is simply not listed; the other still works.

- `getNeuralVoices()` runs `neural-tts --list-voices` once per present family
  per app run and caches the result in memory. Returns none for a family whose
  files are missing. Never blocks startup: `GetAvailableVoices` is already
  called 800 ms after `load`.

- `synthesizeNeuralFile(family, voice, text, rate, entry)`:
  - builds the temp WAV path;
  - starts `neural-tts` with `stdin = text`, `hideProcessWindow`, and
    `cmd.Dir = helperDir`;
  - registers the process through `runTracked` so Stop kills the tree;
  - passes `--espeak-data` resolved from `neural/../piper/espeak-ng-data`
    (or `neural/espeak-ng-data` if present);
  - returns the WAV path on success.

### 5.6 `tts_pipeline.go` dispatch

After the Piper branch, add a neural branch with the same fallback semantics:

```go
if family, voiceID, ok := tts.NeuralVoiceID(voice); ok {
    path, err := a.synthesizeNeuralFile(family, voiceID, text, rate, entry)
    if err == nil { return path, nil }
    if stopped { return "", err }
    // fall through to the native voice, exactly like Piper
}
```

### 5.7 Frontend (`frontend/src/main.js`)

The current dropdown (after the 2026-11-01 native-voice update) lists
"Native voices (n)" first and "Bundled voices (n)" (Piper) second, and selects
the first native voice by default. The new groups are **appended after the
existing two**, so nothing about the current order or default changes:

```
Native voices (n)      existing, default selection
Bundled voices (n)     existing, Piper
Paradee (4)            new
Kokoro (28)            new
```

- Selector recognition: `tabParts[0] === 'paradee' || tabParts[0] === 'kokoro'`,
  value = `<family>\t<voice>`, label = `tabParts[2]`.
- Default selection logic is unchanged: the existing `firstNative` /
  `firstBundled` fallback keeps choosing today's default. Paradee/Kokoro are
  never auto-selected.
- Labels are produced by the helper:
  - Paradee: `Paradee · Heart — en-US`, `Paradee · Puck — en-US`,
    `Paradee · Isabella — en-GB`, `Paradee · Fable — en-GB`.
  - Kokoro: prettified voice keys, e.g. `Kokoro · Heart — en-US`,
    `Kokoro · Bella — en-US`, `Kokoro · George — en-GB`.

## 6. Assets, download, and build

### 6.1 Files

| File | Source | Size |
|---|---|---|
| `paradee_int8.onnx` | `hf.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/onnx/paradee_int8.onnx` | 8.5 MB |
| `paradee-config.json` | same repo `config.json` | ~4 KB |
| `kokoro-v1.0.int8.onnx` | `github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.int8.onnx` | 92,361,271 B |
| `voices-v1.0.bin` | same release `voices-v1.0.bin` | 28,214,398 B |
| `kokoro-config.json` | `kokoro-onnx/src/kokoro_onnx/config.json` (vocab) | small |
| `onnxruntime.dll` | `github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-win-x64-1.20.0.zip` | ~12 MB |

The two `*-config.json` files are committed (text, needed for the vocabulary).
The ONNX files, voice pack, and DLL are gitignored and fetched by script.

### 6.2 `scripts/fetch-neural.ps1`

Mirrors `scripts/fetch-piper.ps1`:

- downloads each file to `resources/neural/` with retry;
- verifies a pinned SHA-256 for every download;
- skips files already present and matching;
- extracts `onnxruntime.dll` from the ORT zip;
- copies `espeak-ng.dll` from the existing Piper Windows bundle (or errors if
  Piper has not been fetched yet).

Add `resources/neural/` to `.gitignore`.

### 6.3 `build.bat`

After the existing Piper bundling step:

1. Verify `gcc` is on PATH; print a clear install hint if not (MinGW is
   required for cgo).
2. Run `scripts\fetch-neural.ps1`.
3. Build the helper:
   `pushd tools\neural-tts && go build -o ..\..\build\bin\neural\neural-tts.exe .`
4. Copy to `build\bin\neural\`: the two ONNX files, `voices-v1.0.bin`, the two
   configs, `onnxruntime.dll`, and `espeak-ng.dll`.
5. `espeak-ng-data` is not copied; the app passes Piper's data directory at
   runtime. If Piper assets are absent, neural voices are hidden.

## 7. Error handling and Stop

- **Missing assets:** `getNeuralVoices` returns none; the dropdown is
  unchanged. No errors surfaced to the user.
- **Helper failure or non-zero exit:** fall back to the native default voice
  for that line, same as Piper. A user Stop is not a failure and does not
  trigger fallback.
- **Stop:** the helper process is stored on the prefetch entry. `Stop` calls
  `killProcessTree` (Windows `taskkill /F /T`), which also ends any espeak
  child. The macOS sweep later adds `neural-tts` to the `pkill` list.
- **Timeouts:** none imposed; the existing Stop is the escape hatch.
- **Model mismatch:** if a graph lacks the expected inputs, the helper exits
  with a descriptive error and the app falls back.

## 8. Performance and the measurement gate

Per-line helper invocation costs: process start + ORT environment init +
model load + inference.

- Paradee (8.5 MB) is expected to be fast (well under the length of most
  lines).
- Kokoro (92 MB int8) is the risk: cold start may approach a second on slower
  disks, though the OS page cache makes repeat loads cheaper.

The spike measures cold and warm per-line time. The requirement is that
rendering line N+1 completes within the playback time of line N, because
prefetch starts as soon as N begins playing.

- If satisfied for typical lines, ship one-shot process mode.
- If short lines (headings) gap audibly, add a persistent `--serve` mode
  (JSON-lines request/response, one loaded session per family) as a follow-up.
  That mode will kill and restart the helper on Stop so Stop stays instant.

## 9. Testing

### Unit (pure packages, no assets needed)

- `internal/misaki`: table-driven cases mirroring the rules and examples in
  `web/misaki.js`, including the British-before-general ordering, syllabic
  consonants, final `əl`, `ɐ` → `ə`, and the American/British branches.
- `internal/phonemes`: clause splitting and punctuation re-attachment; vocab
  lookup; unknown-symbol dropping; leading/trailing `0` pads; the 510 cap and
  space-aware cutting.
- `internal/npz`: parse a synthetic NPZ header and row slice.
- `internal/wav`: header fields (RIFF/WAVE, 24 kHz, mono, 16-bit), sample
  clipping, byte order.
- `internal/tts`: selector parsing and speed mapping.

### Integration (skipped when assets are absent)

- `neural-tts --family paradee …` and `--family kokoro …` render a fixed
  sentence; assert a valid RIFF/WAVE header, 24 kHz mono 16-bit, duration > 0.
- `--list-voices` returns exactly 4 Paradee and 28 Kokoro English voices.
- Timing report printed for the spike.

### Manual

- Listen to both families at 175 WPM against Piper `amy` for naturalness.
- Verify Stop cuts audio and kills the helper immediately.
- Verify prefetch produces no gap between consecutive lines.
- Verify fallback by temporarily renaming the helper.

## 10. Phased delivery

| Phase | Deliverable | Gate |
|---|---|---|
| 0 | Spike: ORT 1.20 + Paradee session in a scratch helper; fixed Go espeak loop; audible sample; timing numbers | Go/no-go |
| 1 | `tools/neural-tts` + Paradee end-to-end in the Windows app, fetch/build scripts, tests, docs | App plays Paradee; tests pass |
| 2 | Kokoro model + style pack + 28 voices + labels | Same, with all 28 listed |
| 3 | macOS: CI builds the helper, `Mac/build.sh` bundles `Resources/neural`, Swift gains `.paradee`/`.kokoro` kinds and `renderNeural` (rebased on the current AVSpeech-based `SpeechEngine` and rewritten `VoiceCatalog`) | macOS CI green |

Phase 3 is out of scope until phases 1–2 are verified.

## 11. Risks and mitigations

| Risk | Mitigation |
|---|---|
| ORT / Go binding version coupling | Pin `onnxruntime_go` v1.14.0 + ORT 1.20.0; both bundled together; spike verifies session creation |
| cgo on Windows requires MinGW | `build.bat` checks `gcc` and explains the fix; release builds use this machine's MinGW |
| cgo pointer loop for espeak | C path validated; Go loop corrected with a `*C.char` pointer; covered by an integration test |
| espeak→misaki accuracy | Author measures ~2% WER with this conversion; accepted, verified by listening |
| Kokoro cold start | Measurement gate in §8; `--serve` fallback designed |
| Kokoro style-row indexing | Follow reference `min(len(tokens), rows)-1`; verify in spike |
| 121 MB of existing Piper voices + ~121 MB neural assets | Build-time fetch, gitignored; users with limited disk can skip `fetch-neural` and simply won't see the voices |
| Upstream voice-catalog churn | The 2026-11-01 upstream change made native voices primary and rewrote the Mac voice catalog; the neural groups are appended without touching default selection, and phase 3 rebases on the current files |

## 12. Licensing and attribution

- Paradee: Apache-2.0 (model and code).
- Kokoro-82M: Apache-2.0.
- `kokoro-onnx` export/voices: Apache-2.0.
- `web/misaki.js` mapping rules: Apache-2.0; the Go port keeps an attribution
  header naming the source repo and file.
- espeak-ng: GPL; already distributed as part of the Piper bundle, so the
  Windows app's existing distribution practice is unchanged.
- Docs: add `Docs/NEURAL_TTS.md` describing engines, assets, licenses, build
  steps, and the fallback behavior; update `README.md`'s voice section.

## 13. Definition of done (phases 1–2)

- [ ] Paradee and Kokoro English voices appear in the dropdown only when
      assets are present, in their own groups.
- [ ] Selecting either voice reads lines with prefetch and instant Stop.
- [ ] Any failure falls back to the native voice; a Stop never falls back.
- [ ] `go vet ./...` and `go test ./...` pass at the repo root (no cgo needed).
- [ ] `tools/neural-tts` unit and integration tests pass on this machine.
- [ ] `build.bat` produces a runnable `build/bin/MDViewer.exe` with bundled
      assets.
- [ ] `Docs/NEURAL_TTS.md`, README, and `Project-Tracker.md` updated.
