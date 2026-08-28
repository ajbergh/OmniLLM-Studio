import { useCallback, useEffect, useRef, useState } from 'react';
import type { PreviewPixelateBackdropLayer, PreviewPixelateBackdropReady } from './previewPixelateBackdrop';
import {
  pixelatePreviewRgba,
} from './previewPixelateRaster';
import {
  paintPreviewCanvasMediaLayer,
  resolvePreviewCanvasMediaLayerPlan,
} from './previewFrameWeightedPairCanvas';
import {
  previewPixelateRegionIsOpaque,
  resolvePreviewPixelateCanvasRegion,
  type PreviewPixelateCanvasRegion,
} from './previewPixelateCanvasRuntime';

export type PreviewPixelateCanvasStatusKind = 'pending' | 'ready' | 'deferred' | 'failed';

export interface PreviewPixelateCanvasStatus {
  executionKey: string;
  targetClipId: string;
  status: PreviewPixelateCanvasStatusKind;
  reason?: string;
}

interface PreviewPixelateCanvasProps<T extends PreviewPixelateBackdropLayer> {
  plan: PreviewPixelateBackdropReady<T>;
  canvasWidth: number;
  canvasHeight: number;
  stageScale: number;
  sourceForClip: (clipId: string) => HTMLImageElement | HTMLVideoElement | null;
  executionKey: string;
  active: boolean;
  onStatusChange: (status: PreviewPixelateCanvasStatus) => void;
}

type RasterSource = HTMLImageElement | HTMLVideoElement;

/**
 * Exact deterministic pixelate surface for the deliberately narrow backdrop
 * admission in preview-pixelate-backdrop-plan-v1. The existing preview media
 * element remains the sole decoder/source-time authority. This consumer paints
 * that decoded frame through canonical media geometry, proves the entire target
 * region opaque, then runs preview-pixelate-raster-v1. Any missing readiness,
 * alpha, or runtime error leaves the CSS compatibility painter available.
 */
