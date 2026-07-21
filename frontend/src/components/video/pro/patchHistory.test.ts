import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument, VideoTimelineTransform } from '../../../types/video';
import {
  PatchHistory,
  applyTimelinePatch,
  createTimelinePatch,
  revertTimelinePatch,
} from './patchHistory';

const transform = (): VideoTimelineTransform => ({
  x: 0,
  y: 0,
  scale: 1,
  rotation: 0,
  opacity: 1,
});

function documentAt(startMs: number): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 100, height: 100, fps: 30, background: '#000' },
    duration_ms: 1000,
    markers: [],
    metadata: {},
    tracks: [{
      id: 'track',
      type: 'layer',
      name: 'Layer',
      locked: false,
      muted: false,
      visible: true,
      clips: [{
        id: 'clip',
        start_ms: startMs,
        duration_ms: 100,
        trim_in_ms: 0,
        trim_out_ms: 100,
        transform: transform(),
        effects: [],
        keyframes: [],
        transitions: [],
      }],
    }],
  };
}

describe('patch history', () => {
  it('round trips timeline changes', () => {
    const before = documentAt(0);
    const after = documentAt(250);
    const patch = createTimelinePatch(before, after);

    expect(applyTimelinePatch(before, patch).tracks[0].clips[0].start_ms).toBe(250);
    expect(revertTimelinePatch(after, patch).tracks[0].clips[0].start_ms).toBe(0);
  });

  it('represents structural array edits safely', () => {
    const before = documentAt(0);
    const after = structuredClone(before);
    after.tracks[0].clips.push({
      ...structuredClone(after.tracks[0].clips[0]),
      id: 'clip-2',
      start_ms: 300,
    });
    const patch = createTimelinePatch(before, after);

    expect(applyTimelinePatch(before, patch).tracks[0].clips).toHaveLength(2);
    expect(revertTimelinePatch(after, patch).tracks[0].clips).toHaveLength(1);
  });

  it('evicts old patches to respect the byte budget', () => {
    const history = new PatchHistory(1);
    history.record(createTimelinePatch(documentAt(0), documentAt(100)));
    history.record(createTimelinePatch(documentAt(100), documentAt(200)));
    expect(history.undo).toHaveLength(1);
  });
});
