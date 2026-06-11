import { Crop, Crosshair, Grid3x3, Pause, Play } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { videoApi } from '../../api';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { composePreviewFilter } from './effects/effectRegistry';
import { sampleKeyframes } from './effects/keyframeUtils';
import type { VideoAsset, VideoTimelineClip, VideoTimelineTrack } from '../../types/video';

function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

const VISUAL_TRACK_TYPES = ['video', 'image', 'text', 'caption', 'shape', 'callout'];

// Mounted <video> elements are expensive; only the topmost few video layers get
// real elements — deeper ones render a lightweight placeholder card.
const MAX_VIDEO_ELEMENTS = 4;

type CropBox = { top: number; right: number; bottom: number; left: number };

const EMPTY_CROP: CropBox = { top: 0, right: 0, bottom: 0, left: 0 };

/** Clamp one crop side so each side stays in [0, 0.95] and ≥10% of the frame survives. */
function clampCropSide(value: number, opposite: number): number {
  return Math.max(0, Math.min(0.95, Math.min(value, 0.9 - opposite)));
}

interface LayerEntry {
  track: VideoTimelineTrack;
  trackIndex: number;
  clip: VideoTimelineClip;
  asset?: VideoAsset;
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

interface DragState {
  mode: 'move' | 'scale' | 'rotate';
  clipId: string;
  pointerId: number;
  startClientX: number;
  startClientY: number;
  base: { x: number; y: number; scale: number; rotation: number };
  centerClientX: number;
  centerClientY: number;
  startDist: number;
  startAngle: number;
}

export function VideoPreviewCanvas() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const updateClipTransform = useVideoStudioStore((state) => state.updateClipTransform);

  const rafRef = useRef<number | null>(null);
  const timelineDurationRef = useRef<number>(0);
  const videoRefs = useRef(new Map<string, HTMLVideoElement>());
  const audioRefs = useRef(new Map<string, HTMLAudioElement>());
  const layersRef = useRef<LayerEntry[]>([]);
  const audioLayersRef = useRef<Array<{ clip: VideoTimelineClip; asset: VideoAsset }>>([]);
  const fitRef = useRef<HTMLDivElement | null>(null);
  const [stageSize, setStageSize] = useState<{ width: number; height: number }>({ width: 0, height: 0 });
  const [showGrid, setShowGrid] = useState(false);
  const [showSafeAreas, setShowSafeAreas] = useState(false);
  const [liveTransform, setLiveTransform] = useState<{ clipId: string; patch: Partial<DragState['base']> } | null>(null);
  const [cropMode, setCropMode] = useState(false);
  const [cropDraft, setCropDraft] = useState<CropBox | null>(null);
  const [moveSnap, setMoveSnap] = useState<{ x: boolean; y: boolean } | null>(null);
  const dragRef = useRef<DragState | null>(null);

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
    .sort((a, b) => (a.trackIndex - b.trackIndex) || ((a.clip.z_index ?? 0) - (b.clip.z_index ?? 0)));

  // Audio/music clips active at the playhead mount hidden <audio> elements so
  // the preview is audible. Muted tracks contribute no audio (matching export).
  const audioLayers = (timeline?.tracks || [])
    .filter((track) => (track.type === 'audio' || track.type === 'music') && !track.muted)
    .flatMap((track) => track.clips)
    .filter((clip) => playheadMs >= clip.start_ms && playheadMs < clip.start_ms + clip.duration_ms)
    .map((clip) => ({ clip, asset: clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined }))
    .filter((entry): entry is { clip: VideoTimelineClip; asset: VideoAsset } =>
      Boolean(entry.asset && entry.asset.mime_type.startsWith('audio/')));

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

