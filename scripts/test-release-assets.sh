#!/usr/bin/env bash
# Deterministic release staging contract; no actual model or desktop builds.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/dist/OmniLLM-Studio-linux-amd64" \
         "$tmp/dist/OmniLLM-Studio-macos-arm64" \
         "$tmp/dist/OmniLLM-Studio-windows-amd64"
printf 'Linux fixture\n' > "$tmp/dist/OmniLLM-Studio-linux-amd64/OmniLLM-Studio"
printf 'macOS fixture\n' > "$tmp/dist/OmniLLM-Studio-macos-arm64/OmniLLM-Studio-macos-arm64.zip"
printf 'Windows fixture\n' > "$tmp/dist/OmniLLM-Studio-windows-amd64/OmniLLM-Studio.exe"

bash "$repo_root/scripts/stage-release-assets.sh" "$tmp/dist" "$tmp/assets"
test "$(find "$tmp/assets" -maxdepth 1 -type f | wc -l | tr -d ' ')" -eq 4
(
  cd "$tmp/assets"
  sha256sum --check SHA256SUMS.txt
)
# Verify corruption is detected by the published manifest.
printf 'tampered\n' >> "$tmp/assets/OmniLLM-Studio-windows-amd64.exe"
if (cd "$tmp/assets" && sha256sum --check SHA256SUMS.txt >/dev/null 2>&1); then
  echo "Checksum verification accepted a tampered binary" >&2
  exit 1
fi
# Verify incomplete releases fail closed rather than publishing two platforms.
rm "$tmp/dist/OmniLLM-Studio-macos-arm64/OmniLLM-Studio-macos-arm64.zip"
if bash "$repo_root/scripts/stage-release-assets.sh" "$tmp/dist" "$tmp/incomplete" >/dev/null 2>&1; then
  echo "Staging accepted a missing platform binary" >&2
  exit 1
fi
test ! -e "$tmp/incomplete"
echo "Release artifact staging and verification passed."
