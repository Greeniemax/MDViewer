# Bundled Piper TTS assets

Contents here are downloaded by `scripts/fetch-piper.sh` / `.ps1` and are
**not** committed (see `.gitignore`).

Layout:
- `engine/<target>/piper/` — Piper binary + libs + `espeak-ng-data/`
  (`<target>` ∈ `windows`, `darwin-amd64`, `darwin-arm64`)
- `voices/<modelKey>.onnx` (+ `.onnx.json`) — neural voice models (shared across platforms)

The build scripts copy the correct-OS engine and the voices into the build
output so MDViewer can speak with natural voices offline.
