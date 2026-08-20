import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateTransitionPaint,
  TRANSITION_PAINT_CONTRACT_V1,
  type CanonicalTransitionPaint,
} from '../src/video/renderContractTransitionPaint';
import type { CanonicalTransitionState } from '../src/video/renderContractTransitions';

interface TransitionPaintFixture {
  version: number;
  cases: Array<{
    name: string;
    owner_clip_id: string;
    state: CanonicalTransitionState;
    expected: CanonicalTransitionPaint | null;
  }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-paint-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as TransitionPaintFixture;

describe('canonical transition paint', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const paint = evaluateTransitionPaint(sample.owner_clip_id, sample.state);
      if (sample.expected === null) {
        expect(paint, sample.name).toBeUndefined();
      } else {
        expect(paint?.contract_version).toBe(TRANSITION_PAINT_CONTRACT_V1);
        expect(paint, sample.name).toEqual(sample.expected);
      }
    }
  });

  it('fails closed when crossfade is not a between transition', () => {
    const state: CanonicalTransitionState = {
      contract_version: 'transition-state-v1',
      id: 'crossfade',
      type: 'crossfade',
      placement: 'in',
      role: 'incoming',
      start_frame: 0,
      end_frame: 10,
      progress: 0.5,
      active: true,
    };
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/crossfade requires between/);
  });

  it('fails closed when between roles are not complementary', () => {
    const state: CanonicalTransitionState = {
      contract_version: 'transition-state-v1',
      id: 'crossfade',
      type: 'crossfade',
      placement: 'between',
      peer_clip_id: 'peer',
      role: 'outgoing',
      peer_role: 'outgoing',
      start_frame: 0,
      end_frame: 10,
      progress: 0.5,
      active: true,
    };
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/complementary/);
  });

  it('fails closed for transition families without canonical paint', () => {
    const state: CanonicalTransitionState = {
      contract_version: 'transition-state-v1',
      id: 'zoom',
      type: 'zoom',
      placement: 'out',
      role: 'outgoing',
      start_frame: 0,
      end_frame: 10,
      progress: 0.5,
      active: true,
    };
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/does not yet have canonical paint semantics/);
  });

  it('fails closed for a non-canonical slide direction', () => {
    const state = {
      contract_version: 'transition-state-v1',
      id: 'slide',
      type: 'slide',
      placement: 'in',
      direction: 'diagonal',
      role: 'incoming',
      start_frame: 0,
      end_frame: 10,
      progress: 0.5,
      active: true,
    } as unknown as CanonicalTransitionState;
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/direction left, right, up, or down/);
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

  it('rejects non-canonical progress', () => {
    const state: CanonicalTransitionState = {
      contract_version: 'transition-state-v1',
      id: 'fade',
      type: 'fade',
      placement: 'in',
      role: 'incoming',
      start_frame: 0,
      end_frame: 10,
      progress: 1.1,
      active: true,
    };
    expect(() => evaluateTransitionPaint('owner', state)).toThrow(/within \[0,1\]/);
  });
});
