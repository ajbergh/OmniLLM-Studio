import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { composeWeightedTransitionPairRgba } from '../../video/renderContractTransitionPairPixelKernel';
import type { PreviewTransitionPairSlot } from './previewFrameTransitionPairs';
import {
  paintPreviewWeightedPairCanvasLayer,
  resolvePreviewWeightedPairCanvasLayerPlan,
  type PreviewWeightedPairCanvasLayer,
} from './previewFrameWeightedPairCanvas';
import {
  createPreviewWeightedPairWebGLCompositor,
  type PreviewWeightedPairWebGLCompositor,
} from './previewWeightedPairWebGL';

export type PreviewWeightedPairCanvasStatusKind = 'pending' | 'ready' | 'failed';

export interface PreviewWeightedPairCanvasStatus {
  executionKey: string;
  transitionId: string;
  status: PreviewWeightedPairCanvasStatusKind;
  reason?: string;
}

interface PreviewWeightedPairCanvasProps<T extends PreviewWeightedPairCanvasLayer> {
  slot: PreviewTransitionPairSlot<T>;
  canvasWidth: number;
  canvasHeight: number;
  stageWidth: number;
  stageHeight: number;
  sourceForClip: (clipId: string) => HTMLImageElement | HTMLVideoElement | null;
  /** Exact-frame runtime key. Deterministic parity callers may omit it. */
  executionKey?: string;
  /** Hidden preparation is used by normal playback until every pair is ready. */
  active?: boolean;
  surfaceRole?: 'deterministic' | 'playback';
  onStatusChange?: (status: PreviewWeightedPairCanvasStatus) => void;
}

type RasterSource = HTMLImageElement | HTMLVideoElement;
type PresentationVideo = HTMLVideoElement & {
  requestVideoFrameCallback?: (callback: (now: number, metadata: { mediaTime: number }) => void) => number;
  cancelVideoFrameCallback?: (handle: number) => void;
};

/**
 * Deterministic weighted pair surface. The source elements are the already
 * mounted image/video nodes owned and synchronized by the existing preview, so
 * this consumer adds no decoder and no second source-time authority. Geometry
 * always uses the shared canonical 2D painter. Deterministic/static rendering
 * keeps the byte-exact CPU #277 kernel; normal playback uses the equivalent
 * WebGL2 linear-sRGB compositor so full-resolution composition does not stall
 * the media/UI clock.
 */
