import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument } from '../../types/video';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import {
  frameAddressMatchesTimelineMs,
  mediaSeekToleranceSeconds,
  sourceTimeForAddressMs,
  sourceTimeForPreviewMediaMs,
} from './sourceTiming';

describe('media source timing', () => {
  it('keeps free-running playback sub-frame responsive', () => {
    expect(sourceTimeForAddressMs(
      { kind: 'time', timelineMs: 1000.5 },
      500,
      100,
      1.25,
    )).toBeCloseTo(725.625, 9);
  });

  it('derives deterministic source time directly from output-frame identity', () => {
    expect(sourceTimeForAddressMs(
      { kind: 'frame', frameIndex: 1, fps: 120 },
      0,
      10,
      2,
    )).toBeCloseTo(26.6666666667, 9);
  });

  it('does not round a deterministic frame through integer milliseconds', () => {
    const frameAddressed = sourceTimeForAddressMs(
      { kind: 'frame', frameIndex: 1, fps: 120 },
      0,
      0,
      2,
    );
    const roundedMillisecond = sourceTimeForAddressMs(
      { kind: 'time', timelineMs: 8 },
      0,
      0,
      2,
    );

    expect(frameAddressed).toBeCloseTo(16.6666666667, 9);
    expect(roundedMillisecond).toBe(16);
    expect(frameAddressed).not.toBe(roundedMillisecond);
  });

  it('applies clip start, trim, and rate in the frame domain', () => {
    expect(sourceTimeForAddressMs(
      { kind: 'frame', frameIndex: 1, fps: 120 },
      5,
      40,
      1.5,
    )).toBeCloseTo(45, 9);
  });

  it('consumes the canonical source time projected by a transition-free Timeline v1 frame', () => {
    const document: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 100, height: 100, fps: 120, background: '#000000' },
      duration_ms: 100,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'track-1',
        type: 'layer',
        name: 'Layer 1',
        locked: false,
        muted: false,
        visible: true,
        clips: [{
          id: 'clip-1',
          start_ms: 5,
          duration_ms: 20,
          trim_in_ms: 40,
          trim_out_ms: 70,
          playback_rate: 1.5,
          transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
          effects: [],
          keyframes: [],
          transitions: [],
        }],
      }],
    };
    const entry = queryActiveClipsAtFrame(buildTimelineIntervalIndex(document, []), 1, 120)[0];

    expect(entry.canonicalState).toBeDefined();
    expect(entry.canonicalState?.source_time_ms).toBeCloseTo(45, 9);
    expect(sourceTimeForPreviewMediaMs(
      { kind: 'frame', frameIndex: 1, fps: 120 },
      entry.canonicalState,
      entry.clip.start_ms,
      entry.clip.trim_in_ms,
      entry.clip.playback_rate ?? 1,
    )).toBe(entry.canonicalState?.source_time_ms);
  });

  it('prefers canonical FrameState source time for deterministic visual media', () => {
    expect(sourceTimeForPreviewMediaMs(
      { kind: 'frame', frameIndex: 7, fps: 120 },
      { source_time_ms: 432.125 },
      1000,
      0,
      4,
    )).toBe(432.125);
  });

  it('preserves an exact canonical zero source time', () => {
    expect(sourceTimeForPreviewMediaMs(
      { kind: 'frame', frameIndex: 0, fps: 120 },
      { source_time_ms: 0 },
      1000,
      250,
      4,
    )).toBe(0);
  });

  it('keeps free-running visual media on timeline-time evaluation', () => {
    expect(sourceTimeForPreviewMediaMs(
      { kind: 'time', timelineMs: 1000.5 },
      { source_time_ms: 9999 },
      500,
      100,
      1.25,
    )).toBeCloseTo(725.625, 9);
  });

  it('falls back to frame-address evaluation when canonical projection is unavailable', () => {
    expect(sourceTimeForPreviewMediaMs(
      { kind: 'frame', frameIndex: 1, fps: 120 },
      undefined,
      0,
      10,
      2,
    )).toBeCloseTo(26.6666666667, 9);
  });

  it('recognizes the exact playhead generated from a frame address', () => {
    expect(frameAddressMatchesTimelineMs(1, 120, 1000 / 120)).toBe(true);
    expect(frameAddressMatchesTimelineMs(1, 120, 8)).toBe(false);
    expect(frameAddressMatchesTimelineMs(-1, 120, 0)).toBe(false);
  });

  it('uses a sub-millisecond deterministic seek tolerance', () => {
    expect(mediaSeekToleranceSeconds({ kind: 'frame', frameIndex: 1, fps: 120 })).toBeLessThan(1 / 120);
    expect(mediaSeekToleranceSeconds({ kind: 'time', timelineMs: 8 })).toBe(0.05);
  });
});
