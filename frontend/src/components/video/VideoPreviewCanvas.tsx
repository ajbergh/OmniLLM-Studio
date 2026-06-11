/**
 * Preview canvas: composites every visible track at the playhead (track order
 * + z-index) with transforms, fades, crop, effects, keyframes, and text, and
 * doubles as the direct-manipulation surface — 8-handle resize (uniform scale
 * for media/text, true width/height for shapes) with Shift/Alt/Ctrl
 * modifiers, rotation, smart-guide snapping against the stage and other
 * layers, double-click inline text editing, an 8-handle crop mode with
 * explicit apply/cancel, and a cursor-metadata overlay. Drags preview via
 * liveTransform and commit once on pointer-up; playback advances the store
 * playhead via rAF and keeps mounted <video>/<audio> elements in sync.
 */
import { Check, Crop, Crosshair, Grid3x3, Maximize2, Minimize2, Pause, Play, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { videoApi } from '../../api';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import { composePreviewFilter } from './effects/effectRegistry';
import { sampleKeyframes } from './effects/keyframeUtils';
import { ShapePreview } from './ShapePreview';
import type { VideoAsset, VideoTimelineClip, VideoTimelineCursor, VideoTimelineTrack } from '../../types/video';

function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

const VISUAL_TRACK_TYPES = ['layer', 'video', 'image', 'text', 'caption', 'shape', 'callout'];

// Mounted <video> elements are expensive; only the topmost few video layers get
// real elements — deeper ones render a lightweight placeholder card.
const MAX_VIDEO_ELEMENTS = 4;

const SNAP_THRESHOLD_PX = 8;

type CropBox = { top: number; right: number; bottom: number; left: number };

const EMPTY_CROP: CropBox = { top: 0, right: 0, bottom: 0, left: 0 };

/** Clamp one crop side so each side stays in [0, 0.95] and ≥10% of the frame survives. */
function clampCropSide(value: number, opposite: number): number {
  return Math.max(0, Math.min(0.95, Math.min(value, 0.9 - opposite)));
}

type HandleId = 'nw' | 'n' | 'ne' | 'e' | 'se' | 's' | 'sw' | 'w';

const RESIZE_HANDLES: Array<{ id: HandleId; hx: -1 | 0 | 1; hy: -1 | 0 | 1; className: string }> = [
  { id: 'nw', hx: -1, hy: -1, className: '-left-1.5 -top-1.5 cursor-nwse-resize' },
  { id: 'n', hx: 0, hy: -1, className: 'left-1/2 -top-1.5 -translate-x-1/2 cursor-ns-resize' },
  { id: 'ne', hx: 1, hy: -1, className: '-right-1.5 -top-1.5 cursor-nesw-resize' },
  { id: 'e', hx: 1, hy: 0, className: '-right-1.5 top-1/2 -translate-y-1/2 cursor-ew-resize' },
  { id: 'se', hx: 1, hy: 1, className: '-right-1.5 -bottom-1.5 cursor-nwse-resize' },
  { id: 's', hx: 0, hy: 1, className: 'left-1/2 -bottom-1.5 -translate-x-1/2 cursor-ns-resize' },
  { id: 'sw', hx: -1, hy: 1, className: '-left-1.5 -bottom-1.5 cursor-nesw-resize' },
  { id: 'w', hx: -1, hy: 0, className: '-left-1.5 top-1/2 -translate-y-1/2 cursor-ew-resize' },
];

interface LayerEntry {
  track: VideoTimelineTrack;
  trackIndex: number;
  clip: VideoTimelineClip;
  asset?: VideoAsset;
}

/** Interpolated cursor position at a clip-relative time, or null when hidden/empty. */
function sampleCursor(cursor: VideoTimelineCursor | undefined, clipTimeMs: number): { x: number; y: number; click: boolean } | null {
  if (!cursor || cursor.visible === false) return null;
  const events = cursor.events || [];
  if (events.length === 0) return null;
  const clickNearby = events.some((event) => event.click && Math.abs(event.time_ms - clipTimeMs) < 300);
  let prev = events[0];
  if (clipTimeMs <= prev.time_ms) return { x: prev.x, y: prev.y, click: clickNearby };
  for (let i = 1; i < events.length; i += 1) {
    const next = events[i];
    if (clipTimeMs <= next.time_ms) {
      const t = (clipTimeMs - prev.time_ms) / Math.max(1, next.time_ms - prev.time_ms);
      return { x: prev.x + (next.x - prev.x) * t, y: prev.y + (next.y - prev.y) * t, click: clickNearby };
    }
    prev = next;
  }
  return { x: prev.x, y: prev.y, click: clickNearby };
}

/** Opacity factor from clip fades at the current playhead (matches export semantics). */
function fadeFactor(clip: VideoTimelineClip, playheadMs: number): number {
  const elapsed = playheadMs - clip.start_ms;
  const remaining = clip.start_ms + clip.duration_ms - playheadMs;
  let factor = 1;
  if (clip.fade_in_ms && clip.fade_in_ms > 0) factor = Math.min(factor, elapsed / clip.fade_in_ms);
  if (clip.fade_out_ms && clip.fade_out_ms > 0) factor = Math.min(factor, remaining / clip.fade_out_ms);
  return Math.max(0, Math.min(1, factor));
}

interface LivePatch {
  x?: number;
  y?: number;
  scale?: number;
  rotation?: number;
  shapeWidth?: number;
  shapeHeight?: number;
}

interface DragState {
  mode: 'move' | 'resize' | 'rotate';
  handle?: { hx: -1 | 0 | 1; hy: -1 | 0 | 1 };
  clipId: string;
  isShape: boolean;
  pointerId: number;
  startClientX: number;
  startClientY: number;
  base: { x: number; y: number; scale: number; rotation: number; shapeWidth: number; shapeHeight: number };
  centerClientX: number;
  centerClientY: number;
  anchorClientX: number;
  anchorClientY: number;
  startAngle: number;
  baseRect: { left: number; top: number; width: number; height: number };
  stageRect: { left: number; top: number; width: number; height: number };
  candidatesX: number[];
  candidatesY: number[];
}

export function VideoPreviewCanvas() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const soloTrackId = useVideoStudioStore((state) => state.soloTrackId);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const updateClipTransform = useVideoStudioStore((state) => state.updateClipTransform);
  const updateClipText = useVideoStudioStore((state) => state.updateClipText);
  const resizeShapeClip = useVideoStudioStore((state) => state.resizeShapeClip);
  const bringClipForward = useVideoStudioStore((state) => state.bringClipForward);
  const sendClipBackward = useVideoStudioStore((state) => state.sendClipBackward);
  const addTextClip = useVideoStudioStore((state) => state.addTextClip);
  const addShapeClip = useVideoStudioStore((state) => state.addShapeClip);

  const rafRef = useRef<number | null>(null);
  const timelineDurationRef = useRef<number>(0);
  const videoRefs = useRef(new Map<string, HTMLVideoElement>());
  const audioRefs = useRef(new Map<string, HTMLAudioElement>());
  const layersRef = useRef<LayerEntry[]>([]);
  const audioLayersRef = useRef<Array<{ clip: VideoTimelineClip; asset: VideoAsset }>>([]);
  const fitRef = useRef<HTMLDivElement | null>(null);
  const stageRef = useRef<HTMLDivElement | null>(null);
  const [stageSize, setStageSize] = useState<{ width: number; height: number }>({ width: 0, height: 0 });
  const [showGrid, setShowGrid] = useState(false);
  const [showSafeAreas, setShowSafeAreas] = useState(false);
  const [showSmartGuides, setShowSmartGuides] = useState(true);
  const [snapToObjects, setSnapToObjects] = useState(true);
  const [menu, setMenu] = useState<{ clipId: string | null; x: number; y: number } | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [editingTextClipId, setEditingTextClipId] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  const toggleFullscreen = () => {
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else if (rootRef.current) {
      void rootRef.current.requestFullscreen();
    }
  };
  const [liveTransform, setLiveTransform] = useState<{ clipId: string; patch: LivePatch } | null>(null);
  const [cropMode, setCropMode] = useState(false);
  const [cropDraft, setCropDraft] = useState<CropBox | null>(null);
  const [smartGuides, setSmartGuides] = useState<Array<{ axis: 'x' | 'y'; pos: number }>>([]);
  const [dragReadout, setDragReadout] = useState<string | null>(null);
  const dragRef = useRef<DragState | null>(null);
  // Refs mirror the live drag state so pointer-up commits can read the final
  // value without side effects inside setState updaters (StrictMode runs
  // updaters twice in dev, which would double-commit).
  const liveTransformRef = useRef<{ clipId: string; patch: LivePatch } | null>(null);
  const cropDraftRef = useRef<CropBox | null>(null);
  const applyLiveTransform = (value: { clipId: string; patch: LivePatch } | null) => {
    liveTransformRef.current = value;
    setLiveTransform(value);
  };
  const applyCropDraft = (value: CropBox | null) => {
    cropDraftRef.current = value;
    setCropDraft(value);
  };

  const canvasWidth = timeline?.canvas.width || 1920;
  const canvasHeight = timeline?.canvas.height || 1080;
  const stageScale = stageSize.width > 0 ? stageSize.width / canvasWidth : 0;

  // Visible tracks contribute visuals even when muted (mute only silences audio,
  // matching export semantics); hidden tracks contribute nothing.
  const layers: LayerEntry[] = (timeline?.tracks || [])
    .flatMap((track, trackIndex) => track.clips.map((clip) => ({ track, trackIndex, clip })))
    .filter(({ track }) => track.visible && VISUAL_TRACK_TYPES.includes(track.type))
    .filter(({ clip }) => playheadMs >= clip.start_ms && playheadMs < clip.start_ms + clip.duration_ms)
    .map((entry) => ({ ...entry, asset: entry.clip.asset_id ? assets.find((item) => item.id === entry.clip.asset_id) : undefined }))
    // Audio-only and detached-audio clips contribute no visuals.
    .filter(({ clip, asset }) => !clip.audio_only && (Boolean(clip.text) || !asset || !asset.mime_type.startsWith('audio/')))
    .sort((a, b) => (a.trackIndex - b.trackIndex) || ((a.clip.z_index ?? 0) - (b.clip.z_index ?? 0)));

  // Audio clips active at the playhead mount hidden <audio> elements so the
  // preview is audible — on any unmuted track, matching export semantics where
  // the asset (not the track type) decides audio contribution.
  const audioLayers = (timeline?.tracks || [])
    .filter((track) => !track.muted && (!soloTrackId || track.id === soloTrackId))
    .flatMap((track) => track.clips)
    .filter((clip) => playheadMs >= clip.start_ms && playheadMs < clip.start_ms + clip.duration_ms)
    .map((clip) => ({ clip, asset: clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined }))
    // Audio assets play directly; detached-audio (audio_only) video clips
    // play their soundtrack through a hidden <audio> element too.
    .filter((entry): entry is { clip: VideoTimelineClip; asset: VideoAsset } =>
      Boolean(entry.asset && !entry.clip.muted &&
        (entry.asset.mime_type.startsWith('audio/') || (entry.clip.audio_only && entry.asset.mime_type.startsWith('video/')))));

  layersRef.current = layers;
  audioLayersRef.current = audioLayers;
  timelineDurationRef.current = timeline?.duration_ms ?? 0;

  // Topmost video layers get real <video> elements, up to the cap.
  const allowedVideoIds = new Set<string>();
  for (let i = layers.length - 1; i >= 0 && allowedVideoIds.size < MAX_VIDEO_ELEMENTS; i--) {
    if (layers[i].asset?.mime_type.startsWith('video/')) allowedVideoIds.add(layers[i].clip.id);
  }

  const selectedEntry = layers.find((layer) => layer.clip.id === selectedClipId);
  const selectedIsMedia = Boolean(
    selectedEntry?.asset && (selectedEntry.asset.mime_type.startsWith('video/') || selectedEntry.asset.mime_type.startsWith('image/')),
  );
  const canCrop = Boolean(selectedEntry && selectedIsMedia && !selectedEntry.track.locked && !isPlaying);

  // Crop mode is per-selection; leaving the clip (or playing) leaves the mode.
  useEffect(() => {
    setCropMode(false);
    setCropDraft(null);
    cropDraftRef.current = null;
  }, [selectedClipId, isPlaying]);

  // Text editing follows the selection; playing always ends the edit session.
  useEffect(() => {
    setEditingTextClipId(null);
  }, [selectedClipId, isPlaying]);

  // Fit the stage to the available area while preserving the canvas aspect ratio,
  // so layer math can use exact pixel scale.
  useEffect(() => {
    const node = fitRef.current;
    if (!node) return;
    const observer = new ResizeObserver(() => {
      const availWidth = node.clientWidth;
      const availHeight = node.clientHeight;
      if (availWidth <= 0 || availHeight <= 0) return;
      const ratio = canvasWidth / canvasHeight;
      const width = Math.min(availWidth, availHeight * ratio);
      setStageSize({ width, height: width / ratio });
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [canvasWidth, canvasHeight]);

  // rAF loop: advance playheadMs in the store while playing.
  useEffect(() => {
    if (!isPlaying) {
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
      return;
    }
    const startWall = performance.now();
    const startPlayhead = useVideoStudioStore.getState().playheadMs;
    const tick = () => {
      const elapsed = performance.now() - startWall;
      const next = startPlayhead + elapsed;
      const dur = timelineDurationRef.current;
      if (dur > 0 && next >= dur) {
        setPlayhead(0);
        setPlaying(false);
        return;
      }
      setPlayhead(next);
      rafRef.current = requestAnimationFrame(tick);
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, [isPlaying, setPlayhead, setPlaying]);

  // Keep every mounted media element in sync with the playhead on every tick.
  // This is what starts clips that mount mid-playback: a clip whose element
  // appears after play began is paused, so the next tick seeks it to its trim
  // offset and plays it (previously only clips mounted at play-start played).
  // While paused it doubles as the scrub-seek path.
  useEffect(() => {
    const syncElement = (element: HTMLMediaElement, clip: VideoTimelineClip) => {
      const target = ((clip.trim_in_ms ?? 0) + Math.max(0, playheadMs - clip.start_ms)) / 1000;
      if (isPlaying) {
        if (element.paused && !element.ended) {
          element.currentTime = target;
          element.play().catch(() => { /* autoplay policy */ });
        } else if (Math.abs(element.currentTime - target) > 0.35) {
          // Drift correction (tab throttling, slow decode).
          element.currentTime = target;
        }
      } else {
        if (!element.paused) element.pause();
        if (Math.abs(element.currentTime - target) > 0.05) {
          element.currentTime = target;
        }
      }
    };
    for (const [clipId, video] of videoRefs.current) {
      const entry = layersRef.current.find((layer) => layer.clip.id === clipId);
      if (entry) syncElement(video, entry.clip);
    }
    for (const [clipId, audio] of audioRefs.current) {
      const entry = audioLayersRef.current.find((layer) => layer.clip.id === clipId);
      if (!entry) continue;
      // Element volume caps at 1; clip volumes above 1 only boost at export.
      audio.volume = Math.min(1, Math.max(0, (entry.clip.volume ?? 1) * fadeFactor(entry.clip, playheadMs)));
      syncElement(audio, entry.clip);
    }
  }, [playheadMs, isPlaying]);

  /** Alignment candidates in client coordinates, captured once per drag. */
  const collectSnapCandidates = (excludeClipId: string) => {
    const stage = stageRef.current;
    const candidatesX: number[] = [];
    const candidatesY: number[] = [];
    if (!stage) return { candidatesX, candidatesY, stageRect: { left: 0, top: 0, width: 0, height: 0 } };
    const rect = stage.getBoundingClientRect();
    candidatesX.push(rect.left, rect.left + rect.width / 2, rect.right);
    candidatesY.push(rect.top, rect.top + rect.height / 2, rect.bottom);
    // Safe-area bounds double as common margins.
    candidatesX.push(rect.left + rect.width * 0.05, rect.right - rect.width * 0.05, rect.left + rect.width * 0.1, rect.right - rect.width * 0.1);
    candidatesY.push(rect.top + rect.height * 0.05, rect.bottom - rect.height * 0.05, rect.top + rect.height * 0.1, rect.bottom - rect.height * 0.1);
    if (snapToObjects) {
      stage.querySelectorAll<HTMLElement>('[data-preview-clip-id]').forEach((node) => {
        if (node.dataset.previewClipId === excludeClipId) return;
        const other = node.getBoundingClientRect();
        candidatesX.push(other.left, other.left + other.width / 2, other.right);
        candidatesY.push(other.top, other.top + other.height / 2, other.bottom);
      });
    }
    return { candidatesX, candidatesY, stageRect: { left: rect.left, top: rect.top, width: rect.width, height: rect.height } };
  };

  const beginDrag = (
    mode: DragState['mode'],
    entry: LayerEntry,
    event: ReactPointerEvent<HTMLElement>,
    layerElement: HTMLElement,
    handle?: { hx: -1 | 0 | 1; hy: -1 | 0 | 1 },
  ) => {
    event.preventDefault();
    event.stopPropagation();
    const transform = entry.clip.transform || { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };
    const rect = layerElement.getBoundingClientRect();
    const centerClientX = rect.left + rect.width / 2;
    const centerClientY = rect.top + rect.height / 2;
    const { candidatesX, candidatesY, stageRect } = collectSnapCandidates(entry.clip.id);
    dragRef.current = {
      mode,
      handle,
      clipId: entry.clip.id,
      isShape: Boolean(entry.clip.shape),
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      base: {
        x: transform.x ?? 0,
        y: transform.y ?? 0,
        scale: transform.scale ?? 1,
        rotation: transform.rotation ?? 0,
        shapeWidth: entry.clip.shape?.width || 320,
        shapeHeight: entry.clip.shape?.height || 180,
      },
      centerClientX,
      centerClientY,
      anchorClientX: handle ? centerClientX - handle.hx * rect.width / 2 : centerClientX,
      anchorClientY: handle ? centerClientY - handle.hy * rect.height / 2 : centerClientY,
      startAngle: Math.atan2(event.clientY - centerClientY, event.clientX - centerClientX),
      baseRect: { left: rect.left, top: rect.top, width: rect.width, height: rect.height },
      stageRect,
      candidatesX,
      candidatesY,
    };

    const axisDistance = (px: number, py: number, ax: number, ay: number, h: { hx: number; hy: number }) => {
      if (h.hx !== 0 && h.hy !== 0) return Math.hypot(px - ax, py - ay);
      if (h.hx !== 0) return Math.abs(px - ax);
      return Math.abs(py - ay);
    };

    const onMove = (moveEvent: PointerEvent) => {
      const drag = dragRef.current;
      if (!drag || moveEvent.pointerId !== drag.pointerId || stageScale <= 0) return;
      if (drag.mode === 'move') {
        let dxClient = moveEvent.clientX - drag.startClientX;
        let dyClient = moveEvent.clientY - drag.startClientY;
        const guides: Array<{ axis: 'x' | 'y'; pos: number }> = [];
        // Ctrl/Cmd temporarily bypasses snapping.
        if (snappingEnabled && !moveEvent.ctrlKey && !moveEvent.metaKey) {
          const moved = {
            left: drag.baseRect.left + dxClient,
            right: drag.baseRect.left + drag.baseRect.width + dxClient,
            cx: drag.baseRect.left + drag.baseRect.width / 2 + dxClient,
            top: drag.baseRect.top + dyClient,
            bottom: drag.baseRect.top + drag.baseRect.height + dyClient,
            cy: drag.baseRect.top + drag.baseRect.height / 2 + dyClient,
          };
          let bestX: { adjust: number; pos: number } | null = null;
          for (const candidate of drag.candidatesX) {
            for (const edge of [moved.left, moved.cx, moved.right]) {
              const distance = candidate - edge;
              if (Math.abs(distance) <= SNAP_THRESHOLD_PX && (!bestX || Math.abs(distance) < Math.abs(bestX.adjust))) {
                bestX = { adjust: distance, pos: candidate };
              }
            }
          }
          let bestY: { adjust: number; pos: number } | null = null;
          for (const candidate of drag.candidatesY) {
            for (const edge of [moved.top, moved.cy, moved.bottom]) {
              const distance = candidate - edge;
              if (Math.abs(distance) <= SNAP_THRESHOLD_PX && (!bestY || Math.abs(distance) < Math.abs(bestY.adjust))) {
                bestY = { adjust: distance, pos: candidate };
              }
            }
          }
          if (bestX) {
            dxClient += bestX.adjust;
            guides.push({ axis: 'x', pos: bestX.pos - drag.stageRect.left });
          }
          if (bestY) {
            dyClient += bestY.adjust;
            guides.push({ axis: 'y', pos: bestY.pos - drag.stageRect.top });
          }
        }
        const x = Math.round(drag.base.x + dxClient / stageScale);
        const y = Math.round(drag.base.y + dyClient / stageScale);
        setSmartGuides(showSmartGuides ? guides : []);
        setDragReadout(`x ${x}  y ${y}`);
        applyLiveTransform({ clipId: drag.clipId, patch: { x, y } });
      } else if (drag.mode === 'resize' && drag.handle) {
        const h = drag.handle;
        if (drag.isShape) {
          // Shapes resize their real width/height in canvas pixels; deltas are
          // rotated into the shape's local axes first.
          const factor = stageScale * drag.base.scale;
          const rad = (-drag.base.rotation * Math.PI) / 180;
          const rawDx = moveEvent.clientX - drag.startClientX;
          const rawDy = moveEvent.clientY - drag.startClientY;
          const dx = (rawDx * Math.cos(rad) - rawDy * Math.sin(rad)) / Math.max(0.0001, factor);
          const dy = (rawDx * Math.sin(rad) + rawDy * Math.cos(rad)) / Math.max(0.0001, factor);
          const fromCenter = moveEvent.altKey;
          let width = drag.base.shapeWidth;
          let height = drag.base.shapeHeight;
          if (h.hx !== 0) width = Math.max(8, drag.base.shapeWidth + h.hx * dx * (fromCenter ? 2 : 1));
          if (h.hy !== 0) height = Math.max(8, drag.base.shapeHeight + h.hy * dy * (fromCenter ? 2 : 1));
          if (moveEvent.shiftKey && h.hx !== 0 && h.hy !== 0) {
            height = Math.max(8, width * (drag.base.shapeHeight / Math.max(1, drag.base.shapeWidth)));
          }
          const patch: LivePatch = { shapeWidth: Math.round(width), shapeHeight: Math.round(height) };
          if (!fromCenter) {
            // Keep the opposite edge anchored: the center shifts by half the
            // size change along the handle direction (in rotated coords).
            const shiftX = (h.hx * (width - drag.base.shapeWidth)) / 2 * drag.base.scale;
            const shiftY = (h.hy * (height - drag.base.shapeHeight)) / 2 * drag.base.scale;
            const rotRad = (drag.base.rotation * Math.PI) / 180;
            patch.x = Math.round(drag.base.x + shiftX * Math.cos(rotRad) - shiftY * Math.sin(rotRad));
            patch.y = Math.round(drag.base.y + shiftX * Math.sin(rotRad) + shiftY * Math.cos(rotRad));
          }
          setDragReadout(`${Math.round(width)} × ${Math.round(height)} px`);
          applyLiveTransform({ clipId: drag.clipId, patch });
        } else {
          // Media and text scale uniformly. Alt resizes around the center;
          // otherwise the opposite corner/edge stays anchored.
          const fromCenter = moveEvent.altKey;
          const ax = fromCenter ? drag.centerClientX : drag.anchorClientX;
          const ay = fromCenter ? drag.centerClientY : drag.anchorClientY;
          const startDistance = Math.max(8, axisDistance(drag.startClientX, drag.startClientY, ax, ay, h));
          const distance = axisDistance(moveEvent.clientX, moveEvent.clientY, ax, ay, h);
          const scale = Math.min(4, Math.max(0.05, drag.base.scale * (distance / startDistance)));
          const patch: LivePatch = { scale };
          if (!fromCenter) {
            const ratio = scale / drag.base.scale;
            const newCenterX = ax + (drag.centerClientX - ax) * ratio;
            const newCenterY = ay + (drag.centerClientY - ay) * ratio;
            patch.x = Math.round(drag.base.x + (newCenterX - drag.centerClientX) / stageScale);
            patch.y = Math.round(drag.base.y + (newCenterY - drag.centerClientY) / stageScale);
          }
          setDragReadout(`${Math.round(canvasWidth * scale)} × ${Math.round(canvasHeight * scale)} px · ${Math.round(scale * 100)}%`);
          applyLiveTransform({ clipId: drag.clipId, patch });
        }
      } else {
        const angle = Math.atan2(moveEvent.clientY - drag.centerClientY, moveEvent.clientX - drag.centerClientX);
        let rotation = drag.base.rotation + ((angle - drag.startAngle) * 180) / Math.PI;
        while (rotation > 180) rotation -= 360;
        while (rotation < -180) rotation += 360;
        setDragReadout(`${Math.round(rotation)}°`);
        applyLiveTransform({ clipId: drag.clipId, patch: { rotation: Math.round(rotation) } });
      }
    };
    const onUp = (upEvent: PointerEvent) => {
      const drag = dragRef.current;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      if (!drag || upEvent.pointerId !== drag.pointerId) return;
      dragRef.current = null;
      setSmartGuides([]);
      setDragReadout(null);
      // Commit once on release: a single undo entry and a single save.
      const live = liveTransformRef.current;
      if (live && live.clipId === drag.clipId && Object.keys(live.patch).length > 0) {
        const { shapeWidth, shapeHeight, ...transformPatch } = live.patch;
        if (shapeWidth !== undefined || shapeHeight !== undefined) {
          void resizeShapeClip(live.clipId, { width: shapeWidth, height: shapeHeight, x: transformPatch.x, y: transformPatch.y });
        } else if (Object.keys(transformPatch).length > 0) {
          void updateClipTransform(live.clipId, transformPatch);
        }
      }
      applyLiveTransform(null);
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  const beginCropDrag = (
    edges: Array<keyof CropBox>,
    entry: LayerEntry,
    event: ReactPointerEvent<HTMLElement>,
    boxElement: HTMLElement,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    const rect = boxElement.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return;
    const base: CropBox = { ...EMPTY_CROP, ...(cropDraft || entry.clip.transform?.crop || {}) };
    const pointerId = event.pointerId;
    const startX = event.clientX;
    const startY = event.clientY;

    const onMove = (moveEvent: PointerEvent) => {
      if (moveEvent.pointerId !== pointerId) return;
      const next: CropBox = { ...base };
      for (const edge of edges) {
        if (edge === 'top') next.top = clampCropSide(base.top + (moveEvent.clientY - startY) / rect.height, base.bottom);
        if (edge === 'bottom') next.bottom = clampCropSide(base.bottom + (startY - moveEvent.clientY) / rect.height, base.top);
        if (edge === 'left') next.left = clampCropSide(base.left + (moveEvent.clientX - startX) / rect.width, base.right);
        if (edge === 'right') next.right = clampCropSide(base.right + (startX - moveEvent.clientX) / rect.width, base.left);
      }
      setDragReadout(`crop ${Math.round(next.left * 100)}% ${Math.round(next.top * 100)}% ${Math.round(next.right * 100)}% ${Math.round(next.bottom * 100)}%`);
      applyCropDraft(next);
    };
    const onUp = (upEvent: PointerEvent) => {
      if (upEvent.pointerId !== pointerId) return;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      setDragReadout(null);
      // The draft persists across drags; Apply commits it as one undo entry.
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  const applyCrop = () => {
    const draft = cropDraftRef.current;
    if (!draft || !selectedClipId) return;
    const rounded: CropBox = {
      top: Math.round(draft.top * 1000) / 1000,
      right: Math.round(draft.right * 1000) / 1000,
      bottom: Math.round(draft.bottom * 1000) / 1000,
      left: Math.round(draft.left * 1000) / 1000,
    };
    void updateClipTransform(selectedClipId, { crop: rounded });
    applyCropDraft(null);
    setCropMode(false);
  };

  const cancelCrop = () => {
    applyCropDraft(null);
    setCropMode(false);
  };

  const commitTextEdit = (clipId: string, value: string) => {
    setEditingTextClipId(null);
    const entry = layersRef.current.find((layer) => layer.clip.id === clipId);
    const currentText = entry?.clip.text?.text ?? '';
    const nextText = value.replace(/\r\n/g, '\n');
    if (nextText !== currentText) {
      void updateClipText(clipId, { text: nextText });
    }
  };

  const renderLayer = (entry: LayerEntry) => {
    const { clip, track, asset } = entry;
    const transform = { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1, ...(clip.transform || {}) };
    // Keyframes (clip-relative time) override static transform values; an
    // in-flight canvas drag overrides both.
    const clipTimeMs = playheadMs - clip.start_ms;
    for (const property of ['x', 'y', 'scale', 'rotation', 'opacity'] as const) {
      const sampled = sampleKeyframes(clip.keyframes, property, clipTimeMs);
      if (sampled !== null) transform[property] = sampled;
    }
    let liveShapeWidth: number | undefined;
    let liveShapeHeight: number | undefined;
    if (liveTransform && liveTransform.clipId === clip.id) {
      const { shapeWidth, shapeHeight, ...transformPatch } = liveTransform.patch;
      Object.assign(transform, transformPatch);
      liveShapeWidth = shapeWidth;
      liveShapeHeight = shapeHeight;
    }
    const opacity = Math.max(0, Math.min(1, transform.opacity)) * fadeFactor(clip, playheadMs);
    const selected = clip.id === selectedClipId;
    const isMedia = Boolean(asset && (asset.mime_type.startsWith('video/') || asset.mime_type.startsWith('image/')));
    // Crop editing renders the full frame with dimmed margins; rotation is
    // suppressed while editing so handle math stays axis-aligned.
    const inCropEdit = cropMode && selected && isMedia && !isPlaying;
    const effectiveCrop: CropBox = { ...EMPTY_CROP, ...(transform.crop || {}), ...(inCropEdit && cropDraft ? cropDraft : {}) };
    const crop = transform.crop;
    const clipPath = crop && !inCropEdit
      ? `inset(${(crop.top || 0) * 100}% ${(crop.right || 0) * 100}% ${(crop.bottom || 0) * 100}% ${(crop.left || 0) * 100}%)`
      : undefined;

    const wrapperStyle: CSSProperties = {
      left: '50%',
      top: '50%',
      width: isMedia ? stageSize.width : undefined,
      height: isMedia ? stageSize.height : undefined,
      maxWidth: stageSize.width,
      transform: `translate(-50%, -50%) translate(${transform.x * stageScale}px, ${transform.y * stageScale}px) scale(${transform.scale}) rotate(${inCropEdit ? 0 : transform.rotation}deg)`,
      opacity,
      filter: composePreviewFilter(clip.effects),
    };

    const isEditingText = editingTextClipId === clip.id;

    let content = null;
    if (asset && asset.mime_type.startsWith('video/') && !allowedVideoIds.has(clip.id)) {
      content = (
        <div className="flex h-full w-full items-center justify-center bg-neutral-900/80" style={{ clipPath }}>
          <div className="max-w-md px-6 text-center">
            <p className="truncate text-sm font-medium text-white/80">{asset.file_name}</p>
            <p className="mt-1 text-[10px] text-white/40">preview capped — layer renders in export</p>
          </div>
        </div>
      );
    } else if (asset && asset.mime_type.startsWith('video/')) {
      content = (
        <video
          ref={(node) => {
            if (node) videoRefs.current.set(clip.id, node);
            else videoRefs.current.delete(clip.id);
          }}
          key={asset.id}
          src={videoApi.downloadUrl(asset.id)}
          className="h-full w-full object-contain"
          style={{ clipPath }}
          controls={false}
          playsInline
          autoPlay={false}
          muted={track.muted || Boolean(clip.muted)}
          aria-label={asset.file_name}
        />
      );
    } else if (asset && asset.mime_type.startsWith('image/')) {
      content = (
        <img src={videoApi.downloadUrl(asset.id)} alt={asset.file_name} className="h-full w-full object-contain" style={{ clipPath }} />
      );
    } else if (clip.shape) {
      content = (
        <ShapePreview
          shape={clip.shape}
          clip={clip}
          stageScale={stageScale}
          canvasHeight={canvasHeight}
          liveWidth={liveShapeWidth}
          liveHeight={liveShapeHeight}
        />
      );
    } else if (clip.text) {
      const text = clip.text;
      const fontSize = (text.font_size || Math.round(canvasHeight / 18)) * stageScale;
      const textStyle: CSSProperties = {
        fontSize,
        fontFamily: text.font_family || undefined,
        fontWeight: (text.font_weight as CSSProperties['fontWeight']) || 700,
        color: text.color || '#ffffff',
        background: text.background || undefined,
        borderRadius: text.border_radius ? text.border_radius * stageScale : undefined,
        textAlign: text.text_align || 'center',
        lineHeight: text.line_height || undefined,
        letterSpacing: text.letter_spacing ? text.letter_spacing * stageScale : undefined,
        WebkitTextStroke: text.stroke ? `${(text.stroke_width || 2) * stageScale}px ${text.stroke}` : undefined,
        textShadow: text.shadow ? '2px 2px 4px rgba(0,0,0,0.7)' : undefined,
        padding: text.background ? `${8 * stageScale}px ${18 * stageScale}px` : undefined,
      };
      if (isEditingText) {
        // contentEditable manages its own children; React renders none so
        // re-renders can't clobber the in-progress edit.
        content = (
          <div
            ref={(node) => {
              if (node && node.dataset.init !== '1') {
                node.dataset.init = '1';
                node.textContent = text.text;
                node.focus();
                const range = document.createRange();
                range.selectNodeContents(node);
                const selection = window.getSelection();
                selection?.removeAllRanges();
                selection?.addRange(range);
              }
            }}
            contentEditable
            suppressContentEditableWarning
            role="textbox"
            aria-multiline="true"
            aria-label="Edit text"
            className="min-w-8 whitespace-pre-wrap outline outline-2 outline-primary/80"
            style={{ ...textStyle, cursor: 'text' }}
            onPointerDown={(event) => event.stopPropagation()}
            onKeyDown={(event) => {
              event.stopPropagation();
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault();
                commitTextEdit(clip.id, event.currentTarget.textContent ?? '');
              } else if (event.key === 'Escape') {
                event.preventDefault();
                setEditingTextClipId(null);
              }
            }}
            onBlur={(event) => commitTextEdit(clip.id, event.currentTarget.textContent ?? '')}
          />
        );
      } else {
        content = (
          <div className="whitespace-pre-wrap" style={textStyle}>
            {text.text}
          </div>
        );
      }
    } else if (asset) {
      content = (
        <div className="max-w-md px-6 text-center">
          <p className="truncate text-sm font-medium text-white">{asset.file_name}</p>
          <p className="mt-1 text-[10px] text-white/40">{asset.kind} · {asset.mime_type}</p>
        </div>
      );
    } else {
      return null;
    }

    return (
      <div
        key={clip.id}
        data-preview-clip-id={clip.id}
        className={`absolute flex items-center justify-center ${selected && !isPlaying ? 'outline outline-1 outline-primary' : ''} ${
          track.locked ? '' : 'cursor-move'
        }`}
        style={wrapperStyle}
        onPointerDown={(event) => {
          if (isPlaying || inCropEdit || isEditingText) return;
          if (clip.id !== selectedClipId) {
            selectClip(clip.id, track.id);
            return;
          }
          if (!track.locked) beginDrag('move', entry, event, event.currentTarget);
        }}
        onDoubleClick={(event) => {
          if (isPlaying || track.locked || !clip.text || clip.shape) return;
          event.preventDefault();
          event.stopPropagation();
          if (clip.id !== selectedClipId) selectClip(clip.id, track.id);
          setEditingTextClipId(clip.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          event.stopPropagation();
          if (clip.id !== selectedClipId) selectClip(clip.id, track.id);
          setMenu({ clipId: clip.id, x: event.clientX, y: event.clientY });
        }}
      >
        {content}
        {/* Cursor-effect overlay for clips carrying recorded cursor metadata */}
        {clip.cursor && (() => {
          const sample = sampleCursor(clip.cursor, clipTimeMs);
          if (!sample) return null;
          const size = 16 * (clip.cursor.scale || 1) * stageScale * 4;
          const left = sample.x * stageScale;
          const top = sample.y * stageScale;
          return (
            <div className="pointer-events-none absolute" style={{ left, top }} aria-hidden="true">
              {clip.cursor.highlight && (
                <div
                  className="absolute rounded-full bg-yellow-300/30"
                  style={{ width: size * 2.2, height: size * 2.2, left: -size * 1.1, top: -size * 1.1 }}
                />
              )}
              {sample.click && clip.cursor.click_rings && (
                <div
                  className="absolute rounded-full border-2 border-sky-400/80"
                  style={{ width: size * 2.6, height: size * 2.6, left: -size * 1.3, top: -size * 1.3 }}
                />
              )}
              <svg width={size} height={size} viewBox="0 0 16 16">
                <path d="M2 1 L2 12 L5.5 9.5 L7.5 14 L9.5 13 L7.5 8.8 L12 8.5 Z" fill="#ffffff" stroke="#111827" strokeWidth="1" />
              </svg>
            </div>
          );
        })()}
        {selected && !isPlaying && !track.locked && !inCropEdit && !isEditingText && (
          <>
            {RESIZE_HANDLES.map(({ id, hx, hy, className }) => (
              <button
                key={id}
                type="button"
                className={`absolute h-3 w-3 rounded-sm border border-white bg-primary ${className}`}
                title="Drag to resize — Shift keeps aspect, Alt resizes from center, Ctrl bypasses snapping"
                aria-label={`Resize clip (${id})`}
                onPointerDown={(event) => beginDrag('resize', entry, event, event.currentTarget.parentElement as HTMLElement, { hx, hy })}
              />
            ))}
            <button
              type="button"
              className="absolute -top-5 left-1/2 h-3 w-3 -translate-x-1/2 cursor-grab rounded-full border border-white bg-primary"
              title="Drag to rotate"
              aria-label="Rotate clip"
              onPointerDown={(event) => beginDrag('rotate', entry, event, event.currentTarget.parentElement as HTMLElement)}
            />
          </>
        )}
        {inCropEdit && (() => {
          const { top, right, bottom, left } = effectiveCrop;
          const centerXPct = (left + (1 - left - right) / 2) * 100;
          const centerYPct = (top + (1 - top - bottom) / 2) * 100;
          const handles: Array<{ key: string; edges: Array<keyof CropBox>; style: CSSProperties; cursor: string }> = [
            { key: 'top', edges: ['top'], style: { left: `${centerXPct}%`, top: `${top * 100}%` }, cursor: 'cursor-ns-resize' },
            { key: 'bottom', edges: ['bottom'], style: { left: `${centerXPct}%`, top: `${(1 - bottom) * 100}%` }, cursor: 'cursor-ns-resize' },
            { key: 'left', edges: ['left'], style: { left: `${left * 100}%`, top: `${centerYPct}%` }, cursor: 'cursor-ew-resize' },
            { key: 'right', edges: ['right'], style: { left: `${(1 - right) * 100}%`, top: `${centerYPct}%` }, cursor: 'cursor-ew-resize' },
            { key: 'top-left', edges: ['top', 'left'], style: { left: `${left * 100}%`, top: `${top * 100}%` }, cursor: 'cursor-nwse-resize' },
            { key: 'top-right', edges: ['top', 'right'], style: { left: `${(1 - right) * 100}%`, top: `${top * 100}%` }, cursor: 'cursor-nesw-resize' },
            { key: 'bottom-left', edges: ['bottom', 'left'], style: { left: `${left * 100}%`, top: `${(1 - bottom) * 100}%` }, cursor: 'cursor-nesw-resize' },
            { key: 'bottom-right', edges: ['bottom', 'right'], style: { left: `${(1 - right) * 100}%`, top: `${(1 - bottom) * 100}%` }, cursor: 'cursor-nwse-resize' },
          ];
          const cropLeft = left * 100;
          const cropTop = top * 100;
          const cropWidth = (1 - left - right) * 100;
          const cropHeight = (1 - top - bottom) * 100;
          return (
            <div className="absolute inset-0">
              <div className="absolute inset-x-0 top-0 bg-black/60" style={{ height: `${top * 100}%` }} />
              <div className="absolute inset-x-0 bottom-0 bg-black/60" style={{ height: `${bottom * 100}%` }} />
              <div className="absolute left-0 bg-black/60" style={{ top: `${top * 100}%`, bottom: `${bottom * 100}%`, width: `${left * 100}%` }} />
              <div className="absolute right-0 bg-black/60" style={{ top: `${top * 100}%`, bottom: `${bottom * 100}%`, width: `${right * 100}%` }} />
              <div
                className="pointer-events-none absolute border border-dashed border-white/80"
                style={{ top: `${cropTop}%`, left: `${cropLeft}%`, width: `${cropWidth}%`, height: `${cropHeight}%` }}
              >
                {/* Thirds grid inside the crop boundary */}
                <div className="absolute left-1/3 top-0 h-full w-px bg-white/30" />
                <div className="absolute left-2/3 top-0 h-full w-px bg-white/30" />
                <div className="absolute left-0 top-1/3 h-px w-full bg-white/30" />
                <div className="absolute left-0 top-2/3 h-px w-full bg-white/30" />
              </div>
              {handles.map(({ key, edges, style, cursor }) => (
                <button
                  key={key}
                  type="button"
                  className={`absolute h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-sm border border-white bg-amber-400 ${cursor}`}
                  style={style}
                  title={`Crop ${key}`}
                  aria-label={`Crop ${key}`}
                  onPointerDown={(event) => beginCropDrag(edges, entry, event, event.currentTarget.parentElement as HTMLElement)}
                />
              ))}
            </div>
          );
        })()}
      </div>
    );
  };

  return (
    <div ref={rootRef} className="flex h-full min-h-[320px] flex-col rounded-lg border border-border bg-black">
      <div className="hidden">
        {audioLayers.map(({ clip, asset }) => (
          <audio
            key={clip.id}
            ref={(node) => {
              if (node) audioRefs.current.set(clip.id, node);
              else audioRefs.current.delete(clip.id);
            }}
            src={videoApi.downloadUrl(asset.id)}
            preload="auto"
            aria-hidden="true"
          />
        ))}
      </div>
      <div className="flex items-center justify-between border-b border-white/10 px-3 py-2 text-xs text-white/70">
        <span>Preview</span>
        <div className="flex items-center gap-1">
          <button
            onClick={() => (cropMode ? cancelCrop() : setCropMode(true))}
            disabled={!canCrop}
            className={`inline-flex h-6 w-6 items-center justify-center rounded border disabled:cursor-not-allowed disabled:opacity-35 ${cropMode ? 'border-amber-400/60 bg-amber-400/15 text-amber-300' : 'border-white/15 text-white/55 hover:text-white'}`}
            title={canCrop ? (cropMode ? 'Exit crop mode' : 'Crop selected clip') : 'Select an unlocked video/image clip to crop'}
            aria-label="Toggle crop mode"
          >
            <Crop size={12} />
          </button>
          {cropMode && canCrop && (
            <>
              <button
                onClick={applyCrop}
                disabled={!cropDraft}
                className="inline-flex items-center gap-1 rounded border border-emerald-400/50 bg-emerald-400/10 px-1.5 py-0.5 text-[10px] text-emerald-300 disabled:cursor-not-allowed disabled:opacity-40"
                title="Apply the crop to the selected clip"
              >
                <Check size={10} /> Apply
              </button>
              <button
                onClick={cancelCrop}
                className="inline-flex items-center gap-1 rounded border border-white/15 px-1.5 py-0.5 text-[10px] text-white/55 hover:text-white"
                title="Discard crop changes"
              >
                <X size={10} /> Cancel
              </button>
              {selectedEntry?.clip.transform?.crop && (
                <button
                  onClick={() => {
                    applyCropDraft(null);
                    void updateClipTransform(selectedClipId as string, { crop: undefined });
                  }}
                  className="rounded border border-white/15 px-1.5 py-0.5 text-[10px] text-white/55 hover:text-white"
                  title="Remove crop from the selected clip"
                >
                  Reset crop
                </button>
              )}
            </>
          )}
          <button
            onClick={() => setShowGrid((value) => !value)}
            className={`inline-flex h-6 w-6 items-center justify-center rounded border ${showGrid ? 'border-primary/50 bg-primary/15 text-primary' : 'border-white/15 text-white/55 hover:text-white'}`}
            title="Toggle thirds grid"
            aria-label="Toggle thirds grid"
          >
            <Grid3x3 size={12} />
          </button>
          <button
            onClick={() => setShowSafeAreas((value) => !value)}
            className={`inline-flex h-6 w-6 items-center justify-center rounded border ${showSafeAreas ? 'border-primary/50 bg-primary/15 text-primary' : 'border-white/15 text-white/55 hover:text-white'}`}
            title="Toggle safe-area guides"
            aria-label="Toggle safe-area guides"
          >
            <Crosshair size={12} />
          </button>
          <button
            onClick={toggleFullscreen}
            className="inline-flex h-6 w-6 items-center justify-center rounded border border-white/15 text-white/55 hover:text-white"
            title={isFullscreen ? 'Exit fullscreen preview' : 'Fullscreen preview'}
            aria-label={isFullscreen ? 'Exit fullscreen preview' : 'Fullscreen preview'}
          >
            {isFullscreen ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
          </button>
          <span className="ml-2">{timeline ? `${timeline.canvas.width}x${timeline.canvas.height} · ${timeline.canvas.fps}fps` : 'No timeline'}</span>
        </div>
      </div>
      <div ref={fitRef} className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden p-4">
        <div
          ref={stageRef}
          className="relative overflow-hidden border border-white/10"
          style={{
            width: stageSize.width || undefined,
            height: stageSize.height || undefined,
            background: timeline?.canvas.background || '#000000',
          }}
          onPointerDown={(event) => {
            // Clicking empty stage space deselects.
            if (event.target === event.currentTarget) selectClip(null);
          }}
          onContextMenu={(event) => {
            if (event.target !== event.currentTarget) return;
            event.preventDefault();
            setMenu({ clipId: null, x: event.clientX, y: event.clientY });
          }}
        >
          {layers.map((entry) => renderLayer(entry))}
          {layers.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center text-xs text-white/55">
              No active visual clip at playhead
            </div>
          )}
          {smartGuides.length > 0 && (
            <div className="pointer-events-none absolute inset-0 z-10">
              {smartGuides.map((guide) => guide.axis === 'x'
                ? <div key={`x-${guide.pos}`} className="absolute top-0 h-full w-px bg-primary/90" style={{ left: guide.pos }} />
                : <div key={`y-${guide.pos}`} className="absolute left-0 h-px w-full bg-primary/90" style={{ top: guide.pos }} />)}
            </div>
          )}
          {dragReadout && (
            <div className="pointer-events-none absolute left-2 top-2 z-20 rounded bg-black/75 px-1.5 py-0.5 font-mono text-[10px] tabular-nums text-white">
              {dragReadout}
            </div>
          )}
          {showGrid && (
            <div className="pointer-events-none absolute inset-0">
              <div className="absolute left-1/3 top-0 h-full w-px bg-white/25" />
              <div className="absolute left-2/3 top-0 h-full w-px bg-white/25" />
              <div className="absolute left-0 top-1/3 h-px w-full bg-white/25" />
              <div className="absolute left-0 top-2/3 h-px w-full bg-white/25" />
            </div>
          )}
          {showSafeAreas && (
            <div className="pointer-events-none absolute inset-0">
              <div className="absolute inset-[5%] border border-dashed border-amber-300/40" title="Action safe" />
              <div className="absolute inset-[10%] border border-dashed border-amber-300/60" title="Title safe" />
              <div className="absolute left-1/2 top-0 h-full w-px bg-white/15" />
              <div className="absolute left-0 top-1/2 h-px w-full bg-white/15" />
            </div>
          )}
        </div>
      </div>
      <div className="flex items-center gap-3 border-t border-white/10 px-3 py-2">
        <button
          onClick={() => setPlaying(!isPlaying)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-white/15 bg-white/5 text-white/75 hover:text-white"
          title={isPlaying ? 'Pause preview' : 'Play preview'}
          aria-label={isPlaying ? 'Pause preview' : 'Play preview'}
        >
          {isPlaying ? <Pause size={14} /> : <Play size={14} />}
        </button>
        <span className="text-xs text-white/65">
          {formatTime(playheadMs)} / {formatTime(timeline?.duration_ms || 0)}
        </span>
        <div className="h-1 flex-1 rounded-full bg-white/10">
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${timeline ? Math.min(100, (playheadMs / Math.max(1, timeline.duration_ms)) * 100) : 0}%` }}
          />
        </div>
      </div>
      {menu && (() => {
        const entry = menu.clipId ? layers.find((layer) => layer.clip.id === menu.clipId) : undefined;
        // Cover the canvas while preserving the asset's aspect ratio
        // (object-contain renders at scale 1 = fit).
        const fillScale = (asset?: VideoAsset): number => {
          if (!asset?.width || !asset?.height) return 1;
          const assetAR = asset.width / asset.height;
          const canvasAR = canvasWidth / canvasHeight;
          return assetAR >= canvasAR ? assetAR / canvasAR : canvasAR / assetAR;
        };
        const viewToggles: ContextMenuEntry[] = [
          { label: showGrid ? 'Hide grid' : 'Show grid', action: () => setShowGrid((value) => !value) },
          { label: showSafeAreas ? 'Hide safe areas' : 'Show safe areas', action: () => setShowSafeAreas((value) => !value) },
          { label: showSmartGuides ? 'Hide smart guides' : 'Show smart guides', action: () => setShowSmartGuides((value) => !value) },
          { label: snapToObjects ? 'Disable snap to objects' : 'Snap to objects', action: () => setSnapToObjects((value) => !value) },
        ];
        let items: ContextMenuEntry[];
        if (entry) {
          const { clip, track, asset } = entry;
          const isMedia = Boolean(asset && (asset.mime_type.startsWith('video/') || asset.mime_type.startsWith('image/')));
          const stackIndex = layers.findIndex((layer) => layer.clip.id === clip.id);
          items = [
            {
              label: 'Select next clip underneath',
              disabled: layers.length < 2,
              action: () => {
                const next = layers[(stackIndex - 1 + layers.length) % layers.length];
                selectClip(next.clip.id, next.track.id);
              },
            },
            'divider',
          ];
          if (clip.text && !clip.shape) {
            items.push({ label: 'Edit text', disabled: track.locked, action: () => setEditingTextClipId(clip.id) });
          }
          items.push(
            { label: 'Crop clip', disabled: !isMedia || track.locked || isPlaying, action: () => setCropMode(true) },
            { label: 'Reset crop', disabled: !clip.transform?.crop, action: () => { void updateClipTransform(clip.id, { crop: undefined }); } },
            'divider',
            { label: 'Fit to canvas', disabled: !isMedia || track.locked, action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: 1 }); } },
            { label: 'Fill canvas', disabled: !isMedia || track.locked, action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: fillScale(asset) }); } },
            { label: 'Center on canvas', disabled: track.locked, action: () => { void updateClipTransform(clip.id, { x: 0, y: 0 }); } },
            { label: 'Reset transform', disabled: track.locked, action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1, crop: undefined }); } },
            'divider',
            { label: 'Bring forward', disabled: track.locked, action: () => { void bringClipForward(clip.id); } },
            { label: 'Send backward', disabled: track.locked, action: () => { void sendClipBackward(clip.id); } },
            'divider',
            ...viewToggles,
          );
        } else {
          items = [
            { label: 'Add title card', action: () => { void addTextClip(); } },
            { label: 'Add highlight box', action: () => { void addShapeClip('highlight'); } },
            { label: 'Add rectangle callout', action: () => { void addShapeClip('rectangle'); } },
            { label: 'Add blur region', action: () => { void addShapeClip('blur'); } },
            { label: 'Add arrow', action: () => { void addShapeClip('arrow'); } },
            { label: 'Add ellipse', action: () => { void addShapeClip('ellipse'); } },
            { label: 'Add spotlight', action: () => { void addShapeClip('spotlight'); } },
            { label: 'Add label callout', action: () => { void addShapeClip('label'); } },
            { label: 'Add numbered step', action: () => { void addShapeClip('step_marker'); } },
            'divider',
            ...viewToggles,
          ];
        }
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
    </div>
  );
}
