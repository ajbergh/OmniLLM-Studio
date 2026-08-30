#!/usr/bin/env node
import fs from 'node:fs';

const read = (path) => fs.readFileSync(path, 'utf8');
const write = (path, content) => fs.writeFileSync(path, content);

function replaceOnce(path, before, after) {
  const source = read(path);
  const index = source.indexOf(before);
  if (index < 0) throw new Error(`guarded replacement not found in ${path}: ${before.slice(0, 100)}`);
  if (source.indexOf(before, index + before.length) >= 0) throw new Error(`guarded replacement is ambiguous in ${path}`);
  write(path, source.slice(0, index) + after + source.slice(index + before.length));
}

function appendBefore(path, marker, content) {
  replaceOnce(path, marker, `${content}${marker}`);
}

write('frontend/src/components/video/previewVideoPresentation.ts', `export const PREVIEW_VIDEO_PRESENTATION_V1 = 'preview-video-presentation-v1' as const;

export type PreviewVideoPresentationProof =
  | { status: 'ready' }
  | { status: 'pending'; reason: 'decoded-video-presentation-pending' }
  | { status: 'deferred'; reason: string };

interface PresentationState {
  token: string;
  callbackId: number | null;
  attempts: number;
}

interface VideoFrameMetadataLike {
  mediaTime: number;
}

type PresentationVideo = HTMLVideoElement & {
  requestVideoFrameCallback?: (callback: (now: number, metadata: VideoFrameMetadataLike) => void) => number;
  cancelVideoFrameCallback?: (handle: number) => void;
};

const presentationState = new WeakMap<HTMLVideoElement, PresentationState>();

export function previewVideoPresentationToken(
  clipId: string,
  frameIndex: number,
  sourceTimeMs: number,
): string {
  if (!clipId.trim()) throw new Error('video presentation clip id is required');
  if (!Number.isInteger(frameIndex) || frameIndex < 0) throw new Error('video presentation frame index must be a non-negative integer');
  if (!Number.isFinite(sourceTimeMs) || sourceTimeMs < 0) throw new Error('video presentation source time must be finite and non-negative');
  return `${PREVIEW_VIDEO_PRESENTATION_V1}:${encodeURIComponent(clipId)}:${frameIndex}:${sourceTimeMs.toFixed(6)}`;
}

export function previewVideoPresentationMediaTimeMatches(
  sourceTimeMs: number,
  mediaTimeSeconds: number,
  toleranceSeconds: number,
): boolean {
  if (!Number.isFinite(sourceTimeMs) || sourceTimeMs < 0) return false;
  if (!Number.isFinite(mediaTimeSeconds) || mediaTimeSeconds < 0) return false;
  if (!Number.isFinite(toleranceSeconds) || toleranceSeconds < 0) return false;
  return Math.abs(mediaTimeSeconds - sourceTimeMs / 1000) <= toleranceSeconds;
}

export function resetPreviewVideoPresentation(video: HTMLVideoElement): void {
  const capable = video as PresentationVideo;
  const state = presentationState.get(video);
  if (state?.callbackId !== null && typeof capable.cancelVideoFrameCallback === 'function') {
    capable.cancelVideoFrameCallback(state.callbackId);
  }
  presentationState.delete(video);
  delete video.dataset.videoPreviewPresentationRequestToken;
  delete video.dataset.videoPreviewPresentationReadyToken;
  delete video.dataset.videoPreviewPresentationStatus;
  delete video.dataset.videoPreviewPresentationMediaTime;
  delete video.dataset.videoPreviewPresentationAttempts;
}

export function ensurePreviewVideoPresentation(options: {
  video: HTMLVideoElement;
  token: string;
  sourceTimeMs: number;
  seekSeconds: number;
  toleranceSeconds: number;
  maxAttempts?: number;
}): void {
  const {
    video,
    token,
    sourceTimeMs,
    seekSeconds,
    toleranceSeconds,
    maxAttempts = 3,
  } = options;
  if (!token) throw new Error('video presentation token is required');
  if (!Number.isFinite(seekSeconds) || seekSeconds < 0) throw new Error('video presentation seek target must be finite and non-negative');
  if (!Number.isInteger(maxAttempts) || maxAttempts < 1) throw new Error('video presentation max attempts must be a positive integer');

  const capable = video as PresentationVideo;
  let state = presentationState.get(video);
  if (state?.token === token) {
    if (video.dataset.videoPreviewPresentationReadyToken === token
      && video.dataset.videoPreviewPresentationStatus === 'ready') return;
    if (state.callbackId !== null) return;
    if (video.dataset.videoPreviewPresentationStatus === 'deferred'
      || video.dataset.videoPreviewPresentationStatus === 'unsupported') return;
  } else {
    if (state?.callbackId !== null && typeof capable.cancelVideoFrameCallback === 'function') {
      capable.cancelVideoFrameCallback(state.callbackId);
    }
    state = { token, callbackId: null, attempts: 0 };
    presentationState.set(video, state);
    video.dataset.videoPreviewPresentationRequestToken = token;
    delete video.dataset.videoPreviewPresentationReadyToken;
    delete video.dataset.videoPreviewPresentationMediaTime;
    video.dataset.videoPreviewPresentationStatus = 'pending';
    video.dataset.videoPreviewPresentationAttempts = '0';
  }

  const seek = () => {
    try {
      video.currentTime = seekSeconds;
    } catch {
      // Metadata may still be loading. The mounted preview's normal media events
      // will leave this request pending; the pixelate consumer remains bounded
      // and fail-closed instead of inventing presentation readiness.
    }
  };

  if (typeof capable.requestVideoFrameCallback !== 'function') {
    seek();
    video.dataset.videoPreviewPresentationStatus = 'unsupported';
    return;
  }

  const schedule = () => {
    if (!state || state.token !== token) return;
    state.attempts += 1;
    video.dataset.videoPreviewPresentationStatus = 'pending';
    video.dataset.videoPreviewPresentationAttempts = String(state.attempts);
    state.callbackId = capable.requestVideoFrameCallback!((_now, metadata) => {
      const current = presentationState.get(video);
      if (!current || current !== state || current.token !== token) return;
      current.callbackId = null;
      video.dataset.videoPreviewPresentationMediaTime = Number.isFinite(metadata.mediaTime)
        ? metadata.mediaTime.toFixed(9)
        : 'invalid';
      if (previewVideoPresentationMediaTimeMatches(sourceTimeMs, metadata.mediaTime, toleranceSeconds)) {
        video.dataset.videoPreviewPresentationReadyToken = token;
        video.dataset.videoPreviewPresentationStatus = 'ready';
        return;
      }
      delete video.dataset.videoPreviewPresentationReadyToken;
      if (current.attempts >= maxAttempts) {
        video.dataset.videoPreviewPresentationStatus = 'deferred';
        return;
      }
      schedule();
      seek();
    });
    // Arm the presentation callback before the deterministic seek so a fast
    // decoder cannot present the requested frame between seek and subscription.
    seek();
  };

  schedule();
}

export function resolvePreviewVideoPresentation(
  video: HTMLVideoElement,
  expectedToken: string | undefined,
): PreviewVideoPresentationProof {
  if (!expectedToken) {
    return { status: 'deferred', reason: 'decoded-video-presentation-request-missing' };
  }
  if (video.dataset.videoPreviewPresentationRequestToken !== expectedToken) {
    return { status: 'pending', reason: 'decoded-video-presentation-pending' };
  }
  if (video.dataset.videoPreviewPresentationReadyToken === expectedToken
    && video.dataset.videoPreviewPresentationStatus === 'ready') {
    return { status: 'ready' };
  }
  if (video.dataset.videoPreviewPresentationStatus === 'unsupported') {
    return { status: 'deferred', reason: 'decoded-video-presentation-unsupported' };
  }
  if (video.dataset.videoPreviewPresentationStatus === 'deferred') {
    return { status: 'deferred', reason: 'decoded-video-presentation-mismatch' };
  }
  return { status: 'pending', reason: 'decoded-video-presentation-pending' };
}
`);

