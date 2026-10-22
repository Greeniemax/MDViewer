#!/usr/bin/env bash
# Installs MDViewer.app into /Applications, registers it with Launch Services,
# and makes it the default app for Markdown files (.md / .markdown).
#
#   bash register.sh
#
# macOS 15+ does not let a command-line tool change file associations
# silently. The app performs the request itself
# (`MDViewer --set-default-markdown`), which is the supported path.
set -euo pipefail
cd "$(dirname "$0")"

APP="build/MDViewer.app"
DEST="/Applications/MDViewer.app"
BUNDLE_ID="com.mdviewer.mac"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

handler_bundle() {
    local tmp
    tmp="$(mktemp -t mdviewer-handler).swift"
    cat > "$tmp" <<'SWIFT'
import AppKit
import UniformTypeIdentifiers
let md = UTType("net.daringfireball.markdown")!
if let url = NSWorkspace.shared.urlForApplication(toOpen: md) {
    print(Bundle(url: url)?.bundleIdentifier ?? url.path)
} else {
    print("")
}
SWIFT
    swift "$tmp" 2>/dev/null
    rm -f "$tmp"
}

# Always refresh the bundle so the installed copy matches the sources.
./build.sh

echo "==> Installing $DEST..."
rm -rf "$DEST"
ditto "$APP" "$DEST"

if [ -x "$LSREGISTER" ]; then
    "$LSREGISTER" -f "$DEST"
fi

echo "==> Making MDViewer the default Markdown app..."
current="$(handler_bundle)"
if [ "$current" = "$BUNDLE_ID" ]; then
    echo "    already the default handler for net.daringfireball.markdown"
else
    if pgrep -f "$DEST/Contents/MacOS/MDViewer" >/dev/null 2>&1; then
        echo "    MDViewer is running; use File > Set as Default Markdown App."
    else
        # Launching the bundled binary with the flag lets the app request the
        # association through Launch Services.
        "$DEST/Contents/MacOS/MDViewer" --set-default-markdown >/dev/null 2>&1 &
        for _ in 1 2 3 4 5 6 7 8 9 10; do
            sleep 0.5
            current="$(handler_bundle)"
            [ "$current" = "$BUNDLE_ID" ] && break
        done
    fi

    if [ "$current" = "$BUNDLE_ID" ]; then
        echo "    net.daringfireball.markdown -> $current"
    else
        echo "    still: ${current:-(none)}"
        echo "    macOS blocked the change. Approve it in Finder:"
        echo "      right-click a .md file > Get Info > Open with > MDViewer > Change All..."
    fi
fi

echo
echo "Installed: $DEST"
echo "Run:       open \"$DEST\""
echo "Open file: open -a \"$DEST\" some-file.md"
