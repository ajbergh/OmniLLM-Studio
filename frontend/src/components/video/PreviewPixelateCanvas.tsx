import { useCallback, useEffect, useRef, useState } from 'react';
import type { PreviewPixelateBackdropLayer, PreviewPixelateBackdropReady } from './previewPixelateBackdrop';
import { resolvePreviewCanvasBackgroundColor } from './previewCanvasBackground';
import {
  pixelatePreviewRgba,
} from './previewPixelateRaster';
import {
  paintPreviewCanvasMediaLayer,
  resolvePreviewCanvasMediaLayerPlan,
} from './previewFrameWeightedPairCanvas';
import {
  resolvePreviewPixelateCanvasRegion,
  resolvePreviewPixelateOpacityProof,
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
  canvasBackground: string;
  stageScale: number;
  sourceForClip: (clipId: string) => HTMLImageElement | HTMLVideoElement | null;
  executionKey: string;
  active: boolean;
  onStatusChange: (status: PreviewPixelateCanvasStatus) => void;
}

type RasterSource = HTMLImageElement | HTMLVideoElement;

const MAX_PENDING_SOURCE_RETRIES = 120;
const MAX_VIDEO_OPACITY_PROOF_RETRIES = 60;

/**
 * Exact deterministic pixelate surface for the deliberately narrow backdrop
 * admission in preview-pixelate-backdrop-plan-v1. The existing preview media
 * element remains the sole decoder/source-time authority. This consumer mirrors
 * renderer composition order for the admitted one-media-layer path: paint the
 * opaque canonical project background, source-over the decoded media through
 * canonical geometry, then run preview-pixelate-raster-v1 on the composed RGB
 * backdrop.
 *
 * Transparent/partial-alpha images are therefore handled by browser Canvas
 * source-over composition rather than by inspecting or unpremultiplying their
 * source bytes. Decoded video retains the existing source-only alpha/readiness
 * proof before composition because Chromium can report HAVE_CURRENT_DATA before
 * a newly sought frame is rasterizable. Exact alpha 255 is still required for
 * video in this slice; transparent video remains fail-closed.
 *
 * Canvas bitmap dimensions and placement are owned imperatively by draw().
 * React deliberately keeps static 1x1 virtual width/height props so a status
 * render cannot re-apply changed bitmap dimensions and clear pixels that were
 * just painted into the DOM canvas.
 */
