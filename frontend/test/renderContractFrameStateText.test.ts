import { describe, expect, it } from 'vitest';
import { evaluateVisualFrameState } from '../src/video/renderContractFrameState';
import { TEXT_STATE_CONTRACT_V1 } from '../src/video/renderContractText';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

function baseDocument(): TimelineV2Document {
  return {
    version: 2,
    canvas: { width: 640, height: 360, fps: 30, background: '#000000' },
    duration_ms: 1000,
    metadata: {},
    markers: [],
    scenes: [],
    tracks: [],
  };
}

describe('canonical text FrameState projection', () => {
  it('projects text-state-v1 and clears generic text debt', () => {
    const document = baseDocument();
    document.tracks = [{
      id: 'text-track', type: 'text', name: 'Text', locked: false, muted: false, visible: true,
      clips: [{
        id: 'title', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        text: { text: 'Canonical', background: '#111111', box_width: 320, box_height: 90 },
        effects: [], keyframes: [],
      }],
    }];
    const state = evaluateVisualFrameState(document, 0);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].text?.contract_version).toBe(TEXT_STATE_CONTRACT_V1);
    expect(state.layers[0].text?.text).toBe('Canonical');
    expect(state.layers[0].content_bounds).toEqual({ x: 0, y: 0, width: 320, height: 90 });
    expect(state.layers[0].unresolved).toEqual([]);
    expect(state.layers[0].authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.authoritative).toBe(true);
  });

  it('leaves shape and cursor debt untouched', () => {
    const document = baseDocument();
    document.tracks = [{
      id: 'track', type: 'layer', name: 'Layer', locked: false, muted: false, visible: true,
      clips: [{
        id: 'mixed', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        text: { text: 'Text' },
        shape: { kind: 'rectangle' },
        cursor: { visible: true },
        effects: [], keyframes: [],
      }],
    }];
    const state = evaluateVisualFrameState(document, 0);
    expect(state.unresolved).toEqual(['mixed:cursor', 'mixed:shape']);
  });
});
