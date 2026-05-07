#!/usr/bin/env bash
# OmniLLM-Studio — Build All Platforms
# Orchestrator script for CI/CD pipelines.
#
# Usage:
#   ./build-all.sh                           # Wails build for current OS (amd64)
#   ./build-all.sh --platform linux          # Wails: Linux amd64
#   ./build-all.sh --platform linux arm64    # Wails: Linux arm64
#   ./build-all.sh --web                     # Web (non-Wails) build for current OS
#   ./build-all.sh --web --platform linux    # Web: Linux amd64
#   ./build-all.sh --all                     # All Wails platforms (current OS only)
#   ./build-all.sh --all --web               # All platforms, both Wails + Web
#
# Note: Cross-compiling CGO (required for SQLite) needs platform-specific
# toolchains. For full cross-platform builds, use the GitHub Actions workflow
# described in docs/Wails Build Plan.md.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
    echo "Usage: $0 [--platform <os> [arch]] [--web] [--all] [--help]"
    echo ""
    echo "Options:"
    echo "  --platform <os> [arch]  Build for a specific platform and optional arch"
    echo "                          OS: windows | linux | macos"
    echo "                          Arch: amd64 | arm64 | universal (macOS only)"
    echo "  --web                   Build the web server (non-Wails) instead of desktop"
    echo "  --all                   Build all platforms for current OS (ignores cross-compile limits)"
    echo "  --help                  Show this help"
    echo ""
    echo "If no option is given, builds Wails desktop for the current OS (amd64)."
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
    local mode="$1"   # wails | web
    local platform="$2"
    local arch="${3:-amd64}"

    echo ""
    echo ">>> Building [$mode] for $platform/$arch..."
    echo ""

    case "$platform" in
        windows)
            if [ "$(detect_platform)" = "windows" ]; then
                if [ "$mode" = "web" ]; then
                    cmd.exe /c "$SCRIPT_DIR\\build-web-windows.bat $arch"
                else
                    cmd.exe /c "$SCRIPT_DIR\\build-wails-windows.bat $arch"
                fi
            else
                echo "[WARN] Cross-compiling for Windows from $(detect_platform) requires MinGW-w64 toolchain."
                echo "       Skipping. Use GitHub Actions for cross-platform CI builds."
                return 1
            fi
            ;;
        linux)
            if [ "$mode" = "web" ]; then
                bash "$SCRIPT_DIR/build-web-linux.sh" "$arch"
            else
                bash "$SCRIPT_DIR/build-wails-linux.sh" "$arch"
            fi
            ;;
        macos)
            if [ "$mode" = "web" ]; then
                bash "$SCRIPT_DIR/build-web-macos.sh" "$arch"
            else
                bash "$SCRIPT_DIR/build-wails-macos.sh" "$arch"
            fi
            ;;
        *)
            echo "[ERROR] Unknown platform: $platform"
            return 1
            ;;
    esac
}

# --- Parse arguments ---
BUILDS=()   # Each entry is "mode:platform:arch"
WEB_MODE=false
BUILD_ALL=false
NEXT_PLATFORM=""
NEXT_ARCH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --platform)
            NEXT_PLATFORM="$2"
            shift 2
            # Optional arch argument
            if [[ $# -gt 0 && ! "$1" =~ ^-- ]]; then
                NEXT_ARCH="$1"
                shift
            else
                NEXT_ARCH="amd64"
            fi
            BUILDS+=("PLACEHOLDER:$NEXT_PLATFORM:$NEXT_ARCH")
            ;;
        --web)
            WEB_MODE=true
            shift
            ;;
        --all)
            BUILD_ALL=true
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

# Resolve mode into builds
MODE="wails"
$WEB_MODE && MODE="web"

if $BUILD_ALL; then
    CURRENT="$(detect_platform)"
    BUILDS=(
        "${MODE}:${CURRENT}:amd64"
        "${MODE}:${CURRENT}:arm64"
    )
    # macOS gets universal too (Wails mode only)
    if [ "$CURRENT" = "macos" ] && [ "$MODE" = "wails" ]; then
        BUILDS+=("wails:macos:universal")
    fi
elif [ ${#BUILDS[@]} -gt 0 ]; then
    # Replace PLACEHOLDER mode with resolved mode
    RESOLVED=()
    for entry in "${BUILDS[@]}"; do
        platform="${entry#PLACEHOLDER:}"
        platform="${platform%%:*}"
        arch="${entry##*:}"
        RESOLVED+=("${MODE}:${platform}:${arch}")
    done
    BUILDS=("${RESOLVED[@]}")
else
    # Default: current platform, amd64
    CURRENT="$(detect_platform)"
    if [ "$CURRENT" = "unknown" ]; then
        echo "[ERROR] Could not detect current platform. Use --platform to specify."
        exit 1
    fi
    BUILDS=("${MODE}:${CURRENT}:amd64")
fi

# --- Build ---
echo "=========================================="
echo "  OmniLLM-Studio — Multi-Platform Build"
echo "  Targets: ${BUILDS[*]}"
echo "=========================================="

FAILED=()
for entry in "${BUILDS[@]}"; do
    IFS=':' read -r b_mode b_platform b_arch <<< "$entry"
    if ! build_platform "$b_mode" "$b_platform" "$b_arch"; then
        FAILED+=("$entry")
    fi
done

echo ""
echo "=========================================="
if [ ${#FAILED[@]} -eq 0 ]; then
    echo "  All builds succeeded!"
else
    echo "  Some builds failed: ${FAILED[*]}"
fi
echo "  Output directory: build/bin/ and build/web/"
echo "=========================================="
echo ""

[ ${#FAILED[@]} -eq 0 ]

