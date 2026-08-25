import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument } from '../../../types/video';
import {
  clampTransitionDurationToPeer,
  transitionPeerOptions,
  transitionPlacementSupported,
  visualTransitionOwnerEligible,
} from './transitionAuthoring';

function timeline(): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 640, height: 360, fps: 30, background: '#000000' },
    duration_ms: 2000,
    tracks: [
      {
        id: 'layer-1', type: 'layer', name: 'Layer 1', locked: false, muted: false, visible: true,
        clips: [
          { id: 'owner', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000, effects: [], keyframes: [] },
          { id: 'peer', start_ms: 800, duration_ms: 600, trim_in_ms: 0, trim_out_ms: 600, effects: [], keyframes: [] },
          { id: 'same-start', start_ms: 0, duration_ms: 500, trim_in_ms: 0, trim_out_ms: 500, effects: [], keyframes: [] },
          { id: 'late', start_ms: 1200, duration_ms: 400, trim_in_ms: 0, trim_out_ms: 400, effects: [], keyframes: [] },
        ],
      },
      {
        id: 'audio-1', type: 'audio', name: 'Audio', locked: false, muted: false, visible: true,
        clips: [{ id: 'audio-peer', start_ms: 900, duration_ms: 500, trim_in_ms: 0, trim_out_ms: 500, effects: [], keyframes: [] }],
      },
      {
        id: 'hidden-1', type: 'layer', name: 'Hidden', locked: false, muted: false, visible: false,
        clips: [{ id: 'hidden-peer', start_ms: 850, duration_ms: 500, trim_in_ms: 0, trim_out_ms: 500, effects: [], keyframes: [] }],
      },
    ],
    markers: [], metadata: {},
  };
}

describe('transition authoring peers', () => {
  it('offers only distinct-start real overlapping visible visual peers with exact overlap', () => {
    expect(transitionPeerOptions(timeline(), 'owner')).toEqual([
      { id: 'peer', label: 'Layer 1 · peer · 200ms overlap', overlap_ms: 200, track_index: 0, clip_index: 1 },
    ]);
  });

  it('rejects audio owners for visual transition authoring', () => {
    expect(visualTransitionOwnerEligible(timeline(), 'owner')).toBe(true);
    expect(visualTransitionOwnerEligible(timeline(), 'audio-peer')).toBe(false);
    expect(transitionPeerOptions(timeline(), 'audio-peer')).toEqual([]);
  });

  it('matches transition-paint-v1 placement/type compatibility', () => {
    expect(transitionPlacementSupported('fade', 'in')).toBe(true);
    expect(transitionPlacementSupported('fade', 'between')).toBe(false);
    expect(transitionPlacementSupported('crossfade', 'between')).toBe(true);
    expect(transitionPlacementSupported('crossfade', 'out')).toBe(false);
    expect(transitionPlacementSupported('slide', 'between')).toBe(true);
  });

  it('clamps pair duration to authored overlap and the editor minimum', () => {
    const peer = transitionPeerOptions(timeline(), 'owner')[0];
    expect(clampTransitionDurationToPeer(500, peer)).toBe(200);
    expect(clampTransitionDurationToPeer(50, peer)).toBe(100);
  });
});
