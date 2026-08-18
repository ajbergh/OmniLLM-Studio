/**
 * Keyframe sampling shared by the preview canvas, timeline clip envelopes,
 * and the keyframe lane. Built-in easing, cubic Bezier, and segment-local
 * spring semantics come from the renderer-independent render contract.
 */
import type { VideoMotionCurve, VideoTimelineKeyframe } from '../../../types/video';
import { curveProgress } from '../../../video/renderContract';

export type KeyframeProperty = VideoTimelineKeyframe['property'];

export const KEYFRAME_PROPERTIES: KeyframeProperty[] = [
  'x', 'y', 'z', 'scale', 'scale_x', 'scale_y',
  'rotation', 'rotation_x', 'rotation_y', 'rotation_z', 'opacity', 'volume',
];

export const KEYFRAME_EASINGS = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'step'] as const;

const curveSampleCache = new Map<string, number>();

export function applyMotionCurve(t: number, curve?: VideoMotionCurve, easing?: string): number {
  const normalized = Math.max(0, Math.min(1, t));
  const cacheKey = curve ? `${JSON.stringify(curve)}:${Math.round(normalized * 10000)}` : '';
  if (cacheKey) {
    const cached = curveSampleCache.get(cacheKey);
    if (cached !== undefined) return cached;
  }
  const value = curveProgress(normalized, curve, easing);
  if (cacheKey) {
    if (curveSampleCache.size >= 4096) curveSampleCache.clear();
    curveSampleCache.set(cacheKey, value);
  }
  return value;
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
      return prev.value + (next.value - prev.value) * applyMotionCurve(t, next.curve, next.easing);
    }
  }
  return points[points.length - 1].value;
}
