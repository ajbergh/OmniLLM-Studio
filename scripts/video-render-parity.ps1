param(
  [Parameter(Mandatory = $true)][string]$PreviewDir,
  [Parameter(Mandatory = $true)][string]$RenderedDir,
  [string]$OutputDir = "output/video-parity",
  [int]$FPS = 30,
  [string]$Fixture = "parity-torture-v1",
  [string]$TimelineSHA256 = "",
  [string]$ManifestSHA256 = "",
  [string]$PreviewAudio = "",
  [string]$RenderedAudio = ""
)

$ErrorActionPreference = "Stop"
$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backend = Join-Path $workspace "backend"

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
}

Push-Location $backend
try {
  & go @arguments
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
