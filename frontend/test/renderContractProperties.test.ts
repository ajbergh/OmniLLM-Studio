import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateCameraProperty,
  evaluateClipProperty,
  samplePropertyKeyframes,
  type CanonicalPropertyKeyframe,
} from '../src/video/renderContractProperties';
import type { TimelineV2Camera, TimelineV2Clip } from '../src/video/renderContractTypes';

interface PropertyEvaluationFixture {
  version: number;
  keyframe_cases: Array<{
    name: string;
    property: string;
    time_ms: number;
    keyframes: CanonicalPropertyKeyframe[];
    expected?: number;
    found?: boolean;
  }>;
  clip_cases: Array<{
    name: string;
    property: string;
    time_ms: number;
    clip: Pick<TimelineV2Clip, 'transform' | 'volume' | 'keyframes'>;
    expected: number;
  }>;
  camera_cases: Array<{
    name: string;
    property: string;
    time_ms: number;
    camera: TimelineV2Camera;
    expected: number;
  }>;
  unsupported_cases: Array<{ scope: 'clip' | 'camera'; property: string }>;
}

function loadFixture(): PropertyEvaluationFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/property-evaluation-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as PropertyEvaluationFixture;
}

describe('canonical numeric property evaluation', () => {
  const fixture = loadFixture();

  it('uses the versioned cross-runtime fixture', () => {
    expect(fixture.version).toBe(1);
  });

  for (const sample of fixture.keyframe_cases) {
    it(`samples ${sample.name}`, () => {
      const value = samplePropertyKeyframes(sample.keyframes, sample.property, sample.time_ms);
      if (sample.found === false) {
        expect(value).toBeNull();
      } else {
        expect(value).toBeCloseTo(sample.expected as number, 9);
      }
    });
  }

  for (const sample of fixture.clip_cases) {
    it(`evaluates clip base/keyframes: ${sample.name}`, () => {
      expect(evaluateClipProperty(sample.clip, sample.property, sample.time_ms)).toBeCloseTo(sample.expected, 9);
    });
  }

  for (const sample of fixture.camera_cases) {
    it(`evaluates camera base/keyframes: ${sample.name}`, () => {
      expect(evaluateCameraProperty(sample.camera, sample.property, sample.time_ms)).toBeCloseTo(sample.expected, 9);
    });
  }

  for (const sample of fixture.unsupported_cases) {
    it(`fails closed for unsupported ${sample.scope} property ${sample.property}`, () => {
      if (sample.scope === 'clip') {
        expect(() => evaluateClipProperty({ keyframes: [] }, sample.property, 0)).toThrow(/unsupported canonical clip property/);
      } else {
        expect(() => evaluateCameraProperty({}, sample.property, 0)).toThrow(/unsupported canonical camera property/);
      }
    });
  }
});
