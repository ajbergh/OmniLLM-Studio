import type { VideoAsset, VideoTimelineClip as Clip } from '../../../types/video';

function clipLabel(clip: Clip, asset?: VideoAsset): string {
  if (clip.text?.text) return clip.text.text;
  return asset?.file_name || clip.asset_id || 'Clip';
}

export function TimelineClip({
  clip,
  asset,
  selected,
  pxPerMs,
  trackId,
  onSelect,
}: {
  clip: Clip;
  asset?: VideoAsset;
  selected: boolean;
  pxPerMs: number;
  trackId: string;
  onSelect: (clipId: string, trackId: string) => void;
}) {
  const left = clip.start_ms * pxPerMs;
  const width = Math.max(36, clip.duration_ms * pxPerMs);
  const kind = asset?.kind || (clip.text ? 'text' : 'video');
  const tone =
    kind === 'audio' || kind === 'music'
      ? 'border-emerald-400/40 bg-emerald-500/15 text-emerald-200'
      : kind === 'image'
        ? 'border-cyan-400/40 bg-cyan-500/15 text-cyan-100'
        : kind === 'text' || kind === 'caption'
          ? 'border-amber-400/40 bg-amber-500/15 text-amber-100'
          : 'border-primary/40 bg-primary/15 text-primary';

  return (
    <button
      draggable
      onDragStart={(event) => {
        event.dataTransfer.setData('application/x-video-clip-id', clip.id);
        event.dataTransfer.effectAllowed = 'move';
      }}
      onClick={(event) => {
        event.stopPropagation();
        onSelect(clip.id, trackId);
      }}
      className={`absolute top-1 h-9 rounded-md border px-2 text-left text-[11px] transition-colors ${tone} ${
        selected ? 'ring-2 ring-primary ring-offset-1 ring-offset-surface' : ''
      }`}
      style={{ left, width }}
      title={clipLabel(clip, asset)}
    >
      <span className="block truncate font-medium">{clipLabel(clip, asset)}</span>
      <span className="block truncate text-[10px] opacity-75">{Math.round(clip.duration_ms / 100) / 10}s</span>
    </button>
  );
}
