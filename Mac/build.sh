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

# The macOS 26.6+/27 SDK declares SwiftUI property wrappers (@State, @Binding,
# @ObservedObject, ...) as macros. Their implementation plugin (SwiftUIMacros)
# ships with full Xcode, not with the Command Line Tools, so a CLT-only machine
# fails with "external macro implementation type ... could not be found".
# When that plugin is missing, build against an SDK that still has the
# property-wrapper declarations instead.
SWIFT_BIN="$(xcrun -f swift 2>/dev/null || command -v swift)"
PLUGIN_DIR="$(cd "$(dirname "$SWIFT_BIN")/../lib/swift/host/plugins" 2>/dev/null && pwd || true)"
SDK_FLAGS=()
if [ -n "$PLUGIN_DIR" ] && [ ! -f "$PLUGIN_DIR/libSwiftUIMacros.dylib" ]; then
    SDK_ROOT="$(dirname "$(xcrun --show-sdk-path)")"
    while IFS= read -r candidate; do
        iface="$(ls "$candidate"/System/Library/Frameworks/SwiftUICore.framework/Modules/SwiftUICore.swiftmodule/*-apple-macos.swiftinterface 2>/dev/null | head -1)"
        # SDKs whose SwiftUI declares @State as a macro need the Xcode plugin.
        if [ -n "$iface" ] && ! grep -q 'macro State' "$iface"; then
            SDK_FLAGS=(--sdk "$candidate")
            echo "    no SwiftUIMacros plugin in Command Line Tools; using SDK $(basename "$candidate")"
            break
        fi
    done < <(ls -d "$SDK_ROOT"/MacOSX*.sdk 2>/dev/null | sort -V -r)
fi

echo "==> Building MDViewer (release)..."
swift build -c release ${SDK_FLAGS[@]+"${SDK_FLAGS[@]}"}

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
        # The release binary links the engine dylibs via @rpath but ships no
        # LC_RPATH, so bundle them next to it and point @rpath there.
        if command -v install_name_tool >/dev/null 2>&1; then
            install_name_tool -add_rpath "@executable_path" "$PIPER_DEST/piper" 2>/dev/null || true
        fi
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
