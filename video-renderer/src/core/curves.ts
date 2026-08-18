export type EasingType =
  | 'linear'
  | 'ease-in'
  | 'ease-out'
  | 'ease-in-out'
  | 'step-start'
  | 'step-end';

export function evaluateEasing(progress: number, easing: EasingType = 'linear'): number {
  const t = Math.max(0, Math.min(1, progress));
  switch (easing) {
    case 'linear':
      return t;
    case 'ease-in':
      return t * t;
    case 'ease-out':
      return t * (2 - t);
    case 'ease-in-out':
      // Canonical piecewise quadratic ease-in-out matching editor preview
      return t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t;
    case 'step-start':
      return t >= 0 ? 1 : 0;
    case 'step-end':
      return t >= 1 ? 1 : 0;
    default:
      return t;
  }
}

export interface RenderKeyframe {
  id: string;
  property: string;
  timeMs: number;
  value: number;
  easing?: EasingType;
}

export function sampleKeyframeSequence(
  keyframes: RenderKeyframe[],
  property: string,
  timeMs: number,
  fallbackValue = 0
): number {
  const matching = keyframes
    .filter((kf) => kf.property === property)
    .sort((a, b) => a.timeMs - b.timeMs);

  if (matching.length === 0) return fallbackValue;
  if (timeMs <= matching[0].timeMs) return matching[0].value;
  if (timeMs >= matching[matching.length - 1].timeMs) return matching[matching.length - 1].value;

  for (let i = 0; i < matching.length - 1; i++) {
    const kf0 = matching[i];
    const kf1 = matching[i + 1];
    if (timeMs >= kf0.timeMs && timeMs <= kf1.timeMs) {
      const segDuration = kf1.timeMs - kf0.timeMs;
      if (segDuration <= 0) return kf0.value;
      const progress = (timeMs - kf0.timeMs) / segDuration;
      const eased = evaluateEasing(progress, kf1.easing ?? 'linear');
      return kf0.value + (kf1.value - kf0.value) * eased;
    }
  }

  return fallbackValue;
}
