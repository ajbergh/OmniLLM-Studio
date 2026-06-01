function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

export function TimelineRuler({
  durationMs,
  pxPerMs,
  onSeek,
}: {
  durationMs: number;
  pxPerMs: number;
  onSeek: (timeMs: number) => void;
}) {
  const width = Math.max(900, durationMs * pxPerMs);
  const stepMs = pxPerMs > 0.04 ? 1000 : pxPerMs > 0.015 ? 2500 : 5000;
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
        <div key={time} className="absolute top-0 h-full border-l border-border/70 pl-1" style={{ left: time * pxPerMs }}>
          <span className="mt-1 block">{formatTime(time)}</span>
        </div>
      ))}
    </div>
  );
}
