import { describe, expect, it } from 'vitest';
import { applyMotionCurve, sampleKeyframes } from './keyframeUtils';

describe('motion curve sampling', () => {
  it('samples Bezier and segment-local spring curves deterministically', () => {
    const bezier = { type: 'bezier' as const, x1: 0.25, y1: 0.1, x2: 0.25, y2: 1 };
    const spring = { type: 'spring' as const, stiffness: 170, damping: 18, mass: 1 };
    expect(applyMotionCurve(0.4, bezier)).toBeCloseTo(applyMotionCurve(0.4, bezier), 12);
    expect(applyMotionCurve(0, spring)).toBeCloseTo(0, 10);
    expect(applyMotionCurve(1, spring)).toBeCloseTo(1, 10);
  });

  it('lets curve win while keeping easing as fallback', () => {
    const value = sampleKeyframes([
      { id: 'a', property: 'x', time_ms: 0, value: 0, easing: 'linear' },
      { id: 'b', property: 'x', time_ms: 1000, value: 100, easing: 'linear', curve: { type: 'bezier', x1: 0.42, y1: 0, x2: 1, y2: 1 } },
    ], 'x', 500);
    expect(value).not.toBe(50);
  });
});
