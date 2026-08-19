import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateVisualFrameState } from '../src/video/renderContractFrameState';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

interface TransitionFixture {
  version: number;
  document: TimelineV2Document;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-state-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as TransitionFixture;

function ownerLayer(document: TimelineV2Document, frameIndex: number) {
  return evaluateVisualFrameState(document, frameIndex).layers.find((layer) => layer.clip_id === 'owner');
}

describe('visual FrameState transition paint consumption', () => {
  it('resolves active fade-in paint and restores FrameState authority', () => {
    const state = evaluateVisualFrameState(fixture.document, 15);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(owner?.authoritative).toBe(true);
    expect(owner?.transition_paint).toEqual([
      expect.objectContaining({
        contract_version: 'transition-paint-v1',
        transition_id: 'fade-in',
        composition: 'owner-opacity',
        owner_opacity: 0.5,
      }),
    ]);
  });

  it('resolves true pair-crossfade weights at the exact sampled frame', () => {
    const state = evaluateVisualFrameState(fixture.document, 55);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.layers).toHaveLength(2);
    expect(owner?.transition_paint).toHaveLength(1);
    const paint = owner?.transition_paint?.[0];
    expect(paint?.composition).toBe('pair-crossfade');
    expect(paint?.outgoing_clip_id).toBe('owner');
    expect(paint?.incoming_clip_id).toBe('peer');
    expect(paint?.outgoing_weight).toBeCloseTo(2 / 3, 12);
    expect(paint?.incoming_weight).toBeCloseTo(1 / 3, 12);
  });

  it('retains unresolved debt for slide until slide paint is canonical', () => {
    const state = evaluateVisualFrameState(fixture.document, 65);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(false);
    expect(state.unresolved).toEqual(['owner:transition_paint:slide-out']);
    expect(owner?.transition_paint).toBeUndefined();
  });

  it('consumes dip-to-black paint when that supported family is authored', () => {
    const document = structuredClone(fixture.document);
    document.tracks[0].clips[0].transitions![0].type = 'dip_to_black';
    const state = evaluateVisualFrameState(document, 15);
    const owner = ownerLayer(document, 15);
    expect(state.authoritative).toBe(true);
    expect(owner?.transition_paint).toEqual([
      expect.objectContaining({
        composition: 'dip-to-black',
        incoming_clip_id: 'owner',
        incoming_weight: 0.5,
        black_weight: 0.5,
      }),
    ]);
  });

  it('fails closed for invalid state inside a supported paint family', () => {
    const document = structuredClone(fixture.document);
    document.tracks[0].clips[0].transitions![0].type = 'crossfade';
    expect(() => evaluateVisualFrameState(document, 15)).toThrow(/crossfade requires between/);
  });
});