  const beginDrag = (
    mode: DragState['mode'],
    entry: LayerEntry,
    event: ReactPointerEvent<HTMLElement>,
    layerElement: HTMLElement,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    const transform = entry.clip.transform || { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };
    const rect = layerElement.getBoundingClientRect();
    const centerClientX = rect.left + rect.width / 2;
    const centerClientY = rect.top + rect.height / 2;
    dragRef.current = {
      mode,
      clipId: entry.clip.id,
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      base: {
        x: transform.x ?? 0,
        y: transform.y ?? 0,
        scale: transform.scale ?? 1,
        rotation: transform.rotation ?? 0,
      },
      centerClientX,
      centerClientY,
      startDist: Math.max(8, Math.hypot(event.clientX - centerClientX, event.clientY - centerClientY)),
      startAngle: Math.atan2(event.clientY - centerClientY, event.clientX - centerClientX),
    };

    const onMove = (moveEvent: PointerEvent) => {
      const drag = dragRef.current;
      if (!drag || moveEvent.pointerId !== drag.pointerId || stageScale <= 0) return;
      if (drag.mode === 'move') {
        let x = Math.round(drag.base.x + (moveEvent.clientX - drag.startClientX) / stageScale);
        let y = Math.round(drag.base.y + (moveEvent.clientY - drag.startClientY) / stageScale);
        let snappedX = false;
        let snappedY = false;
        if (snappingEnabled) {
          const thresholdCanvasPx = 8 / stageScale;
          if (Math.abs(x) < thresholdCanvasPx) {
            x = 0;
            snappedX = true;
          }
          if (Math.abs(y) < thresholdCanvasPx) {
            y = 0;
            snappedY = true;
          }
        }
        setMoveSnap(snappedX || snappedY ? { x: snappedX, y: snappedY } : null);
        setLiveTransform({ clipId: drag.clipId, patch: { x, y } });
      } else if (drag.mode === 'scale') {
        const dist = Math.hypot(moveEvent.clientX - drag.centerClientX, moveEvent.clientY - drag.centerClientY);
        setLiveTransform({
          clipId: drag.clipId,
          patch: { scale: Math.min(4, Math.max(0.05, drag.base.scale * (dist / drag.startDist))) },
        });
      } else {
        const angle = Math.atan2(moveEvent.clientY - drag.centerClientY, moveEvent.clientX - drag.centerClientX);
        let rotation = drag.base.rotation + ((angle - drag.startAngle) * 180) / Math.PI;
        while (rotation > 180) rotation -= 360;
        while (rotation < -180) rotation += 360;
        setLiveTransform({ clipId: drag.clipId, patch: { rotation: Math.round(rotation) } });
      }
    };
    const onUp = (upEvent: PointerEvent) => {
      const drag = dragRef.current;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      if (!drag || upEvent.pointerId !== drag.pointerId) return;
      dragRef.current = null;
      setMoveSnap(null);
      // Commit once on release: a single undo entry and a single save.
      setLiveTransform((live) => {
        if (live && live.clipId === drag.clipId && Object.keys(live.patch).length > 0) {
          void updateClipTransform(live.clipId, live.patch);
        }
        return null;
      });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  const beginCropDrag = (
    edge: keyof CropBox,
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
      if (edge === 'top') next.top = clampCropSide(base.top + (moveEvent.clientY - startY) / rect.height, base.bottom);
      if (edge === 'bottom') next.bottom = clampCropSide(base.bottom + (startY - moveEvent.clientY) / rect.height, base.top);
      if (edge === 'left') next.left = clampCropSide(base.left + (moveEvent.clientX - startX) / rect.width, base.right);
      if (edge === 'right') next.right = clampCropSide(base.right + (startX - moveEvent.clientX) / rect.width, base.left);
      setCropDraft(next);
    };
    const onUp = (upEvent: PointerEvent) => {
      if (upEvent.pointerId !== pointerId) return;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      // Commit once on release: a single undo entry and a single save.
      setCropDraft((draft) => {
        if (draft) {
          const rounded: CropBox = {
            top: Math.round(draft.top * 1000) / 1000,
            right: Math.round(draft.right * 1000) / 1000,
            bottom: Math.round(draft.bottom * 1000) / 1000,
            left: Math.round(draft.left * 1000) / 1000,
          };
          void updateClipTransform(entry.clip.id, { crop: rounded });
        }
        return null;
      });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
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
    if (liveTransform && liveTransform.clipId === clip.id) {
      Object.assign(transform, liveTransform.patch);
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
          muted={track.muted}
          aria-label={asset.file_name}
        />
      );
    } else if (asset && asset.mime_type.startsWith('image/')) {
      content = (
        <img src={videoApi.downloadUrl(asset.id)} alt={asset.file_name} className="h-full w-full object-contain" style={{ clipPath }} />
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
      content = (
        <div className="whitespace-pre-wrap" style={textStyle}>
          {text.text}
        </div>
      );
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
        className={`absolute flex items-center justify-center ${selected && !isPlaying ? 'outline outline-1 outline-primary' : ''} ${
          track.locked ? '' : 'cursor-move'
        }`}
        style={wrapperStyle}
        onPointerDown={(event) => {
          if (isPlaying || inCropEdit) return;
          if (clip.id !== selectedClipId) {
            selectClip(clip.id, track.id);
            return;
          }
          if (!track.locked) beginDrag('move', entry, event, event.currentTarget);
        }}
      >
        {content}
        {selected && !isPlaying && !track.locked && !inCropEdit && (
          <>
            <button
              type="button"
              className="absolute -right-1.5 -bottom-1.5 h-3 w-3 cursor-nwse-resize rounded-sm border border-white bg-primary"
              title="Drag to scale"
              aria-label="Scale clip"
              onPointerDown={(event) => beginDrag('scale', entry, event, event.currentTarget.parentElement as HTMLElement)}
            />
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
          const handles: Array<{ edge: keyof CropBox; style: CSSProperties; cursor: string }> = [
            { edge: 'top', style: { left: `${centerXPct}%`, top: `${top * 100}%` }, cursor: 'cursor-ns-resize' },
            { edge: 'bottom', style: { left: `${centerXPct}%`, top: `${(1 - bottom) * 100}%` }, cursor: 'cursor-ns-resize' },
            { edge: 'left', style: { left: `${left * 100}%`, top: `${centerYPct}%` }, cursor: 'cursor-ew-resize' },
            { edge: 'right', style: { left: `${(1 - right) * 100}%`, top: `${centerYPct}%` }, cursor: 'cursor-ew-resize' },
          ];
          return (
            <div className="absolute inset-0">
              <div className="absolute inset-x-0 top-0 bg-black/60" style={{ height: `${top * 100}%` }} />
              <div className="absolute inset-x-0 bottom-0 bg-black/60" style={{ height: `${bottom * 100}%` }} />
              <div className="absolute left-0 bg-black/60" style={{ top: `${top * 100}%`, bottom: `${bottom * 100}%`, width: `${left * 100}%` }} />
              <div className="absolute right-0 bg-black/60" style={{ top: `${top * 100}%`, bottom: `${bottom * 100}%`, width: `${right * 100}%` }} />
              <div
                className="pointer-events-none absolute border border-dashed border-white/80"
                style={{ top: `${top * 100}%`, right: `${right * 100}%`, bottom: `${bottom * 100}%`, left: `${left * 100}%` }}
              />
              {handles.map(({ edge, style, cursor }) => (
                <button
                  key={edge}
                  type="button"
                  className={`absolute h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-sm border border-white bg-amber-400 ${cursor}`}
                  style={style}
                  title={`Crop ${edge}`}
                  aria-label={`Crop ${edge} edge`}
                  onPointerDown={(event) => beginCropDrag(edge, entry, event, event.currentTarget.parentElement as HTMLElement)}
                />
              ))}
            </div>
          );
        })()}
      </div>
    );
  };

  return (
    <div className="flex h-full min-h-[320px] flex-col rounded-lg border border-border bg-black">
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
            onClick={() => setCropMode((value) => !value)}
            disabled={!canCrop}
            className={`inline-flex h-6 w-6 items-center justify-center rounded border disabled:cursor-not-allowed disabled:opacity-35 ${cropMode ? 'border-amber-400/60 bg-amber-400/15 text-amber-300' : 'border-white/15 text-white/55 hover:text-white'}`}
            title={canCrop ? (cropMode ? 'Exit crop mode' : 'Crop selected clip') : 'Select an unlocked video/image clip to crop'}
            aria-label="Toggle crop mode"
          >
            <Crop size={12} />
          </button>
          {cropMode && canCrop && selectedEntry?.clip.transform?.crop && (
            <button
              onClick={() => {
                setCropDraft(null);
                void updateClipTransform(selectedClipId as string, { crop: undefined });
              }}
              className="rounded border border-white/15 px-1.5 py-0.5 text-[10px] text-white/55 hover:text-white"
              title="Remove crop from the selected clip"
            >
              Reset crop
            </button>
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
          <span className="ml-2">{timeline ? `${timeline.canvas.width}x${timeline.canvas.height} · ${timeline.canvas.fps}fps` : 'No timeline'}</span>
        </div>
      </div>
      <div ref={fitRef} className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden p-4">
        <div
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
        >
          {layers.map((entry) => renderLayer(entry))}
          {layers.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center text-xs text-white/55">
              No active visual clip at playhead
            </div>
          )}
          {moveSnap && (
            <div className="pointer-events-none absolute inset-0">
              {moveSnap.x && <div className="absolute left-1/2 top-0 h-full w-px bg-primary/90" />}
              {moveSnap.y && <div className="absolute left-0 top-1/2 h-px w-full bg-primary/90" />}
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
    </div>
  );
}
