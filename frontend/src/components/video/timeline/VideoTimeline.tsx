import { useEffect, useMemo, useRef, useState } from 'react';
import { Plus } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { editorModeFeatures } from '../editorModes';
import type { VideoTimelineTrackType } from '../../../types/video';
import { ContextMenu } from '../../common/ContextMenu';
import type { ContextMenuEntry } from '../../common/ContextMenu';
import { TimelinePlayhead } from './TimelinePlayhead';
import { TimelineRuler } from './TimelineRuler';
import { TimelineToolbar } from './TimelineToolbar';
import { TimelineTrack } from './TimelineTrack';

type TimelineMenuState =
  | { kind: 'clip'; clipId: string; trackId: string; x: number; y: number }
  | { kind: 'track'; trackId: string; x: number; y: number }
  | { kind: 'lane'; trackId: string; timeMs: number; x: number; y: number }
  | { kind: 'ruler'; timeMs: number; x: number; y: number };

const TRACK_HEADER_WIDTH = 116;
const ADDABLE_TRACK_TYPES: VideoTimelineTrackType[] = ['layer', 'video', 'image', 'audio', 'music', 'text', 'caption'];

function formatTimecode(ms: number, fps: number): string {
  const clamped = Math.max(0, ms);
  const minutes = Math.floor(clamped / 60_000);
  const seconds = Math.floor((clamped % 60_000) / 1000);
  const frames = Math.floor(((clamped % 1000) / 1000) * Math.max(1, fps));
  return `${minutes}:${String(seconds).padStart(2, '0')}.${String(frames).padStart(2, '0')}`;
}

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
  const editorMode = useVideoStudioStore((state) => state.editorMode);
  const splitClipAt = useVideoStudioStore((state) => state.splitClipAt);
  const trimClipEdgeToPlayhead = useVideoStudioStore((state) => state.trimClipEdgeToPlayhead);
  const groupClips = useVideoStudioStore((state) => state.groupClips);
  const ungroupClips = useVideoStudioStore((state) => state.ungroupClips);
  const alignSelection = useVideoStudioStore((state) => state.alignSelection);
  const updateClipFade = useVideoStudioStore((state) => state.updateClipFade);
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const bringClipForward = useVideoStudioStore((state) => state.bringClipForward);
  const sendClipBackward = useVideoStudioStore((state) => state.sendClipBackward);
  const copySelection = useVideoStudioStore((state) => state.copySelection);
  const cutSelection = useVideoStudioStore((state) => state.cutSelection);
  const pasteClips = useVideoStudioStore((state) => state.pasteClips);
  const copyClipAttributes = useVideoStudioStore((state) => state.copyClipAttributes);
  const pasteClipAttributes = useVideoStudioStore((state) => state.pasteClipAttributes);
  const selectAllClips = useVideoStudioStore((state) => state.selectAllClips);
  const selectClipsRelativeToPlayhead = useVideoStudioStore((state) => state.selectClipsRelativeToPlayhead);
  const selectClipsOnTrack = useVideoStudioStore((state) => state.selectClipsOnTrack);
  const moveClipToAdjacentTrack = useVideoStudioStore((state) => state.moveClipToAdjacentTrack);
  const duplicateTrack = useVideoStudioStore((state) => state.duplicateTrack);
  const clearTrack = useVideoStudioStore((state) => state.clearTrack);
  const toggleTrackSolo = useVideoStudioStore((state) => state.toggleTrackSolo);
  const soloTrackId = useVideoStudioStore((state) => state.soloTrackId);
  const insertTrackAdjacent = useVideoStudioStore((state) => state.insertTrackAdjacent);
  const moveTrackToEdge = useVideoStudioStore((state) => state.moveTrackToEdge);
  const setTimelineDuration = useVideoStudioStore((state) => state.setTimelineDuration);
  const splitAllAtPlayhead = useVideoStudioStore((state) => state.splitAllAtPlayhead);
  const hasClipboard = useVideoStudioStore((state) => Boolean(state.clipClipboard?.length));
  const hasAttributeClipboard = useVideoStudioStore((state) => Boolean(state.attributeClipboard));
  const addTextClip = useVideoStudioStore((state) => state.addTextClip);
  const toggleClipMute = useVideoStudioStore((state) => state.toggleClipMute);
  const detachClipAudio = useVideoStudioStore((state) => state.detachClipAudio);
  const followPlayhead = useVideoStudioStore((state) => state.followPlayhead);
  const toggleFollowPlayhead = useVideoStudioStore((state) => state.toggleFollowPlayhead);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const addTrackRef = useRef<HTMLDivElement | null>(null);
  const [addTrackOpen, setAddTrackOpen] = useState(false);
  const [menu, setMenu] = useState<TimelineMenuState | null>(null);
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

  // Ctrl/Cmd + wheel zooms the timeline (native listener — React's synthetic
  // wheel handler can't preventDefault on passive listeners).
  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    const onWheel = (event: WheelEvent) => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      const factor = event.deltaY < 0 ? 1.15 : 1 / 1.15;
      setZoom(useVideoStudioStore.getState().zoom * factor);
    };
    node.addEventListener('wheel', onWheel, { passive: false });
    return () => node.removeEventListener('wheel', onWheel);
  }, [setZoom]);

  // Keep the playhead in view while playing (toggleable follow mode).
  useEffect(() => {
    if (!isPlaying || !followPlayhead) return;
    const node = scrollRef.current;
    if (!node) return;
    const playheadX = TRACK_HEADER_WIDTH + playheadMs * pxPerMs;
    const viewRight = node.scrollLeft + node.clientWidth;
    if (playheadX > viewRight - 80 || playheadX < node.scrollLeft + TRACK_HEADER_WIDTH) {
      node.scrollLeft = Math.max(0, playheadX - TRACK_HEADER_WIDTH - 40);
    }
  }, [playheadMs, isPlaying, pxPerMs, followPlayhead]);

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
      } else if (event.key.toLowerCase() === 'c' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        copySelection();
      } else if (event.key.toLowerCase() === 'x' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void cutSelection();
      } else if (event.key.toLowerCase() === 'v' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void pasteClips();
      } else if (event.key.toLowerCase() === 'a' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        selectAllClips();
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
      } else if (event.key.toLowerCase() === 'f') {
        const container = scrollRef.current;
        if (container) zoomToFit(Math.max(200, container.clientWidth - TRACK_HEADER_WIDTH - 24));
      } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        const state = useVideoStudioStore.getState();
        if (state.selectedClipIds.length === 0 && !state.selectedClipId) return;
        event.preventDefault();
        const frameMs = Math.max(1, Math.round(1000 / (state.timeline?.canvas.fps || 30)));
        const step = (event.shiftKey ? 10 : 1) * frameMs * (event.key === 'ArrowLeft' ? -1 : 1);
        void nudgeSelection(step);
      } else if (event.key === 'Escape') {
        setShowHelp(false);
        setMenu(null);
        selectClip(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [addMarker, copySelection, cutSelection, deleteClip, nudgeSelection, pasteClips, redoTimeline, saveTimeline, selectAllClips, selectClip, setPlaying, setToolMode, setZoom, splitClipAtPlayhead, trimClipEdgeToPlayhead, undoTimeline, zoomToFit]);

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
        timecode={`${formatTimecode(playheadMs, timeline.canvas.fps)} / ${formatTimecode(timeline.duration_ms, timeline.canvas.fps)}`}
      />
      <div ref={scrollRef} className="relative min-h-0 flex-1 overflow-auto">
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div ref={addTrackRef} className="sticky left-0 z-20 flex h-8 items-center border-b border-r border-border bg-surface-alt px-1">
            <button
              className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-text-muted hover:text-text ${editorModeFeatures(editorMode).addTrack ? '' : 'invisible'}`}
              title="Add track"
              aria-label="Add track"
              onClick={() => setAddTrackOpen((open) => !open)}
            >
              <Plus size={11} />
              Track
            </button>
            {addTrackOpen && editorModeFeatures(editorMode).addTrack && (
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
              onContextMenu={(timeMs, x, y) => setMenu({ kind: 'ruler', timeMs, x, y })}
            />
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div className="sticky left-0 z-20 border-r border-border bg-surface-alt" />
          <div className="relative">
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        {/* Tracks render top-down from the end of the array: the last track is
            the foreground layer, so it appears at the top of the list
            (Camtasia/Premiere convention). trackIndex stays the array index. */}
        {[...timeline.tracks].reverse().map((track) => (
          <TimelineTrack
            key={track.id}
            track={track}
            assets={assets}
            selectedClipIds={selectedClipIds}
            pxPerMs={pxPerMs}
            width={width}
            snappingEnabled={snappingEnabled}
            snapPointsMs={snapPointsMs}
            soloActive={soloTrackId === track.id}
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
            onSetTrackHeight={(trackId, height) => { void setTrackHeight(trackId, height); }}
            toolMode={toolMode}
            onSplitAt={(clipId, timeMs) => { void splitClipAt(clipId, timeMs); }}
            onClipContextMenu={(clipId, trackId, x, y) => {
              if (!useVideoStudioStore.getState().selectedClipIds.includes(clipId)) {
                selectClip(clipId, trackId);
              }
              setMenu({ kind: 'clip', clipId, trackId, x, y });
            }}
            onHeaderContextMenu={(trackId, x, y) => setMenu({ kind: 'track', trackId, x, y })}
            onLaneContextMenu={(trackId, timeMs, x, y) => setMenu({ kind: 'lane', trackId, timeMs, x, y })}
          />
        ))}
        {!hasClips && (
          <div className="pointer-events-none absolute inset-x-0 bottom-3 flex justify-center">
            <span className="rounded-md border border-dashed border-border bg-surface-alt/90 px-3 py-1.5 text-[11px] text-text-muted">
              Drag media from the bin onto a layer, or generate a video and use “Send to Timeline”.
            </span>
          </div>
        )}
      </div>
      {menu && (() => {
        let items: ContextMenuEntry[] = [];
        if (menu.kind === 'clip') {
          const menuClip = timeline.tracks.flatMap((track) => track.clips).find((clip) => clip.id === menu.clipId);
          if (!menuClip) return null;
          const multi = selectedClipIds.length >= 2;
          items = [
            { label: 'Split at playhead', shortcut: 'S', action: () => { void splitClipAt(menu.clipId, playheadMs); } },
            { label: 'Trim start to playhead', shortcut: '[', action: () => { void trimClipEdgeToPlayhead('start'); } },
            { label: 'Trim end to playhead', shortcut: ']', action: () => { void trimClipEdgeToPlayhead('end'); } },
            'divider',
            { label: multi ? `Copy ${selectedClipIds.length} clips` : 'Copy', shortcut: 'Ctrl+C', action: () => copySelection(multi ? undefined : menu.clipId) },
            { label: multi ? `Cut ${selectedClipIds.length} clips` : 'Cut', shortcut: 'Ctrl+X', action: () => { void cutSelection(multi ? undefined : menu.clipId); } },
            { label: 'Paste at playhead', shortcut: 'Ctrl+V', disabled: !hasClipboard, action: () => { void pasteClips(); } },
            { label: multi ? `Duplicate ${selectedClipIds.length} clips` : 'Duplicate', action: () => { void duplicateClip(multi ? undefined : menu.clipId); } },
            'divider',
            { label: 'Copy attributes', action: () => copyClipAttributes(menu.clipId) },
            { label: 'Paste attributes', disabled: !hasAttributeClipboard, action: () => { void pasteClipAttributes(multi ? undefined : menu.clipId); } },
            'divider',
            { label: 'Move to layer above', action: () => { void moveClipToAdjacentTrack(menu.clipId, 'above'); } },
            { label: 'Move to layer below', action: () => { void moveClipToAdjacentTrack(menu.clipId, 'below'); } },
            { label: 'Bring forward', action: () => { void bringClipForward(menu.clipId); } },
            { label: 'Send backward', action: () => { void sendClipBackward(menu.clipId); } },
            'divider',
            { label: 'Add fade in/out', action: () => { void updateClipFade(menu.clipId, { fade_in_ms: 500, fade_out_ms: 500 }); } },
            { label: 'Add fade transition', action: () => { void addClipTransition(menu.clipId, { type: 'fade', duration_ms: 500 }); } },
            { label: menuClip.muted ? 'Unmute clip' : 'Mute clip', action: () => { void toggleClipMute(menu.clipId); } },
          ];
          const menuAsset = menuClip.asset_id ? assets.find((asset) => asset.id === menuClip.asset_id) : undefined;
          if (menuAsset?.mime_type.startsWith('video/') && !menuClip.audio_only) {
            items.push({ label: 'Detach audio', action: () => { void detachClipAudio(menu.clipId); } });
          }
          if (multi && !menuClip.group_id) items.push({ label: `Group ${selectedClipIds.length} clips`, action: () => { void groupClips(); } });
          if (menuClip.group_id) items.push({ label: 'Ungroup', action: () => { void ungroupClips(menuClip.group_id); } });
          if (multi) {
            items.push('divider');
            items.push({ label: 'Align starts', action: () => { void alignSelection('start'); } });
            items.push({ label: 'Align ends', action: () => { void alignSelection('end'); } });
            if (selectedClipIds.length >= 3) items.push({ label: 'Distribute evenly', action: () => { void alignSelection('distribute'); } });
          }
          items.push('divider');
          items.push({ label: multi ? `Delete ${selectedClipIds.length} clips` : 'Delete', danger: true, shortcut: 'Del', action: () => { void deleteClip(multi ? undefined : menu.clipId); } });
        } else if (menu.kind === 'track') {
          const trackIndex = timeline.tracks.findIndex((item) => item.id === menu.trackId);
          const track = timeline.tracks[trackIndex];
          if (!track) return null;
          const atTop = trackIndex >= timeline.tracks.length - 1;
          const atBottom = trackIndex === 0;
          items = [
            {
              label: 'Rename layer…',
              action: () => {
                const name = window.prompt('Layer name', track.name);
                if (name?.trim()) void renameTrack(track.id, name);
              },
            },
            { label: 'Add layer above', action: () => { void insertTrackAdjacent(track.id, 'above'); } },
            { label: 'Add layer below', action: () => { void insertTrackAdjacent(track.id, 'below'); } },
            { label: 'Duplicate layer', action: () => { void duplicateTrack(track.id); } },
            'divider',
            { label: 'Move up', disabled: atTop, action: () => { void reorderTrack(track.id, trackIndex + 1); } },
            { label: 'Move down', disabled: atBottom, action: () => { void reorderTrack(track.id, trackIndex - 1); } },
            { label: 'Move to top', disabled: atTop, action: () => { void moveTrackToEdge(track.id, 'top'); } },
            { label: 'Move to bottom', disabled: atBottom, action: () => { void moveTrackToEdge(track.id, 'bottom'); } },
            'divider',
            { label: track.locked ? 'Unlock layer' : 'Lock layer', action: () => { void toggleTrackLock(track.id); } },
            { label: track.visible ? 'Hide visuals' : 'Show visuals', action: () => { void toggleTrackVisibility(track.id); } },
            { label: track.muted ? 'Unmute audio' : 'Mute audio', action: () => { void toggleTrackMute(track.id); } },
            { label: soloTrackId === track.id ? 'Unsolo' : 'Solo (preview audio)', action: () => toggleTrackSolo(track.id) },
            'divider',
            { label: 'Select all clips on layer', disabled: track.clips.length === 0, action: () => selectClipsOnTrack(track.id) },
            {
              label: 'Clear layer',
              disabled: track.clips.length === 0,
              danger: true,
              action: () => {
                if (window.confirm(`Remove all ${track.clips.length} clip(s) from "${track.name}"?`)) void clearTrack(track.id);
              },
            },
            {
              label: 'Delete layer',
              danger: true,
              action: () => {
                if (track.clips.length === 0 || window.confirm(`Remove "${track.name}" and its ${track.clips.length} clip(s)?`)) void removeTrack(track.id);
              },
            },
          ];
        } else if (menu.kind === 'lane') {
          const track = timeline.tracks.find((item) => item.id === menu.trackId);
          if (!track) return null;
          items = [
            { label: 'Paste here', disabled: !hasClipboard || track.locked, action: () => { void pasteClips(menu.timeMs, track.id); } },
            { label: 'Paste at playhead', shortcut: 'Ctrl+V', disabled: !hasClipboard, action: () => { void pasteClips(); } },
            'divider',
            { label: 'Add text clip here', disabled: track.locked, action: () => { void addTextClip(undefined, { trackId: track.id, startMs: menu.timeMs }); } },
            { label: 'Add marker here', action: () => { void addMarker(menu.timeMs); } },
            'divider',
            { label: 'Add layer above', action: () => { void insertTrackAdjacent(track.id, 'above'); } },
            { label: 'Add layer below', action: () => { void insertTrackAdjacent(track.id, 'below'); } },
            'divider',
            {
              label: 'Select clips after this point',
              action: () => {
                setPlayhead(menu.timeMs);
                selectClipsRelativeToPlayhead('after');
              },
            },
          ];
        } else {
          items = [
            { label: 'Add marker here', shortcut: 'M', action: () => { void addMarker(menu.timeMs); } },
            { label: 'Go to start', action: () => setPlayhead(0) },
            { label: 'Go to end', action: () => setPlayhead(timeline.duration_ms) },
            'divider',
            { label: 'Split selected clip at playhead', shortcut: 'S', disabled: selectedClipIds.length === 0, action: () => { void splitClipAtPlayhead(); } },
            { label: 'Split all clips at playhead', action: () => { void splitAllAtPlayhead(); } },
            { label: 'Select clips before playhead', action: () => selectClipsRelativeToPlayhead('before') },
            { label: 'Select clips after playhead', action: () => selectClipsRelativeToPlayhead('after') },
            'divider',
            { label: 'Set project duration to here', action: () => { void setTimelineDuration(menu.timeMs); } },
            { label: 'Zoom to fit', shortcut: 'F', action: handleZoomToFit },
            { label: followPlayhead ? 'Disable follow playhead' : 'Follow playhead while playing', action: toggleFollowPlayhead },
          ];
        }
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
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
                ['Ctrl/Cmd+C / X / V', 'Copy / cut / paste clips'],
                ['Ctrl/Cmd+A', 'Select all clips'],
                ['S', 'Split selected clip at playhead'],
                ['C / V', 'Blade tool / Select tool'],
                ['[ / ]', 'Trim clip start / end to playhead'],
                ['M', 'Add marker at playhead'],
                ['+ / -', 'Zoom in / out'],
                ['F', 'Zoom to fit'],
                ['← / →', 'Nudge selection by 1 frame'],
                ['Shift+← / →', 'Nudge selection by 10 frames'],
                ['Delete / Backspace', 'Delete selection'],
                ['Ctrl/Cmd/Shift+Click', 'Multi-select clips'],
                ['Escape', 'Deselect / close menus'],
                ['Right-click clip / layer / lane / ruler', 'Context menus'],
                ['Shift+F10', 'Open menu on focused clip'],
                ['Right-click marker', 'Remove marker'],
                ['Double-click layer name', 'Rename layer'],
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
