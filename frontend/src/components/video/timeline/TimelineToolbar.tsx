import { Copy, Magnet, Pause, Play, Save, Scissors, Trash2, ZoomIn, ZoomOut } from 'lucide-react';
import type { ReactNode } from 'react';

export function TimelineToolbar({
  isPlaying,
  snappingEnabled,
  zoom,
  isSaving,
  onPlayPause,
  onSplit,
  onDelete,
  onDuplicate,
  onSave,
  onZoom,
  onToggleSnap,
}: {
  isPlaying: boolean;
  snappingEnabled: boolean;
  zoom: number;
  isSaving: boolean;
  onPlayPause: () => void;
  onSplit: () => void;
  onDelete: () => void;
  onDuplicate: () => void;
  onSave: () => void;
  onZoom: (zoom: number) => void;
  onToggleSnap: () => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1 border-b border-border bg-surface-alt px-2 py-1.5">
      <IconButton label={isPlaying ? 'Pause' : 'Play'} onClick={onPlayPause}>
        {isPlaying ? <Pause size={14} /> : <Play size={14} />}
      </IconButton>
      <IconButton label="Split selected clip" onClick={onSplit}>
        <Scissors size={14} />
      </IconButton>
      <IconButton label="Duplicate selected clip" onClick={onDuplicate}>
        <Copy size={14} />
      </IconButton>
      <IconButton label="Delete selected clip" onClick={onDelete}>
        <Trash2 size={14} />
      </IconButton>
      <span className="mx-1 h-5 w-px bg-border" />
      <IconButton label="Zoom out" onClick={() => onZoom(zoom - 0.2)}>
        <ZoomOut size={14} />
      </IconButton>
      <span className="w-10 text-center text-[11px] text-text-muted">{Math.round(zoom * 100)}%</span>
      <IconButton label="Zoom in" onClick={() => onZoom(zoom + 0.2)}>
        <ZoomIn size={14} />
      </IconButton>
      <IconButton label={snappingEnabled ? 'Disable snapping' : 'Enable snapping'} onClick={onToggleSnap} active={snappingEnabled}>
        <Magnet size={14} />
      </IconButton>
      <span className="mx-1 h-5 w-px bg-border" />
      <IconButton label="Save timeline" onClick={onSave} active={isSaving}>
        <Save size={14} className={isSaving ? 'animate-pulse' : ''} />
      </IconButton>
    </div>
  );
}

function IconButton({
  label,
  active = false,
  onClick,
  children,
}: {
  label: string;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex h-8 w-8 items-center justify-center rounded-md border text-text-muted transition-colors hover:text-text ${
        active ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-surface'
      }`}
      title={label}
      aria-label={label}
    >
      {children}
    </button>
  );
}