write('frontend/src/components/video/previewVideoPresentation.test.ts', `import { describe, expect, it } from 'vitest';
import {
  previewVideoPresentationMediaTimeMatches,
  previewVideoPresentationToken,
} from './previewVideoPresentation';

describe('previewVideoPresentationToken', () => {
  it('binds clip, canonical frame, and canonical source time deterministically', () => {
    expect(previewVideoPresentationToken('clip alpha/video', 15, 500)).toBe(
      'preview-video-presentation-v1:clip%20alpha%2Fvideo:15:500.000000',
    );
  });

  it('rejects invalid frame identities instead of creating ambiguous tokens', () => {
    expect(() => previewVideoPresentationToken('', 0, 0)).toThrow('clip id');
    expect(() => previewVideoPresentationToken('clip', -1, 0)).toThrow('frame index');
    expect(() => previewVideoPresentationToken('clip', 0.5, 0)).toThrow('frame index');
    expect(() => previewVideoPresentationToken('clip', 0, Number.NaN)).toThrow('source time');
  });
});

describe('previewVideoPresentationMediaTimeMatches', () => {
  it('accepts only the presented source timestamp inside the deterministic tolerance', () => {
    expect(previewVideoPresentationMediaTimeMatches(500, 0.5, 0.0005)).toBe(true);
    expect(previewVideoPresentationMediaTimeMatches(500, 0.5004, 0.0005)).toBe(true);
    expect(previewVideoPresentationMediaTimeMatches(500, 0.501, 0.0005)).toBe(false);
  });

  it('fails closed for invalid timestamps or tolerance', () => {
    expect(previewVideoPresentationMediaTimeMatches(-1, 0, 0.0005)).toBe(false);
    expect(previewVideoPresentationMediaTimeMatches(0, Number.NaN, 0.0005)).toBe(false);
    expect(previewVideoPresentationMediaTimeMatches(0, 0, -1)).toBe(false);
  });
});
`);

