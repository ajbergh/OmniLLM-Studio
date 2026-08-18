import { describe, it, expect } from 'vitest';
import {
  frameCountFromDuration,
  frameIndexToTimeMs,
  timeMsToFrameIndex,
  isFrameInHalfOpenInterval,
} from './timebase';
import { evaluateEasing, sampleKeyframeSequence } from './curves';
import { sortClipsForRender } from './ordering';

describe('timebase', () => {
  it('converts duration to frame count', () => {
    expect(frameCountFromDuration(1000, 30)).toBe(30);
    expect(frameCountFromDuration(20000, 30)).toBe(600);
  });

  it('determines half-open membership correctly', () => {
    expect(isFrameInHalfOpenInterval(0, 0, 1000, 30)).toBe(true);
    expect(isFrameInHalfOpenInterval(29, 0, 1000, 30)).toBe(true);
    expect(isFrameInHalfOpenInterval(30, 0, 1000, 30)).toBe(false);
  });
});

describe('curves', () => {
  it('evaluates piecewise quadratic ease-in-out', () => {
    expect(evaluateEasing(0, 'ease-in-out')).toBe(0);
    expect(evaluateEasing(0.5, 'ease-in-out')).toBe(0.5);
    expect(evaluateEasing(1, 'ease-in-out')).toBe(1);
    expect(evaluateEasing(0.25, 'ease-in-out')).toBe(0.125);
  });

  it('samples keyframes correctly', () => {
    const kfs = [
      { id: '1', property: 'volume', timeMs: 0, value: 0, easing: 'linear' as const },
      { id: '2', property: 'volume', timeMs: 1000, value: 2, easing: 'linear' as const },
    ];
    expect(sampleKeyframeSequence(kfs, 'volume', 500)).toBe(1);
  });
});

describe('ordering', () => {
  it('sorts by trackIndex then zIndex then clipIndex', () => {
    const clips = [
      { id: 'c3', trackIndex: 1, zIndex: 0, clipIndex: 0 },
      { id: 'c1', trackIndex: 0, zIndex: 0, clipIndex: 0 },
      { id: 'c2', trackIndex: 0, zIndex: 1, clipIndex: 0 },
    ];
    const sorted = sortClipsForRender(clips);
    expect(sorted.map((c) => c.id)).toEqual(['c1', 'c2', 'c3']);
  });
});
