/**
 * A single timeline clip: kind icon + label, thumbnail/waveform backdrop,
 * state badges (muted, audio-only, grouped, transitions), fade ramps with
 * draggable fade handles, keyframe diamonds, transition regions, a volume
 * envelope with drag-editable volume keyframes, and trim handles with their
 * own context menu. All drags preview locally and commit one store action on
 * pointer-up (one undo entry, one save).
 */
import { useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { Film, Image as ImageIcon, Link2, Music2, Square, Type, VolumeX } from 'lucide-react';
import { videoApi } from '../../../api';
import type { VideoAsset, VideoTimelineClip as Clip, VideoTimelineKeyframe } from '../../../types/video';

function clipLabel(clip: Clip, asset?: VideoAsset): string {
  if (clip.text?.text) return clip.text.text;
  if (clip.shape) return clip.shape.kind === 'highlight' ? 'Highlight' : clip.shape.kind === 'blur' ? 'Blur region' : 'Callout box';
  return asset?.file_name || clip.asset_id || 'Clip';
}

function formatSeconds(ms: number): string {
  const total = Math.max(0, ms) / 1000;
  const minutes = Math.floor(total / 60);
  const seconds = total - minutes * 60;
  return minutes > 0 ? `${minutes}:${seconds.toFixed(1).padStart(4, '0')}` : `${seconds.toFixed(1)}s`;
}

function kindIcon(kind: string, audioOnly?: boolean) {
  if (audioOnly || kind === 'audio' || kind === 'music') return <Music2 size={10} aria-hidden="true" />;
  if (kind === 'image') return <ImageIcon size={10} aria-hidden="true" />;
  if (kind === 'text' || kind === 'caption') return <Type size={10} aria-hidden="true" />;
  if (kind === 'shape') return <Square size={10} aria-hidden="true" />;
  return <Film size={10} aria-hidden="true" />;
}

export function TimelineClip({
  clip,
  asset,
  selected,
  pxPerMs,
  trackId,
  toolMode = 'select',
  onSelect,
  onTrim,
  onSplitAt,
  onContextMenu,
  onEdgeContextMenu,
  onFade,
  onUpdateKeyframe,
  onTransitionContextMenu,
}: {
  clip: Clip;
  asset?: VideoAsset;
  selected: boolean;
  pxPerMs: number;
  trackId: string;
  toolMode?: 'select' | 'blade';
  onSelect: (clipId: string, trackId: string, additive?: boolean) => void;
  onTrim: (clipId: string, updates: Partial<Pick<Clip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => void;
  onSplitAt?: (clipId: string, timeMs: number) => void;
  onContextMenu?: (clipId: string, trackId: string, clientX: number, clientY: number) => void;
  onEdgeContextMenu?: (clipId: string, trackId: string, edge: 'start' | 'end', clientX: number, clientY: number) => void;
  onFade?: (clipId: string, fade: { fade_in_ms?: number; fade_out_ms?: number }) => void;
  onUpdateKeyframe?: (clipId: string, keyframeId: string, patch: Partial<Omit<VideoTimelineKeyframe, 'id'>>) => void;
  onTransitionContextMenu?: (clipId: string, trackId: string, clientX: number, clientY: number) => void;
}) {
  // Live drag previews — committed once on pointer-up.
  const [liveFade, setLiveFade] = useState<{ edge: 'in' | 'out'; ms: number } | null>(null);
  const [liveKeyframe, setLiveKeyframe] = useState<{ id: string; timeMs: number; value: number } | null>(null);
  const left = clip.start_ms * pxPerMs;
  const width = Math.max(36, clip.duration_ms * pxPerMs);
  const kind = clip.shape ? 'shape' : asset?.kind || (clip.text ? 'text' : 'video');
  const tone =
    clip.audio_only || kind === 'audio' || kind === 'music'
      ? 'border-emerald-400/40 bg-emerald-500/15 text-emerald-200'
      : kind === 'image'
        ? 'border-cyan-400/40 bg-cyan-500/15 text-cyan-100'
        : kind === 'text' || kind === 'caption'
          ? 'border-amber-400/40 bg-amber-500/15 text-amber-100'
          : kind === 'shape'
            ? 'border-fuchsia-400/40 bg-fuchsia-500/15 text-fuchsia-100'
            : 'border-primary/40 bg-primary/15 text-primary';

  // Background art: waveforms for audio-bearing clips, thumbnails for visual
  // media. Detached audio twins always read as audio.
  const showWaveform = Boolean(asset?.waveform_path) && (clip.audio_only || asset?.kind === 'audio' || asset?.kind === 'music');
  const thumbnailUrl = !showWaveform && !clip.audio_only && asset
    ? asset.thumbnail_path
      ? videoApi.artifactUrl(asset.id, 'thumbnail')
      : asset.kind === 'image'
        ? videoApi.downloadUrl(asset.id)
        : null
    : null;
  const backgroundStyle = showWaveform && asset
    ? { backgroundImage: `url(${videoApi.artifactUrl(asset.id, 'waveform')})`, backgroundSize: '100% 100%', backgroundRepeat: 'no-repeat' as const }
    : thumbnailUrl
      ? { backgroundImage: `url(${thumbnailUrl})`, backgroundSize: 'auto 100%', backgroundRepeat: 'repeat-x' as const }
      : {};

  const fadeInMs = liveFade?.edge === 'in' ? liveFade.ms : clip.fade_in_ms || 0;
  const fadeOutMs = liveFade?.edge === 'out' ? liveFade.ms : clip.fade_out_ms || 0;
  const fadeInPx = fadeInMs ? Math.min(width, fadeInMs * pxPerMs) : 0;
  const fadeOutPx = fadeOutMs ? Math.min(width, fadeOutMs * pxPerMs) : 0;
  const isAudible = clip.audio_only || asset?.kind === 'audio' || asset?.kind === 'music' || asset?.mime_type.startsWith('video/');
  const transitionPx = (clip.transitions || []).length > 0
    ? Math.min(width / 2, Math.max(...(clip.transitions || []).map((transition) => transition.duration_ms)) * pxPerMs)
    : 0;
  const isMuted = clip.muted || clip.volume === 0;
  const endMs = clip.start_ms + clip.duration_ms;

  const beginTrim = (edge: 'start' | 'end', event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0) return;
    event.stopPropagation();
    event.preventDefault();
    const startX = event.clientX;
    const trimIn = clip.trim_in_ms ?? 0;
    const trimOut = clip.trim_out_ms ?? clip.duration_ms;

    const onPointerUp = (upEvent: PointerEvent) => {
      document.removeEventListener('pointerup', onPointerUp);
      const deltaMs = Math.round((upEvent.clientX - startX) / pxPerMs);
      if (deltaMs === 0) return;
      if (edge === 'start') {
        const clamped = Math.max(-clip.start_ms, Math.min(deltaMs, clip.duration_ms - 100));
        onTrim(clip.id, {
          start_ms: clip.start_ms + clamped,
          duration_ms: clip.duration_ms - clamped,
          trim_in_ms: Math.max(0, trimIn + clamped),
          trim_out_ms: trimOut,
        });
      } else {
        const clamped = Math.max(100 - clip.duration_ms, deltaMs);
        onTrim(clip.id, {
          duration_ms: clip.duration_ms + clamped,
          trim_in_ms: trimIn,
          trim_out_ms: Math.max(trimIn + 100, trimOut + clamped),
        });
      }
    };

    document.addEventListener('pointerup', onPointerUp, { once: true });
  };

  const CLIP_HEIGHT_PX = 36;

  // Drag a fade handle horizontally to set the fade duration; commit on release.
  const beginFadeDrag = (edge: 'in' | 'out', event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || !onFade) return;
    event.stopPropagation();
    event.preventDefault();
    const startX = event.clientX;
    const base = edge === 'in' ? clip.fade_in_ms || 0 : clip.fade_out_ms || 0;
    let live = base;
    const onMove = (moveEvent: PointerEvent) => {
      const deltaMs = Math.round((moveEvent.clientX - startX) / pxPerMs) * (edge === 'in' ? 1 : -1);
      live = Math.max(0, Math.min(clip.duration_ms, base + deltaMs));
      setLiveFade({ edge, ms: live });
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      setLiveFade(null);
      if (live !== base) onFade(clip.id, edge === 'in' ? { fade_in_ms: live } : { fade_out_ms: live });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  // Drag a volume keyframe point in time (x) and value (y); commit on release.
  const beginKeyframeDrag = (keyframe: VideoTimelineKeyframe, event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || !onUpdateKeyframe) return;
    event.stopPropagation();
    event.preventDefault();
    const startX = event.clientX;
    const startY = event.clientY;
    let live = { id: keyframe.id, timeMs: keyframe.time_ms, value: keyframe.value };
    const onMove = (moveEvent: PointerEvent) => {
      const deltaMs = Math.round((moveEvent.clientX - startX) / pxPerMs);
      const deltaValue = -((moveEvent.clientY - startY) / CLIP_HEIGHT_PX) * 2;
      live = {
        id: keyframe.id,
        timeMs: Math.max(0, Math.min(clip.duration_ms, keyframe.time_ms + deltaMs)),
        value: Math.max(0, Math.min(2, keyframe.value + deltaValue)),
      };
      setLiveKeyframe(live);
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      setLiveKeyframe(null);
      onUpdateKeyframe(clip.id, keyframe.id, { time_ms: live.timeMs, value: Math.round(live.value * 100) / 100 });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  // Volume envelope: flat line at the clip volume, or a polyline through the
  // clip's volume keyframes (0–2 mapped bottom→top).
  const volumeEnvelope = (() => {
    // Audio/music clips always show their envelope; video clips with audio
    // only while selected, to reduce clutter.
    const showEnvelope = clip.audio_only || asset?.kind === 'audio' || asset?.kind === 'music' || (selected && isAudible);
    if (!showEnvelope) return null;
    const volumeKeyframes = (clip.keyframes || [])
      .filter((keyframe) => keyframe.property === 'volume')
      .map((keyframe) => (liveKeyframe && liveKeyframe.id === keyframe.id
        ? { ...keyframe, time_ms: liveKeyframe.timeMs, value: liveKeyframe.value }
        : keyframe))
      .sort((a, b) => a.time_ms - b.time_ms);
    const valueToY = (value: number) => CLIP_HEIGHT_PX * (1 - Math.max(0, Math.min(2, value)) / 2);
    const baseVolume = clip.volume ?? 1;
    const points: Array<{ x: number; y: number }> = [];
    if (volumeKeyframes.length === 0) {
      points.push({ x: 0, y: valueToY(baseVolume) }, { x: width, y: valueToY(baseVolume) });
    } else {
      points.push({ x: 0, y: valueToY(volumeKeyframes[0].value) });
      for (const keyframe of volumeKeyframes) {
        points.push({ x: Math.min(width, keyframe.time_ms * pxPerMs), y: valueToY(keyframe.value) });
      }
      points.push({ x: width, y: valueToY(volumeKeyframes[volumeKeyframes.length - 1].value) });
    }
    return (
      <svg className="pointer-events-none absolute inset-0" width={width} height={CLIP_HEIGHT_PX} aria-hidden="true">
        <polyline
          points={points.map((point) => `${point.x},${point.y}`).join(' ')}
          fill="none"
          stroke="rgba(34,211,238,0.9)"
          strokeWidth={1.5}
        />
        {volumeKeyframes.map((keyframe) => (
          <circle
            key={keyframe.id}
            cx={Math.min(width, keyframe.time_ms * pxPerMs)}
            cy={valueToY(keyframe.value)}
            r={3.5}
            fill="#22d3ee"
            stroke="#0e7490"
            className={onUpdateKeyframe && selected ? 'pointer-events-auto cursor-ns-resize' : ''}
            onPointerDown={(event) => beginKeyframeDrag(keyframe, event as unknown as ReactPointerEvent<HTMLElement>)}
          />
        ))}
      </svg>
    );
  })();

  return (
    <div
      role="button"
      tabIndex={0}
      draggable
      data-clip-id={clip.id}
      onDragStart={(event) => {
        event.dataTransfer.setData('application/x-video-clip-id', clip.id);
        // Remember where inside the clip the drag started so drops keep the
        // grabbed point under the cursor instead of jumping to the clip start.
        const rect = event.currentTarget.getBoundingClientRect();
        event.dataTransfer.setData('application/x-video-clip-grab-offset', String(Math.round(event.clientX - rect.left)));
        event.dataTransfer.effectAllowed = 'move';
      }}
      onClick={(event) => {
        event.stopPropagation();
        if (toolMode === 'blade' && onSplitAt) {
          const rect = event.currentTarget.getBoundingClientRect();
          onSplitAt(clip.id, clip.start_ms + Math.round((event.clientX - rect.left) / pxPerMs));
          return;
        }
        onSelect(clip.id, trackId, event.ctrlKey || event.metaKey || event.shiftKey);
      }}
      onContextMenu={(event) => {
        if (!onContextMenu) return;
        event.preventDefault();
        event.stopPropagation();
        onContextMenu(clip.id, trackId, event.clientX, event.clientY);
      }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onSelect(clip.id, trackId, event.ctrlKey || event.metaKey || event.shiftKey);
        } else if ((event.key === 'F10' && event.shiftKey) || event.key === 'ContextMenu') {
          event.preventDefault();
          event.stopPropagation();
          const rect = event.currentTarget.getBoundingClientRect();
          onContextMenu?.(clip.id, trackId, rect.left + rect.width / 2, rect.bottom);
        }
      }}
      className={`group absolute top-1 h-9 overflow-hidden rounded-md border px-2 text-left text-[11px] transition-colors ${tone} ${
        selected ? 'ring-2 ring-primary ring-offset-1 ring-offset-surface' : ''
      } ${toolMode === 'blade' ? 'cursor-crosshair' : ''}`}
      style={{ left, width, ...backgroundStyle }}
      title={`${clipLabel(clip, asset)} — ${formatSeconds(clip.start_ms)} → ${formatSeconds(endMs)} (${formatSeconds(clip.duration_ms)})`}
    >
      {thumbnailUrl && <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-black/55 via-black/25 to-transparent" aria-hidden="true" />}
      {/* Fade ramps */}
      {fadeInPx > 1 && (
        <div
          className="pointer-events-none absolute inset-y-0 left-0 bg-white/20"
          style={{ width: fadeInPx, clipPath: 'polygon(0 100%, 100% 0, 100% 100%)' }}
          aria-hidden="true"
        />
      )}
      {fadeOutPx > 1 && (
        <div
          className="pointer-events-none absolute inset-y-0 right-0 bg-white/20"
          style={{ width: fadeOutPx, clipPath: 'polygon(0 0, 100% 100%, 0 100%)' }}
          aria-hidden="true"
        />
      )}
      {/* Transition regions at both clip ends — right-click to edit */}
      {transitionPx > 1 && (
        <>
          <div
            className="absolute inset-y-0 left-0 border-r border-dashed border-white/40 bg-white/10"
            style={{ width: transitionPx }}
            title="Transition region — right-click to edit"
            onContextMenu={(event) => {
              if (!onTransitionContextMenu) return;
              event.preventDefault();
              event.stopPropagation();
              onTransitionContextMenu(clip.id, trackId, event.clientX, event.clientY);
            }}
          />
          <div
            className="absolute inset-y-0 right-0 border-l border-dashed border-white/40 bg-white/10"
            style={{ width: transitionPx }}
            title="Transition region — right-click to edit"
            onContextMenu={(event) => {
              if (!onTransitionContextMenu) return;
              event.preventDefault();
              event.stopPropagation();
              onTransitionContextMenu(clip.id, trackId, event.clientX, event.clientY);
            }}
          />
        </>
      )}
      {volumeEnvelope}
      {/* Fade handles — drag horizontally to set fade durations */}
      {onFade && (selected || undefined) && (
        <>
          <button
            type="button"
            onPointerDown={(event) => beginFadeDrag('in', event)}
            className="absolute top-0 z-20 h-2.5 w-2.5 -translate-x-1/2 cursor-ew-resize rounded-full border border-white bg-sky-400"
            style={{ left: Math.max(4, fadeInPx) }}
            title={`Fade in: ${(fadeInMs / 1000).toFixed(1)}s — drag to adjust`}
            aria-label="Adjust fade in"
          />
          <button
            type="button"
            onPointerDown={(event) => beginFadeDrag('out', event)}
            className="absolute top-0 z-20 h-2.5 w-2.5 translate-x-1/2 cursor-ew-resize rounded-full border border-white bg-sky-400"
            style={{ right: Math.max(4, fadeOutPx) }}
            title={`Fade out: ${(fadeOutMs / 1000).toFixed(1)}s — drag to adjust`}
            aria-label="Adjust fade out"
          />
        </>
      )}
      <button
        type="button"
        onPointerDown={(event) => beginTrim('start', event)}
        onContextMenu={(event) => {
          if (!onEdgeContextMenu) return;
          event.preventDefault();
          event.stopPropagation();
          onEdgeContextMenu(clip.id, trackId, 'start', event.clientX, event.clientY);
        }}
        className={`absolute left-0 top-0 z-10 h-full w-2 cursor-ew-resize rounded-l-md bg-white/20 transition-opacity ${selected ? 'opacity-60' : 'opacity-0'} hover:!opacity-100 focus:opacity-100 group-hover:opacity-60`}
        title="Trim clip start — right-click for trim options"
        aria-label="Trim clip start"
      >
        <span className="pointer-events-none absolute inset-y-2 left-1/2 w-px -translate-x-1/2 bg-white/70" aria-hidden="true" />
      </button>
      <span className="relative flex items-center gap-1 truncate font-medium">
        <span className="shrink-0 opacity-80">{kindIcon(kind, clip.audio_only)}</span>
        <span className="min-w-0 truncate">{clipLabel(clip, asset)}</span>
      </span>
      <span className="relative flex items-center gap-1 truncate text-[10px] opacity-75">
        {formatSeconds(clip.duration_ms)}
        {isMuted && <VolumeX size={9} aria-label="Muted" />}
        {clip.audio_only && <span className="rounded bg-emerald-400/20 px-0.5 text-[8px] font-semibold uppercase tracking-wide">audio</span>}
        {clip.group_id && <Link2 size={9} aria-label="Grouped" />}
        {(clip.transitions || []).length > 0 && <span className="rounded bg-white/15 px-0.5 text-[8px] font-semibold uppercase tracking-wide">tx</span>}
      </span>
      {/* Keyframe diamonds along the bottom edge */}
      {(clip.keyframes || []).length > 0 && (
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-2" aria-hidden="true">
          {Array.from(new Set((clip.keyframes || []).map((keyframe) => keyframe.time_ms))).map((timeMs) => (
            <span
              key={timeMs}
              className="absolute bottom-0.5 h-1.5 w-1.5 rotate-45 rounded-[1px] bg-white/80 shadow"
              style={{ left: Math.max(1, Math.min(width - 7, timeMs * pxPerMs - 3)) }}
            />
          ))}
        </div>
      )}
      <button
        type="button"
        onPointerDown={(event) => beginTrim('end', event)}
        onContextMenu={(event) => {
          if (!onEdgeContextMenu) return;
          event.preventDefault();
          event.stopPropagation();
          onEdgeContextMenu(clip.id, trackId, 'end', event.clientX, event.clientY);
        }}
        className={`absolute right-0 top-0 z-10 h-full w-2 cursor-ew-resize rounded-r-md bg-white/20 transition-opacity ${selected ? 'opacity-60' : 'opacity-0'} hover:!opacity-100 focus:opacity-100 group-hover:opacity-60`}
        title="Trim clip end — right-click for trim options"
        aria-label="Trim clip end"
      >
        <span className="pointer-events-none absolute inset-y-2 left-1/2 w-px -translate-x-1/2 bg-white/70" aria-hidden="true" />
      </button>
    </div>
  );
}
