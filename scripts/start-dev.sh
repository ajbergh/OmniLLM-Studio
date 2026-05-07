#!/usr/bin/env bash
# OmniLLM-Studio — Development Launch Script (Linux / macOS)
# Starts the Go backend and the Vite frontend dev server in separate terminals.
#
# Usage:
#   ./start-dev.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo ""
echo "================================"
echo "  OmniLLM-Studio — Dev Launch"
echo "================================"
echo ""

# Check prerequisites
for cmd in go node npm; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] '$cmd' not found in PATH."
        exit 1
    fi
done

# Install frontend dependencies if needed
if [ ! -d "$PROJECT_ROOT/frontend/node_modules" ]; then
    echo "[setup] Installing frontend dependencies..."
    cd "$PROJECT_ROOT/frontend"
    npm install --silent
fi

echo "[1/2] Starting backend (Go)..."
(cd "$PROJECT_ROOT/backend" && go run ./cmd/server) &
BACKEND_PID=$!

sleep 1

echo "[2/2] Starting frontend (React + Vite)..."
(cd "$PROJECT_ROOT/frontend" && npm run dev) &
FRONTEND_PID=$!

echo ""
echo "================================"
echo "  Servers Started"
echo "================================"
echo ""
echo "Backend:  http://localhost:8080"
echo "Frontend: http://localhost:5173"
echo ""
echo "Press Ctrl+C to stop both servers."
echo ""

# Wait and forward signals to both processes
trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; exit 0" INT TERM
wait
