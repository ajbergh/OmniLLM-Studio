import type { VideoTimelineKeyframe } from '../../../types/video';

export type KeyframeProperty = VideoTimelineKeyframe['property'];

export const KEYFRAME_PROPERTIES: KeyframeProperty[] = ['x', 'y', 'scale', 'rotation', 'opacity', 'volume'];

export const KEYFRAME_EASINGS = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'step'] as const;

function applyEasing(t: number, easing?: string): number {
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
      return prev.value + (next.value - prev.value) * applyEasing(t, next.easing);
    }
  }
  return points[points.length - 1].value;
}
