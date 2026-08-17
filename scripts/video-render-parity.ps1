param(
  [Parameter(Mandatory = $true)][string]$PreviewDir,
  [Parameter(Mandatory = $true)][string]$RenderedDir,
  [string]$OutputDir = "output/video-parity",
  [int]$FPS = 30,
  [string]$Fixture = "parity-torture-v1",
  [string]$TimelineSHA256 = "",
  [string]$ManifestSHA256 = "",
  [string]$PreviewAudio = "",
  [string]$RenderedAudio = "",
  [string]$FFmpeg = "",
  [string]$DeliveryMedia = "",
  [long]$ExpectedDurationMS = 0,
  [string]$FFprobe = "",
  [switch]$AllowFail
)

$ErrorActionPreference = "Stop"
$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backend = Join-Path $workspace "backend"
$goCache = Join-Path $workspace ".tmp/go-build-cache"
New-Item -ItemType Directory -Force -Path $goCache | Out-Null

$arguments = @(
  "run", "./cmd/video-parity-report",
  "--preview-dir", (Resolve-Path $PreviewDir).Path,
  "--rendered-dir", (Resolve-Path $RenderedDir).Path,
  "--output-dir", (Join-Path $workspace $OutputDir),
  "--fps", $FPS,
  "--fixture", $Fixture,
  "--timeline-sha256", $TimelineSHA256,
  "--manifest-sha256", $ManifestSHA256
)
if ($PreviewAudio -and $RenderedAudio) {
  $arguments += @("--preview-audio", (Resolve-Path $PreviewAudio).Path, "--rendered-audio", (Resolve-Path $RenderedAudio).Path)
  if ($FFmpeg) {
    $arguments += @("--ffmpeg", $FFmpeg)
  }
}
if ($DeliveryMedia) {
  $arguments += @("--delivery-media", (Resolve-Path $DeliveryMedia).Path, "--expected-duration-ms", $ExpectedDurationMS)
  if ($FFprobe) {
    $arguments += @("--ffprobe", $FFprobe)
  }
}
if ($AllowFail) {
  $arguments += "--allow-fail"
}

Push-Location $backend
try {
  $previousGoCache = $env:GOCACHE
  $env:GOCACHE = $goCache
  & go @arguments
  exit $LASTEXITCODE
} finally {
  $env:GOCACHE = $previousGoCache
  Pop-Location
}
