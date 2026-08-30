#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: str, before: str, after: str) -> None:
    p = Path(path)
    source = p.read_text(encoding="utf-8")
    count = source.count(before)
    if count != 1:
        raise RuntimeError(f"guarded replacement count for {path} is {count}, want 1: {before[:120]!r}")
    p.write_text(source.replace(before, after, 1), encoding="utf-8")


def insert_before(path: str, marker: str, addition: str) -> None:
    replace_once(path, marker, addition + marker)


replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\n",
    "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\nimport { ensurePreviewVideoPresentation, previewVideoPresentationToken, resetPreviewVideoPresentation } from './previewVideoPresentation';\n",
)

replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    '''      if (isPlaying) {
        if (element.paused && !element.ended) {
          element.currentTime = target;
          element.play().catch(() => { /* autoplay policy */ });
        } else if (Math.abs(element.currentTime - target) > 0.35) {
          // Drift correction (tab throttling, slow decode).
          element.currentTime = target;
        }
      } else {
        if (!element.paused) element.pause();
        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {
          element.currentTime = element instanceof HTMLVideoElement
            ? deterministicVideoSeekTargetSeconds(address, target)
            : target;
        }
      }
''',
    '''      if (isPlaying) {
        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);
        if (element.paused && !element.ended) {
          element.currentTime = target;
          element.play().catch(() => { /* autoplay policy */ });
        } else if (Math.abs(element.currentTime - target) > 0.35) {
          // Drift correction (tab throttling, slow decode).
          element.currentTime = target;
        }
      } else {
        if (!element.paused) element.pause();
        if (element instanceof HTMLVideoElement && address.kind === 'frame' && canonicalState) {
          ensurePreviewVideoPresentation({
            video: element,
            token: previewVideoPresentationToken(clip.id, address.frameIndex, targetMs),
            sourceTimeMs: targetMs,
            seekSeconds: deterministicVideoSeekTargetSeconds(address, target),
            toleranceSeconds: mediaSeekToleranceSeconds(address),
          });
          return;
        }
        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);
        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {
          element.currentTime = element instanceof HTMLVideoElement
            ? deterministicVideoSeekTargetSeconds(address, target)
            : target;
        }
      }
''',
)

replace_once(
    "frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx",
    "import { frameAddressMatchesTimelineMs } from './sourceTiming';\n",
    "import { frameAddressMatchesTimelineMs } from './sourceTiming';\nimport { previewVideoPresentationToken } from './previewVideoPresentation';\n",
)

replace_once(
    "frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx",
    ''' * Structural admission and runtime decoded-pixel proof both fail closed: normal
 * playback, poster-budget sources, unsupported canonical state, and decoded
 * video that cannot prove its source frame opaque leave the established CSS
 * approximation untouched. Transparent/partial-alpha images are admitted only
 * after canonical project-background composition in PreviewPixelateCanvas.
''',
    ''' * Structural admission and runtime decoded-pixel proof both fail closed: normal
 * playback, poster-budget sources, unsupported canonical state, and decoded
 * video without an exact canonical presentation proof leave the established CSS
 * approximation untouched. Images and alpha video are consumed only after
 * canonical project-background source-over composition in PreviewPixelateCanvas.
''',
)

replace_once(
    "frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx",
    '''  const consume = plan.mode === 'canonical-ready' && !posterDeferredReason;
  const executionKey = consume
    ? `${deterministicFrame}:${plan.target.clip.id}:${plan.backdrop.clip.id}:${canvasWidth}x${canvasHeight}:${canvasBackground}`
    : '';
''',
    '''  const consume = plan.mode === 'canonical-ready' && !posterDeferredReason;
  const videoPresentationToken = plan.mode === 'canonical-ready'
    && !posterDeferredReason
    && deterministicFrame !== null
    && plan.rasterSource.kind === 'video'
    && plan.backdrop.canonicalState
    ? previewVideoPresentationToken(
      plan.backdrop.clip.id,
      deterministicFrame,
      plan.backdrop.canonicalState.source_time_ms,
    )
    : undefined;
  const executionKey = consume
    ? `${deterministicFrame}:${plan.target.clip.id}:${plan.backdrop.clip.id}:${canvasWidth}x${canvasHeight}:${canvasBackground}:${videoPresentationToken ?? 'image'}`
    : '';
''',
)

