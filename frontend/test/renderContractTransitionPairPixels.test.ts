import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateTransitionPairPixelComposition,
  TRANSITION_PAIR_PIXEL_COMPOSITION_V1,
  type CanonicalTransitionPairPixelComposition,
} from '../src/video/renderContractTransitionPairPixels';
import type { CanonicalTransitionPaint } from '../src/video/renderContractTransitionPaint';
import type { CanonicalTransitionPairSurface } from '../src/video/renderContractTransitionPairSurfaces';

interface PairPixelFixture {
  version: number;
  cases: Array<{
    name: string;
    surface: CanonicalTransitionPairSurface;
    paint: CanonicalTransitionPaint;
    expected: CanonicalTransitionPairPixelComposition;
  }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-pair-pixel-composition-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as PairPixelFixture;

describe('canonical transition pair pixel composition', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const composition = evaluateTransitionPairPixelComposition(sample.surface, sample.paint);
      expect(composition.contract_version, sample.name).toBe(TRANSITION_PAIR_PIXEL_COMPOSITION_V1);
      expect(composition, sample.name).toEqual(sample.expected);
    }
  });

  it('fails closed when canonical stack slots are not the transition pair', () => {
    const { surface, paint } = crossfadeSample();
    surface.upper_clip_id = 'unrelated';
    expect(() => evaluateTransitionPairPixelComposition(surface, paint))
      .toThrow(/lower\/upper clips must be the outgoing\/incoming pair/);
  });

  it('fails closed when transition identity differs', () => {
    const { surface, paint } = crossfadeSample();
    paint.transition_id = 'different';
    expect(() => evaluateTransitionPairPixelComposition(surface, paint))
      .toThrow(/transition id must match surface/);
  });

  it('rejects weighted families whose contribution weights do not sum to one', () => {
    const { surface, paint } = crossfadeSample();
    paint.outgoing_weight = 0.8;
    paint.incoming_weight = 0.3;
    expect(() => evaluateTransitionPairPixelComposition(surface, paint))
      .toThrow(/pair weights must sum to 1/);
  });

  it('rejects transition weights on source-over slide/wipe composition', () => {
    const { surface, paint } = crossfadeSample();
    surface.composition = 'pair-slide';
    paint.composition = 'pair-slide';
    paint.outgoing_weight = 0.5;
    delete paint.incoming_weight;
    expect(() => evaluateTransitionPairPixelComposition(surface, paint))
      .toThrow(/must not carry pair weights/);
  });

  it('rejects unsupported pair composition', () => {
    const { surface, paint } = crossfadeSample();
    surface.composition = 'pair-unknown';
    (paint as CanonicalTransitionPaint & { composition: string }).composition = 'pair-unknown';
    delete paint.outgoing_weight;
    delete paint.incoming_weight;
    expect(() => evaluateTransitionPairPixelComposition(surface, paint))
      .toThrow(/does not have pair-pixel semantics/);
  });
});

function crossfadeSample(): {
  surface: CanonicalTransitionPairSurface;
  paint: CanonicalTransitionPaint;
} {
  return {
    surface: {
      transition_id: 'crossfade',
      composition: 'pair-crossfade',
      owner_clip_id: 'outgoing',
      peer_clip_id: 'incoming',
      outgoing_clip_id: 'outgoing',
      incoming_clip_id: 'incoming',
      lower_clip_id: 'outgoing',
      upper_clip_id: 'incoming',
      lower_layer_index: 0,
      upper_layer_index: 1,
      replacement_layer_index: 0,
    },
    paint: {
      contract_version: 'transition-paint-v1',
      transition_id: 'crossfade',
      type: 'crossfade',
      placement: 'between',
      composition: 'pair-crossfade',
      owner_clip_id: 'outgoing',
      peer_clip_id: 'incoming',
      progress: 0.25,
      outgoing_clip_id: 'outgoing',
      incoming_clip_id: 'incoming',
      outgoing_weight: 0.75,
      incoming_weight: 0.25,
    },
  };
}