replaceOnce(
  'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx',
  "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\n",
  "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\nimport { ensurePreviewVideoPresentation, previewVideoPresentationToken, resetPreviewVideoPresentation } from './previewVideoPresentation';\n",
);

replaceOnce(
  'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx',
`      if (isPlaying) {
        if (element.paused && !element.ended) {
          element.currentTime = target;
          element.play().catch(() => { /* autoplay policy */ });
        } else if (Math.abs(element.currentTime - target) > 0.35) {
          // Drift correction (tab throttling, slow decode).
          element.currentTime = target;
        }
      } else {
        if (!element.paused) element.pause();
        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {
          element.currentTime = element instanceof HTMLVideoElement
            ? deterministicVideoSeekTargetSeconds(address, target)
            : target;
        }
      }
`,
`      if (isPlaying) {
        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);
        if (element.paused && !element.ended) {
          element.currentTime = target;
          element.play().catch(() => { /* autoplay policy */ });
        } else if (Math.abs(element.currentTime - target) > 0.35) {
          // Drift correction (tab throttling, slow decode).
          element.currentTime = target;
        }
      } else {
        if (!element.paused) element.pause();
        if (element instanceof HTMLVideoElement && address.kind === 'frame' && canonicalState) {
          ensurePreviewVideoPresentation({
            video: element,
            token: previewVideoPresentationToken(clip.id, address.frameIndex, targetMs),
            sourceTimeMs: targetMs,
            seekSeconds: deterministicVideoSeekTargetSeconds(address, target),
            toleranceSeconds: mediaSeekToleranceSeconds(address),
          });
          return;
        }
        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);
        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {
          element.currentTime = element instanceof HTMLVideoElement
            ? deterministicVideoSeekTargetSeconds(address, target)
            : target;
        }
      }
`,
);

replaceOnce(
  'frontend/src/components/video/previewPixelateBackdrop.ts',
`export type PreviewPixelateBackdropRuntimeRequirement =
  | 'decoded-frame-ready'
  | 'opaque-region-proof';
`,
`export type PreviewPixelateBackdropRuntimeRequirement =
  | 'decoded-frame-ready'
  | 'decoded-frame-presented';
`,
);
replaceOnce(
  'frontend/src/components/video/previewPixelateBackdrop.ts',
`const RUNTIME_REQUIREMENTS: readonly PreviewPixelateBackdropRuntimeRequirement[] = [
  'decoded-frame-ready',
  'opaque-region-proof',
];
`,
`const IMAGE_RUNTIME_REQUIREMENTS: readonly PreviewPixelateBackdropRuntimeRequirement[] = [
  'decoded-frame-ready',
];
const VIDEO_RUNTIME_REQUIREMENTS: readonly PreviewPixelateBackdropRuntimeRequirement[] = [
  'decoded-frame-ready',
  'decoded-frame-presented',
];
`,
);
replaceOnce(
  'frontend/src/components/video/previewPixelateBackdrop.ts',
` * layer. The structural plan is separate from runtime decoded-pixel evidence:
 * a consumer must still prove that the media frame is ready and that the
 * sampled region is opaque before replacing the CSS approximation.
`,
` * layer. The structural plan is separate from runtime decoded-pixel evidence:
 * images must be decoded, while video must additionally prove that the mounted
 * decoder presented the exact canonical source-time request before Canvas can
 * replace the CSS approximation. Alpha is then handled by source-over composition.
`,
);
replaceOnce(
  'frontend/src/components/video/previewPixelateBackdrop.ts',
`    runtimeRequirements: RUNTIME_REQUIREMENTS,
`,
`    runtimeRequirements: rasterSource.kind === 'video'
      ? VIDEO_RUNTIME_REQUIREMENTS
      : IMAGE_RUNTIME_REQUIREMENTS,
`,
);

