#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ ! -d "build/MDViewer.app" ]; then
    ./build.sh
fi

open "build/MDViewer.app" "$@"
