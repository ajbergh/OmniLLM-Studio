import { describe, expect, it } from 'vitest';
import { evaluateVisualFrameState } from '../src/video/renderContractFrameState';
import { SHAPE_STATE_CONTRACT_V1 } from '../src/video/renderContractShape';
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

describe('canonical shape FrameState projection', () => {
  it('projects shape-state-v1, derives bounds from it, and clears generic shape debt', () => {
    const document = baseDocument();
    document.tracks = [{
      id: 'shape-track', type: 'layer', name: 'Shape', locked: false, muted: false, visible: true,
      clips: [{
        id: 'callout', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        shape: { kind: 'label', width: 240, height: 120 },
        effects: [], keyframes: [],
      }],
    }];
    const state = evaluateVisualFrameState(document, 0);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].shape?.contract_version).toBe(SHAPE_STATE_CONTRACT_V1);
    expect(state.layers[0].shape).toMatchObject({ kind: 'label', width: 240, height: 120, stroke: '' });
    expect(state.layers[0].content_bounds).toEqual({ x: 0, y: 0, width: 240, height: 120 });
    expect(state.layers[0].unresolved).toEqual([]);
    expect(state.layers[0].authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.authoritative).toBe(true);
  });

  it('leaves only cursor debt when a shape and cursor share the layer', () => {
    const document = baseDocument();
    document.tracks = [{
      id: 'track', type: 'layer', name: 'Layer', locked: false, muted: false, visible: true,
      clips: [{
        id: 'mixed', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        shape: { kind: 'rectangle' },
        cursor: { visible: true },
        effects: [], keyframes: [],
      }],
    }];
    const state = evaluateVisualFrameState(document, 0);
    expect(state.layers[0].shape?.kind).toBe('rectangle');
    expect(state.unresolved).toEqual(['mixed:cursor']);
  });

  it('fails closed when invalid shape dimensions reach FrameState', () => {
    const document = baseDocument();
    document.tracks = [{
      id: 'track', type: 'layer', name: 'Layer', locked: false, muted: false, visible: true,
      clips: [{
        id: 'bad-shape', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        shape: { kind: 'rectangle', width: 0 },
        effects: [], keyframes: [],
      }],
    }];
    expect(() => evaluateVisualFrameState(document, 0)).toThrow(/shape width/);
  });
});