export function PreviewWeightedPairCanvas<T extends PreviewWeightedPairCanvasLayer>({
  slot,
  canvasWidth,
  canvasHeight,
  stageWidth,
  stageHeight,
  sourceForClip,
  executionKey = '',
  active = true,
  surfaceRole = 'deterministic',
  onStatusChange,
}: PreviewWeightedPairCanvasProps<T>) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const isolatedSurfacesRef = useRef<[HTMLCanvasElement | null, HTMLCanvasElement | null]>([null, null]);
  const playbackCompositorRef = useRef<PreviewWeightedPairWebGLCompositor | null>(null);
  const [sourceRevision, setSourceRevision] = useState(0);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendingReason, setPendingReason] = useState<string | null>(null);
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);

  useEffect(() => () => {
    playbackCompositorRef.current?.dispose();
    playbackCompositorRef.current = null;
  }, []);

  // Normal playback may begin while the deterministic pre-play seek is still
  // settling. Resume the already-mounted decoder during layout even when it is
  // seeking so the legacy passive sync sees a running element and cannot start
  // a new currentTime chase. This bridge never writes source time/rate; the
  // legacy preview remains the sole media-clock authority.
  useLayoutEffect(() => {
    if (surfaceRole !== 'playback') return;
    for (const clipId of [slot.lower.clip.id, slot.upper.clip.id]) {
      const source = sourceForClip(clipId);
      if (!(source instanceof HTMLVideoElement) || !source.paused || source.ended) continue;
      void source.play().catch(() => { /* muted preview; legacy sync retains fallback behavior */ });
    }
  }, [executionKey, slot.lower.clip.id, slot.upper.clip.id, sourceForClip, surfaceRole]);

  useEffect(() => {
    const sources = [
      sourceForClip(slot.lower.clip.id),
      sourceForClip(slot.upper.clip.id),
    ].filter((source): source is RasterSource => Boolean(source));
    const presentationHandles = new Map<PresentationVideo, number>();
    let disposed = false;
    const onSeeking = () => {
      setReady(false);
      bumpSourceRevision();
    };
    const onSettled = () => bumpSourceRevision();
    const schedulePresentedFrame = (video: PresentationVideo) => {
      if (disposed || typeof video.requestVideoFrameCallback !== 'function') return;
      const handle = video.requestVideoFrameCallback(() => {
        presentationHandles.delete(video);
        if (disposed) return;
        // Redraw from the frame the decoder actually submitted for presentation,
        // rather than waiting for a seek/load event that free-running playback
        // does not emit on every decoded frame.
        bumpSourceRevision();
        schedulePresentedFrame(video);
      });
      presentationHandles.set(video, handle);
    };
    for (const source of sources) {
      if (source instanceof HTMLVideoElement) {
        source.addEventListener('seeking', onSeeking);
        source.addEventListener('seeked', onSettled);
        source.addEventListener('loadedmetadata', onSettled);
        source.addEventListener('loadeddata', onSettled);
        source.addEventListener('error', onSettled);
        if (surfaceRole === 'playback') schedulePresentedFrame(source as PresentationVideo);
      } else {
        source.addEventListener('load', onSettled);
        source.addEventListener('error', onSettled);
      }
    }
    return () => {
      disposed = true;
      for (const [video, handle] of presentationHandles) {
        if (typeof video.cancelVideoFrameCallback === 'function') video.cancelVideoFrameCallback(handle);
      }
      presentationHandles.clear();
      for (const source of sources) {
        if (source instanceof HTMLVideoElement) {
          source.removeEventListener('seeking', onSeeking);
          source.removeEventListener('seeked', onSettled);
          source.removeEventListener('loadedmetadata', onSettled);
          source.removeEventListener('loadeddata', onSettled);
          source.removeEventListener('error', onSettled);
        } else {
          source.removeEventListener('load', onSettled);
          source.removeEventListener('error', onSettled);
        }
      }
    };
  }, [bumpSourceRevision, slot.lower.clip.id, slot.upper.clip.id, sourceForClip, surfaceRole]);

  const draw = useCallback((): boolean => {
    const canvas = canvasRef.current;
    if (!canvas || canvasWidth <= 0 || canvasHeight <= 0) return false;
    if (slot.execution !== 'weighted-canvas-deferred' || !slot.weightedRasterSource?.supported) return false;
    const pairLayers = [slot.lower, slot.upper] as const;
    const isolated = new Map<string, HTMLCanvasElement>();

    for (let index = 0; index < pairLayers.length; index += 1) {
      const layer = pairLayers[index];
      const source = sourceForClip(layer.clip.id);
      if (!source || !sourceReady(source)) return false;
      const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
      const layerPlan = resolvePreviewWeightedPairCanvasLayerPlan(
        slot,
        layer,
        intrinsicWidth,
        intrinsicHeight,
      );
      const surface = ensureIsolatedSurface(
        isolatedSurfacesRef.current,
        index,
        canvasWidth,
        canvasHeight,
      );
      const context = surface.getContext('2d', { willReadFrequently: surfaceRole !== 'playback' });
      if (!context) throw new Error('weighted pair Canvas could not create isolated 2D context');
      context.clearRect(0, 0, canvasWidth, canvasHeight);
      paintPreviewWeightedPairCanvasLayer(context, source, layerPlan);
      isolated.set(layer.clip.id, surface);
    }

    const outgoing = isolated.get(slot.pixel.outgoing_clip_id);
    const incoming = isolated.get(slot.pixel.incoming_clip_id);
    if (!outgoing || !incoming) {
      throw new Error(`weighted pair ${JSON.stringify(slot.surface.transition_id)} raster inputs do not match canonical outgoing/incoming ids`);
    }

    if (surfaceRole === 'playback') {
      let compositor = playbackCompositorRef.current;
      if (!compositor) {
        compositor = createPreviewWeightedPairWebGLCompositor(canvas);
        if (!compositor) {
          throw new Error('weighted playback WebGL2 compositor is unavailable');
        }
        playbackCompositorRef.current = compositor;
      }
      compositor.render(outgoing, incoming, slot.pixel);
      return true;
    }

    const outgoingContext = outgoing.getContext('2d', { willReadFrequently: true });
    const incomingContext = incoming.getContext('2d', { willReadFrequently: true });
    if (!outgoingContext || !incomingContext) {
      throw new Error('weighted pair Canvas could not read deterministic isolated surfaces');
    }
    const outgoingImage = outgoingContext.getImageData(0, 0, canvasWidth, canvasHeight);
    const incomingImage = incomingContext.getImageData(0, 0, canvasWidth, canvasHeight);
    const output = composeWeightedTransitionPairRgba(slot.pixel, outgoingImage.data, incomingImage.data);
    const context = canvas.getContext('2d');
    if (!context) throw new Error('weighted pair Canvas could not create output 2D context');
    context.clearRect(0, 0, canvasWidth, canvasHeight);
    context.putImageData(new ImageData(new Uint8ClampedArray(output), canvasWidth, canvasHeight), 0, 0);
    return true;
  }, [canvasHeight, canvasWidth, slot, sourceForClip, surfaceRole]);

  // Playback readiness must settle before paint: the caller keeps this surface
  // hidden while pending, then publishes exact-frame readiness and re-renders
  // the legacy owner into canonical frame mode before this Canvas is revealed.
  useLayoutEffect(() => {
    try {
      const rendered = draw();
      const pending = rendered ? null : weightedPairPendingReason(slot, sourceForClip);
      setError(null);
      setPendingReason(pending);
      setReady(rendered);
      onStatusChange?.({
        executionKey,
        transitionId: slot.surface.transition_id,
        status: rendered ? 'ready' : 'pending',
        ...(pending ? { reason: pending } : {}),
      });
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : String(reason);
      setError(message);
      setPendingReason(null);
      setReady(false);
      onStatusChange?.({
        executionKey,
        transitionId: slot.surface.transition_id,
        status: 'failed',
        reason: message,
      });
    }
  }, [draw, executionKey, onStatusChange, slot, slot.surface.transition_id, sourceForClip, sourceRevision]);

  return (
    <div
      data-preview-transition-pair-id={slot.surface.transition_id}
      data-preview-transition-pair-execution="weighted-canvas"
      data-preview-transition-pair-backend={surfaceRole === 'playback' ? 'webgl2' : 'cpu-exact'}
      data-preview-transition-pair-surface-role={surfaceRole}
      data-preview-transition-pair-runtime-key={executionKey || undefined}
      data-preview-transition-pair-lower-clip={slot.surface.lower_clip_id}
      data-preview-transition-pair-upper-clip={slot.surface.upper_clip_id}
      data-preview-transition-pair-ready={ready ? 'true' : 'false'}
      data-preview-transition-pair-error={error ?? undefined}
      data-preview-transition-pair-pending-reason={pendingReason ?? undefined}
      className="pointer-events-none absolute inset-0"
      style={{ visibility: active ? 'visible' : 'hidden' }}
    >
      <canvas
        ref={canvasRef}
        width={canvasWidth}
        height={canvasHeight}
        className="absolute inset-0"
        style={{ width: stageWidth, height: stageHeight }}
        aria-hidden="true"
      />
    </div>
  );
}