replace_once(
    "frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx",
    '''      sourceForClip={sourceForClip}
      executionKey={executionKey}
''',
    '''      sourceForClip={sourceForClip}
      videoPresentationToken={videoPresentationToken}
      executionKey={executionKey}
''',
)

replace_once(
    "backend/internal/video/render_snapshot.go",
    '''\t\t\tif renderAssetRequiresVideo(stagedAsset) && media.VideoCodec == "" && media.Width == 0 && media.Height == 0 {
\t\t\t\treturn nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable visual stream", strings.Join(references[assetID], ", "), assetID)
\t\t\t}
\t\t}
''',
    '''\t\t\tif renderAssetRequiresVideo(stagedAsset) && media.VideoCodec == "" && media.Width == 0 && media.Height == 0 {
\t\t\t\treturn nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable visual stream", strings.Join(references[assetID], ", "), assetID)
\t\t\t}
\t\t\tstagedAsset.MetadataJSON = mergeProbeMetadataJSON(stagedAsset.MetadataJSON, media)
\t\t}
''',
)

replace_once(
    "backend/internal/video/renderer.go",
    '''\tisAudio         bool
\taudioChannels   int
\t// hasAudio reports whether a video asset carries an audio stream that
''',
    '''\tisAudio         bool
\taudioChannels   int
\tinputDecoder    string
\t// hasAudio reports whether a video asset carries an audio stream that
''',
)

replace_once(
    "backend/internal/video/renderer.go",
    '''\t\t\trc := resolvedClip{
\t\t\t\ttrackIndex: trackIndex,
\t\t\t\tclip:       clip,
\t\t\t\tfilePath:   fullPath,
\t\t\t\tisVideo:    strings.HasPrefix(mime, "video/"),
\t\t\t\tisImage:    strings.HasPrefix(mime, "image/"),
\t\t\t\tisAudio:    strings.HasPrefix(mime, "audio/"),
\t\t\t}
''',
    '''\t\t\trc := resolvedClip{
\t\t\t\ttrackIndex: trackIndex,
\t\t\t\tclip:       clip,
\t\t\t\tfilePath:   fullPath,
\t\t\t\tisVideo:    strings.HasPrefix(mime, "video/"),
\t\t\t\tisImage:    strings.HasPrefix(mime, "image/"),
\t\t\t\tisAudio:    strings.HasPrefix(mime, "audio/"),
\t\t\t}
\t\t\tif rc.isVideo {
\t\t\t\trc.inputDecoder = videoAssetInputDecoder(asset)
\t\t\t}
''',
)

replace_once(
    "backend/internal/video/renderer.go",
    '''\tsourceByPath := map[string]int{}
\tvisualBySource := map[int][]int{}
\taudioBySource := map[int][]int{}
''',
    '''\tsourceByPath := map[string]int{}
\tdecoderByPath := map[string]string{}
\tfor _, clip := range clips {
\t\tif clip.inputDecoder != "" {
\t\t\tdecoderByPath[clip.filePath] = clip.inputDecoder
\t\t}
\t}
\tvisualBySource := map[int][]int{}
\taudioBySource := map[int][]int{}
''',
)

replace_once(
    "backend/internal/video/renderer.go",
    '''\t\t\tsourceByPath[clips[i].filePath] = sourceIdx
\t\t\targs = append(args, "-i", clips[i].filePath)
''',
    '''\t\t\tsourceByPath[clips[i].filePath] = sourceIdx
\t\t\tif decoder := decoderByPath[clips[i].filePath]; decoder != "" {
\t\t\t\targs = append(args, "-c:v", decoder)
\t\t\t}
\t\t\targs = append(args, "-i", clips[i].filePath)
''',
)