export function PreviewPixelateCanvas<T extends PreviewPixelateBackdropLayer>({
  plan,
  canvasWidth,
  canvasHeight,
  canvasBackground,
  stageScale,
  sourceForClip,
  executionKey,
  active,
  onStatusChange,
}: PreviewPixelateCanvasProps<T>) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const pendingRetryRef = useRef(0);
  const opacityProofRetryRef = useRef(0);
  const [sourceRevision, setSourceRevision] = useState(0);
  const [sourceFailed, setSourceFailed] = useState(false);
  const [status, setStatus] = useState<PreviewPixelateCanvasStatusKind>('pending');
  const [reason, setReason] = useState<string | undefined>();
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);
  const resolvedCanvasBackground = resolvePreviewCanvasBackgroundColor(canvasBackground);

  useEffect(() => {
    pendingRetryRef.current = 0;
    opacityProofRetryRef.current = 0;
    setSourceFailed(false);
    setStatus('pending');
    setReason(undefined);
  }, [executionKey]);

  useEffect(() => {
    const source = sourceForClip(plan.backdrop.clip.id);
    if (!source) return;
    const onSeeking = () => {
      pendingRetryRef.current = 0;
      opacityProofRetryRef.current = 0;
      setSourceFailed(false);
      setStatus('pending');
      setReason(undefined);
      bumpSourceRevision();
    };
    const onSettled = () => {
      opacityProofRetryRef.current = 0;
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

  const draw = useCallback((): { status: PreviewPixelateCanvasStatusKind; reason?: string } => {
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
    const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
    const mediaPlan = resolvePreviewCanvasMediaLayerPlan(
      plan.backdrop,
      intrinsicWidth,
      intrinsicHeight,
    );

    setOutputGeometry(output, resolvedRegion.x, resolvedRegion.y, resolvedRegion.width, resolvedRegion.height, stageScale);

    if (source instanceof HTMLVideoElement) {
      const sourceOnly = document.createElement('canvas');
      sourceOnly.width = canvasWidth;
      sourceOnly.height = canvasHeight;
      const sourceOnlyContext = sourceOnly.getContext('2d', { willReadFrequently: true });
      if (!sourceOnlyContext) throw new Error('pixelate Canvas could not create video proof 2D context');
      sourceOnlyContext.clearRect(0, 0, canvasWidth, canvasHeight);
      paintPreviewCanvasMediaLayer(sourceOnlyContext, source, mediaPlan);
      const sourceOnlyInput = sourceOnlyContext.getImageData(
        resolvedRegion.x,
        resolvedRegion.y,
        resolvedRegion.width,
        resolvedRegion.height,
      );
      const opacityProof = resolvePreviewPixelateOpacityProof(
        sourceOnlyInput.data,
        true,
        opacityProofRetryRef.current,
        MAX_VIDEO_OPACITY_PROOF_RETRIES,
      );
      if (opacityProof.status !== 'ready') {
        output.getContext('2d')?.clearRect(0, 0, output.width, output.height);
        return opacityProof;
      }
    }

    const backdrop = document.createElement('canvas');
    backdrop.width = canvasWidth;
    backdrop.height = canvasHeight;
    const backdropContext = backdrop.getContext('2d', { willReadFrequently: true });
    if (!backdropContext) throw new Error('pixelate Canvas could not create backdrop 2D context');
    backdropContext.fillStyle = resolvedCanvasBackground;
    backdropContext.fillRect(0, 0, canvasWidth, canvasHeight);
    paintPreviewCanvasMediaLayer(backdropContext, source, mediaPlan);
    const input = backdropContext.getImageData(
      resolvedRegion.x,
      resolvedRegion.y,
      resolvedRegion.width,
      resolvedRegion.height,
    );

    const pixelated = pixelatePreviewRgba(resolvedRegion.raster, input.data);
    const outputContext = output.getContext('2d');
    if (!outputContext) throw new Error('pixelate Canvas could not create output 2D context');
    outputContext.clearRect(0, 0, output.width, output.height);
    outputContext.putImageData(
      new ImageData(new Uint8ClampedArray(pixelated), resolvedRegion.width, resolvedRegion.height),
      0,
      0,
    );
    return { status: 'ready' };
  }, [canvasHeight, canvasWidth, plan.backdrop, plan.target.canonicalState, resolvedCanvasBackground, sourceFailed, sourceForClip, stageScale]);

  useEffect(() => {
    let retryFrame: number | null = null;
    try {
      const result = draw();
      setStatus(result.status);
      setReason(result.reason);
      if (result.status === 'pending' && result.reason === 'decoded-video-opacity-pending') {
        pendingRetryRef.current = 0;
        if (opacityProofRetryRef.current < MAX_VIDEO_OPACITY_PROOF_RETRIES) {
          opacityProofRetryRef.current += 1;
          retryFrame = window.requestAnimationFrame(bumpSourceRevision);
        }
      } else if (result.status === 'pending' && pendingRetryRef.current < MAX_PENDING_SOURCE_RETRIES) {
        pendingRetryRef.current += 1;
        retryFrame = window.requestAnimationFrame(bumpSourceRevision);
      } else if (result.status !== 'pending') {
        pendingRetryRef.current = 0;
        opacityProofRetryRef.current = 0;
      }
    } catch (error) {
      setStatus('failed');
      setReason(error instanceof Error ? error.message : String(error));
    }
    return () => {
      if (retryFrame !== null) window.cancelAnimationFrame(retryFrame);
    };
  }, [bumpSourceRevision, draw, sourceRevision]);

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
      data-preview-pixelate-background={resolvedCanvasBackground}
      data-preview-pixelate-status={status}
      data-preview-pixelate-reason={reason}
      className="pointer-events-none absolute inset-0"
      style={{ visibility: active && status === 'ready' ? 'visible' : 'hidden' }}
      aria-hidden="true"
    >
      <canvas
        ref={canvasRef}
        width={1}
        height={1}
        className="absolute"
      />
    </div>
  );
}

function setOutputGeometry(
  output: HTMLCanvasElement,
  x: number,
  y: number,
  width: number,
  height: number,
  stageScale: number,
): void {
  // Assign bitmap dimensions before painting. Because React's virtual width and
  // height remain permanently 1x1, later status renders do not overwrite these
  // imperative dimensions and therefore cannot clear the finished bitmap.
  output.width = width;
  output.height = height;
  output.style.left = `${x * stageScale}px`;
  output.style.top = `${y * stageScale}px`;
  output.style.width = `${width * stageScale}px`;
  output.style.height = `${height * stageScale}px`;
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
