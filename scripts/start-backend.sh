#!/usr/bin/env bash
# OmniLLM-Studio — Backend Only (Linux / macOS)
# Starts the Go web server on :8080

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo ""
echo "Starting OmniLLM-Studio Backend..."
echo "  URL: http://localhost:8080"
echo ""

cd "$PROJECT_ROOT/backend"
exec go run ./cmd/server