replaceOnce(
  'frontend/src/components/video/previewPixelateBackdrop.test.ts',
`      runtimeRequirements: ['decoded-frame-ready', 'opaque-region-proof'],
`,
`      runtimeRequirements: ['decoded-frame-ready'],
`,
);
appendBefore(
  'frontend/src/components/video/previewPixelateBackdrop.test.ts',
`  it('keeps the legacy scalar scale path eligible when canonical scale is uniform', () => {
`,
`  it('requires exact decoder presentation only for video backdrops', () => {
    const plan = planPreviewPixelateBackdrop(12, [
      layer('backdrop', mediaState({ source_time_ms: 400 }), 'video/webm'),
      layer('pixelate', pixelateState()),
    ]);
    expect(plan).toMatchObject({
      mode: 'canonical-ready',
      rasterSource: { supported: true, kind: 'video' },
      runtimeRequirements: ['decoded-frame-ready', 'decoded-frame-presented'],
    });
  });

`,
);

write('frontend/src/components/video/PreviewPixelateCanvas.tsx', `import { useCallback, useEffect, useRef, useState } from 'react';
import type { PreviewPixelateBackdropLayer, PreviewPixelateBackdropReady } from './previewPixelateBackdrop';
import { resolvePreviewCanvasBackgroundColor } from './previewCanvasBackground';
import { pixelatePreviewRgba } from './previewPixelateRaster';
import {
  paintPreviewCanvasMediaLayer,
  resolvePreviewCanvasMediaLayerPlan,
} from './previewFrameWeightedPairCanvas';
import { resolvePreviewPixelateCanvasRegion } from './previewPixelateCanvasRuntime';
import { resolvePreviewVideoPresentation } from './previewVideoPresentation';

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
  videoPresentationToken?: string;
  executionKey: string;
  active: boolean;
  onStatusChange: (status: PreviewPixelateCanvasStatus) => void;
}

type RasterSource = HTMLImageElement | HTMLVideoElement;

const MAX_PENDING_SOURCE_RETRIES = 120;
const MAX_VIDEO_PRESENTATION_RETRIES = 120;

/**
 * Exact deterministic pixelate surface for the deliberately narrow backdrop
 * admission in preview-pixelate-backdrop-plan-v1. The existing preview media
 * element remains the sole decoder/source-time authority. This consumer mirrors
 * renderer composition order for the admitted one-media-layer path: paint the
 * opaque canonical project background, source-over the decoded media through
 * canonical geometry, then run preview-pixelate-raster-v1 on the composed RGB
 * backdrop.
 *
 * Images and transparent/partial-alpha video therefore share the same Canvas
 * source-over path. Video is admitted only after VideoPreviewCanvasLegacy has
 * proved with requestVideoFrameCallback that the mounted decoder presented the
 * exact canonical source-time request. Missing/unsupported/mismatched proof is
 * bounded and fail-closed to the existing CSS approximation.
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
  videoPresentationToken,
  executionKey,
  active,
  onStatusChange,
}: PreviewPixelateCanvasProps<T>) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const pendingRetryRef = useRef(0);
  const presentationRetryRef = useRef(0);
  const [sourceRevision, setSourceRevision] = useState(0);
  const [sourceFailed, setSourceFailed] = useState(false);
  const [status, setStatus] = useState<PreviewPixelateCanvasStatusKind>('pending');
  const [reason, setReason] = useState<string | undefined>();
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);
  const resolvedCanvasBackground = resolvePreviewCanvasBackgroundColor(canvasBackground);

  useEffect(() => {
    pendingRetryRef.current = 0;
    presentationRetryRef.current = 0;
    setSourceFailed(false);
    setStatus('pending');
    setReason(undefined);
  }, [executionKey]);

  useEffect(() => {
    const source = sourceForClip(plan.backdrop.clip.id);
    if (!source) return;
    const onSeeking = () => {
      pendingRetryRef.current = 0;
      presentationRetryRef.current = 0;
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

    if (source instanceof HTMLVideoElement) {
      const presentation = resolvePreviewVideoPresentation(source, videoPresentationToken);
      if (presentation.status !== 'ready') {
        output.getContext('2d')?.clearRect(0, 0, output.width, output.height);
        if (presentation.status === 'pending'
          && presentationRetryRef.current >= MAX_VIDEO_PRESENTATION_RETRIES) {
          return { status: 'deferred', reason: 'decoded-video-presentation-timeout' };
        }
        return presentation;
      }
    }

    const resolvedRegion = resolvePreviewPixelateCanvasRegion(targetState, canvasWidth, canvasHeight);
    const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
    const mediaPlan = resolvePreviewCanvasMediaLayerPlan(
      plan.backdrop,
      intrinsicWidth,
      intrinsicHeight,
    );

    setOutputGeometry(output, resolvedRegion.x, resolvedRegion.y, resolvedRegion.width, resolvedRegion.height, stageScale);

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
  }, [canvasHeight, canvasWidth, plan.backdrop, plan.target.canonicalState, resolvedCanvasBackground, sourceFailed, sourceForClip, stageScale, videoPresentationToken]);

  useEffect(() => {
    let retryFrame: number | null = null;
    try {
      const result = draw();
      setStatus(result.status);
      setReason(result.reason);
      if (result.status === 'pending' && result.reason === 'decoded-video-presentation-pending') {
        pendingRetryRef.current = 0;
        if (presentationRetryRef.current < MAX_VIDEO_PRESENTATION_RETRIES) {
          presentationRetryRef.current += 1;
          retryFrame = window.requestAnimationFrame(bumpSourceRevision);
        }
      } else if (result.status === 'pending' && pendingRetryRef.current < MAX_PENDING_SOURCE_RETRIES) {
        pendingRetryRef.current += 1;
        retryFrame = window.requestAnimationFrame(bumpSourceRevision);
      } else if (result.status !== 'pending') {
        pendingRetryRef.current = 0;
        presentationRetryRef.current = 0;
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
`);