insert_before(
    "backend/internal/video/renderer.go",
    "// videoAssetHasAudio reports whether a video asset carries an audio stream.\n",
    '''// videoAssetInputDecoder selects an input decoder only when immutable stream
// facts require one for fidelity. FFmpeg's native VP9 decoder discards WebM
// alpha, while libvpx-vp9 preserves alpha_mode=1 streams.
func videoAssetInputDecoder(asset models.VideoAsset) string {
\tif strings.TrimSpace(asset.MetadataJSON) == "" {
\t\treturn ""
\t}
\tvar metadata struct {
\t\tVideoCodec       string `json:"video_codec"`
\t\tVideoPixelFormat string `json:"video_pixel_format"`
\t\tVideoAlphaMode   string `json:"video_alpha_mode"`
\t}
\tif err := json.Unmarshal([]byte(asset.MetadataJSON), &metadata); err != nil {
\t\treturn ""
\t}
\tprobe := &MediaProbe{
\t\tVideoCodec:       metadata.VideoCodec,
\t\tVideoPixelFormat: metadata.VideoPixelFormat,
\t\tVideoAlphaMode:   metadata.VideoAlphaMode,
\t}
\tif strings.EqualFold(strings.TrimSpace(probe.VideoCodec), "vp9") && probe.VideoHasAlpha() {
\t\treturn "libvpx-vp9"
\t}
\treturn ""
}

''',
)

replace_once(
    "backend/internal/video/renderer_input_fanout_test.go",
    '''import (
\t"strings"
\t"testing"
)
''',
    '''import (
\t"strings"
\t"testing"

\t"github.com/ajbergh/omnillm-studio/internal/models"
)
''',
)

insert_before(
    "backend/internal/video/renderer_input_fanout_test.go",
    "func TestResolvedInputLabelsPreserveLegacyDirectGraphTests(t *testing.T) {\n",
    '''func TestAppendResolvedClipInputsSelectsAlphaPreservingVP9DecoderOnce(t *testing.T) {
\tclips := []resolvedClip{
\t\t{filePath: "input-alpha.webm", isVideo: true, inputDecoder: "libvpx-vp9"},
\t\t{filePath: "input-alpha.webm", isVideo: true},
\t\t{filePath: "input-opaque.mp4", isVideo: true},
\t}
\targs, _ := appendResolvedClipInputs(nil, clips, 1)
\tjoined := strings.Join(args, " ")
\tif strings.Count(joined, "-c:v libvpx-vp9") != 1 {
\t\tt.Fatalf("alpha decoder should be applied once to the shared source: %s", joined)
\t}
\tif !strings.Contains(joined, "-c:v libvpx-vp9 -i input-alpha.webm") {
\t\tt.Fatalf("alpha decoder must be scoped before its input: %s", joined)
\t}
\tif strings.Contains(joined, "libvpx-vp9 -i input-opaque.mp4") {
\t\tt.Fatalf("opaque input must not inherit alpha decoder: %s", joined)
\t}
}

func TestVideoAssetInputDecoderUsesFrozenAlphaFacts(t *testing.T) {
\tasset := models.VideoAsset{MetadataJSON: `{"video_codec":"vp9","video_pixel_format":"yuv420p","video_alpha_mode":"1"}`}
\tif got := videoAssetInputDecoder(asset); got != "libvpx-vp9" {
\t\tt.Fatalf("decoder = %q, want libvpx-vp9", got)
\t}
\tasset.MetadataJSON = `{"video_codec":"vp9","video_pixel_format":"yuv420p"}`
\tif got := videoAssetInputDecoder(asset); got != "" {
\t\tt.Fatalf("opaque decoder = %q, want default", got)
\t}
}

''',
)

print("transparent-video integration patch applied")