function ensureIsolatedSurface(
  surfaces: [HTMLCanvasElement | null, HTMLCanvasElement | null],
  index: number,
  width: number,
  height: number,
): HTMLCanvasElement {
  let surface = surfaces[index];
  if (!surface) {
    surface = document.createElement('canvas');
    surfaces[index] = surface;
  }
  if (surface.width !== width) surface.width = width;
  if (surface.height !== height) surface.height = height;
  return surface;
}

function sourceReady(source: RasterSource): boolean {
  if (source instanceof HTMLVideoElement) {
    return source.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA
      && !source.seeking
      && source.videoWidth > 0
      && source.videoHeight > 0;
  }
  return source.complete && source.naturalWidth > 0 && source.naturalHeight > 0;
}

function weightedPairPendingReason<T extends PreviewWeightedPairCanvasLayer>(
  slot: PreviewTransitionPairSlot<T>,
  sourceForClip: (clipId: string) => RasterSource | null,
): string | null {
  for (const layer of [slot.lower, slot.upper]) {
    const source = sourceForClip(layer.clip.id);
    if (!source) return `${layer.clip.id}:source-missing`;
    if (source instanceof HTMLVideoElement) {
      const blockers: string[] = [];
      if (source.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) blockers.push(`ready-state-${source.readyState}`);
      if (source.seeking) blockers.push('seeking');
      if (source.videoWidth <= 0 || source.videoHeight <= 0) blockers.push('intrinsic-size-missing');
      if (blockers.length > 0) return `${layer.clip.id}:video-${blockers.join('+')}`;
      continue;
    }
    if (!source.complete) return `${layer.clip.id}:image-loading`;
    if (source.naturalWidth <= 0 || source.naturalHeight <= 0) return `${layer.clip.id}:image-intrinsic-size-missing`;
  }
  return 'weighted-canvas-pending-with-ready-sources';
}

function intrinsicSize(source: RasterSource): [number, number] {
  return source instanceof HTMLVideoElement
    ? [source.videoWidth, source.videoHeight]
    : [source.naturalWidth, source.naturalHeight];
}