replaceOnce(
  'frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx',
  "import { frameAddressMatchesTimelineMs } from './sourceTiming';\n",
  "import { frameAddressMatchesTimelineMs } from './sourceTiming';\nimport { previewVideoPresentationToken } from './previewVideoPresentation';\n",
);
replaceOnce(
  'frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx',
` * Structural admission and runtime decoded-pixel proof both fail closed: normal
 * playback, poster-budget sources, unsupported canonical state, and decoded
 * video that cannot prove its source frame opaque leave the established CSS
 * approximation untouched. Transparent/partial-alpha images are admitted only
 * after canonical project-background composition in PreviewPixelateCanvas.
`,
` * Structural admission and runtime decoded-pixel proof both fail closed: normal
 * playback, poster-budget sources, unsupported canonical state, and decoded
 * video without an exact canonical presentation proof leave the established CSS
 * approximation untouched. Images and alpha video are consumed only after
 * canonical project-background source-over composition in PreviewPixelateCanvas.
`,
);
replaceOnce(
  'frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx',
`  const consume = plan.mode === 'canonical-ready' && !posterDeferredReason;
  const executionKey = consume
    ? `${deterministicFrame}:${plan.target.clip.id}:${plan.backdrop.clip.id}:${canvasWidth}x${canvasHeight}:${canvasBackground}`
    : '';
`,
`  const consume = plan.mode === 'canonical-ready' && !posterDeferredReason;
  const videoPresentationToken = plan.mode === 'canonical-ready'
    && !posterDeferredReason
    && deterministicFrame !== null
    && plan.rasterSource.kind === 'video'
    && plan.backdrop.canonicalState
    ? previewVideoPresentationToken(
      plan.backdrop.clip.id,
      deterministicFrame,
      plan.backdrop.canonicalState.source_time_ms,
    )
    : undefined;
  const executionKey = consume
    ? `${deterministicFrame}:${plan.target.clip.id}:${plan.backdrop.clip.id}:${canvasWidth}x${canvasHeight}:${canvasBackground}:${videoPresentationToken ?? 'image'}`
    : '';
`,
);
replaceOnce(
  'frontend/src/components/video/PreviewPixelateBackdropConsumer.tsx',
`      sourceForClip={sourceForClip}
      executionKey={executionKey}
`,
`      sourceForClip={sourceForClip}
      videoPresentationToken={videoPresentationToken}
      executionKey={executionKey}
`,
);

