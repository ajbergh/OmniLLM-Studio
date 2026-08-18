import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  activeClipsAtFrame,
  frameRangeContains,
  frameRangeFromMs,
  sourceTimeAtFrameMs,
  type CanonicalActiveClip,
  type CanonicalFrameRange,
} from '../src/video/renderContractEvaluation';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

interface ActiveClipFixture {
  version: number;
  timeline: TimelineV2Document;
  active_cases: Array<{ frame_index: number; expected: CanonicalActiveClip[] }>;
  range_cases: Array<{ start_ms: number; end_ms: number; fps: number; expected: CanonicalFrameRange }>;
  source_cases: Array<{
    frame_index: number;
    fps: number;
    clip_start_ms: number;
    trim_in_ms: number;
    playback_rate: number;
    expected_ms: number;
  }>;
}

function loadFixture(): ActiveClipFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/active-clips-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as ActiveClipFixture;
}

describe('canonical active clip evaluation', () => {
  const fixture = loadFixture();

  it('uses the versioned cross-runtime fixture', () => {
    expect(fixture.version).toBe(1);
  });

  for (const sample of fixture.active_cases) {
    it(`matches active ordering at frame ${sample.frame_index}`, () => {
      expect(activeClipsAtFrame(fixture.timeline, sample.frame_index)).toEqual(sample.expected);
    });
  }

  for (const sample of fixture.range_cases) {
    it(`maps ${sample.start_ms}-${sample.end_ms}ms at ${sample.fps}fps`, () => {
      const range = frameRangeFromMs(sample.start_ms, sample.end_ms, sample.fps);
      expect(range).toEqual(sample.expected);
      if (range.end_frame > range.start_frame) {
        expect(frameRangeContains(range, range.start_frame)).toBe(true);
        expect(frameRangeContains(range, range.end_frame)).toBe(false);
      }
    });
  }

  for (const sample of fixture.source_cases) {
    it(`maps source time at frame ${sample.frame_index}`, () => {
      expect(sourceTimeAtFrameMs(
        sample.frame_index,
        sample.fps,
        sample.clip_start_ms,
        sample.trim_in_ms,
        sample.playback_rate,
      )).toBeCloseTo(sample.expected_ms, 12);
    });
  }
});
