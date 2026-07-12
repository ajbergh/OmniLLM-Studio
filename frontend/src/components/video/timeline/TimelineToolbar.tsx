/**
 * Timeline toolbar: transport, undo/redo, tool modes (select/blade), marker,
 * split/duplicate/delete, zoom, snap and ripple toggles, save, and a trailing
 * status readout (ripple badge, selection count + duration, and an honest
 * save state that exposes a retry action after a failed write).
 */
import { ChevronsLeftRight, Copy, Flag, HelpCircle, Magnet, Maximize2, MousePointer2, Pause, Play, Redo2, Save, Scissors, Slice, Trash2, Undo2, ZoomIn, ZoomOut } from 'lucide-react';
import type { ReactNode } from 'react';

export function TimelineToolbar({
  isPlaying,
  snappingEnabled,
  rippleEnabled,
  zoom,
  isSaving,
  saveStatus,
  saveError,
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
  onToggleRipple,
  onSetToolMode,
  onAddMarker,
  onHelp,
  timecode,
  selectionSummary,
}: {
  isPlaying: boolean;
  snappingEnabled: boolean;
  rippleEnabled: boolean;
  zoom: number;
  isSaving: boolean;
  saveStatus: 'idle' | 'saving' | 'saved' | 'error';
  saveError?: string | null;
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
  onToggleRipple: () => void;
  onSetToolMode: (mode: 'select' | 'blade') => void;
  onAddMarker: () => void;
  onHelp: () => void;
  timecode?: string;
  selectionSummary?: string;
}) {
  return (
    <div className="flex min-h-16 shrink-0 flex-nowrap items-center gap-2 overflow-x-auto border-b border-border bg-surface-alt px-2 py-2">
      <ToolButton label="Select" title="Select tool (V)" onClick={() => onSetToolMode('select')} active={toolMode === 'select'}>
        <MousePointer2 size={17} />
      </ToolButton>
      <ToolButton label="Blade" title="Blade tool — click a clip to split (C)" onClick={() => onSetToolMode('blade')} active={toolMode === 'blade'}>
        <Slice size={17} />
      </ToolButton>
      <ToolButton label="Split" title="Split selected clip" onClick={onSplit}>
        <Scissors size={17} />
      </ToolButton>
      <ToolButton label="Delete" title="Delete selected clip" onClick={onDelete}>
        <Trash2 size={17} />
      </ToolButton>
      <ToolButton
        label="Ripple"
        title={rippleEnabled ? 'Disable ripple mode (R) — edits stop shifting later clips' : 'Enable ripple mode (R) — deletes/trims shift later clips to close gaps'}
        onClick={onToggleRipple}
        active={rippleEnabled}
      >
        <ChevronsLeftRight size={17} />
      </ToolButton>
      <span className="mx-1 h-9 w-px shrink-0 bg-border" />
      <IconButton label="Undo" onClick={onUndo} disabled={!canUndo}>
        <Undo2 size={15} />
      </IconButton>
      <IconButton label="Redo" onClick={onRedo} disabled={!canRedo}>
        <Redo2 size={15} />
      </IconButton>
      <IconButton label="Add marker at playhead (M)" onClick={onAddMarker}>
        <Flag size={15} />
      </IconButton>
      <IconButton label="Duplicate selected clip" onClick={onDuplicate}>
        <Copy size={15} />
      </IconButton>
      <span className="mx-1 h-9 w-px shrink-0 bg-border" />
      <IconButton label={isPlaying ? 'Pause' : 'Play'} onClick={onPlayPause}>
        {isPlaying ? <Pause size={15} /> : <Play size={15} />}
      </IconButton>
      {timecode && (
        <span className="shrink-0 rounded-md border border-border bg-surface px-2 py-1 font-mono text-[11px] tabular-nums text-text-secondary" title="Playhead / total (min:sec.frames)">
          {timecode}
        </span>
      )}
      <span className="mx-1 h-9 w-px shrink-0 bg-border" />
      <IconButton label="Zoom out" onClick={() => onZoom(zoom - 0.2)}>
        <ZoomOut size={15} />
      </IconButton>
      <select
        value={[0.5, 1, 1.5, 2, 3].some((value) => Math.abs(value - zoom) < 0.05) ? zoom.toFixed(1) : 'custom'}
        onChange={(event) => onZoom(Number(event.target.value))}
        className="h-8 shrink-0 rounded-md border border-border bg-surface px-2 text-xs text-text-secondary"
        aria-label="Timeline zoom preset"
      >
        <option value="custom" disabled>{Math.round(zoom * 100)}%</option>
        {[0.5, 1, 1.5, 2, 3].map((value) => (
          <option key={value} value={value.toFixed(1)}>{value === 1 ? '1/1' : value < 1 ? '1/2' : `${value}x`}</option>
        ))}
      </select>
      <input
        type="range"
        min={0.5}
        max={3}
        step={0.1}
        value={Math.max(0.5, Math.min(3, zoom))}
        onChange={(event) => onZoom(Number(event.target.value))}
        className="w-36 shrink-0"
        aria-label="Timeline zoom"
      />
      <IconButton label="Zoom in" onClick={() => onZoom(zoom + 0.2)}>
        <ZoomIn size={15} />
      </IconButton>
      <IconButton label="Zoom to fit" onClick={onZoomToFit}>
        <Maximize2 size={15} />
      </IconButton>
      <button
        type="button"
        onClick={onToggleSnap}
        className={`inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border px-2 text-xs transition-colors ${
          snappingEnabled ? 'border-primary/45 bg-primary/12 text-primary' : 'border-border bg-surface text-text-muted hover:text-text'
        }`}
        title={snappingEnabled ? 'Disable snapping' : 'Enable snapping'}
        aria-label={snappingEnabled ? 'Disable snapping' : 'Enable snapping'}
      >
        <Magnet size={14} />
        Snap
      </button>
      <span className="ml-auto flex shrink-0 items-center gap-2 text-[10px] text-text-muted">
        <IconButton label="Save timeline" onClick={onSave} active={isSaving}>
          <Save size={15} className={isSaving ? 'animate-pulse' : ''} />
        </IconButton>
        <IconButton label="Keyboard shortcuts (?)" onClick={onHelp}>
          <HelpCircle size={15} />
        </IconButton>
        {rippleEnabled && (
          <span className="rounded bg-primary/15 px-1.5 py-0.5 font-semibold uppercase tracking-wide text-primary" title="Ripple mode is on — deletes and trims shift later clips">
            Ripple
          </span>
        )}
        {selectionSummary && <span className="tabular-nums">{selectionSummary}</span>}
        <button
          type="button"
          onClick={saveStatus === 'error' ? onSave : undefined}
          disabled={saveStatus !== 'error'}
          className={isSaving ? 'text-amber-300' : saveStatus === 'error' ? 'text-red-300 hover:text-red-200' : 'text-text-muted'}
          title={saveStatus === 'error' ? `${saveError || 'Timeline save failed'} — click to retry` : undefined}
        >
          {isSaving ? 'Saving…' : saveStatus === 'error' ? 'Save failed · Retry' : saveStatus === 'idle' ? 'Not saved' : 'Saved'}
        </button>
      </span>
    </div>
  );
}

function ToolButton({
  label,
  title,
  active = false,
  disabled = false,
  onClick,
  children,
}: {
  label: string;
  title: string;
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
      className={`flex h-12 min-w-14 shrink-0 flex-col items-center justify-center gap-0.5 rounded-md border px-2 text-[10px] transition-colors hover:text-text disabled:cursor-not-allowed disabled:opacity-40 ${
        active ? 'border-primary/45 bg-primary/12 text-primary' : 'border-border bg-surface text-text-muted'
      }`}
      title={title}
      aria-label={title}
    >
      {children}
      <span>{label}</span>
    </button>
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
      className={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md border text-text-muted transition-colors hover:text-text disabled:cursor-not-allowed disabled:opacity-40 ${
        active ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-surface'
      }`}
      title={label}
      aria-label={label}
    >
      {children}
    </button>
  );
}