replaceOnce(
  'backend/internal/video/probe.go',
`\tVideoCodec string  \`json:"video_codec,omitempty"\`
\tAudioCodec string  \`json:"audio_codec,omitempty"\`
`,
`\tVideoCodec       string \`json:"video_codec,omitempty"\`
\tVideoPixelFormat string \`json:"video_pixel_format,omitempty"\`
\tVideoAlphaMode   string \`json:"video_alpha_mode,omitempty"\`
\tAudioCodec       string \`json:"audio_codec,omitempty"\`
`,
);
replaceOnce(
  'backend/internal/video/probe.go',
`\t\t\tDuration     string \`json:"duration"\`
\t\t\tChannels     int    \`json:"channels"\`
`,
`\t\t\tDuration     string \`json:"duration"\`
\t\t\tPixFmt       string \`json:"pix_fmt"\`
\t\t\tTags         struct {
\t\t\t\tAlphaMode string \`json:"alpha_mode"\`
\t\t\t} \`json:"tags"\`
\t\t\tChannels     int    \`json:"channels"\`
`,
);
replaceOnce(
  'backend/internal/video/probe.go',
`\t\t\tprobe.VideoCodec = strings.TrimSpace(stream.CodecName)
\t\t\tprobe.FPS = parseFrameRate(stream.AvgFrameRate)
`,
`\t\t\tprobe.VideoCodec = strings.TrimSpace(stream.CodecName)
\t\t\tprobe.VideoPixelFormat = strings.TrimSpace(stream.PixFmt)
\t\t\tprobe.VideoAlphaMode = strings.TrimSpace(stream.Tags.AlphaMode)
\t\t\tprobe.FPS = parseFrameRate(stream.AvgFrameRate)
`,
);
replaceOnce(
  'backend/internal/video/probe.go',
`\tif probe.VideoCodec != "" {
\t\tmeta["video_codec"] = probe.VideoCodec
\t}
`,
`\tif probe.VideoCodec != "" {
\t\tmeta["video_codec"] = probe.VideoCodec
\t}
\tif probe.VideoPixelFormat != "" {
\t\tmeta["video_pixel_format"] = probe.VideoPixelFormat
\t}
\tif probe.VideoAlphaMode != "" {
\t\tmeta["video_alpha_mode"] = probe.VideoAlphaMode
\t}
`,
);
appendBefore(
  'backend/internal/video/probe.go',
`// parseFrameRate parses ffprobe rational frame rates like "30000/1001" or "30/1".
`,
`// VideoHasAlpha reports stream-level alpha facts advertised by ffprobe. VP9
// WebM commonly reports yuv420p while carrying alpha_mode=1, so pixel format
// alone is not a sufficient signal.
func (p *MediaProbe) VideoHasAlpha() bool {
\tif p == nil {
\t\treturn false
\t}
\tif strings.TrimSpace(p.VideoAlphaMode) == "1" {
\t\treturn true
\t}
\treturn videoPixelFormatHasAlpha(p.VideoPixelFormat)
}

func videoPixelFormatHasAlpha(value string) bool {
\tpixelFormat := strings.ToLower(strings.TrimSpace(value))
\tif strings.HasPrefix(pixelFormat, "yuva") || strings.HasPrefix(pixelFormat, "gbrap") {
\t\treturn true
\t}
\tswitch pixelFormat {
\tcase "rgba", "bgra", "argb", "abgr", "ya8", "ya16le", "ya16be":
\t\treturn true
\tdefault:
\t\treturn false
\t}
}

// mergeProbeMetadataJSON freezes current stream facts into an immutable staged
// asset while preserving unrelated authored/import metadata.
func mergeProbeMetadataJSON(existing string, probe *MediaProbe) string {
\tmetadata := map[string]any{}
\tif strings.TrimSpace(existing) != "" {
\t\t_ = json.Unmarshal([]byte(existing), &metadata)
\t}
\tprobeJSON := ProbeMetadataJSON(probe)
\tif probeJSON != "" {
\t\tprobeMetadata := map[string]any{}
\t\tif json.Unmarshal([]byte(probeJSON), &probeMetadata) == nil {
\t\t\tfor key, value := range probeMetadata {
\t\t\t\tmetadata[key] = value
\t\t\t}
\t\t}
\t}
\tdata, err := json.Marshal(metadata)
\tif err != nil {
\t\treturn existing
\t}
\treturn string(data)
}

`,
);

