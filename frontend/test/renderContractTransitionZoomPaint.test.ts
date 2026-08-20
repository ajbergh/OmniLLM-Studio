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

function expectCanonicalPaintClose(actual: CanonicalTransitionPaint | undefined, expected: CanonicalTransitionPaint, name: string) {
  expect(actual, name).toBeDefined();
  if (!actual) return;

  for (const [key, expectedValue] of Object.entries(expected)) {
    const actualValue = (actual as unknown as Record<string, unknown>)[key];
    if (typeof expectedValue === 'number') {
      expect(typeof actualValue, `${name}:${key}`).toBe('number');
      expect(actualValue as number, `${name}:${key}`).toBeCloseTo(expectedValue, 12);
      continue;
    }
    expect(actualValue, `${name}:${key}`).toEqual(expectedValue);
  }
  expect(Object.keys(actual).sort(), `${name}:keys`).toEqual(Object.keys(expected).sort());
}

describe('canonical zoom transition paint', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const paint = evaluateTransitionPaint(sample.owner_clip_id, sample.state);
      expect(paint?.contract_version, sample.name).toBe('transition-paint-v1');
      expectCanonicalPaintClose(paint, sample.expected, sample.name);
    }
  });
});