export function PreviewPixelateCanvas<T extends PreviewPixelateBackdropLayer>({
  plan,
  canvasWidth,
  canvasHeight,
  stageScale,
  sourceForClip,
  executionKey,
  active,
  onStatusChange,
}: PreviewPixelateCanvasProps<T>) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [sourceRevision, setSourceRevision] = useState(0);
  const [sourceFailed, setSourceFailed] = useState(false);
  const [region, setRegion] = useState<PreviewPixelateCanvasRegion | null>(null);
  const [status, setStatus] = useState<PreviewPixelateCanvasStatusKind>('pending');
  const [reason, setReason] = useState<string | undefined>();
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);

  useEffect(() => {
    setSourceFailed(false);
    setRegion(null);
    setStatus('pending');
    setReason(undefined);
  }, [executionKey]);

  useEffect(() => {
    const source = sourceForClip(plan.backdrop.clip.id);
    if (!source) return;
    const onSeeking = () => {
      setSourceFailed(false);
      setStatus('pending');
      setReason(undefined);
      bumpSourceRevision();
    };
    const onSettled = () => {
      setSourceFailed(false);
      bumpSourceRevision();
    };
    const onError = () => {
      setSourceFailed(true);
      bumpSourceRevision();
    };
    if (source instanceof HTMLVideoElement) {
      source.addEventListener('seeking', onSeeking);
      source.addEventListener('seeked', onSettled);
      source.addEventListener('loadedmetadata', onSettled);
      source.addEventListener('loadeddata', onSettled);
      source.addEventListener('error', onError);
    } else {
      source.addEventListener('load', onSettled);
      source.addEventListener('error', onError);
    }
    return () => {
      if (source instanceof HTMLVideoElement) {
        source.removeEventListener('seeking', onSeeking);
        source.removeEventListener('seeked', onSettled);
        source.removeEventListener('loadedmetadata', onSettled);
        source.removeEventListener('loadeddata', onSettled);
        source.removeEventListener('error', onError);
      } else {
        source.removeEventListener('load', onSettled);
        source.removeEventListener('error', onError);
      }
    };
  }, [bumpSourceRevision, plan.backdrop.clip.id, sourceForClip]);

  const draw = useCallback((): { status: PreviewPixelateCanvasStatusKind; region?: PreviewPixelateCanvasRegion; reason?: string } => {
    const output = canvasRef.current;
    if (!output || canvasWidth <= 0 || canvasHeight <= 0 || stageScale <= 0) {
      return { status: 'pending' };
    }
    if (sourceFailed) return { status: 'failed', reason: 'decoded-frame-error' };
    const source = sourceForClip(plan.backdrop.clip.id);
    if (!source || !sourceReady(source)) return { status: 'pending' };
    const targetState = plan.target.canonicalState;
    if (!targetState) return { status: 'failed', reason: 'pixelate-target-state-missing' };

    const resolvedRegion = resolvePreviewPixelateCanvasRegion(targetState, canvasWidth, canvasHeight);
    const backdrop = document.createElement('canvas');
    backdrop.width = canvasWidth;
    backdrop.height = canvasHeight;
    const backdropContext = backdrop.getContext('2d', { willReadFrequently: true });
    if (!backdropContext) throw new Error('pixelate Canvas could not create backdrop 2D context');
    backdropContext.clearRect(0, 0, canvasWidth, canvasHeight);

    const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
    const mediaPlan = resolvePreviewCanvasMediaLayerPlan(
      plan.backdrop,
      intrinsicWidth,
      intrinsicHeight,
    );
    paintPreviewCanvasMediaLayer(backdropContext, source, mediaPlan);
    const input = backdropContext.getImageData(
      resolvedRegion.x,
      resolvedRegion.y,
      resolvedRegion.width,
      resolvedRegion.height,
    );
    if (!previewPixelateRegionIsOpaque(input.data)) {
      output.width = resolvedRegion.width;
      output.height = resolvedRegion.height;
      output.getContext('2d')?.clearRect(0, 0, output.width, output.height);
      return { status: 'deferred', region: resolvedRegion, reason: 'opaque-region-proof' };
    }

    const pixelated = pixelatePreviewRgba(resolvedRegion.raster, input.data);
    output.width = resolvedRegion.width;
    output.height = resolvedRegion.height;
    const outputContext = output.getContext('2d');
    if (!outputContext) throw new Error('pixelate Canvas could not create output 2D context');
    outputContext.clearRect(0, 0, output.width, output.height);
    outputContext.putImageData(
      new ImageData(new Uint8ClampedArray(pixelated), resolvedRegion.width, resolvedRegion.height),
      0,
      0,
    );
    return { status: 'ready', region: resolvedRegion };
  }, [canvasHeight, canvasWidth, plan.backdrop, plan.target.canonicalState, sourceFailed, sourceForClip, stageScale]);

  useEffect(() => {
    try {
      const result = draw();
      setStatus(result.status);
      setRegion(result.region ?? null);
      setReason(result.reason);
    } catch (error) {
      setStatus('failed');
      setRegion(null);
      setReason(error instanceof Error ? error.message : String(error));
    }
  }, [draw, sourceRevision]);

  useEffect(() => {
    onStatusChange({
      executionKey,
      targetClipId: plan.target.clip.id,
      status,
      ...(reason ? { reason } : {}),
    });
  }, [executionKey, onStatusChange, plan.target.clip.id, reason, status]);

  return (
    <div
      data-preview-pixelate-execution="canvas"
      data-preview-pixelate-target-clip={plan.target.clip.id}
      data-preview-pixelate-backdrop-clip={plan.backdrop.clip.id}
      data-preview-pixelate-status={status}
      data-preview-pixelate-reason={reason}
      className="pointer-events-none absolute inset-0"
      style={{ visibility: active && status === 'ready' ? 'visible' : 'hidden' }}
      aria-hidden="true"
    >
      <canvas
        ref={canvasRef}
        width={region?.width ?? 1}
        height={region?.height ?? 1}
        className="absolute"
        style={region ? {
          left: region.x * stageScale,
          top: region.y * stageScale,
          width: region.width * stageScale,
          height: region.height * stageScale,
        } : undefined}
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