replaceOnce(
  'backend/internal/video/render_snapshot.go',
`\t\t\tif renderAssetRequiresVideo(stagedAsset) && media.VideoCodec == "" && media.Width == 0 && media.Height == 0 {
\t\t\t\treturn nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable visual stream", strings.Join(references[assetID], ", "), assetID)
\t\t\t}
\t\t}
`,
`\t\t\tif renderAssetRequiresVideo(stagedAsset) && media.VideoCodec == "" && media.Width == 0 && media.Height == 0 {
\t\t\t\treturn nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable visual stream", strings.Join(references[assetID], ", "), assetID)
\t\t\t}
\t\t\tstagedAsset.MetadataJSON = mergeProbeMetadataJSON(stagedAsset.MetadataJSON, media)
\t\t}
`,
);

replaceOnce(
  'backend/internal/video/renderer.go',
`\taudioChannels   int
\t// hasAudio reports whether a video asset carries an audio stream that
`,
`\taudioChannels   int
\tinputDecoder    string
\t// hasAudio reports whether a video asset carries an audio stream that
`,
);
replaceOnce(
  'backend/internal/video/renderer.go',
`\t\t\trc := resolvedClip{
\t\t\t\ttrackIndex: trackIndex,
\t\t\t\tclip:       clip,
\t\t\t\tfilePath:   fullPath,
\t\t\t\tisVideo:    strings.HasPrefix(mime, "video/"),
\t\t\t\tisImage:    strings.HasPrefix(mime, "image/"),
\t\t\t\tisAudio:    strings.HasPrefix(mime, "audio/"),
\t\t\t}
`,
`\t\t\trc := resolvedClip{
\t\t\t\ttrackIndex: trackIndex,
\t\t\t\tclip:       clip,
\t\t\t\tfilePath:   fullPath,
\t\t\t\tisVideo:    strings.HasPrefix(mime, "video/"),
\t\t\t\tisImage:    strings.HasPrefix(mime, "image/"),
\t\t\t\tisAudio:    strings.HasPrefix(mime, "audio/"),
\t\t\t}
\t\t\tif rc.isVideo {
\t\t\t\trc.inputDecoder = videoAssetInputDecoder(asset)
\t\t\t}
`,
);
replaceOnce(
  'backend/internal/video/renderer.go',
`\tsourceByPath := map[string]int{}
\tvisualBySource := map[int][]int{}
`,
`\tsourceByPath := map[string]int{}
\tdecoderByPath := map[string]string{}
\tfor _, clip := range clips {
\t\tif clip.inputDecoder != "" {
\t\t\tdecoderByPath[clip.filePath] = clip.inputDecoder
\t\t}
\t}
\tvisualBySource := map[int][]int{}
`,
);
replaceOnce(
  'backend/internal/video/renderer.go',
`\t\t\tsourceByPath[clips[i].filePath] = sourceIdx
\t\t\targs = append(args, "-i", clips[i].filePath)
`,
`\t\t\tsourceByPath[clips[i].filePath] = sourceIdx
\t\t\tif decoder := decoderByPath[clips[i].filePath]; decoder != "" {
\t\t\t\targs = append(args, "-c:v", decoder)
\t\t\t}
\t\t\targs = append(args, "-i", clips[i].filePath)
`,
);
appendBefore(
  'backend/internal/video/renderer.go',
`// videoAssetHasAudio reports whether a video asset carries an audio stream.
`,
`// videoAssetInputDecoder selects an input decoder only when immutable stream
// facts require one for fidelity. FFmpeg's native VP9 decoder discards WebM
// alpha, while libvpx-vp9 preserves alpha_mode=1 streams.
func videoAssetInputDecoder(asset models.VideoAsset) string {
\tif strings.TrimSpace(asset.MetadataJSON) == "" {
\t\treturn ""
\t}
\tvar metadata struct {
\t\tVideoCodec       string \`json:"video_codec"\`
\t\tVideoPixelFormat string \`json:"video_pixel_format"\`
\t\tVideoAlphaMode   string \`json:"video_alpha_mode"\`
\t}
\tif err := json.Unmarshal([]byte(asset.MetadataJSON), &metadata); err != nil {
\t\treturn ""
\t}
\tprobe := &MediaProbe{
\t\tVideoCodec:       metadata.VideoCodec,
\t\tVideoPixelFormat: metadata.VideoPixelFormat,
\t\tVideoAlphaMode:   metadata.VideoAlphaMode,
\t}
\tif strings.EqualFold(strings.TrimSpace(probe.VideoCodec), "vp9") && probe.VideoHasAlpha() {
\t\treturn "libvpx-vp9"
\t}
\treturn ""
}

`,
);

