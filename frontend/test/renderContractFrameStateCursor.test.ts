import { describe, expect, it } from 'vitest';
import { CURSOR_STATE_CONTRACT_V1 } from '../src/video/renderContractCursor';
import { evaluateVisualFrameState } from '../src/video/renderContractFrameState';
import type { TimelineV2Cursor, TimelineV2Document } from '../src/video/renderContractTypes';

function cursorFrameStateDocument(cursor: TimelineV2Cursor): TimelineV2Document {
  return {
    version: 2,
    canvas: { width: 640, height: 360, fps: 120, background: '#000000' },
    duration_ms: 1000,
    metadata: {},
    markers: [],
    scenes: [],
    tracks: [{
      id: 'cursor-track', type: 'layer', name: 'Cursor', locked: false, muted: false, visible: true,
      clips: [{
        id: 'cursor-clip', start_ms: 5, duration_ms: 500, trim_in_ms: 0, trim_out_ms: 500,
        cursor, effects: [], keyframes: [],
      }],
    }],
  };
}

describe('canonical cursor FrameState projection', () => {
  it('projects cursor-state-v1 at exact rational clip-relative time and clears cursor debt', () => {
    const state = evaluateVisualFrameState(cursorFrameStateDocument({
      scale: 1.5,
      highlight: true,
      click_rings: true,
      events: [
        { time_ms: 0, x: 10, y: 20 },
        { time_ms: 10, x: 40, y: 50, click: true },
      ],
    }), 1);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].cursor).toEqual({
      contract_version: CURSOR_STATE_CONTRACT_V1,
      visible: true,
      scale: 1.5,
      highlight: true,
      click_rings: true,
      x: 20,
      y: 30,
      click: true,
    });
    expect(state.layers[0].unresolved).toEqual([]);
    expect(state.layers[0].authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.authoritative).toBe(true);
  });

  it.each([
    ['hidden', { visible: false, events: [{ time_ms: 0, x: 1, y: 2 }] }],
    ['empty', {}],
  ] as const)('treats %s cursor as resolved no-paint state', (_name, cursor) => {
    const state = evaluateVisualFrameState(cursorFrameStateDocument(cursor), 1);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].cursor).toBeUndefined();
    expect(state.layers[0].unresolved).toEqual([]);
    expect(state.layers[0].authoritative).toBe(true);
    expect(state.authoritative).toBe(true);
  });

  it('fails closed when invalid cursor authoring reaches FrameState', () => {
    expect(() => evaluateVisualFrameState(cursorFrameStateDocument({
      smoothing: true,
      events: [{ time_ms: 0, x: 1, y: 2 }],
    }), 1)).toThrow(/smoothing/);
  });
});
