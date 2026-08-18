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

export interface CanonicalMotionCurve {
  type: string;
  x1?: number;
  y1?: number;
  x2?: number;
  y2?: number;
  stiffness?: number;
  damping?: number;
  mass?: number;
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

/**
 * Evaluates the canonical curve for one keyframe segment. Curve wins when
 * present; fallback easing preserves v1 documents that only authored easing.
 */
export function curveProgress(t: number, curve?: CanonicalMotionCurve, fallback?: string): number {
  const normalized = clamp01(t);
  if (!curve) return easeProgress(normalized, fallback);
  switch (curve.type.trim().toLowerCase()) {
    case 'bezier':
      return cubicBezierProgress(
        normalized,
        curve.x1 ?? 0,
        curve.y1 ?? 0,
        curve.x2 ?? 0,
        curve.y2 ?? 0,
      );
    case 'spring':
      return springProgress(
        normalized,
        curve.stiffness ?? 170,
        curve.damping ?? 26,
        curve.mass ?? 1,
      );
    default:
      return easeProgress(normalized, curve.type);
  }
}

function cubicBezierProgress(x: number, x1: number, y1: number, x2: number, y2: number): number {
  const bezier = (t: number, a: number, b: number) => 3 * (1 - t) ** 2 * t * a + 3 * (1 - t) * t ** 2 * b + t ** 3;
  const derivative = (t: number, a: number, b: number) => 3 * (1 - t) ** 2 * a + 6 * (1 - t) * t * (b - a) + 3 * t ** 2 * (1 - b);
  let parameter = clamp01(x);
  for (let index = 0; index < 8; index += 1) {
    const slope = derivative(parameter, x1, x2);
    if (Math.abs(slope) < 1e-7) break;
    parameter = clamp01(parameter - (bezier(parameter, x1, x2) - x) / slope);
  }
  let low = 0;
  let high = 1;
  for (let index = 0; index < 12; index += 1) {
    const value = bezier(parameter, x1, x2);
    if (Math.abs(value - x) < 1e-7) break;
    if (value < x) low = parameter;
    else high = parameter;
    parameter = (low + high) / 2;
  }
  return bezier(parameter, y1, y2);
}

function springProgress(t: number, stiffness: number, damping: number, mass: number): number {
  const k = stiffness > 0 ? stiffness : 170;
  const c = damping > 0 ? damping : 26;
  const m = mass > 0 ? mass : 1;
  const response = (at: number) => {
    const omega0 = Math.sqrt(k / m);
    const zeta = c / (2 * Math.sqrt(k * m));
    if (zeta < 1 - 1e-6) {
      const omegaD = omega0 * Math.sqrt(1 - zeta ** 2);
      return 1 - Math.exp(-zeta * omega0 * at) * (Math.cos(omegaD * at) + (zeta * omega0 / omegaD) * Math.sin(omegaD * at));
    }
    if (zeta > 1 + 1e-6) {
      const root = Math.sqrt(zeta ** 2 - 1);
      const r1 = -omega0 * (zeta - root);
      const r2 = -omega0 * (zeta + root);
      return 1 - (r2 * Math.exp(r1 * at) - r1 * Math.exp(r2 * at)) / (r2 - r1);
    }
    return 1 - Math.exp(-omega0 * at) * (1 + omega0 * at);
  };
  const end = response(1);
  return Math.abs(end) < 1e-9 ? t : response(t) / end;
}
