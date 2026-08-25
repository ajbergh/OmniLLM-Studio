import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateTransitionPairSurfacePlan,
  TRANSITION_PAIR_SURFACE_PLAN_V1,
  type CanonicalTransitionPairSurfacePlan,
} from '../src/video/renderContractTransitionPairSurfaces';
import type { CanonicalVisualFrameState } from '../src/video/renderContractFrameState';

interface PairSurfaceFixture {
  version: number;
  cases: Array<{
    name: string;
    state: CanonicalVisualFrameState;
    expected: CanonicalTransitionPairSurfacePlan;
  }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-pair-surface-plan-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as PairSurfaceFixture;

describe('canonical transition pair surface plan', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const plan = evaluateTransitionPairSurfacePlan(sample.state);
      expect(plan.contract_version, sample.name).toBe(TRANSITION_PAIR_SURFACE_PLAN_V1);
      expect(plan, sample.name).toEqual(sample.expected);
    }
  });

  it('fails closed when the pair paint owner does not match the containing layer', () => {
    const state = {
      contract_version: 'visual-frame-state-v1',
      frame_index: 5,
      authoritative: true,
      layers: [
        {
          clip_id: 'owner',
          authoritative: true,
          transition_paint: [{
            contract_version: 'transition-paint-v1',
            transition_id: 'crossfade',
            type: 'crossfade',
            placement: 'between',
            composition: 'pair-crossfade',
            owner_clip_id: 'wrong-owner',
            peer_clip_id: 'peer',
            progress: 0.5,
            outgoing_clip_id: 'owner',
            incoming_clip_id: 'peer',
          }],
        },
        { clip_id: 'peer', authoritative: true },
      ],
    } as unknown as CanonicalVisualFrameState;
    expect(() => evaluateTransitionPairSurfacePlan(state)).toThrow(/owner clip id must match containing layer/);
  });

  it('fails closed when one clip is claimed by multiple resolved pair surfaces', () => {
    const paint = (transitionId: string, owner: string, peer: string, outgoing: string, incoming: string) => ({
      contract_version: 'transition-paint-v1' as const,
      transition_id: transitionId,
      type: 'crossfade',
      placement: 'between',
      composition: 'pair-crossfade',
      owner_clip_id: owner,
      peer_clip_id: peer,
      progress: 0.5,
      outgoing_clip_id: outgoing,
      incoming_clip_id: incoming,
    });
    const state = {
      contract_version: 'visual-frame-state-v1',
      frame_index: 6,
      authoritative: true,
      layers: [
        { clip_id: 'a', authoritative: true, transition_paint: [paint('ab', 'a', 'b', 'a', 'b')] },
        { clip_id: 'b', authoritative: true, transition_paint: [paint('bc', 'b', 'c', 'b', 'c')] },
        { clip_id: 'c', authoritative: true },
      ],
    } as unknown as CanonicalVisualFrameState;
    expect(() => evaluateTransitionPairSurfacePlan(state)).toThrow(/multiple pair surfaces/);
  });
});
