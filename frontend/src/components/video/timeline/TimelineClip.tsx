import type { PointerEvent as ReactPointerEvent } from 'react';
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
  onTrim,
}: {
  clip: Clip;
  asset?: VideoAsset;
  selected: boolean;
  pxPerMs: number;
  trackId: string;
  onSelect: (clipId: string, trackId: string, additive?: boolean) => void;
  onTrim: (clipId: string, updates: Partial<Pick<Clip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => void;
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

  const beginTrim = (edge: 'start' | 'end', event: ReactPointerEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    event.preventDefault();
    const startX = event.clientX;
    const trimIn = clip.trim_in_ms ?? 0;
    const trimOut = clip.trim_out_ms ?? clip.duration_ms;

    const onPointerUp = (upEvent: PointerEvent) => {
      document.removeEventListener('pointerup', onPointerUp);
      const deltaMs = Math.round((upEvent.clientX - startX) / pxPerMs);
      if (deltaMs === 0) return;
      if (edge === 'start') {
        const clamped = Math.max(-clip.start_ms, Math.min(deltaMs, clip.duration_ms - 100));
        onTrim(clip.id, {
          start_ms: clip.start_ms + clamped,
          duration_ms: clip.duration_ms - clamped,
          trim_in_ms: Math.max(0, trimIn + clamped),
          trim_out_ms: trimOut,
        });
      } else {
        const clamped = Math.max(100 - clip.duration_ms, deltaMs);
        onTrim(clip.id, {
          duration_ms: clip.duration_ms + clamped,
          trim_in_ms: trimIn,
          trim_out_ms: Math.max(trimIn + 100, trimOut + clamped),
        });
      }
    };

    document.addEventListener('pointerup', onPointerUp, { once: true });
  };

  return (
    <div
      role="button"
      tabIndex={0}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.setData('application/x-video-clip-id', clip.id);
        // Remember where inside the clip the drag started so drops keep the
        // grabbed point under the cursor instead of jumping to the clip start.
        const rect = event.currentTarget.getBoundingClientRect();
        event.dataTransfer.setData('application/x-video-clip-grab-offset', String(Math.round(event.clientX - rect.left)));
        event.dataTransfer.effectAllowed = 'move';
      }}
      onClick={(event) => {
        event.stopPropagation();
        onSelect(clip.id, trackId, event.ctrlKey || event.metaKey || event.shiftKey);
      }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onSelect(clip.id, trackId, event.ctrlKey || event.metaKey || event.shiftKey);
        }
      }}
      className={`absolute top-1 h-9 rounded-md border px-2 text-left text-[11px] transition-colors ${tone} ${
        selected ? 'ring-2 ring-primary ring-offset-1 ring-offset-surface' : ''
      }`}
      style={{ left, width }}
      title={clipLabel(clip, asset)}
    >
      <button
        type="button"
        onPointerDown={(event) => beginTrim('start', event)}
        className="absolute left-0 top-0 h-full w-2 cursor-ew-resize rounded-l-md bg-white/10 opacity-0 hover:opacity-100 focus:opacity-100"
        title="Trim clip start"
        aria-label="Trim clip start"
      />
      <span className="block truncate font-medium">{clipLabel(clip, asset)}</span>
      <span className="block truncate text-[10px] opacity-75">{Math.round(clip.duration_ms / 100) / 10}s</span>
      <button
        type="button"
        onPointerDown={(event) => beginTrim('end', event)}
        className="absolute right-0 top-0 h-full w-2 cursor-ew-resize rounded-r-md bg-white/10 opacity-0 hover:opacity-100 focus:opacity-100"
        title="Trim clip end"
        aria-label="Trim clip end"
      />
    </div>
  );
}
