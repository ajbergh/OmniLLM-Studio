/**
 * Keyframe sampling shared by the preview canvas, timeline clip envelopes,
 * and the keyframe lane. Canonical interpolation lives in the renderer-
 * independent render contract; this module retains editor-facing property
 * lists and the small curve cache used by interactive UI callers.
 */
import type { VideoMotionCurve, VideoTimelineKeyframe } from '../../../types/video';
import { curveProgress } from '../../../video/renderContract';
import { samplePropertyKeyframes } from '../../../video/renderContractProperties';

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
 * Compatibility wrapper for editor callers. Sampling semantics are owned by
 * the canonical render contract: flat holds outside the authored range and the
 * LATER keyframe's curve/easing for each segment.
 */
export function sampleKeyframes(
  keyframes: VideoTimelineKeyframe[] | undefined,
  property: KeyframeProperty,
  clipTimeMs: number,
): number | null {
  return samplePropertyKeyframes(keyframes, property, clipTimeMs);
}
