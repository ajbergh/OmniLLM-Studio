import { Copy, HelpCircle, Magnet, Maximize2, MousePointer2, Pause, Play, Redo2, Save, Scissors, Slice, Trash2, Undo2, ZoomIn, ZoomOut } from 'lucide-react';
import type { ReactNode } from 'react';

export function TimelineToolbar({
  isPlaying,
  snappingEnabled,
  zoom,
  isSaving,
  canUndo,
  canRedo,
  toolMode,
  onPlayPause,
  onUndo,
  onRedo,
  onSplit,
  onDelete,
  onDuplicate,
  onSave,
  onZoom,
  onZoomToFit,
  onToggleSnap,
  onSetToolMode,
  onHelp,
  timecode,
}: {
  isPlaying: boolean;
  snappingEnabled: boolean;
  zoom: number;
  isSaving: boolean;
  canUndo: boolean;
  canRedo: boolean;
  toolMode: 'select' | 'blade';
  onPlayPause: () => void;
  onUndo: () => void;
  onRedo: () => void;
  onSplit: () => void;
  onDelete: () => void;
  onDuplicate: () => void;
  onSave: () => void;
  onZoom: (zoom: number) => void;
  onZoomToFit: () => void;
  onToggleSnap: () => void;
  onSetToolMode: (mode: 'select' | 'blade') => void;
  onHelp: () => void;
  timecode?: string;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1 border-b border-border bg-surface-alt px-2 py-1.5">
      <IconButton label={isPlaying ? 'Pause' : 'Play'} onClick={onPlayPause}>
        {isPlaying ? <Pause size={14} /> : <Play size={14} />}
      </IconButton>
      {timecode && (
        <span className="mx-1 font-mono text-[11px] tabular-nums text-text-secondary" title="Playhead / total (min:sec.frames)">
          {timecode}
        </span>
      )}
      <IconButton label="Undo" onClick={onUndo} disabled={!canUndo}>
        <Undo2 size={14} />
      </IconButton>
      <IconButton label="Redo" onClick={onRedo} disabled={!canRedo}>
        <Redo2 size={14} />
      </IconButton>
      <span className="mx-1 h-5 w-px bg-border" />
      <IconButton label="Select tool (V)" onClick={() => onSetToolMode('select')} active={toolMode === 'select'}>
        <MousePointer2 size={14} />
      </IconButton>
      <IconButton label="Blade tool — click a clip to split (C)" onClick={() => onSetToolMode('blade')} active={toolMode === 'blade'}>
        <Slice size={14} />
      </IconButton>
      <span className="mx-1 h-5 w-px bg-border" />
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
      <IconButton label="Zoom to fit" onClick={onZoomToFit}>
        <Maximize2 size={14} />
      </IconButton>
      <IconButton label={snappingEnabled ? 'Disable snapping' : 'Enable snapping'} onClick={onToggleSnap} active={snappingEnabled}>
        <Magnet size={14} />
      </IconButton>
      <span className="mx-1 h-5 w-px bg-border" />
      <IconButton label="Save timeline" onClick={onSave} active={isSaving}>
        <Save size={14} className={isSaving ? 'animate-pulse' : ''} />
      </IconButton>
      <IconButton label="Keyboard shortcuts (?)" onClick={onHelp}>
        <HelpCircle size={14} />
      </IconButton>
    </div>
  );
}

function IconButton({
  label,
  active = false,
  disabled = false,
  onClick,
  children,
}: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex h-8 w-8 items-center justify-center rounded-md border text-text-muted transition-colors hover:text-text disabled:cursor-not-allowed disabled:opacity-40 ${
        active ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-surface'
      }`}
      title={label}
      aria-label={label}
    >
      {children}
    </button>
  );
}
