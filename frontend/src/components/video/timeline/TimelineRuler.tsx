import type { VideoTimelineMarker } from '../../../types/video';

function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

export function TimelineRuler({
  durationMs,
  pxPerMs,
  markers = [],
  onSeek,
  onRemoveMarker,
}: {
  durationMs: number;
  pxPerMs: number;
  markers?: VideoTimelineMarker[];
  onSeek: (timeMs: number) => void;
  onRemoveMarker?: (markerId: string) => void;
}) {
  const width = Math.max(900, durationMs * pxPerMs);
  // At high zoom show 250ms sub-second ticks (labels stay on whole seconds).
  const stepMs = pxPerMs > 0.055 ? 250 : pxPerMs > 0.04 ? 1000 : pxPerMs > 0.015 ? 2500 : 5000;
  const ticks = [];
  for (let time = 0; time <= durationMs; time += stepMs) {
    ticks.push(time);
  }

  return (
    <div
      className="relative h-8 border-b border-border bg-surface-alt text-[10px] text-text-muted"
      style={{ width }}
      onClick={(event) => {
        const rect = event.currentTarget.getBoundingClientRect();
        onSeek(Math.max(0, Math.round((event.clientX - rect.left) / pxPerMs)));
      }}
    >
      {ticks.map((time) => (
        time % 1000 === 0 ? (
          <div key={time} className="absolute top-0 h-full border-l border-border/70 pl-1" style={{ left: time * pxPerMs }}>
            <span className="mt-1 block">{formatTime(time)}</span>
          </div>
        ) : (
          <div key={time} className="absolute bottom-0 h-2 border-l border-border/50" style={{ left: time * pxPerMs }} />
        )
      ))}
      {markers.map((marker) => (
        <button
          key={marker.id}
          className="absolute bottom-0 z-10 h-4 w-2 -translate-x-1/2 rounded-t-sm bg-amber-400/90 hover:bg-amber-300"
          style={{ left: marker.time_ms * pxPerMs }}
          title={`${marker.label || 'Marker'} · ${formatTime(marker.time_ms)} — click to jump, right-click to remove`}
          aria-label={`Marker ${marker.label || formatTime(marker.time_ms)}`}
          onClick={(event) => {
            event.stopPropagation();
            onSeek(marker.time_ms);
          }}
          onContextMenu={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onRemoveMarker?.(marker.id);
          }}
        />
      ))}
    </div>
  );
}
