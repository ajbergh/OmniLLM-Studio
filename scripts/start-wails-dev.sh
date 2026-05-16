#!/usr/bin/env bash
# OmniLLM-Studio — Wails Dev Mode (Linux / macOS)
# Starts the app in Wails dev mode with hot-reload.
# Wails proxies the frontend Vite dev server and wraps it in a WebView window.
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   - Linux: GCC, WebKit2GTK (libwebkit2gtk-4.0-dev)
#   - macOS:  Xcode Command Line Tools (xcode-select --install)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Wails Dev Mode"
echo "=========================================="
echo ""

# --- Check prerequisites ---
for cmd in go node npm wails; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] '$cmd' not found in PATH."
        [ "$cmd" = "wails" ] && echo "  Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
        exit 1
    fi
done

# Linux: check WebKit2GTK
if [[ "$(uname -s)" == "Linux" ]]; then
    if ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
        echo "[ERROR] WebKit2GTK not found."
        echo "  Debian/Ubuntu: sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"
        echo "  Fedora:        sudo dnf install gtk3-devel webkit2gtk4.0-devel"
        exit 1
    fi
fi

# --- Install frontend deps if needed ---
if [ ! -d "$PROJECT_ROOT/frontend/node_modules" ]; then
    echo "[setup] Installing frontend dependencies..."
    cd "$PROJECT_ROOT/frontend"
    npm install --silent
fi

# --- Launch Wails dev ---
echo "Starting Wails dev server..."
echo "  The app window will open automatically."
echo "  Press Ctrl+C to stop."
echo ""

cd "$PROJECT_ROOT/backend/cmd/desktop"
export OMNILLM_BROWSER_ENABLED=true
exec wails dev -frontenddevserverurl http://localhost:5173
