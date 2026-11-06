#!/bin/bash

# MDViewer Run Script (Wails app)
# This script runs the built Wails application.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

APP_PATH="build/bin/MDViewer.app"

echo "🚀 Starting MDViewer..."

# Check if the app exists
if [ ! -d "$APP_PATH" ]; then
    echo "❌ Error: MDViewer.app not found at $APP_PATH"
    echo ""
    echo "Please build the application first:"
    echo "  bash scripts/build.sh"
    echo ""
    echo "Or run in development mode:"
    echo "  wails dev"
    exit 1
fi

# Launch the application
echo "📂 Opening $APP_PATH"
open "$APP_PATH"

echo "✅ Application launched!"
