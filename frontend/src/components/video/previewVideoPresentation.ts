export const PREVIEW_VIDEO_PRESENTATION_V1 = 'preview-video-presentation-v1' as const;

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
      // Media metadata can still be loading. The mounted preview's media events
      // leave this request pending; the pixelate consumer is bounded and
      // fail-closed instead of inventing presentation readiness.
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
    // Subscribe before the deterministic seek so a fast decoder cannot present
    // the requested frame between seek and requestVideoFrameCallback setup.
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
