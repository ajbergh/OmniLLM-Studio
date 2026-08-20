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

describe('visual FrameState zoom transition paint consumption', () => {
  it('resolves the continuous zoom envelope and restores authority', () => {
    const document = structuredClone(fixture.document);
    document.tracks[0].clips[0].transitions![0].type = 'zoom';

    const state = evaluateVisualFrameState(document, 15);
    const owner = state.layers.find((layer) => layer.clip_id === 'owner');
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(owner?.transition_paint).toEqual([
      expect.objectContaining({
        transition_id: 'fade-in',
        composition: 'owner-zoom',
        scale_space: 'layer-multiplier',
        owner_opacity: 0.5,
        owner_scale: 0.955,
      }),
    ]);
  });
});
