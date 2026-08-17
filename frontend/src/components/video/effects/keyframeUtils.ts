/**
 * Keyframe sampling shared by the preview canvas, timeline clip envelopes,
 * and the keyframe lane. The FFmpeg fidelity expander mirrors these easing,
 * cubic Bezier, and segment-local spring semantics for deterministic export.
 */
import type { VideoMotionCurve, VideoTimelineKeyframe } from '../../../types/video';

export type KeyframeProperty = VideoTimelineKeyframe['property'];

export const KEYFRAME_PROPERTIES: KeyframeProperty[] = [
  'x', 'y', 'z', 'scale', 'scale_x', 'scale_y',
  'rotation', 'rotation_x', 'rotation_y', 'rotation_z', 'opacity', 'volume',
];

export const KEYFRAME_EASINGS = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'step'] as const;

const curveSampleCache = new Map<string, number>();

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value));
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

export function applyMotionCurve(t: number, curve?: VideoMotionCurve, easing?: string): number {
  const normalized = clamp01(t);
  const cacheKey = curve ? `${JSON.stringify(curve)}:${Math.round(normalized * 10000)}` : '';
  if (cacheKey) {
    const cached = curveSampleCache.get(cacheKey);
    if (cached !== undefined) return cached;
  }
  let value: number;
  if (curve?.type === 'bezier') value = cubicBezierProgress(normalized, curve.x1, curve.y1, curve.x2, curve.y2);
  else if (curve?.type === 'spring') value = springProgress(normalized, curve.stiffness, curve.damping, curve.mass);
  else {
    const selected = curve?.type || easing;
    switch (selected) {
      case 'ease-in': value = normalized * normalized; break;
      case 'ease-out': value = 1 - (1 - normalized) * (1 - normalized); break;
      case 'ease-in-out': value = normalized < 0.5 ? 2 * normalized * normalized : 1 - Math.pow(-2 * normalized + 2, 2) / 2; break;
      case 'step': value = normalized >= 1 ? 1 : 0; break;
      default: value = normalized;
    }
  }
  if (cacheKey) {
    if (curveSampleCache.size >= 4096) curveSampleCache.clear();
    curveSampleCache.set(cacheKey, value);
  }
  return value;
}

function applyEasing(t: number, easing?: string, curve?: VideoMotionCurve): number {
  if (curve) return applyMotionCurve(t, curve, easing);
  switch (easing) {
    case 'ease-in':
      return t * t;
    case 'ease-out':
      return 1 - (1 - t) * (1 - t);
    case 'ease-in-out':
      return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
    case 'step':
      return t >= 1 ? 1 : 0;
    default:
      return t;
  }
}

/**
 * Samples one property's keyframes at a clip-relative time (`time_ms` is
 * measured from the clip start). Returns null when the property has no
 * keyframes. The value holds flat before the first and after the last
 * keyframe; each segment eases using the LATER keyframe's easing.
 */
export function sampleKeyframes(
  keyframes: VideoTimelineKeyframe[] | undefined,
  property: KeyframeProperty,
  clipTimeMs: number,
): number | null {
  const points = (keyframes || [])
    .filter((keyframe) => keyframe.property === property)
    .sort((a, b) => a.time_ms - b.time_ms);
  if (points.length === 0) return null;
  if (clipTimeMs <= points[0].time_ms) return points[0].value;
  for (let i = 1; i < points.length; i += 1) {
    if (clipTimeMs <= points[i].time_ms) {
      const prev = points[i - 1];
      const next = points[i];
      const span = next.time_ms - prev.time_ms;
      const t = span <= 0 ? 1 : (clipTimeMs - prev.time_ms) / span;
      return prev.value + (next.value - prev.value) * applyEasing(t, next.easing, next.curve);
    }
  }
  return points[points.length - 1].value;
}
