import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { composeWeightedTransitionPairRgba } from '../../video/renderContractTransitionPairPixelKernel';
import type { PreviewTransitionPairSlot } from './previewFrameTransitionPairs';
import {
  paintPreviewWeightedPairCanvasLayer,
  resolvePreviewWeightedPairCanvasLayerPlan,
  type PreviewWeightedPairCanvasLayer,
} from './previewFrameWeightedPairCanvas';

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

/**
 * Deterministic weighted pair surface. The source elements are the already
 * mounted image/video nodes owned and synchronized by the existing preview, so
 * this consumer adds no decoder and no second source-time authority. It owns
 * only readiness, isolated canonical 2D rasterization, and the exact #277
 * weighted pixel kernel.
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
  const [sourceRevision, setSourceRevision] = useState(0);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);

  useEffect(() => {
    const sources = [
      sourceForClip(slot.lower.clip.id),
      sourceForClip(slot.upper.clip.id),
    ].filter((source): source is RasterSource => Boolean(source));
    const onSeeking = () => {
      setReady(false);
      bumpSourceRevision();
    };
    const onSettled = () => bumpSourceRevision();
    for (const source of sources) {
      if (source instanceof HTMLVideoElement) {
        source.addEventListener('seeking', onSeeking);
        source.addEventListener('seeked', onSettled);
        source.addEventListener('loadedmetadata', onSettled);
        source.addEventListener('loadeddata', onSettled);
        source.addEventListener('error', onSettled);
      } else {
        source.addEventListener('load', onSettled);
        source.addEventListener('error', onSettled);
      }
    }
    return () => {
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
  }, [bumpSourceRevision, slot.lower.clip.id, slot.upper.clip.id, sourceForClip]);

  const draw = useCallback((): boolean => {
    const canvas = canvasRef.current;
    if (!canvas || canvasWidth <= 0 || canvasHeight <= 0) return false;
    if (slot.execution !== 'weighted-canvas-deferred' || !slot.weightedRasterSource?.supported) return false;
    const pairLayers = [slot.lower, slot.upper] as const;
    const isolated = new Map<string, ImageData>();

    for (const layer of pairLayers) {
      const source = sourceForClip(layer.clip.id);
      if (!source || !sourceReady(source)) return false;
      const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
      const layerPlan = resolvePreviewWeightedPairCanvasLayerPlan(
        slot,
        layer,
        intrinsicWidth,
        intrinsicHeight,
      );
      const surface = document.createElement('canvas');
      surface.width = canvasWidth;
      surface.height = canvasHeight;
      const context = surface.getContext('2d', { willReadFrequently: true });
      if (!context) throw new Error('weighted pair Canvas could not create isolated 2D context');
      context.clearRect(0, 0, canvasWidth, canvasHeight);
      paintPreviewWeightedPairCanvasLayer(context, source, layerPlan);
      isolated.set(layer.clip.id, context.getImageData(0, 0, canvasWidth, canvasHeight));
    }

    const outgoing = isolated.get(slot.pixel.outgoing_clip_id);
    const incoming = isolated.get(slot.pixel.incoming_clip_id);
    if (!outgoing || !incoming) {
      throw new Error(`weighted pair ${JSON.stringify(slot.surface.transition_id)} raster inputs do not match canonical outgoing/incoming ids`);
    }
    const output = composeWeightedTransitionPairRgba(slot.pixel, outgoing.data, incoming.data);
    const context = canvas.getContext('2d');
    if (!context) throw new Error('weighted pair Canvas could not create output 2D context');
    context.clearRect(0, 0, canvasWidth, canvasHeight);
    context.putImageData(new ImageData(new Uint8ClampedArray(output), canvasWidth, canvasHeight), 0, 0);
    return true;
  }, [canvasHeight, canvasWidth, slot, sourceForClip]);

  // Playback readiness must settle before paint: the caller keeps this surface
  // hidden while pending, then publishes exact-frame readiness and re-renders
  // the legacy owner into canonical frame mode before this Canvas is revealed.
  useLayoutEffect(() => {
    try {
      const rendered = draw();
      setError(null);
      setReady(rendered);
      onStatusChange?.({
        executionKey,
        transitionId: slot.surface.transition_id,
        status: rendered ? 'ready' : 'pending',
      });
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : String(reason);
      setError(message);
      setReady(false);
      onStatusChange?.({
        executionKey,
        transitionId: slot.surface.transition_id,
        status: 'failed',
        reason: message,
      });
    }
  }, [draw, executionKey, onStatusChange, slot.surface.transition_id, sourceRevision]);

  return (
    <div
      data-preview-transition-pair-id={slot.surface.transition_id}
      data-preview-transition-pair-execution="weighted-canvas"
      data-preview-transition-pair-surface-role={surfaceRole}
      data-preview-transition-pair-runtime-key={executionKey || undefined}
      data-preview-transition-pair-lower-clip={slot.surface.lower_clip_id}
      data-preview-transition-pair-upper-clip={slot.surface.upper_clip_id}
      data-preview-transition-pair-ready={ready ? 'true' : 'false'}
      data-preview-transition-pair-error={error ?? undefined}
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

function sourceReady(source: RasterSource): boolean {
  if (source instanceof HTMLVideoElement) {
    return source.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA
      && !source.seeking
      && source.videoWidth > 0
      && source.videoHeight > 0;
  }
  return source.complete && source.naturalWidth > 0 && source.naturalHeight > 0;
}

function intrinsicSize(source: RasterSource): [number, number] {
  return source instanceof HTMLVideoElement
    ? [source.videoWidth, source.videoHeight]
    : [source.naturalWidth, source.naturalHeight];
}
