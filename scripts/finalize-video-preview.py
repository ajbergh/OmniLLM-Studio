#!/usr/bin/env python3
"""One-shot deterministic preview scalability patch for the large canvas file."""

from pathlib import Path

BRANCH = "feature/video-renderer-reliability-transcription-scalability-20260720"
ROOT = Path(__file__).resolve().parents[1]
PREVIEW = ROOT / "frontend/src/components/video/VideoPreviewCanvas.tsx"
WORKFLOW = ROOT / ".github/workflows/ci.yml"

source = PREVIEW.read_text(encoding="utf-8")
old_index = """  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);
  const activeIndexed = queryActiveClips(intervalIndex, playheadMs)
    .filter(({ track }) => track.visible)
    .filter(({ clip, asset }) => !clip.audio_only && (Boolean(clip.text) || !asset || !asset.mime_type.startsWith('audio/')))
    .sort((a, b) => (a.trackIndex - b.trackIndex) || ((a.clip.z_index ?? 0) - (b.clip.z_index ?? 0)));
  const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));
  const budgeted = applyDecoderBudget(activeIndexed, decoderLimit);
  const layers: LayerEntry[] = budgeted.mounted;
  const posterLayers = budgeted.posters;

  // Audio clips active at the playhead mount hidden <audio> elements so the
  // preview is audible — on any unmuted track, matching export semantics where
  // the asset (not the track type) decides audio contribution.
  const audioLayers = (timeline?.tracks || [])
    .filter((track) => !track.muted && (!soloTrackId || track.id === soloTrackId))
    .flatMap((track) => track.clips)
    .filter((clip) => playheadMs >= clip.start_ms && playheadMs < clip.start_ms + clip.duration_ms)
    .map((clip) => ({ clip, asset: clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined }))
    // Route every time-based asset through managed hidden audio. Visual video
    // elements stay muted, which gives ordinary video soundtracks the same
    // fades, keyframes, solo, master gain, and retiming as detached audio.
    .filter((entry): entry is { clip: VideoTimelineClip; asset: VideoAsset } =>
      Boolean(entry.asset && !entry.clip.muted &&
        (entry.asset.mime_type.startsWith('audio/') || entry.asset.mime_type.startsWith('video/'))));

  layersRef.current = layers;
  // posterLayers remain represented by their generated thumbnails without mounting additional video decoders.
  void posterLayers;
  audioLayersRef.current = audioLayers;
"""
new_index = """  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);
  const activeIndexed = queryActiveClips(intervalIndex, playheadMs)
    .filter(({ track }) => track.visible)
    .sort((a, b) => (a.trackIndex - b.trackIndex) || ((a.clip.z_index ?? 0) - (b.clip.z_index ?? 0)));
  const visualIndexed = activeIndexed.filter(({ clip, asset }) => (
    !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))
  ));
  const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));
  const budgeted = applyDecoderBudget(visualIndexed, decoderLimit, selectedClipId);
  const layers: LayerEntry[] = budgeted.mounted;
  const posterLayers: LayerEntry[] = budgeted.posters;
  const posterClipIds = new Set(posterLayers.map(({ clip }) => clip.id));
  const previewLayers = [...layers, ...posterLayers]
    .sort((a, b) => (a.trackIndex - b.trackIndex) || ((a.clip.z_index ?? 0) - (b.clip.z_index ?? 0)));

  // Audio uses the same interval index as visuals, eliminating full timeline
  // scans and repeated asset lookups on every playhead update.
  const audioLayers = activeIndexed
    .filter(({ track }) => !track.muted && (!soloTrackId || track.id === soloTrackId))
    .filter((entry): entry is LayerEntry & { asset: VideoAsset } => Boolean(
      entry.asset && !entry.clip.muted &&
      (entry.asset.mime_type.startsWith('audio/') || entry.asset.mime_type.startsWith('video/')),
    ))
    .map(({ clip, asset }) => ({ clip, asset }));

  layersRef.current = layers;
  audioLayersRef.current = audioLayers;
"""
if old_index not in source:
    raise SystemExit("preview index block not found")
