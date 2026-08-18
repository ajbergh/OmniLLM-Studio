import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { activeAtFrame, easeProgress, endFrame, frameTime, sourceTimeMs, startFrame } from './renderContract';

interface ContractFixture {
  version: number;
  easing: Array<{ name: string; t: number; want: number }>;
  frame_ranges: Array<{
    start_ms: number;
    duration_ms: number;
    fps: number;
    start_frame: number;
    end_frame: number;
  }>;
  source_times: Array<{
    timeline_ms: number;
    clip_start_ms: number;
    trim_in_ms: number;
    playback_rate: number;
    want_ms: number;
  }>;
}

function loadFixture(): ContractFixture {
  const path = resolve(process.cwd(), '..', 'video-renderer', 'test', 'fixtures', 'render-contract-v1.json');
  return JSON.parse(readFileSync(path, 'utf8')) as ContractFixture;
}

describe('canonical render contract', () => {
  const fixture = loadFixture();

  it('uses the versioned cross-runtime fixture', () => {
    expect(fixture.version).toBe(1);
  });

  it('matches canonical easing samples', () => {
    for (const sample of fixture.easing) {
      expect(easeProgress(sample.t, sample.name)).toBeCloseTo(sample.want, 12);
    }
  });

  it('matches canonical half-open frame boundaries', () => {
    for (const sample of fixture.frame_ranges) {
      expect(startFrame(sample.start_ms, sample.fps)).toBe(sample.start_frame);
      expect(endFrame(sample.start_ms + sample.duration_ms, sample.fps)).toBe(sample.end_frame);
      if (sample.start_frame < sample.end_frame) {
        expect(activeAtFrame(sample.start_frame, sample.start_ms, sample.duration_ms, sample.fps)).toBe(true);
        expect(activeAtFrame(sample.end_frame, sample.start_ms, sample.duration_ms, sample.fps)).toBe(false);
      }
    }
  });

  it('matches canonical source-time mapping', () => {
    for (const sample of fixture.source_times) {
      expect(sourceTimeMs(
        sample.timeline_ms,
        sample.clip_start_ms,
        sample.trim_in_ms,
        sample.playback_rate,
      )).toBeCloseTo(sample.want_ms, 12);
    }
  });

  it('keeps frame time rational', () => {
    expect(frameTime(1_000_001, 120)).toEqual({ numerator: 1_000_001, denominator: 120 });
  });
});
