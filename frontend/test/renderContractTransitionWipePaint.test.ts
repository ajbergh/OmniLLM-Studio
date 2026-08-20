import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateTransitionPaint, type CanonicalTransitionPaint } from '../src/video/renderContractTransitionPaint';
import type { CanonicalTransitionState } from '../src/video/renderContractTransitions';

interface TransitionWipePaintFixture {
  version: number;
  cases: Array<{
    name: string;
    owner_clip_id: string;
    state: CanonicalTransitionState;
    expected: CanonicalTransitionPaint;
  }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-wipe-paint-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as TransitionWipePaintFixture;

describe('canonical wipe transition paint', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      expect(evaluateTransitionPaint(sample.owner_clip_id, sample.state), sample.name).toEqual(sample.expected);
    }
  });

  it('fails closed for a non-canonical wipe direction', () => {
    const state = {
      contract_version: 'transition-state-v1',
      id: 'wipe',
      type: 'wipe',
      placement: 'in',
      direction: 'diagonal',
      role: 'incoming',
      start_frame: 0,
      end_frame: 10,
      progress: 0.5,
      active: true,
    } as unknown as CanonicalTransitionState;
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/wipe requires direction left, right, up, or down/);
  });
});
