#!/usr/bin/env bash
# Downloads the pinned Paradee model, ONNX Runtime 1.20.0, and links the
# Piper-bundled espeak-ng assets for the macOS neural TTS helper.
#
# Kokoro is opt-in: pass --all (or set MDVIEWER_NEURAL_KOKORO=1). It is hidden
# by default because it renders far below real time on most Macs.
#
# Note: the upstream Piper macOS releases ship x86_64 binaries even in the
# "aarch64" tarball, so on Apple Silicon the helper is built for the same
# architecture as the bundled espeak-ng dylib (x86_64, running under Rosetta).
# Everything then shares one architecture. If a native Piper bundle is used,
# the script picks arm64 automatically.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/resources/neural"
if [ "$(uname -m)" = "x86_64" ]; then
    PIPER_ENGINE="$ROOT/resources/piper/engine/darwin-amd64/piper"
else
    PIPER_ENGINE="$ROOT/resources/piper/engine/darwin-arm64/piper"
fi
mkdir -p "$DEST"

if [ ! -d "$PIPER_ENGINE" ]; then
    echo "missing $PIPER_ENGINE; run scripts/fetch-piper.sh first" >&2
    exit 1
fi

ALL=0
[ "${1:-}" = "--all" ] && ALL=1
[ "${MDVIEWER_NEURAL_KOKORO:-}" = "1" ] && ALL=1

sha256() { shasum -a 256 "$1" | awk '{print $1}'; }

fetch_pinned() { # url out sha
    local url="$1" out="$2" sha="$3"
    if [ -f "$out" ] && [ "$(sha256 "$out")" = "$sha" ]; then
        echo "present: $(basename "$out")"
        return
    fi
    rm -f "$out"
    curl -fL --retry 3 --progress-bar -o "$out" "$url"
    local got
    got="$(sha256 "$out")"
    if [ "$got" != "$sha" ]; then
        rm -f "$out"
        echo "checksum mismatch for $(basename "$out"): $got" >&2
        exit 1
    fi
}

fetch_pinned \
    "https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/onnx/paradee_int8.onnx" \
    "$DEST/paradee_int8.onnx" \
    "a6989796692ac173597e15d405c971989202d5c73cfad65f22e90bd7b6c97da1"

if [ "$ALL" = "1" ]; then
    fetch_pinned \
        "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.int8.onnx" \
        "$DEST/kokoro-v1.0.int8.onnx" \
        "6e742170d309016e5891a994e1ce1559c702a2ccd0075e67ef7157974f6406cb"
    fetch_pinned \
        "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin" \
        "$DEST/voices-v1.0.bin" \
        "bca610b8308e8d99f32e6fe4197e7ec01679264efed0cac9140fe9c29f1fbf7d"
fi

# espeak-ng comes from the Piper bundle. Link (not copy) it so the 12 MB data
# set and the dylib are stored once; the packaging step mirrors these links.
ESPEAK_LIB=""
for lib in libespeak-ng.1.dylib libespeak-ng.dylib; do
    if [ -e "$PIPER_ENGINE/$lib" ]; then
        ESPEAK_LIB="$PIPER_ENGINE/$lib"
        ln -sfn "$ESPEAK_LIB" "$DEST/$lib"
    fi
done
if [ -z "$ESPEAK_LIB" ]; then
    echo "libespeak-ng dylib missing from $PIPER_ENGINE" >&2
    exit 1
fi
ln -sfn "$PIPER_ENGINE/espeak-ng-data" "$DEST/espeak-ng-data"

# The helper must match the espeak dylib's architecture, and ONNX Runtime must
# match the helper's.
if file -b "$ESPEAK_LIB" | grep -q "arm64"; then
    ORT_ARCH="arm64"
else
    ORT_ARCH="x86_64"
fi

ORT_LIB="$DEST/libonnxruntime.1.20.0.dylib"
ORT_NAME="onnxruntime-osx-$ORT_ARCH-1.20.0.tgz"
if [ ! -f "$ORT_LIB" ] || ! file -b "$ORT_LIB" | grep -q "$ORT_ARCH"; then
    tmp="$(mktemp -d)"
    echo "fetching $ORT_NAME"
    curl -fL --retry 3 --progress-bar -o "$tmp/ort.tgz" \
        "https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/$ORT_NAME"
    tar -xzf "$tmp/ort.tgz" -C "$tmp"
    found="$(find "$tmp" -name 'libonnxruntime.1.20.0.dylib' -not -path '*.dSYM*' | head -1)"
    if [ -z "$found" ]; then
        echo "libonnxruntime.1.20.0.dylib not found in $ORT_NAME" >&2
        exit 1
    fi
    rm -f "$ORT_LIB"
    cp "$found" "$ORT_LIB"
    rm -rf "$tmp"
fi
ln -sfn "libonnxruntime.1.20.0.dylib" "$DEST/libonnxruntime.dylib"

for cfg in paradee-config.json kokoro-config.json; do
    if [ ! -f "$DEST/$cfg" ]; then
        echo "$cfg is missing from resources/neural (it is committed to the repo)" >&2
        exit 1
    fi
done

echo "Neural assets ready in resources/neural ($ORT_ARCH)"
