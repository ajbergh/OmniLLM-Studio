import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateTransitionPaint, type CanonicalTransitionPaint } from '../src/video/renderContractTransitionPaint';
import type { CanonicalTransitionState } from '../src/video/renderContractTransitions';

interface TransitionZoomPaintFixture {
  version: number;
  cases: Array<{
    name: string;
    owner_clip_id: string;
    state: CanonicalTransitionState;
    expected: CanonicalTransitionPaint;
  }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-zoom-paint-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as TransitionZoomPaintFixture;

describe('canonical zoom transition paint', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const paint = evaluateTransitionPaint(sample.owner_clip_id, sample.state);
      expect(paint?.contract_version, sample.name).toBe('transition-paint-v1');
      expect(paint, sample.name).toEqual(sample.expected);
    }
  });
});
