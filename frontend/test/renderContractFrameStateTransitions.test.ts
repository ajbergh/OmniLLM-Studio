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

function ownerLayer(frameIndex: number) {
  return evaluateVisualFrameState(fixture.document, frameIndex).layers.find((layer) => layer.clip_id === 'owner');
}

describe('visual FrameState transition integration', () => {
  it('keeps transition timing state without making inactive transition frames unresolved', () => {
    const state = evaluateVisualFrameState(fixture.document, 30);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.layers).toHaveLength(1);
    expect(owner?.authoritative).toBe(true);
    expect(owner?.transitions).toHaveLength(3);
    expect(owner?.transitions?.filter((transition) => transition.active)).toEqual([]);
  });

  it.each([
    [15, 'fade-in', 1],
    [55, 'between', 2],
    [65, 'slide-out', 2],
  ] as const)('keeps active transition timing authoritative at frame %i', (frameIndex, activeID, layerCount) => {
    const state = evaluateVisualFrameState(fixture.document, frameIndex);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.layers).toHaveLength(layerCount);
    expect(owner?.authoritative).toBe(true);
    expect(owner?.transitions).toHaveLength(3);
    expect(owner?.transitions?.filter((transition) => transition.active).map((transition) => transition.id)).toEqual([activeID]);
    expect(owner?.transition_paint).toHaveLength(1);
    expect(owner?.transition_paint?.[0].transition_id).toBe(activeID);
  });

  it('honors the owner exclusive end frame and leaves the peer authoritative', () => {
    const state = evaluateVisualFrameState(fixture.document, 70);
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].clip_id).toBe('peer');
    expect(ownerLayer(70)).toBeUndefined();
  });
});
