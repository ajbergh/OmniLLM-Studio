#!/usr/bin/env bash
# OmniLLM-Studio — Build All Platforms
# Orchestrator script for CI/CD pipelines.
#
# Usage:
#   ./build-all.sh                    # Build for current OS only
#   ./build-all.sh --platform linux   # Build for a specific platform
#   ./build-all.sh --all              # Build all (only works with cross-compile support)
#
# Note: Cross-compiling CGO (required for SQLite) needs platform-specific
# toolchains. For full cross-platform builds, use the GitHub Actions workflow
# described in docs/Wails Build Plan.md.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
    echo "Usage: $0 [--platform windows|linux|macos] [--all]"
    echo ""
    echo "Options:"
    echo "  --platform <os>   Build for a specific platform"
    echo "  --all             Attempt to build all platforms (requires cross-compile toolchains)"
    echo "  --help            Show this help"
    echo ""
    echo "If no option is given, builds for the current OS."
}

detect_platform() {
    case "$(uname -s)" in
        Linux*)   echo "linux" ;;
        Darwin*)  echo "macos" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)        echo "unknown" ;;
    esac
}

build_platform() {
    local platform="$1"
    echo ""
    echo ">>> Building for $platform..."
    echo ""

    case "$platform" in
        windows)
            if [ "$(detect_platform)" = "windows" ]; then
                # Running in Git Bash / MSYS on Windows
                cmd.exe /c "$SCRIPT_DIR\\build-windows.bat"
            else
                echo "[WARN] Cross-compiling for Windows from $(detect_platform) requires MinGW-w64 toolchain."
                echo "       Skipping. Use GitHub Actions for cross-platform CI builds."
                return 1
            fi
            ;;
        linux)
            bash "$SCRIPT_DIR/build-linux.sh"
            ;;
        macos)
            bash "$SCRIPT_DIR/build-macos.sh"
            ;;
        *)
            echo "[ERROR] Unknown platform: $platform"
            return 1
            ;;
    esac
}

# --- Parse arguments ---
PLATFORMS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --platform)
            PLATFORMS+=("$2")
            shift 2
            ;;
        --all)
            PLATFORMS=("windows" "linux" "macos")
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "[ERROR] Unknown argument: $1"
            usage
            exit 1
            ;;
    esac
done

# Default to current platform
if [ ${#PLATFORMS[@]} -eq 0 ]; then
    CURRENT="$(detect_platform)"
    if [ "$CURRENT" = "unknown" ]; then
        echo "[ERROR] Could not detect current platform. Use --platform to specify."
        exit 1
    fi
    PLATFORMS=("$CURRENT")
fi

# --- Build ---
echo "=========================================="
echo "  OmniLLM-Studio — Multi-Platform Build"
echo "  Targets: ${PLATFORMS[*]}"
echo "=========================================="

FAILED=()
for p in "${PLATFORMS[@]}"; do
    if ! build_platform "$p"; then
        FAILED+=("$p")
    fi
done

echo ""
echo "=========================================="
if [ ${#FAILED[@]} -eq 0 ]; then
    echo "  All builds succeeded!"
else
    echo "  Some builds failed: ${FAILED[*]}"
    echo "  Successful: $(echo "${PLATFORMS[@]}" "${FAILED[@]}" | tr ' ' '\n' | sort | uniq -u | tr '\n' ' ')"
fi
echo "  Output directory: build/bin/"
echo "=========================================="
echo ""

[ ${#FAILED[@]} -eq 0 ]