source = source.replace(old_index, new_index, 1)
source = source.replace(
    "  const selectedEntry = layers.find((layer) => layer.clip.id === selectedClipId);",
    "  const selectedEntry = previewLayers.find((layer) => layer.clip.id === selectedClipId);",
    1,
)
source = source.replace(
    "  const renderLayer = (entry: LayerEntry) => {",
    "  const renderLayer = (entry: LayerEntry, poster = false) => {",
    1,
)
old_video = """    if (asset && asset.mime_type.startsWith('video/')) {
      content = (
        <video
          ref={(node) => {
            if (node) videoRefs.current.set(clip.id, node);
            else videoRefs.current.delete(clip.id);
          }}
          key={asset.id}
          src={videoApi.downloadUrl(asset.id)}
          className="h-full w-full object-contain"
          style={{ clipPath }}
          controls={false}
          playsInline
          autoPlay={false}
          muted
          aria-label={asset.file_name}
        />
      );
"""
new_video = """    if (asset && asset.mime_type.startsWith('video/')) {
      content = poster ? (
        <div className="relative h-full w-full bg-black/60" title="Video decoder budget: showing thumbnail">
          <img
            src={videoApi.artifactUrl(asset.id, 'thumbnail')}
            alt={`${asset.file_name} thumbnail`}
            className="h-full w-full object-contain"
            style={{ clipPath }}
          />
          <span className="absolute bottom-1 right-1 rounded bg-black/70 px-1 text-[9px] text-white/60">proxy frame</span>
        </div>
      ) : (
        <video
          ref={(node) => {
            if (node) videoRefs.current.set(clip.id, node);
            else videoRefs.current.delete(clip.id);
          }}
          key={asset.id}
          src={videoApi.downloadUrl(asset.id)}
          className="h-full w-full object-contain"
          style={{ clipPath }}
          controls={false}
          playsInline
          autoPlay={false}
          muted
          aria-label={asset.file_name}
        />
      );
"""
if old_video not in source:
    raise SystemExit("preview video block not found")
source = source.replace(old_video, new_video, 1)
source = source.replace(
    "          {layers.map((entry) => renderLayer(entry))}\n          {layers.length === 0 && (",
    "          {previewLayers.map((entry) => renderLayer(entry, posterClipIds.has(entry.clip.id)))}\n          {previewLayers.length === 0 && (",
    1,
)
source = source.replace(
    "        const entry = menu.clipId ? layers.find((layer) => layer.clip.id === menu.clipId) : undefined;",
    "        const entry = menu.clipId ? previewLayers.find((layer) => layer.clip.id === menu.clipId) : undefined;",
    1,
)
PREVIEW.write_text(source, encoding="utf-8")

workflow = WORKFLOW.read_text(encoding="utf-8")
workflow = workflow.replace("    permissions:\n      contents: write\n", "", 1)
workflow = workflow.replace(
    "      - uses: actions/checkout@v7\n        with:\n          ref: ${{ github.event.pull_request.head.sha || github.sha }}\n          fetch-depth: 0\n"
    "      - name: Finalize preview decoder budgeting\n"
    "        if: github.event_name == 'pull_request' && github.head_ref == '" + BRANCH + "'\n"
    "        run: |\n"
    "          set -euo pipefail\n"
    "          python scripts/finalize-video-preview.py\n"
    "          git config user.name 'github-actions[bot]'\n"
    "          git config user.email '41898282+github-actions[bot]@users.noreply.github.com'\n"
    "          git add -A\n"
    "          git diff --cached --check\n"
    "          git commit -m 'perf(video): complete preview decoder budgeting'\n"
    "          git push origin HEAD:" + BRANCH + "\n",
    "      - uses: actions/checkout@v7\n",
    1,
)
WORKFLOW.write_text(workflow, encoding="utf-8")
Path(__file__).unlink()
