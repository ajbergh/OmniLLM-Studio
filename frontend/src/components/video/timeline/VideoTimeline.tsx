import { useEffect, useMemo, useRef, useState } from 'react';
import { Plus } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import type { VideoTimelineTrackType } from '../../../types/video';
import { TimelinePlayhead } from './TimelinePlayhead';
import { TimelineRuler } from './TimelineRuler';
import { TimelineToolbar } from './TimelineToolbar';
import { TimelineTrack } from './TimelineTrack';

const TRACK_HEADER_WIDTH = 116;
const ADDABLE_TRACK_TYPES: VideoTimelineTrackType[] = ['video', 'image', 'audio', 'music', 'text', 'caption'];

export function VideoTimeline() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const selectedClipIds = useVideoStudioStore((state) => state.selectedClipIds);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const zoom = useVideoStudioStore((state) => state.zoom);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const isSavingTimeline = useVideoStudioStore((state) => state.isSavingTimeline);
  const canUndo = useVideoStudioStore((state) => state.timelineUndoStack.length > 0);
  const canRedo = useVideoStudioStore((state) => state.timelineRedoStack.length > 0);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const setZoom = useVideoStudioStore((state) => state.setZoom);
  const zoomToFit = useVideoStudioStore((state) => state.zoomToFit);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);
  const toggleSnapping = useVideoStudioStore((state) => state.toggleSnapping);
  const undoTimeline = useVideoStudioStore((state) => state.undoTimeline);
  const redoTimeline = useVideoStudioStore((state) => state.redoTimeline);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);
  const moveClip = useVideoStudioStore((state) => state.moveClip);
  const trimClip = useVideoStudioStore((state) => state.trimClip);
  const splitClipAtPlayhead = useVideoStudioStore((state) => state.splitClipAtPlayhead);
  const deleteClip = useVideoStudioStore((state) => state.deleteClip);
  const duplicateClip = useVideoStudioStore((state) => state.duplicateClip);
  const saveTimeline = useVideoStudioStore((state) => state.saveTimeline);
  const toggleTrackMute = useVideoStudioStore((state) => state.toggleTrackMute);
  const toggleTrackLock = useVideoStudioStore((state) => state.toggleTrackLock);
  const toggleTrackVisibility = useVideoStudioStore((state) => state.toggleTrackVisibility);
  const addTrack = useVideoStudioStore((state) => state.addTrack);
  const removeTrack = useVideoStudioStore((state) => state.removeTrack);
  const renameTrack = useVideoStudioStore((state) => state.renameTrack);
  const reorderTrack = useVideoStudioStore((state) => state.reorderTrack);
  const setTrackHeight = useVideoStudioStore((state) => state.setTrackHeight);
  const addMarker = useVideoStudioStore((state) => state.addMarker);
  const removeMarker = useVideoStudioStore((state) => state.removeMarker);
  const nudgeSelection = useVideoStudioStore((state) => state.nudgeSelection);
  const toolMode = useVideoStudioStore((state) => state.toolMode);
  const setToolMode = useVideoStudioStore((state) => state.setToolMode);
  const splitClipAt = useVideoStudioStore((state) => state.splitClipAt);
  const trimClipEdgeToPlayhead = useVideoStudioStore((state) => state.trimClipEdgeToPlayhead);
  const groupClips = useVideoStudioStore((state) => state.groupClips);
  const ungroupClips = useVideoStudioStore((state) => state.ungroupClips);
  const alignSelection = useVideoStudioStore((state) => state.alignSelection);
  const updateClipFade = useVideoStudioStore((state) => state.updateClipFade);
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const bringClipForward = useVideoStudioStore((state) => state.bringClipForward);
  const sendClipBackward = useVideoStudioStore((state) => state.sendClipBackward);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const addTrackRef = useRef<HTMLDivElement | null>(null);
  const [addTrackOpen, setAddTrackOpen] = useState(false);
  const [clipMenu, setClipMenu] = useState<{ clipId: string; trackId: string; x: number; y: number } | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const pxPerMs = useMemo(() => 0.02 * zoom, [zoom]);
  const width = Math.max(900, (timeline?.duration_ms || 30000) * pxPerMs);

  // Snap targets: every clip edge plus the playhead.
  const snapPointsMs = useMemo(() => {
    if (!timeline) return [] as number[];
    const points = new Set<number>([0, playheadMs]);
    for (const track of timeline.tracks) {
      for (const clip of track.clips) {
        points.add(clip.start_ms);
        points.add(clip.start_ms + clip.duration_ms);
      }
    }
    for (const marker of timeline.markers || []) {
      points.add(marker.time_ms);
    }
    return Array.from(points);
  }, [timeline, playheadMs]);

  useEffect(() => {
    if (!addTrackOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!addTrackRef.current?.contains(event.target as Node)) setAddTrackOpen(false);
    };
    window.addEventListener('pointerdown', onPointerDown);
    return () => window.removeEventListener('pointerdown', onPointerDown);
  }, [addTrackOpen]);

  useEffect(() => {
    if (!clipMenu) return;
    const onPointerDown = () => setClipMenu(null);
    window.addEventListener('pointerdown', onPointerDown);
    return () => window.removeEventListener('pointerdown', onPointerDown);
  }, [clipMenu]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable) return;
      if (event.key === ' ') {
        event.preventDefault();
        setPlaying(!useVideoStudioStore.getState().isPlaying);
      } else if (event.key.toLowerCase() === 'z' && (event.ctrlKey || event.metaKey) && event.shiftKey) {
        event.preventDefault();
        void redoTimeline();
      } else if (event.key.toLowerCase() === 'z' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void undoTimeline();
      } else if (event.key.toLowerCase() === 'y' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void redoTimeline();
      } else if (event.key === 'Delete' || event.key === 'Backspace') {
        void deleteClip();
      } else if (event.key.toLowerCase() === 's' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void saveTimeline();
      } else if (event.key.toLowerCase() === 's') {
        void splitClipAtPlayhead();
      } else if (event.key.toLowerCase() === 'm') {
        void addMarker();
      } else if (event.key.toLowerCase() === 'c') {
        setToolMode('blade');
      } else if (event.key.toLowerCase() === 'v') {
        setToolMode('select');
      } else if (event.key === '[') {
        void trimClipEdgeToPlayhead('start');
      } else if (event.key === ']') {
        void trimClipEdgeToPlayhead('end');
      } else if (event.key === '?') {
        setShowHelp((open) => !open);
      } else if (event.key === '+' || event.key === '=') {
        setZoom(useVideoStudioStore.getState().zoom * 1.25);
      } else if (event.key === '-' || event.key === '_') {
        setZoom(useVideoStudioStore.getState().zoom / 1.25);
      } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        const state = useVideoStudioStore.getState();
        if (state.selectedClipIds.length === 0 && !state.selectedClipId) return;
        event.preventDefault();
        const frameMs = Math.max(1, Math.round(1000 / (state.timeline?.canvas.fps || 30)));
        const step = (event.shiftKey ? 10 : 1) * frameMs * (event.key === 'ArrowLeft' ? -1 : 1);
        void nudgeSelection(step);
      } else if (event.key === 'Escape') {
        setShowHelp(false);
        setClipMenu(null);
        selectClip(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [addMarker, deleteClip, nudgeSelection, redoTimeline, saveTimeline, selectClip, setPlaying, setToolMode, setZoom, splitClipAtPlayhead, trimClipEdgeToPlayhead, undoTimeline]);

  if (!timeline) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-text-muted">
        Timeline will load with the active project.
      </div>
    );
  }

  const hasClips = timeline.tracks.some((track) => track.clips.length > 0);

  const handleZoomToFit = () => {
    const container = scrollRef.current;
    if (!container) return;
    zoomToFit(Math.max(200, container.clientWidth - TRACK_HEADER_WIDTH - 24));
  };

  return (
    <div className="flex h-full min-h-0 flex-col rounded-lg border border-border bg-surface">
      <TimelineToolbar
        isPlaying={isPlaying}
        snappingEnabled={snappingEnabled}
        zoom={zoom}
        isSaving={isSavingTimeline}
        canUndo={canUndo}
        canRedo={canRedo}
        toolMode={toolMode}
        onPlayPause={() => setPlaying(!isPlaying)}
        onUndo={() => { void undoTimeline(); }}
        onRedo={() => { void redoTimeline(); }}
        onSplit={() => { void splitClipAtPlayhead(); }}
        onDelete={() => { void deleteClip(); }}
        onDuplicate={() => { void duplicateClip(); }}
        onSave={() => { void saveTimeline(); }}
        onZoom={setZoom}
        onZoomToFit={handleZoomToFit}
        onToggleSnap={toggleSnapping}
        onSetToolMode={setToolMode}
        onHelp={() => setShowHelp(true)}
      />
      <div ref={scrollRef} className="relative min-h-0 flex-1 overflow-auto">
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div ref={addTrackRef} className="relative flex h-8 items-center border-b border-r border-border bg-surface-alt px-1">
            <button
              className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-text-muted hover:text-text"
              title="Add track"
              aria-label="Add track"
              onClick={() => setAddTrackOpen((open) => !open)}
            >
              <Plus size={11} />
              Track
            </button>
            {addTrackOpen && (
              <div className="absolute left-1 top-full z-30 w-32 rounded-md border border-border bg-surface p-1 shadow-lg">
                {ADDABLE_TRACK_TYPES.map((type) => (
                  <button
                    key={type}
                    className="block w-full rounded px-2 py-1 text-left text-[11px] capitalize text-text-secondary hover:bg-surface-alt hover:text-text"
                    onClick={() => {
                      setAddTrackOpen(false);
                      void addTrack(type);
                    }}
                  >
                    {type}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div className="relative">
            <TimelineRuler
              durationMs={timeline.duration_ms}
              pxPerMs={pxPerMs}
              markers={timeline.markers}
              onSeek={setPlayhead}
              onRemoveMarker={(markerId) => { void removeMarker(markerId); }}
            />
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div className="border-r border-border bg-surface-alt" />
          <div className="relative">
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        {timeline.tracks.map((track, trackIndex) => (
          <TimelineTrack
            key={track.id}
            track={track}
            trackIndex={trackIndex}
            trackCount={timeline.tracks.length}
            assets={assets}
            selectedClipIds={selectedClipIds}
            pxPerMs={pxPerMs}
            width={width}
            snappingEnabled={snappingEnabled}
            snapPointsMs={snapPointsMs}
            onMoveClip={(clipId, trackId, startMs) => { void moveClip(clipId, trackId, startMs); }}
            onTrimClip={(clipId, updates) => { void trimClip(clipId, updates); }}
            onAddAsset={(assetId, trackId, trackType, startMs) => {
              void addAssetToTimeline(assetId, { track_id: trackId, track_type: trackType, start_ms: startMs });
            }}
            onSelectClip={(clipId, trackId, additive) => selectClip(clipId || null, clipId ? trackId : null, additive)}
            onToggleMute={(trackId) => { void toggleTrackMute(trackId); }}
            onToggleLock={(trackId) => { void toggleTrackLock(trackId); }}
            onToggleVisibility={(trackId) => { void toggleTrackVisibility(trackId); }}
            onRenameTrack={(trackId, name) => { void renameTrack(trackId, name); }}
            onReorderTrack={(trackId, targetIndex) => { void reorderTrack(trackId, targetIndex); }}
            onRemoveTrack={(trackId) => {
              const target = timeline.tracks.find((item) => item.id === trackId);
              if (target && target.clips.length > 0 && !window.confirm(`Remove "${target.name}" and its ${target.clips.length} clip(s)?`)) {
                return;
              }
              void removeTrack(trackId);
            }}
            onSetTrackHeight={(trackId, height) => { void setTrackHeight(trackId, height); }}
            toolMode={toolMode}
            onSplitAt={(clipId, timeMs) => { void splitClipAt(clipId, timeMs); }}
            onClipContextMenu={(clipId, trackId, x, y) => {
              if (!useVideoStudioStore.getState().selectedClipIds.includes(clipId)) {
                selectClip(clipId, trackId);
              }
              setClipMenu({ clipId, trackId, x, y });
            }}
          />
        ))}
        {!hasClips && (
          <div className="pointer-events-none absolute inset-x-0 bottom-3 flex justify-center">
            <span className="rounded-md border border-dashed border-border bg-surface-alt/90 px-3 py-1.5 text-[11px] text-text-muted">
              Drag media from the bin onto a track, or generate a video and use “Send to Timeline”.
            </span>
          </div>
        )}
      </div>
      {clipMenu && (() => {
        const menuClip = timeline.tracks.flatMap((track) => track.clips).find((clip) => clip.id === clipMenu.clipId);
        if (!menuClip) return null;
        const multi = selectedClipIds.length >= 2;
        const items: Array<{ label: string; action: () => void } | 'divider'> = [
          { label: 'Split at playhead (S)', action: () => { void splitClipAt(clipMenu.clipId, playheadMs); } },
          { label: 'Trim start to playhead [', action: () => { void trimClipEdgeToPlayhead('start'); } },
          { label: 'Trim end to playhead ]', action: () => { void trimClipEdgeToPlayhead('end'); } },
          'divider',
          { label: multi ? `Duplicate ${selectedClipIds.length} clips` : 'Duplicate', action: () => { void duplicateClip(multi ? undefined : clipMenu.clipId); } },
          { label: 'Bring forward', action: () => { void bringClipForward(clipMenu.clipId); } },
          { label: 'Send backward', action: () => { void sendClipBackward(clipMenu.clipId); } },
          { label: 'Add fade in/out', action: () => { void updateClipFade(clipMenu.clipId, { fade_in_ms: 500, fade_out_ms: 500 }); } },
          { label: 'Add fade transition', action: () => { void addClipTransition(clipMenu.clipId, { type: 'fade', duration_ms: 500 }); } },
        ];
        if (multi && !menuClip.group_id) items.push({ label: `Group ${selectedClipIds.length} clips`, action: () => { void groupClips(); } });
        if (menuClip.group_id) items.push({ label: 'Ungroup', action: () => { void ungroupClips(menuClip.group_id); } });
        if (multi) {
          items.push('divider');
          items.push({ label: 'Align starts', action: () => { void alignSelection('start'); } });
          items.push({ label: 'Align ends', action: () => { void alignSelection('end'); } });
          if (selectedClipIds.length >= 3) items.push({ label: 'Distribute evenly', action: () => { void alignSelection('distribute'); } });
        }
        items.push('divider');
        items.push({ label: multi ? `Delete ${selectedClipIds.length} clips` : 'Delete', action: () => { void deleteClip(multi ? undefined : clipMenu.clipId); } });
        return (
          <div
            className="fixed z-50 w-44 rounded-md border border-border bg-surface p-1 shadow-xl"
            style={{ left: Math.min(clipMenu.x, window.innerWidth - 190), top: Math.min(clipMenu.y, window.innerHeight - items.length * 26 - 16) }}
            onPointerDown={(event) => event.stopPropagation()}
          >
            {items.map((item, index) =>
              item === 'divider' ? (
                <div key={`divider-${index}`} className="my-1 h-px bg-border" />
              ) : (
                <button
                  key={item.label}
                  className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-secondary hover:bg-surface-alt hover:text-text"
                  onClick={() => {
                    setClipMenu(null);
                    item.action();
                  }}
                >
                  {item.label}
                </button>
              ),
            )}
          </div>
        );
      })()}
      {showHelp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowHelp(false)}>
          <div className="max-h-[80vh] w-80 overflow-y-auto rounded-lg border border-border bg-surface p-4 shadow-xl" onClick={(event) => event.stopPropagation()}>
            <h3 className="mb-3 text-sm font-semibold text-text">Timeline shortcuts</h3>
            <dl className="space-y-1.5">
              {[
                ['Space', 'Play / pause'],
                ['Ctrl/Cmd+Z', 'Undo'],
                ['Ctrl/Cmd+Shift+Z · Ctrl/Cmd+Y', 'Redo'],
                ['Ctrl/Cmd+S', 'Save timeline'],
                ['S', 'Split selected clip at playhead'],
                ['C / V', 'Blade tool / Select tool'],
                ['[ / ]', 'Trim clip start / end to playhead'],
                ['M', 'Add marker at playhead'],
                ['+ / -', 'Zoom in / out'],
                ['← / →', 'Nudge selection by 1 frame'],
                ['Shift+← / →', 'Nudge selection by 10 frames'],
                ['Delete / Backspace', 'Delete selection'],
                ['Ctrl/Cmd/Shift+Click', 'Multi-select clips'],
                ['Escape', 'Deselect / close menus'],
                ['Right-click clip', 'Clip actions menu'],
                ['Right-click track name', 'Track actions menu'],
                ['Right-click marker', 'Remove marker'],
                ['Double-click track name', 'Rename track'],
              ].map(([keys, description]) => (
                <div key={keys} className="flex items-baseline justify-between gap-3">
                  <dt className="shrink-0 rounded bg-surface-alt px-1.5 py-0.5 font-mono text-[10px] text-text-secondary">{keys}</dt>
                  <dd className="text-right text-[11px] text-text-muted">{description}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      )}
    </div>
  );
}