appendBefore(
  'backend/internal/video/probe_test.go',
`func TestParseFrameRate(t *testing.T) {
`,
`func TestParseProbePayloadPreservesVideoAlphaFacts(t *testing.T) {
\tpayload := []byte(\`{
\t\t"format": {"duration": "2.0"},
\t\t"streams": [{
\t\t\t"codec_type": "video",
\t\t\t"codec_name": "vp9",
\t\t\t"pix_fmt": "yuv420p",
\t\t\t"width": 512,
\t\t\t"height": 512,
\t\t\t"r_frame_rate": "30/1",
\t\t\t"tags": {"alpha_mode": "1"}
\t\t}]
\t}\`)
\tprobe, err := parseProbePayload(payload)
\tif err != nil || probe == nil {
\t\tt.Fatalf("alpha probe = %+v err=%v", probe, err)
\t}
\tif probe.VideoCodec != "vp9" || probe.VideoPixelFormat != "yuv420p" || probe.VideoAlphaMode != "1" || !probe.VideoHasAlpha() {
\t\tt.Fatalf("alpha stream facts = %+v", probe)
\t}
\tmetadata := mergeProbeMetadataJSON(\`{"source":"fixture","video_codec":"stale"}\`, probe)
\tvar got map[string]any
\tif err := json.Unmarshal([]byte(metadata), &got); err != nil {
\t\tt.Fatalf("merged metadata: %v", err)
\t}
\tif got["source"] != "fixture" || got["video_codec"] != "vp9" || got["video_alpha_mode"] != "1" {
\t\tt.Fatalf("merged metadata = %#v", got)
\t}
}

`,
);
replaceOnce(
  'backend/internal/video/probe_test.go',
`import "testing"
`,
`import (
\t"encoding/json"
\t"testing"
)
`,
);

appendBefore(
  'backend/internal/video/renderer_input_fanout_test.go',
`func TestResolvedInputLabelsPreserveLegacyDirectGraphTests(t *testing.T) {
`,
`func TestAppendResolvedClipInputsSelectsAlphaPreservingVP9DecoderOnce(t *testing.T) {
\tclips := []resolvedClip{
\t\t{filePath: "input-alpha.webm", isVideo: true, inputDecoder: "libvpx-vp9"},
\t\t{filePath: "input-alpha.webm", isVideo: true},
\t\t{filePath: "input-opaque.mp4", isVideo: true},
\t}
\targs, _ := appendResolvedClipInputs(nil, clips, 1)
\tjoined := strings.Join(args, " ")
\tif strings.Count(joined, "-c:v libvpx-vp9") != 1 {
\t\tt.Fatalf("alpha decoder should be applied once to the shared source: %s", joined)
\t}
\tif !strings.Contains(joined, "-c:v libvpx-vp9 -i input-alpha.webm") {
\t\tt.Fatalf("alpha decoder must be scoped before its input: %s", joined)
\t}
\tif strings.Contains(joined, "libvpx-vp9 -i input-opaque.mp4") {
\t\tt.Fatalf("opaque input must not inherit alpha decoder: %s", joined)
\t}
}

func TestVideoAssetInputDecoderUsesFrozenAlphaFacts(t *testing.T) {
\tasset := models.VideoAsset{MetadataJSON: \`{"video_codec":"vp9","video_pixel_format":"yuv420p","video_alpha_mode":"1"}\`}
\tif got := videoAssetInputDecoder(asset); got != "libvpx-vp9" {
\t\tt.Fatalf("decoder = %q, want libvpx-vp9", got)
\t}
\tasset.MetadataJSON = \`{"video_codec":"vp9","video_pixel_format":"yuv420p"}\`
\tif got := videoAssetInputDecoder(asset); got != "" {
\t\tt.Fatalf("opaque decoder = %q, want default", got)
\t}
}

`,
);
replaceOnce(
  'backend/internal/video/renderer_input_fanout_test.go',
`import (
\t"strings"
\t"testing"
)
`,
`import (
\t"strings"
\t"testing"

\t"github.com/ajbergh/omnillm-studio/internal/models"
)
`,
);

console.log('transparent-video core patch applied');
