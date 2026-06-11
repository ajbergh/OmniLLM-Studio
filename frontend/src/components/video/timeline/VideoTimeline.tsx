/**
 * Timeline container: owns keyboard shortcuts (guarded against inputs and
 * documented in the `?` help modal), every timeline context menu (clip, trim
 * edge, transition region, layer header, lane, ruler), marquee selection,
 * snap-point collection, the layer-first add menu, ripple-aware trim/delete
 * routing, and the app-native dialogs that replaced window.confirm/prompt.
 * Tracks render reversed so the last document track (foreground) is the top
 * row, matching the preview/export stacking order.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { Plus } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { editorModeFeatures } from '../editorModes';
import type { VideoTimelineTrack, VideoTimelineTrackType } from '../../../types/video';
import { ContextMenu } from '../../common/ContextMenu';
import type { ContextMenuEntry } from '../../common/ContextMenu';
import { ConfirmDialog, InputDialog } from '../../common/AppDialog';
import { KeyframeLane } from './KeyframeLane';
import { TimelinePlayhead } from './TimelinePlayhead';
import { TimelineRuler } from './TimelineRuler';
import { TimelineToolbar } from './TimelineToolbar';
import { TimelineTrack } from './TimelineTrack';
import type { SnapPoint } from './TimelineTrack';

type TimelineMenuState =
  | { kind: 'clip'; clipId: string; trackId: string; x: number; y: number }
  | { kind: 'clipEdge'; clipId: string; trackId: string; edge: 'start' | 'end'; x: number; y: number }
  | { kind: 'transition'; clipId: string; trackId: string; x: number; y: number }
  | { kind: 'track'; trackId: string; x: number; y: number }
  | { kind: 'lane'; trackId: string; timeMs: number; x: number; y: number }
  | { kind: 'ruler'; timeMs: number; x: number; y: number };

type TimelineDialogState =
  | { kind: 'renameTrack'; trackId: string; name: string }
  | { kind: 'clearTrack'; trackId: string; name: string; clipCount: number }
  | { kind: 'removeTrack'; trackId: string; name: string; clipCount: number }
  | { kind: 'trimPrecise'; clipId: string; edge: 'start' | 'end'; initialSeconds: number }
  | { kind: 'transitionDuration'; clipId: string; transitionId: string; initialSeconds: number };

const TRACK_HEADER_WIDTH = 116;
const LEGACY_TRACK_TYPES: VideoTimelineTrackType[] = ['video', 'image', 'audio', 'music', 'text', 'caption'];

function formatTimecode(ms: number, fps: number): string {
  const clamped = Math.max(0, ms);
  const minutes = Math.floor(clamped / 60_000);
  const seconds = Math.floor((clamped % 60_000) / 1000);
  const frames = Math.floor(((clamped % 1000) / 1000) * Math.max(1, fps));
  return `${minutes}:${String(seconds).padStart(2, '0')}.${String(frames).padStart(2, '0')}`;
}

/** Empty span on a layer containing timeMs, bounded by neighbor clips (or timeline start). */
function findGapAt(track: VideoTimelineTrack, timeMs: number): { start: number; end: number } | null {
  const clips = [...track.clips].sort((a, b) => a.start_ms - b.start_ms);
  let cursor = 0;
  for (const clip of clips) {
    if (timeMs >= cursor && timeMs < clip.start_ms) {
      return { start: cursor, end: clip.start_ms };
    }
    cursor = Math.max(cursor, clip.start_ms + clip.duration_ms);
    if (timeMs < cursor) return null;
  }
  return null;
}

