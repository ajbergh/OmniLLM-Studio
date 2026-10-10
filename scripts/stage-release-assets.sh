#!/usr/bin/env bash
# Stage the exact desktop release assets with collision-free platform names.
# Usage: bash scripts/stage-release-assets.sh [download-root] [release-dir]
set -euo pipefail

download_root="${1:-dist}"
release_dir="${2:-release-assets}"

if [[ -e "$release_dir" ]]; then
  echo "Release staging destination already exists: $release_dir" >&2
  exit 1
fi

sources=(
  "OmniLLM-Studio-linux-amd64/OmniLLM-Studio"
  "OmniLLM-Studio-macos-arm64/OmniLLM-Studio-macos-arm64.zip"
  "OmniLLM-Studio-windows-amd64/OmniLLM-Studio.exe"
)
destinations=(
  "OmniLLM-Studio-linux-amd64"
  "OmniLLM-Studio-macos-arm64.zip"
  "OmniLLM-Studio-windows-amd64.exe"
)

# Check all source artifacts before exposing any staged output.
for src in "${sources[@]}"; do
  if [[ ! -s "$download_root/$src" ]]; then
    echo "Missing or empty release artifact: $download_root/$src" >&2
    exit 1
  fi
done

mkdir -p "$release_dir"
for i in "${!sources[@]}"; do
  cp -- "$download_root/${sources[$i]}" "$release_dir/${destinations[$i]}"
done

(
  cd "$release_dir"
  sha256sum "${destinations[@]}" > SHA256SUMS.txt
  sha256sum --check SHA256SUMS.txt
)
