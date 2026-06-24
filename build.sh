#!/bin/bash

# MDViewer Build Script
# This script builds the MDViewer application for production

echo "🔨 Building MDViewer..."
echo ""

# Check if wails is installed
if ! command -v wails &> /dev/null
then
    echo "❌ Error: Wails CLI is not installed"
    echo "Please install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

# Clean previous build (optional)
# echo "🧹 Cleaning previous build..."
# rm -rf build/bin

# Build the application
echo "📦 Building application..."
wails build

# Check if build was successful
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful!"

    # --- Bundle Piper into the .app ---
    echo "🔊 Bundling Piper TTS engine + voices..."
    bash "$(dirname "$0")/scripts/fetch-piper.sh"

    case "$(uname -m)" in
      arm64|aarch64) PIPER_TARGET="darwin-arm64" ;;
      *)             PIPER_TARGET="darwin-amd64" ;;
    esac

    APP_RES="build/bin/MDViewer.app/Contents/Resources/piper"
    mkdir -p "$APP_RES/voices"
    cp -R "resources/piper/engine/$PIPER_TARGET/piper/." "$APP_RES/"
    cp resources/piper/voices/*.onnx resources/piper/voices/*.onnx.json "$APP_RES/voices/"
    # Local dev: clear quarantine so Gatekeeper allows the bundled binary.
    xattr -dr com.apple.quarantine "$APP_RES" 2>/dev/null || true
    echo "   Piper bundled at $APP_RES"

    echo "📍 Application location: build/bin/MDViewer.app"
    echo ""
    echo "To run the app, use: ./run.sh"
    echo "Or manually: open build/bin/MDViewer.app"
else
    echo ""
    echo "❌ Build failed!"
    exit 1
fi
