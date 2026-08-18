import { describe, it, expect } from 'vitest';
import {
  evaluateTransform,
  evaluateFrame,
} from './evaluateFrame';
import { adaptTimelineV1ToV2 } from './normalizeTimeline';
import { ContractTimelineClip, ContractTimelineDocument } from '../contract/timeline';

describe('evaluateTransform edge cases', () => {
  it('handles clip with keyframed spatial properties', () => {
    const clip: ContractTimelineClip = {
      id: 'clip-spatial',
      startMs: 0,
      durationMs: 2000,
      transform: { x: 0, y: 0, scaleX: 1, scaleY: 1, rotationZ: 0, opacity: 1, anchorX: 10, anchorY: -10 },
      keyframes: [
        { id: 'kf-x-0', property: 'x', timeMs: 0, value: 0, easing: 'linear' },
        { id: 'kf-x-1', property: 'x', timeMs: 1000, value: 200, easing: 'ease-in-out' },
        { id: 'kf-rot-0', property: 'rotation_z', timeMs: 0, value: 0, easing: 'linear' },
        { id: 'kf-rot-1', property: 'rotation_z', timeMs: 2000, value: 180, easing: 'linear' },
      ],
    };

    const t0 = evaluateTransform(clip, 0);
    expect(t0.x).toBe(0);
    expect(t0.rotationZ).toBe(0);
    expect(t0.anchorX).toBe(10);
    expect(t0.anchorY).toBe(-10);

    // At 500ms, ease-in-out at progress 0.5 should be 0.5 * 200 = 100
    const t500 = evaluateTransform(clip, 500);
    expect(t500.x).toBe(100);
    expect(t500.rotationZ).toBe(45);

    // Beyond keyframe range, should clamp to boundary
    const t1500 = evaluateTransform(clip, 1500);
    expect(t1500.x).toBe(200);
    expect(t1500.rotationZ).toBe(135);
  });

  it('correctly calculates fade in and fade out opacity', () => {
    const clip: ContractTimelineClip = {
      id: 'clip-fades',
      startMs: 0,
      durationMs: 1000,
      fadeInMs: 200,
      fadeOutMs: 300,
      transform: { opacity: 1 },
    };

    // At 0ms, opacity is 0
    expect(evaluateTransform(clip, 0).opacity).toBe(0);
    // At 100ms (halfway through fadeInMs=200), opacity is 0.5
    expect(evaluateTransform(clip, 100).opacity).toBeCloseTo(0.5, 5);
    // At 500ms (between fades), opacity is 1
    expect(evaluateTransform(clip, 500).opacity).toBe(1);
    // At 850ms (halfway through fadeOutMs=300, 150ms left of 300ms), opacity is 0.5
    expect(evaluateTransform(clip, 850).opacity).toBeCloseTo(0.5, 5);
    // At 1000ms, opacity is 0
    expect(evaluateTransform(clip, 1000).opacity).toBe(0);
  });
});

describe('evaluateFrame track solo and mute semantics', () => {
  it('respects track mute and track solo priority', () => {
    const doc: ContractTimelineDocument = {
      version: 2,
      width: 1920,
      height: 1080,
      fps: 30,
      durationMs: 2000,
      aspectRatio: '16:9',
      tracks: [
        {
          id: 'track-solo',
          type: 'video',
          solo: true,
          muted: false,
          visible: true,
          clips: [{ id: 'clip-solo', startMs: 0, durationMs: 2000 }],
        },
        {
          id: 'track-unsolo',
          type: 'video',
          solo: false,
          muted: false,
          visible: true,
          clips: [{ id: 'clip-ignored', startMs: 0, durationMs: 2000 }],
        },
      ],
    };

    const frame = evaluateFrame(doc, 0);
    expect(frame.activeClips.map((c) => c.id)).toEqual(['clip-solo']);
  });

  it('excludes clips when track is invisible or muted', () => {
    const doc: ContractTimelineDocument = {
      version: 2,
      width: 1920,
      height: 1080,
      fps: 30,
      durationMs: 2000,
      aspectRatio: '16:9',
      tracks: [
        {
          id: 'track-hidden',
          type: 'video',
          visible: false,
          clips: [{ id: 'clip-hidden', startMs: 0, durationMs: 2000 }],
        },
        {
          id: 'track-muted',
          type: 'video',
          muted: true,
          visible: true,
          clips: [{ id: 'clip-muted', startMs: 0, durationMs: 2000 }],
        },
      ],
    };

    const frame = evaluateFrame(doc, 0);
    expect(frame.activeClips).toHaveLength(0);
  });
});
