export interface RationalTime {
  numerator: number;
  denominator: number;
}

export function rationalFromSeconds(seconds: number, precision = 1000): RationalTime {
  const num = Math.round(seconds * precision);
  return { numerator: num, denominator: precision };
}

export function frameCountFromDuration(durationMs: number, fps: number): number {
  if (durationMs <= 0 || fps <= 0) return 0;
  return Math.max(1, Math.round((durationMs * fps) / 1000));
}

export function frameIndexToTimeMs(frameIndex: number, fps: number): number {
  if (fps <= 0) return 0;
  return (frameIndex * 1000) / fps;
}

export function timeMsToFrameIndex(timeMs: number, fps: number): number {
  if (fps <= 0 || timeMs <= 0) return 0;
  return Math.floor((timeMs * fps) / 1000);
}

export function isFrameInHalfOpenInterval(
  frameIndex: number,
  startMs: number,
  durationMs: number,
  fps: number
): boolean {
  if (durationMs <= 0 || fps <= 0) return false;
  const startFrame = Math.floor((startMs * fps) / 1000);
  const endFrame = Math.ceil(((startMs + durationMs) * fps) / 1000);
  return frameIndex >= startFrame && frameIndex < endFrame;
}
