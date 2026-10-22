#!/usr/bin/env bash
# Builds MDViewer.app for macOS.
#
# Requirements:
#   - macOS 13+
#   - Xcode Command Line Tools:  xcode-select --install
#   - Swift 5.9+ (ships with the command line tools)
#
# Optional but recommended: native Piper voices.
#   bash ../scripts/fetch-piper.sh
#
# If Piper assets are missing the app still works and falls back to the
# built-in macOS `say` voices.
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building MDViewer (release)..."
swift build -c release

ARCH="$(uname -m)"
case "$ARCH" in
    arm64)  ENGINE_ARCH="darwin-arm64" ;;
    x86_64) ENGINE_ARCH="darwin-amd64" ;;
    *)      echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

APP="build/MDViewer.app"
BIN=".build/release/MDViewer"
if [ ! -x "$BIN" ]; then
    echo "ERROR: expected $BIN to exist. Did swift build fail?"
    exit 1
fi

echo "==> Packaging $APP..."
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/MDViewer"
chmod +x "$APP/Contents/MacOS/MDViewer"
cp "Info.plist" "$APP/Contents/Info.plist"

PIPER_DEST="$APP/Contents/Resources/piper"
mkdir -p "$PIPER_DEST/voices"

SRC_ENGINE="../resources/piper/engine/$ENGINE_ARCH/piper"
if [ -d "$SRC_ENGINE" ]; then
    cp -R "$SRC_ENGINE/." "$PIPER_DEST/"
    if [ -f "$PIPER_DEST/piper" ]; then
        chmod +x "$PIPER_DEST/piper"
    fi
    echo "    bundled Piper engine: $ENGINE_ARCH"
else
    echo "    WARNING: $SRC_ENGINE not found."
    echo "             Run 'bash ../scripts/fetch-piper.sh' for natural voices."
fi

if ls ../resources/piper/voices/*.onnx >/dev/null 2>&1; then
    cp ../resources/piper/voices/*.onnx "$PIPER_DEST/voices/"
    if ls ../resources/piper/voices/*.onnx.json >/dev/null 2>&1; then
        cp ../resources/piper/voices/*.onnx.json "$PIPER_DEST/voices/"
    fi
    echo "    bundled Piper voices"
else
    echo "    WARNING: no Piper voices found; using macOS 'say' voices."
fi

# Ad-hoc signature. This is enough for local use; it is not notarized.
if command -v codesign >/dev/null 2>&1; then
    codesign --force --deep --sign - "$APP" >/dev/null 2>&1 \
        && echo "    ad-hoc signed" \
        || echo "    WARNING: codesign failed (the app may still run)."
fi

echo
echo "Built: $(pwd)/$APP"
echo "Run:   open \"$APP\""