export function VideoTimeline() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const selectedClipIds = useVideoStudioStore((state) => state.selectedClipIds);
  const selectedTrackId = useVideoStudioStore((state) => state.selectedTrackId);
  const selectedAssetId = useVideoStudioStore((state) => state.selectedAssetId);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const zoom = useVideoStudioStore((state) => state.zoom);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const rippleEnabled = useVideoStudioStore((state) => state.rippleEnabled);
  const isSavingTimeline = useVideoStudioStore((state) => state.isSavingTimeline);
  const canUndo = useVideoStudioStore((state) => state.timelineUndoStack.length > 0);
  const canRedo = useVideoStudioStore((state) => state.timelineRedoStack.length > 0);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const setZoom = useVideoStudioStore((state) => state.setZoom);
  const zoomToFit = useVideoStudioStore((state) => state.zoomToFit);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);
  const toggleSnapping = useVideoStudioStore((state) => state.toggleSnapping);
  const toggleRipple = useVideoStudioStore((state) => state.toggleRipple);
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
  const rippleDeleteClip = useVideoStudioStore((state) => state.rippleDeleteClip);
  const rippleTrimClip = useVideoStudioStore((state) => state.rippleTrimClip);
  const removeGap = useVideoStudioStore((state) => state.removeGap);
  const removeAllGaps = useVideoStudioStore((state) => state.removeAllGaps);
  const insertClipAt = useVideoStudioStore((state) => state.insertClipAt);
  const overwriteClipAt = useVideoStudioStore((state) => state.overwriteClipAt);
  const groupClips = useVideoStudioStore((state) => state.groupClips);
  const ungroupClips = useVideoStudioStore((state) => state.ungroupClips);
  const alignSelection = useVideoStudioStore((state) => state.alignSelection);
  const updateClipFade = useVideoStudioStore((state) => state.updateClipFade);
  const removeClipFades = useVideoStudioStore((state) => state.removeClipFades);
  const updateKeyframe = useVideoStudioStore((state) => state.updateKeyframe);
  const duckMusicUnderNarration = useVideoStudioStore((state) => state.duckMusicUnderNarration);
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const updateClipTransition = useVideoStudioStore((state) => state.updateClipTransition);
  const removeClipTransition = useVideoStudioStore((state) => state.removeClipTransition);
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
  const addShapeClip = useVideoStudioStore((state) => state.addShapeClip);
  const toggleClipMute = useVideoStudioStore((state) => state.toggleClipMute);
  const detachClipAudio = useVideoStudioStore((state) => state.detachClipAudio);
  const followPlayhead = useVideoStudioStore((state) => state.followPlayhead);
  const toggleFollowPlayhead = useVideoStudioStore((state) => state.toggleFollowPlayhead);
  const setSelectedClips = useVideoStudioStore((state) => state.setSelectedClips);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const addTrackRef = useRef<HTMLDivElement | null>(null);
  const [addTrackOpen, setAddTrackOpen] = useState(false);
  const [legacyTypesOpen, setLegacyTypesOpen] = useState(false);
  const [menu, setMenu] = useState<TimelineMenuState | null>(null);
  const [dialog, setDialog] = useState<TimelineDialogState | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const marqueeRef = useRef<{ startX: number; startY: number; moved: boolean } | null>(null);
  const [marqueeRect, setMarqueeRect] = useState<{ left: number; top: number; width: number; height: number } | null>(null);

  // Drag on empty lane space sweeps out a marquee; clips intersecting the
  // rectangle become the selection on release.
  const beginMarquee = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || toolMode !== 'select') return;
    if ((event.target as HTMLElement).dataset?.lane === undefined) return;
    marqueeRef.current = { startX: event.clientX, startY: event.clientY, moved: false };
    const onMove = (move: PointerEvent) => {
      const start = marqueeRef.current;
      if (!start) return;
      if (!start.moved && Math.abs(move.clientX - start.startX) < 4 && Math.abs(move.clientY - start.startY) < 4) return;
      start.moved = true;
      setMarqueeRect({
        left: Math.min(start.startX, move.clientX),
        top: Math.min(start.startY, move.clientY),
        width: Math.abs(move.clientX - start.startX),
        height: Math.abs(move.clientY - start.startY),
      });
    };
    const onUp = (up: PointerEvent) => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      const start = marqueeRef.current;
      marqueeRef.current = null;
      setMarqueeRect(null);
      if (!start?.moved) return;
      const left = Math.min(start.startX, up.clientX);
      const right = Math.max(start.startX, up.clientX);
      const top = Math.min(start.startY, up.clientY);
      const bottom = Math.max(start.startY, up.clientY);
      const ids: string[] = [];
      document.querySelectorAll<HTMLElement>('[data-clip-id]').forEach((node) => {
        const rect = node.getBoundingClientRect();
        if (rect.left < right && rect.right > left && rect.top < bottom && rect.bottom > top && node.dataset.clipId) {
          ids.push(node.dataset.clipId);
        }
      });
      setSelectedClips(ids);
      // Swallow the click that follows pointerup so the lane's click-to-
      // deselect doesn't clear the fresh selection.
      window.addEventListener('click', (clickEvent) => {
        clickEvent.stopPropagation();
        clickEvent.preventDefault();
      }, { capture: true, once: true });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };
  const pxPerMs = useMemo(() => 0.02 * zoom, [zoom]);
  const width = Math.max(900, (timeline?.duration_ms || 30000) * pxPerMs);

  // Snap targets, typed so the drop guide can say what it snapped to. When a
  // position matches several targets, precedence is playhead > marker > clip
  // edge > timeline start/end.
  const snapPoints = useMemo(() => {
    if (!timeline) return [] as SnapPoint[];
    const points = new Map<number, SnapPoint['kind']>();
    const add = (ms: number, kind: SnapPoint['kind']) => {
      if (!points.has(ms)) points.set(ms, kind);
    };
    add(playheadMs, 'playhead');
    for (const marker of timeline.markers || []) {
      add(marker.time_ms, 'marker');
    }
    for (const track of timeline.tracks) {
      for (const clip of track.clips) {
        add(clip.start_ms, 'clip');
        add(clip.start_ms + clip.duration_ms, 'clip');
      }
    }
    add(0, 'edge');
    add(timeline.duration_ms, 'edge');
    return Array.from(points, ([ms, kind]) => ({ ms, kind }));
  }, [timeline, playheadMs]);

  useEffect(() => {
    if (!addTrackOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!addTrackRef.current?.contains(event.target as Node)) {
        setAddTrackOpen(false);
        setLegacyTypesOpen(false);
      }
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

  // Jump targets for PageUp/PageDown: markers plus every clip edge.
  const jumpToEditPoint = (direction: -1 | 1) => {
    const state = useVideoStudioStore.getState();
    const doc = state.timeline;
    if (!doc) return;
    const points = new Set<number>([0, doc.duration_ms]);
    for (const marker of doc.markers || []) points.add(marker.time_ms);
    for (const track of doc.tracks) {
      for (const clip of track.clips) {
        points.add(clip.start_ms);
        points.add(clip.start_ms + clip.duration_ms);
      }
    }
    const sorted = Array.from(points).sort((a, b) => a - b);
    const playhead = state.playheadMs;
    const target = direction === 1
      ? sorted.find((point) => point > playhead + 1)
      : [...sorted].reverse().find((point) => point < playhead - 1);
    if (target !== undefined) setPlayhead(target);
  };

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
      } else if ((event.key === 'Delete' || event.key === 'Backspace') && event.shiftKey) {
        void rippleDeleteClip();
      } else if (event.key === 'Delete' || event.key === 'Backspace') {
        if (useVideoStudioStore.getState().rippleEnabled) {
          void rippleDeleteClip();
        } else {
          void deleteClip();
        }
      } else if (event.key.toLowerCase() === 's' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void saveTimeline();
      } else if (event.key.toLowerCase() === 'c' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        copySelection();
      } else if (event.key.toLowerCase() === 'x' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void cutSelection();
      } else if (event.key.toLowerCase() === 'v' && (event.ctrlKey || event.metaKey) && event.shiftKey) {
        event.preventDefault();
        void pasteClipAttributes();
      } else if (event.key.toLowerCase() === 'v' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void pasteClips();
      } else if (event.key.toLowerCase() === 'a' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        selectAllClips();
      } else if (event.key.toLowerCase() === 'd' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void duplicateClip();
      } else if (event.key.toLowerCase() === 's') {
        void splitClipAtPlayhead();
      } else if (event.key.toLowerCase() === 'm') {
        void addMarker();
      } else if (event.key.toLowerCase() === 'c') {
        setToolMode('blade');
      } else if (event.key.toLowerCase() === 'v') {
        setToolMode('select');
      } else if (event.key.toLowerCase() === 'r') {
        toggleRipple();
      } else if (event.key.toLowerCase() === 'g' && event.shiftKey) {
        void ungroupClips();
      } else if (event.key.toLowerCase() === 'g') {
        void groupClips();
      } else if (event.key === '[') {
        void trimClipEdgeToPlayhead('start');
      } else if (event.key === ']') {
        void trimClipEdgeToPlayhead('end');
      } else if (event.key === 'Home') {
        event.preventDefault();
        setPlayhead(0);
      } else if (event.key === 'End') {
        event.preventDefault();
        setPlayhead(useVideoStudioStore.getState().timeline?.duration_ms || 0);
      } else if (event.key === 'PageUp') {
        event.preventDefault();
        jumpToEditPoint(-1);
      } else if (event.key === 'PageDown') {
        event.preventDefault();
        jumpToEditPoint(1);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [addMarker, copySelection, cutSelection, deleteClip, duplicateClip, groupClips, nudgeSelection, pasteClipAttributes, pasteClips, redoTimeline, rippleDeleteClip, saveTimeline, selectAllClips, selectClip, setPlayhead, setPlaying, setToolMode, setZoom, splitClipAtPlayhead, toggleRipple, trimClipEdgeToPlayhead, undoTimeline, ungroupClips, zoomToFit]);

  if (!timeline) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-text-muted">
        Timeline will load with the active project.
      </div>
    );
  }

  const hasClips = timeline.tracks.some((track) => track.clips.length > 0);
  const selectedAsset = selectedAssetId ? assets.find((asset) => asset.id === selectedAssetId) : undefined;

  const handleZoomToFit = () => {
    const container = scrollRef.current;
    if (!container) return;
    zoomToFit(Math.max(200, container.clientWidth - TRACK_HEADER_WIDTH - 24));
  };

  // In ripple mode, edge drags route to ripple trims so later clips follow.
  // Extending the start edge leftward has no ripple meaning — fall through.
  const handleTrimClip = (clipId: string, updates: Partial<{ start_ms: number; duration_ms: number; trim_in_ms: number; trim_out_ms: number }>) => {
    const state = useVideoStudioStore.getState();
    if (state.rippleEnabled && state.timeline) {
      const clip = state.timeline.tracks.flatMap((track) => track.clips).find((item) => item.id === clipId);
      if (clip) {
        if (updates.start_ms !== undefined && updates.start_ms > clip.start_ms) {
          void rippleTrimClip(clipId, 'start', updates.start_ms);
          return;
        }
        if (updates.start_ms === undefined && updates.duration_ms !== undefined) {
          void rippleTrimClip(clipId, 'end', clip.start_ms + updates.duration_ms);
          return;
        }
      }
    }
    void trimClip(clipId, updates);
  };

  const selectionSummary = (() => {
    if (selectedClipIds.length === 0) return undefined;
    const selected = timeline.tracks.flatMap((track) => track.clips).filter((clip) => selectedClipIds.includes(clip.id));
    const total = selected.reduce((sum, clip) => sum + clip.duration_ms, 0);
    return `${selected.length} clip${selected.length === 1 ? '' : 's'} · ${formatTimecode(total, timeline.canvas.fps)}`;
  })();

  const addLayerMenu = (() => {
    const bottomTrack = timeline.tracks[0];
    const selectedTrack = selectedTrackId ? timeline.tracks.find((track) => track.id === selectedTrackId) : undefined;
    const close = () => {
      setAddTrackOpen(false);
      setLegacyTypesOpen(false);
    };
    return (
      <div className="absolute left-1 top-full z-30 w-44 rounded-md border border-border bg-surface p-1 shadow-lg">
        <button
          className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-secondary hover:bg-surface-alt hover:text-text"
          onClick={() => { close(); void addTrack('layer'); }}
        >
          Add layer at top
        </button>
        {selectedTrack && (
          <>
            <button
              className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-secondary hover:bg-surface-alt hover:text-text"
              onClick={() => { close(); void insertTrackAdjacent(selectedTrack.id, 'above'); }}
            >
              Add layer above “{selectedTrack.name}”
            </button>
            <button
              className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-secondary hover:bg-surface-alt hover:text-text"
              onClick={() => { close(); void insertTrackAdjacent(selectedTrack.id, 'below'); }}
            >
              Add layer below “{selectedTrack.name}”
            </button>
          </>
        )}
        {bottomTrack && (
          <button
            className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-secondary hover:bg-surface-alt hover:text-text"
            onClick={() => { close(); void insertTrackAdjacent(bottomTrack.id, 'below'); }}
          >
            Add layer at bottom
          </button>
        )}
        <div className="my-1 h-px bg-border" />
        <button
          className="block w-full rounded px-2 py-1 text-left text-[11px] text-text-muted hover:bg-surface-alt hover:text-text"
          onClick={() => setLegacyTypesOpen((open) => !open)}
        >
          Advanced legacy track types {legacyTypesOpen ? '▾' : '▸'}
        </button>
        {legacyTypesOpen && LEGACY_TRACK_TYPES.map((type) => (
          <button
            key={type}
            className="block w-full rounded px-2 py-1 pl-4 text-left text-[11px] capitalize text-text-muted hover:bg-surface-alt hover:text-text"
            onClick={() => { close(); void addTrack(type); }}
          >
            {type}
          </button>
        ))}
      </div>
    );
  })();

  return (
    <div className="flex h-full min-h-0 flex-col rounded-lg border border-border bg-surface">
      <TimelineToolbar
        isPlaying={isPlaying}
        snappingEnabled={snappingEnabled}
        rippleEnabled={rippleEnabled}
        zoom={zoom}
        isSaving={isSavingTimeline}
        canUndo={canUndo}
        canRedo={canRedo}
        toolMode={toolMode}
        onPlayPause={() => setPlaying(!isPlaying)}
        onUndo={() => { void undoTimeline(); }}
        onRedo={() => { void redoTimeline(); }}
        onSplit={() => { void splitClipAtPlayhead(); }}
        onDelete={() => { void (rippleEnabled ? rippleDeleteClip() : deleteClip()); }}
        onDuplicate={() => { void duplicateClip(); }}
        onSave={() => { void saveTimeline(); }}
        onZoom={setZoom}
        onZoomToFit={handleZoomToFit}
        onToggleSnap={toggleSnapping}
        onToggleRipple={toggleRipple}
        onSetToolMode={setToolMode}
        onAddMarker={() => { void addMarker(); }}
        onHelp={() => setShowHelp(true)}
        timecode={`${formatTimecode(playheadMs, timeline.canvas.fps)} / ${formatTimecode(timeline.duration_ms, timeline.canvas.fps)}`}
        selectionSummary={selectionSummary}
      />
      <div ref={scrollRef} className="relative min-h-0 flex-1 overflow-auto" onPointerDown={beginMarquee}>
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div ref={addTrackRef} className="sticky left-0 z-20 flex h-8 items-center border-b border-r border-border bg-surface-alt px-1">
            <button
              className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-text-muted hover:text-text ${editorModeFeatures(editorMode).addTrack ? '' : 'invisible'}`}
              title="Add layer — higher layers overlay lower layers in the preview"
              aria-label="Add layer"
              onClick={() => setAddTrackOpen((open) => !open)}
            >
              <Plus size={11} />
              Layer
            </button>
            {addTrackOpen && editorModeFeatures(editorMode).addTrack && addLayerMenu}
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
            snapPoints={snapPoints}
            soloActive={soloTrackId === track.id}
            onMoveClip={(clipId, trackId, startMs) => { void moveClip(clipId, trackId, startMs); }}
            onTrimClip={handleTrimClip}
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
            onEdgeContextMenu={(clipId, trackId, edge, x, y) => {
              if (!useVideoStudioStore.getState().selectedClipIds.includes(clipId)) {
                selectClip(clipId, trackId);
              }
              setMenu({ kind: 'clipEdge', clipId, trackId, edge, x, y });
            }}
            onFadeClip={(clipId, fade) => { void updateClipFade(clipId, fade); }}
            onUpdateClipKeyframe={(clipId, keyframeId, patch) => { void updateKeyframe(clipId, keyframeId, patch); }}
            onTransitionContextMenu={(clipId, trackId, x, y) => setMenu({ kind: 'transition', clipId, trackId, x, y })}
            onHeaderContextMenu={(trackId, x, y) => setMenu({ kind: 'track', trackId, x, y })}
            onLaneContextMenu={(trackId, timeMs, x, y) => setMenu({ kind: 'lane', trackId, timeMs, x, y })}
          />
        ))}
        <KeyframeLane pxPerMs={pxPerMs} />
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
            { label: multi ? `Duplicate ${selectedClipIds.length} clips` : 'Duplicate', shortcut: 'Ctrl+D', action: () => { void duplicateClip(multi ? undefined : menu.clipId); } },
            'divider',
            { label: 'Copy attributes', action: () => copyClipAttributes(menu.clipId) },
            { label: 'Paste attributes', shortcut: 'Ctrl+Shift+V', disabled: !hasAttributeClipboard, action: () => { void pasteClipAttributes(multi ? undefined : menu.clipId); } },
            'divider',
            { label: 'Move to layer above', action: () => { void moveClipToAdjacentTrack(menu.clipId, 'above'); } },
            { label: 'Move to layer below', action: () => { void moveClipToAdjacentTrack(menu.clipId, 'below'); } },
            { label: 'Bring forward', action: () => { void bringClipForward(menu.clipId); } },
            { label: 'Send backward', action: () => { void sendClipBackward(menu.clipId); } },
            'divider',
            { label: 'Add fade in/out', action: () => { void updateClipFade(menu.clipId, { fade_in_ms: 500, fade_out_ms: 500 }); } },
            {
              label: multi ? `Remove fades from ${selectedClipIds.length} clips` : 'Remove fades',
              disabled: !multi && !menuClip.fade_in_ms && !menuClip.fade_out_ms,
              action: () => { void removeClipFades(multi ? selectedClipIds : [menu.clipId]); },
            },
            { label: 'Add fade transition', action: () => { void addClipTransition(menu.clipId, { type: 'fade', duration_ms: 500 }); } },
            { label: menuClip.muted ? 'Unmute clip' : 'Mute clip', action: () => { void toggleClipMute(menu.clipId); } },
            { label: 'Duck music under narration', action: () => { void duckMusicUnderNarration(); } },
          ];
          const menuAsset = menuClip.asset_id ? assets.find((asset) => asset.id === menuClip.asset_id) : undefined;
          if (menuAsset?.mime_type.startsWith('video/') && !menuClip.audio_only) {
            items.push({ label: 'Detach audio', action: () => { void detachClipAudio(menu.clipId); } });
          }
          if (multi && !menuClip.group_id) items.push({ label: `Group ${selectedClipIds.length} clips`, shortcut: 'G', action: () => { void groupClips(); } });
          if (menuClip.group_id) items.push({ label: 'Ungroup', shortcut: 'Shift+G', action: () => { void ungroupClips(menuClip.group_id); } });
          if (multi) {
            items.push('divider');
            items.push({ label: 'Align starts', action: () => { void alignSelection('start'); } });
            items.push({ label: 'Align ends', action: () => { void alignSelection('end'); } });
            if (selectedClipIds.length >= 3) items.push({ label: 'Distribute evenly', action: () => { void alignSelection('distribute'); } });
          }
          items.push('divider');
          items.push({ label: multi ? `Delete ${selectedClipIds.length} clips` : 'Delete', danger: true, shortcut: 'Del', action: () => { void deleteClip(multi ? undefined : menu.clipId); } });
          items.push({ label: multi ? `Ripple delete ${selectedClipIds.length} clips` : 'Ripple delete', danger: true, shortcut: 'Shift+Del', action: () => { void rippleDeleteClip(multi ? undefined : menu.clipId); } });
        } else if (menu.kind === 'clipEdge') {
          const menuClip = timeline.tracks.flatMap((track) => track.clips).find((clip) => clip.id === menu.clipId);
          if (!menuClip) return null;
          const edgeLabel = menu.edge === 'start' ? 'start' : 'end';
          const playheadInside = playheadMs > menuClip.start_ms && playheadMs < menuClip.start_ms + menuClip.duration_ms;
          items = [
            {
              label: `Trim ${edgeLabel} to playhead`,
              shortcut: menu.edge === 'start' ? '[' : ']',
              disabled: !playheadInside,
              action: () => { void trimClipEdgeToPlayhead(menu.edge); },
            },
            {
              label: `Ripple trim ${edgeLabel} to playhead`,
              disabled: menu.edge === 'start' ? !playheadInside : playheadMs <= menuClip.start_ms,
              action: () => { void rippleTrimClip(menu.clipId, menu.edge, playheadMs); },
            },
            'divider',
            {
              label: 'Set trim precisely…',
              action: () => {
                const initial = menu.edge === 'start' ? menuClip.start_ms : menuClip.start_ms + menuClip.duration_ms;
                setDialog({ kind: 'trimPrecise', clipId: menu.clipId, edge: menu.edge, initialSeconds: initial / 1000 });
              },
            },
          ];
        } else if (menu.kind === 'transition') {
          const menuClip = timeline.tracks.flatMap((track) => track.clips).find((clip) => clip.id === menu.clipId);
          if (!menuClip || (menuClip.transitions || []).length === 0) return null;
          items = [];
          for (const transition of menuClip.transitions || []) {
            items.push({
              label: `${transition.type} — edit duration (${(transition.duration_ms / 1000).toFixed(1)}s)…`,
              action: () => setDialog({ kind: 'transitionDuration', clipId: menu.clipId, transitionId: transition.id, initialSeconds: transition.duration_ms / 1000 }),
            });
            if (transition.type === 'slide') {
              const directions = ['left', 'right', 'up', 'down'] as const;
              const nextDirection = directions[(directions.indexOf(transition.direction || 'left') + 1) % directions.length];
              items.push({
                label: `${transition.type} — direction: ${transition.direction || 'left'} → ${nextDirection}`,
                action: () => { void updateClipTransition(menu.clipId, transition.id, { direction: nextDirection }); },
              });
            }
            items.push({
              label: `${transition.type} — remove`,
              danger: true,
              action: () => { void removeClipTransition(menu.clipId, transition.id); },
            });
            items.push('divider');
          }
          items.pop(); // trailing divider
        } else if (menu.kind === 'track') {
          const trackIndex = timeline.tracks.findIndex((item) => item.id === menu.trackId);
          const track = timeline.tracks[trackIndex];
          if (!track) return null;
          const atTop = trackIndex >= timeline.tracks.length - 1;
          const atBottom = trackIndex === 0;
          items = [
            {
              label: 'Rename layer…',
              action: () => setDialog({ kind: 'renameTrack', trackId: track.id, name: track.name }),
            },
            { label: 'Add layer above', action: () => { void insertTrackAdjacent(track.id, 'above'); } },
            { label: 'Add layer below', action: () => { void insertTrackAdjacent(track.id, 'below'); } },
            { label: 'Duplicate layer', action: () => { void duplicateTrack(track.id); } },
            'divider',
            { label: 'Move up (toward foreground)', disabled: atTop, action: () => { void reorderTrack(track.id, trackIndex + 1); } },
            { label: 'Move down (toward background)', disabled: atBottom, action: () => { void reorderTrack(track.id, trackIndex - 1); } },
            { label: 'Move to top', disabled: atTop, action: () => { void moveTrackToEdge(track.id, 'top'); } },
            { label: 'Move to bottom', disabled: atBottom, action: () => { void moveTrackToEdge(track.id, 'bottom'); } },
            'divider',
            { label: track.locked ? 'Unlock layer' : 'Lock layer', action: () => { void toggleTrackLock(track.id); } },
            { label: track.visible ? 'Hide visuals' : 'Show visuals', action: () => { void toggleTrackVisibility(track.id); } },
            { label: track.muted ? 'Unmute audio' : 'Mute audio', action: () => { void toggleTrackMute(track.id); } },
            { label: soloTrackId === track.id ? 'Unsolo' : 'Solo (preview audio)', action: () => toggleTrackSolo(track.id) },
            'divider',
            { label: 'Select all clips on layer', disabled: track.clips.length === 0, action: () => selectClipsOnTrack(track.id) },
            { label: 'Remove all gaps on layer', disabled: track.clips.length < 2 || track.locked, action: () => { void removeAllGaps(track.id); } },
            {
              label: 'Clear layer',
              disabled: track.clips.length === 0,
              danger: true,
              action: () => setDialog({ kind: 'clearTrack', trackId: track.id, name: track.name, clipCount: track.clips.length }),
            },
            {
              label: 'Delete layer',
              danger: true,
              action: () => {
                if (track.clips.length === 0) {
                  void removeTrack(track.id);
                } else {
                  setDialog({ kind: 'removeTrack', trackId: track.id, name: track.name, clipCount: track.clips.length });
                }
              },
            },
          ];
        } else if (menu.kind === 'lane') {
          const track = timeline.tracks.find((item) => item.id === menu.trackId);
          if (!track) return null;
          const gap = findGapAt(track, menu.timeMs);
          items = [
            { label: 'Paste here', disabled: !hasClipboard || track.locked, action: () => { void pasteClips(menu.timeMs, track.id); } },
            { label: 'Paste at playhead', shortcut: 'Ctrl+V', disabled: !hasClipboard, action: () => { void pasteClips(); } },
          ];
          if (selectedAsset) {
            items.push('divider');
            items.push({
              label: `Insert “${selectedAsset.file_name}” here (ripple)`,
              disabled: track.locked,
              action: () => { void insertClipAt(selectedAsset.id, track.id, menu.timeMs, true); },
            });
            items.push({
              label: `Overwrite with “${selectedAsset.file_name}” here`,
              disabled: track.locked,
              action: () => { void overwriteClipAt(selectedAsset.id, track.id, menu.timeMs); },
            });
          }
          items.push('divider');
          items.push({
            label: gap ? `Remove gap (${((gap.end - gap.start) / 1000).toFixed(1)}s)` : 'Remove gap',
            disabled: !gap || track.locked,
            action: () => { if (gap) void removeGap(track.id, gap.start, gap.end); },
          });
          items.push({ label: 'Remove all gaps on layer', disabled: track.clips.length < 2 || track.locked, action: () => { void removeAllGaps(track.id); } });
          items.push('divider');
          items.push({ label: 'Add text clip here', disabled: track.locked, action: () => { void addTextClip(undefined, { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add highlight box here', disabled: track.locked, action: () => { void addShapeClip('highlight', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add rectangle callout here', disabled: track.locked, action: () => { void addShapeClip('rectangle', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add blur region here', disabled: track.locked, action: () => { void addShapeClip('blur', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add arrow here', disabled: track.locked, action: () => { void addShapeClip('arrow', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add spotlight here', disabled: track.locked, action: () => { void addShapeClip('spotlight', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add label callout here', disabled: track.locked, action: () => { void addShapeClip('label', { trackId: track.id, startMs: menu.timeMs }); } });
          items.push({ label: 'Add marker here', action: () => { void addMarker(menu.timeMs); } });
          items.push('divider');
          items.push({ label: 'Add layer above', action: () => { void insertTrackAdjacent(track.id, 'above'); } });
          items.push({ label: 'Add layer below', action: () => { void insertTrackAdjacent(track.id, 'below'); } });
          items.push('divider');
          items.push({
            label: 'Select clips after this point',
            action: () => {
              setPlayhead(menu.timeMs);
              selectClipsRelativeToPlayhead('after');
            },
          });
        } else {
          items = [
            { label: 'Add marker here', shortcut: 'M', action: () => { void addMarker(menu.timeMs); } },
            { label: 'Go to start', shortcut: 'Home', action: () => setPlayhead(0) },
            { label: 'Go to end', shortcut: 'End', action: () => setPlayhead(timeline.duration_ms) },
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
      {dialog?.kind === 'renameTrack' && (
        <InputDialog
          title="Rename layer"
          initialValue={dialog.name}
          placeholder="Layer name"
          validate={(value) => (value.trim() ? null : 'Enter a layer name')}
          onSubmit={(value) => {
            setDialog(null);
            void renameTrack(dialog.trackId, value);
          }}
          onCancel={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'clearTrack' && (
        <ConfirmDialog
          title="Clear layer"
          message={`Remove all ${dialog.clipCount} clip(s) from “${dialog.name}”?`}
          confirmLabel="Clear layer"
          danger
          onConfirm={() => {
            setDialog(null);
            void clearTrack(dialog.trackId);
          }}
          onCancel={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'removeTrack' && (
        <ConfirmDialog
          title="Delete layer"
          message={`Remove “${dialog.name}” and its ${dialog.clipCount} clip(s)? This can be undone with Ctrl+Z.`}
          confirmLabel="Delete layer"
          danger
          onConfirm={() => {
            setDialog(null);
            void removeTrack(dialog.trackId);
          }}
          onCancel={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'trimPrecise' && (
        <InputDialog
          title={`Trim clip ${dialog.edge} to…`}
          label="Timeline position in seconds"
          initialValue={dialog.initialSeconds.toFixed(2)}
          inputType="number"
          submitLabel="Trim"
          validate={(value) => (Number.isFinite(Number(value)) && Number(value) >= 0 ? null : 'Enter a non-negative number of seconds')}
          onSubmit={(value) => {
            setDialog(null);
            const timeMs = Math.round(Number(value) * 1000);
            const clip = timeline.tracks.flatMap((track) => track.clips).find((item) => item.id === dialog.clipId);
            if (!clip) return;
            if (dialog.edge === 'start') {
              const offset = timeMs - clip.start_ms;
              if (offset <= 0 || offset >= clip.duration_ms) return;
              void trimClip(dialog.clipId, {
                start_ms: timeMs,
                duration_ms: clip.duration_ms - offset,
                trim_in_ms: clip.trim_in_ms + offset,
                trim_out_ms: clip.trim_out_ms,
              });
            } else {
              const duration = timeMs - clip.start_ms;
              if (duration < 100) return;
              void trimClip(dialog.clipId, {
                duration_ms: duration,
                trim_out_ms: clip.trim_in_ms + duration,
              });
            }
          }}
          onCancel={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'transitionDuration' && (
        <InputDialog
          title="Transition duration"
          label="Duration in seconds"
          initialValue={dialog.initialSeconds.toFixed(1)}
          inputType="number"
          submitLabel="Save"
          validate={(value) => (Number(value) > 0 ? null : 'Enter a positive number of seconds')}
          onSubmit={(value) => {
            setDialog(null);
            void updateClipTransition(dialog.clipId, dialog.transitionId, { duration_ms: Math.round(Number(value) * 1000) });
          }}
          onCancel={() => setDialog(null)}
        />
      )}
      {marqueeRect && (
        <div className="pointer-events-none fixed z-[90] border border-primary/70 bg-primary/10" style={marqueeRect} />
      )}
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
                ['Ctrl/Cmd+Shift+V', 'Paste attributes'],
                ['Ctrl/Cmd+A', 'Select all clips'],
                ['Ctrl/Cmd+D', 'Duplicate selection'],
                ['S', 'Split selected clip at playhead'],
                ['C / V', 'Blade tool / Select tool'],
                ['R', 'Toggle ripple mode'],
                ['G / Shift+G', 'Group / ungroup selection'],
                ['[ / ]', 'Trim clip start / end to playhead'],
                ['M', 'Add marker at playhead'],
                ['Home / End', 'Go to start / end'],
                ['PageUp / PageDown', 'Previous / next edit point'],
                ['+ / -', 'Zoom in / out'],
                ['F', 'Zoom to fit'],
                ['← / →', 'Nudge selection by 1 frame'],
                ['Shift+← / →', 'Nudge selection by 10 frames'],
                ['Delete / Backspace', 'Delete selection (ripples when ripple mode is on)'],
                ['Shift+Delete', 'Ripple delete selection'],
                ['Ctrl/Cmd/Shift+Click', 'Multi-select clips'],
                ['Drag on empty lane', 'Marquee-select clips'],
                ['Escape', 'Deselect / close menus'],
                ['Right-click clip / edge / layer / lane / ruler', 'Context menus'],
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
