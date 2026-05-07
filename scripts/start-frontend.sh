#!/usr/bin/env bash
# OmniLLM-Studio — Frontend Only (Linux / macOS)
# Starts the React + Vite dev server on :5173

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo ""
echo "Starting OmniLLM-Studio Frontend..."
echo "  URL: http://localhost:5173"
echo ""

cd "$PROJECT_ROOT/frontend"

if [ ! -d node_modules ]; then
    echo "[setup] Installing frontend dependencies..."
    npm install --silent
fi

exec npm run dev
