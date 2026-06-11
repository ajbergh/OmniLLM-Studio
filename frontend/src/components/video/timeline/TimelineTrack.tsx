/**
 * One timeline layer row: a sticky header (rename, mute/lock/visibility, solo
 * badge, drag-to-resize height) plus the clip lane, which handles asset/clip
 * drag-and-drop with snapping to clip edges, markers, and the playhead.
 */
import { useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { Eye, EyeOff, Lock, Unlock, Volume2, VolumeX } from 'lucide-react';
import type { VideoAsset, VideoTimelineClip, VideoTimelineTrack as Track, VideoTimelineTrackType } from '../../../types/video';
import { TimelineClip } from './TimelineClip';

// Snap a drop position to nearby clip edges / playhead within this pixel radius.
const SNAP_RADIUS_PX = 8;
const DEFAULT_TRACK_HEIGHT = 52;

/** A snap target with what it represents, so drag guides can say why they snapped. */
export interface SnapPoint {
  ms: number;
  kind: 'playhead' | 'marker' | 'clip' | 'edge';
}

const SNAP_GUIDE_STYLES: Record<SnapPoint['kind'], { line: string; label: string }> = {
  playhead: { line: 'bg-primary/90', label: 'playhead' },
  marker: { line: 'bg-amber-400/90', label: 'marker' },
  clip: { line: 'bg-sky-400/90', label: 'clip edge' },
  edge: { line: 'bg-white/70', label: 'timeline' },
};

/** Nearest snap point within the radius, or null when nothing is close enough. */
function snapToPoints(startMs: number, snapPoints: SnapPoint[], pxPerMs: number): SnapPoint | null {
  let best: SnapPoint | null = null;
  let bestDistPx = SNAP_RADIUS_PX + 1;
  for (const point of snapPoints) {
    const distPx = Math.abs(point.ms - startMs) * pxPerMs;
    if (distPx < bestDistPx) {
      best = point;
      bestDistPx = distPx;
    }
  }
  return best;
}

export function TimelineTrack({
  track,
  assets,
  selectedClipIds,
  pxPerMs,
  width,
  snappingEnabled,
  snapPoints,
  soloActive = false,
  onMoveClip,
  onTrimClip,
  onAddAsset,
  onSelectClip,
  onToggleMute,
  onToggleLock,
  onToggleVisibility,
  onRenameTrack,
  onSetTrackHeight,
  toolMode = 'select',
  onSplitAt,
  onClipContextMenu,
  onEdgeContextMenu,
  onHeaderContextMenu,
  onLaneContextMenu,
  onFadeClip,
  onUpdateClipKeyframe,
  onTransitionContextMenu,
}: {
  track: Track;
  assets: VideoAsset[];
  selectedClipIds: string[];
  pxPerMs: number;
  width: number;
  snappingEnabled: boolean;
  snapPoints: SnapPoint[];
  soloActive?: boolean;
  onMoveClip: (clipId: string, trackId: string, startMs: number) => void;
  onTrimClip: (clipId: string, updates: Partial<Pick<VideoTimelineClip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => void;
  onAddAsset: (assetId: string, trackId: string, trackType: VideoTimelineTrackType, startMs: number) => void;
  onSelectClip: (clipId: string, trackId: string, additive?: boolean) => void;
  onToggleMute: (trackId: string) => void;
  onToggleLock: (trackId: string) => void;
  onToggleVisibility: (trackId: string) => void;
  onRenameTrack: (trackId: string, name: string) => void;
  onSetTrackHeight: (trackId: string, height: number) => void;
  toolMode?: 'select' | 'blade';
  onSplitAt?: (clipId: string, timeMs: number) => void;
  onClipContextMenu?: (clipId: string, trackId: string, clientX: number, clientY: number) => void;
  onEdgeContextMenu?: (clipId: string, trackId: string, edge: 'start' | 'end', clientX: number, clientY: number) => void;
  onFadeClip?: (clipId: string, fade: { fade_in_ms?: number; fade_out_ms?: number }) => void;
  onUpdateClipKeyframe?: (clipId: string, keyframeId: string, patch: Partial<Omit<VideoTimelineClip['keyframes'][number], 'id'>>) => void;
  onTransitionContextMenu?: (clipId: string, trackId: string, clientX: number, clientY: number) => void;
  onHeaderContextMenu?: (trackId: string, clientX: number, clientY: number) => void;
  onLaneContextMenu?: (trackId: string, timeMs: number, clientX: number, clientY: number) => void;
}) {
  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState(track.name);
  const [dropGuide, setDropGuide] = useState<{ x: number; kind: SnapPoint['kind'] | null } | null>(null);
  const [liveHeight, setLiveHeight] = useState<number | null>(null);
  // Mirrors liveHeight so pointer-up can commit without a side effect inside
  // a setState updater (StrictMode runs updaters twice in dev).
  const liveHeightRef = useRef<number | null>(null);
  const headerRef = useRef<HTMLDivElement | null>(null);
  const trackHeight = liveHeight ?? track.height ?? DEFAULT_TRACK_HEIGHT;

  // Drag the strip at the bottom of the header to resize; commit once on release.
  const beginHeightDrag = (event: ReactPointerEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const pointerId = event.pointerId;
    const startY = event.clientY;
    const base = track.height || DEFAULT_TRACK_HEIGHT;
    const onMove = (moveEvent: PointerEvent) => {
      if (moveEvent.pointerId !== pointerId) return;
      const next = Math.max(32, Math.min(160, Math.round(base + (moveEvent.clientY - startY))));
      liveHeightRef.current = next;
      setLiveHeight(next);
    };
    const onUp = (upEvent: PointerEvent) => {
      if (upEvent.pointerId !== pointerId) return;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      const height = liveHeightRef.current;
      if (height !== null && height !== (track.height || DEFAULT_TRACK_HEIGHT)) {
        onSetTrackHeight(track.id, height);
      }
      liveHeightRef.current = null;
      setLiveHeight(null);
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  const commitRename = () => {
    setRenaming(false);
    if (draftName.trim() && draftName.trim() !== track.name) {
      onRenameTrack(track.id, draftName);
    }
  };

  return (
    <div className="grid grid-cols-[116px_minmax(0,1fr)] border-b border-border last:border-b-0" style={{ minHeight: trackHeight }}>
      <div
        ref={headerRef}
        className="sticky left-0 z-20 flex min-w-0 items-center gap-1 border-r border-border bg-surface-alt px-2"
        onContextMenu={(event) => {
          event.preventDefault();
          onHeaderContextMenu?.(track.id, event.clientX, event.clientY);
        }}
      >
        {renaming ? (
          <input
            autoFocus
            className="min-w-0 flex-1 rounded border border-border bg-surface px-1 text-[11px] text-text"
            value={draftName}
            onChange={(event) => setDraftName(event.target.value)}
            onBlur={commitRename}
            onKeyDown={(event) => {
              if (event.key === 'Enter') commitRename();
              if (event.key === 'Escape') {
                setDraftName(track.name);
                setRenaming(false);
              }
              event.stopPropagation();
            }}
          />
        ) : (
          <span
            className="min-w-0 flex-1 truncate text-[11px] font-medium text-text-secondary"
            title={`${track.name} — double-click to rename, right-click for track options`}
            onDoubleClick={() => {
              setDraftName(track.name);
              setRenaming(true);
            }}
          >
            {track.name}
          </span>
        )}
        <button
          onClick={() => onToggleMute(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.muted ? 'Unmute track' : 'Mute track'}
          aria-label={track.muted ? 'Unmute track' : 'Mute track'}
        >
          {track.muted ? <VolumeX size={12} /> : <Volume2 size={12} />}
        </button>
        <button
          onClick={() => onToggleLock(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.locked ? 'Unlock track' : 'Lock track'}
          aria-label={track.locked ? 'Unlock track' : 'Lock track'}
        >
          {track.locked ? <Lock size={12} /> : <Unlock size={12} />}
        </button>
        <button
          onClick={() => onToggleVisibility(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.visible ? 'Hide track' : 'Show track'}
          aria-label={track.visible ? 'Hide track' : 'Show track'}
        >
          {track.visible ? <Eye size={12} /> : <EyeOff size={12} />}
        </button>
        {soloActive && (
          <span className="rounded bg-amber-400/20 px-1 text-[9px] font-semibold text-amber-300" title="Solo — only this layer plays audio">
            S
          </span>
        )}
        <div
          className="absolute inset-x-0 bottom-0 h-1.5 cursor-ns-resize hover:bg-primary/30"
          onPointerDown={beginHeightDrag}
          title="Drag to resize track height"
        />
      </div>
      <div
        className="relative bg-surface"
        style={{ width }}
        data-lane={track.id}
        onDragOver={(event) => {
          if (!track.locked && (event.dataTransfer.types.includes('application/x-video-clip-id') || event.dataTransfer.types.includes('application/x-video-asset-id'))) {
            event.preventDefault();
            const rect = event.currentTarget.getBoundingClientRect();
            // Without the grab offset (unavailable during dragover) the guide marks
            // the cursor position, snapped to the same targets the drop will use.
            let guideMs = Math.max(0, Math.round((event.clientX - rect.left) / pxPerMs));
            let guideKind: SnapPoint['kind'] | null = null;
            if (snappingEnabled) {
              const snapped = snapToPoints(guideMs, snapPoints, pxPerMs);
              if (snapped) {
                guideMs = Math.max(0, snapped.ms);
                guideKind = snapped.kind;
              }
            }
            setDropGuide({ x: guideMs * pxPerMs, kind: guideKind });
          }
        }}
        onDragLeave={() => setDropGuide(null)}
        onDrop={(event) => {
          event.preventDefault();
          setDropGuide(null);
          if (track.locked) return;
          const clipId = event.dataTransfer.getData('application/x-video-clip-id');
          const assetId = event.dataTransfer.getData('application/x-video-asset-id');
          const grabOffsetPx = Number(event.dataTransfer.getData('application/x-video-clip-grab-offset')) || 0;
          const rect = event.currentTarget.getBoundingClientRect();
          let startMs = Math.max(0, Math.round((event.clientX - rect.left - grabOffsetPx) / pxPerMs));
          if (snappingEnabled) {
            const snapped = snapToPoints(startMs, snapPoints, pxPerMs);
            if (snapped) startMs = Math.max(0, snapped.ms);
          }
          if (clipId) {
            onMoveClip(clipId, track.id, startMs);
          } else if (assetId) {
            onAddAsset(assetId, track.id, track.type, startMs);
          }
        }}
        onClick={() => onSelectClip('', track.id)}
        onContextMenu={(event) => {
          if (!onLaneContextMenu) return;
          event.preventDefault();
          const rect = event.currentTarget.getBoundingClientRect();
          onLaneContextMenu(track.id, Math.max(0, Math.round((event.clientX - rect.left) / pxPerMs)), event.clientX, event.clientY);
        }}
      >
        {dropGuide !== null && (
          <div className="pointer-events-none absolute top-0 z-10 h-full" style={{ left: dropGuide.x }}>
            <div className={`h-full w-px ${dropGuide.kind ? SNAP_GUIDE_STYLES[dropGuide.kind].line : 'bg-primary/50'}`} />
            {dropGuide.kind && (
              <span className={`absolute left-1 top-0.5 rounded px-1 text-[8px] font-semibold uppercase tracking-wide text-black/80 ${SNAP_GUIDE_STYLES[dropGuide.kind].line}`}>
                {SNAP_GUIDE_STYLES[dropGuide.kind].label}
              </span>
            )}
          </div>
        )}
        {track.clips.map((clip) => (
          <TimelineClip
            key={clip.id}
            clip={clip}
            asset={assets.find((asset) => asset.id === clip.asset_id)}
            selected={selectedClipIds.includes(clip.id)}
            pxPerMs={pxPerMs}
            trackId={track.id}
            toolMode={toolMode}
            onSelect={onSelectClip}
            onTrim={onTrimClip}
            onSplitAt={onSplitAt}
            onContextMenu={onClipContextMenu}
            onEdgeContextMenu={onEdgeContextMenu}
            onFade={track.locked ? undefined : onFadeClip}
            onUpdateKeyframe={track.locked ? undefined : onUpdateClipKeyframe}
            onTransitionContextMenu={onTransitionContextMenu}
          />
        ))}
      </div>
    </div>
  );
}
