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

describe('visual FrameState wipe transition paint consumption', () => {
  it('resolves a left-edge wipe into normalized layer clipping', () => {
    const document = structuredClone(fixture.document);
    const transition = document.tracks[0].clips[0].transitions![0];
    transition.type = 'wipe';
    transition.direction = 'left';

    const state = evaluateVisualFrameState(document, 15);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(owner?.transition_paint).toEqual([
      expect.objectContaining({
        transition_id: 'fade-in',
        composition: 'owner-wipe',
        clip_space: 'layer-fraction',
        owner_clip_top: 0,
        owner_clip_right: 0.5,
        owner_clip_bottom: 0,
        owner_clip_left: 0,
      }),
    ]);
  });
});
