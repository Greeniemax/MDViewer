#!/usr/bin/env bash
# Downloads the pinned Piper engine (per OS) and default voices into resources/.
set -euo pipefail

PIPER_VERSION="2023.11.14-2"
PHONEMIZE_VERSION="2023.11.14-4"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENG="$ROOT/resources/piper/engine"
VOICES="$ROOT/resources/piper/voices"
REL="https://github.com/rhasspy/piper/releases/download/$PIPER_VERSION"
PP_REL="https://github.com/rhasspy/piper-phonemize/releases/download/$PHONEMIZE_VERSION"
HF="https://huggingface.co/rhasspy/piper-voices/resolve/main"
mkdir -p "$ENG" "$VOICES"

dl() { curl -fL --retry 3 -o "$1" "$2"; }

fetch_engine_tar() { # target asset
  local target="$1" asset="$2"
  if [ -e "$ENG/$target/piper/piper" ]; then echo "engine $target present"; return; fi
  local tmp; tmp="$(mktemp -d)"
  dl "$tmp/p.tgz" "$REL/$asset"
  mkdir -p "$ENG/$target"
  tar -xzf "$tmp/p.tgz" -C "$ENG/$target"   # extracts a top-level piper/ dir
  rm -rf "$tmp"
}

fetch_engine_zip() { # target asset
  local target="$1" asset="$2"
  if [ -e "$ENG/$target/piper/piper.exe" ]; then echo "engine $target present"; return; fi
  if ! command -v unzip >/dev/null 2>&1; then
    echo "skip $target (unzip not available)"; return; fi
  local tmp; tmp="$(mktemp -d)"
  dl "$tmp/p.zip" "$REL/$asset"
  mkdir -p "$ENG/$target"
  unzip -o "$tmp/p.zip" -d "$ENG/$target" >/dev/null   # extracts a top-level piper/ dir
  rm -rf "$tmp"
}

fetch_voice() { # modelKey hf-subpath
  local key="$1" sub="$2"
  if [ -e "$VOICES/$key.onnx" ] && [ -e "$VOICES/$key.onnx.json" ]; then echo "voice $key present"; return; fi
  dl "$VOICES/$key.onnx"      "$HF/$sub/$key.onnx?download=true"
  dl "$VOICES/$key.onnx.json" "$HF/$sub/$key.onnx.json?download=true"
}

fetch_macos_dylibs() { # target asset
  local target="$1" asset="$2"
  local dest="$ENG/$target/piper"
  if [ ! -d "$dest" ]; then echo "engine $target not present; skipping dylibs"; return; fi
  if [ -e "$dest/libonnxruntime.1.14.1.dylib" ]; then echo "dylibs $target present"; return; fi
  local tmp; tmp="$(mktemp -d)"
  dl "$tmp/pp.tgz" "$PP_REL/$asset"
  mkdir -p "$tmp/pp"
  tar -xzf "$tmp/pp.tgz" -C "$tmp/pp"
  # Copy every dylib next to the piper binary so @rpath/... resolves at launch.
  # Skip .dSYM bundles: their DWARF companions are named like the dylibs and
  # would overwrite the real libraries with debug-only Mach-O files.
  find "$tmp/pp" -name '*.dSYM' -prune -o -name '*.dylib' -exec cp -f {} "$dest/" \;
  rm -rf "$tmp"
}

fetch_engine_tar "darwin-amd64" "piper_macos_x64.tar.gz"
fetch_engine_tar "darwin-arm64" "piper_macos_aarch64.tar.gz"
fetch_macos_dylibs "darwin-amd64" "piper-phonemize_macos_x64.tar.gz"
fetch_macos_dylibs "darwin-arm64" "piper-phonemize_macos_aarch64.tar.gz"
fetch_engine_zip "windows"      "piper_windows_amd64.zip"

fetch_voice "en_US-amy-medium"  "en/en_US/amy/medium"
fetch_voice "en_US-ryan-medium" "en/en_US/ryan/medium"

echo "Piper assets ready in resources/piper/"
