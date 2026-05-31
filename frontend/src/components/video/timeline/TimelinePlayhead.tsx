export function TimelinePlayhead({ x }: { x: number }) {
  return (
    <div
      className="pointer-events-none absolute bottom-0 top-0 z-20 w-px bg-primary"
      style={{ left: x }}
    >
      <div className="-ml-1.5 h-3 w-3 rounded-sm bg-primary" />
    </div>
  );
}
