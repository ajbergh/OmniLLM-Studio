import { describe, expect, it } from 'vitest';
import fixture from '../../video-renderer/test/fixtures/shape-state-v1.json';
import { evaluateShapeState } from '../src/video/renderContractShape';
import type { TimelineV2Shape } from '../src/video/renderContractTypes';

describe('shape-state-v1', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    for (const testCase of fixture.cases) {
      expect(evaluateShapeState(testCase.input as TimelineV2Shape), testCase.name).toEqual(testCase.expected);
    }
  });

  it('fails closed for unsupported and invalid authored shape state', () => {
    expect(() => evaluateShapeState({ kind: 'triangle' } as TimelineV2Shape)).toThrow(/kind/);
    expect(() => evaluateShapeState({ kind: 'rectangle', width: 0 })).toThrow(/width/);
    expect(() => evaluateShapeState({ kind: 'rectangle', stroke_width: -1 })).toThrow(/stroke_width/);
    expect(() => evaluateShapeState({ kind: 'blur', blur_radius: Number.NaN })).toThrow(/blur_radius/);
  });
});
