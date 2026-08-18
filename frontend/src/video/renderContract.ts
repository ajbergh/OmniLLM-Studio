/**
 * Renderer-independent timeline primitives shared by editor preview callers.
 * Keep these semantics mechanically verified against the Go rendercontract
 * package through video-renderer/test/fixtures/render-contract-v1.json.
 */

export type CanonicalEasing = 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out' | 'step';

export interface RationalTime {
  numerator: number;
  denominator: number;
}

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value));
}

export function frameTime(frameIndex: number, fps: number): RationalTime {
  return {
    numerator: Math.max(0, Math.trunc(frameIndex)),
    denominator: fps > 0 ? Math.trunc(fps) : 1,
  };
}

export function startFrame(ms: number, fps: number): number {
  if (ms <= 0 || fps <= 0) return 0;
  return Math.floor((ms * fps) / 1000);
}

export function endFrame(ms: number, fps: number): number {
  if (ms <= 0 || fps <= 0) return 0;
  return Math.ceil((ms * fps) / 1000);
}

export function frameCount(durationMs: number, fps: number): number {
  return endFrame(durationMs, fps);
}

export function activeAtFrame(frameIndex: number, startMs: number, durationMs: number, fps: number): boolean {
  if (frameIndex < 0 || durationMs <= 0 || fps <= 0) return false;
  const first = startFrame(startMs, fps);
  const endExclusive = endFrame(startMs + durationMs, fps);
  return first <= frameIndex && frameIndex < endExclusive;
}

export function sourceTimeMs(
  timelineMs: number,
  clipStartMs: number,
  trimInMs: number,
  playbackRate: number,
): number {
  const rate = playbackRate === 0 ? 1 : playbackRate;
  return trimInMs + Math.max(0, timelineMs - clipStartMs) * rate;
}

/**
 * Canonical v1-compatible easing. ease-in-out intentionally preserves the
 * editor preview's piecewise quadratic curve.
 */
export function easeProgress(t: number, easing?: string): number {
  const normalized = clamp01(t);
  switch (easing?.trim().toLowerCase()) {
    case 'ease-in':
      return normalized * normalized;
    case 'ease-out':
      return 1 - (1 - normalized) * (1 - normalized);
    case 'ease-in-out':
      return normalized < 0.5
        ? 2 * normalized * normalized
        : 1 - Math.pow(-2 * normalized + 2, 2) / 2;
    case 'step':
      return normalized >= 1 ? 1 : 0;
    default:
      return normalized;
  }
}
