import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { TimelineV2Document } from '../src/video/renderContractTypes';
import {
  normalizeTimelineV2EvaluationInputs,
  TIMELINE_V2_RUNTIME_INVALID,
  TimelineV2RuntimeError,
} from '../src/video/renderContractNormalize';

interface TimelineV2NormalizationFixture {
  version: number;
  success_cases: Array<{ name: string; input: TimelineV2Document; expected: TimelineV2Document }>;
  error_cases: Array<{ name: string; input: TimelineV2Document; path: string }>;
}

function loadFixture(): TimelineV2NormalizationFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/timeline-v2-normalization-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as TimelineV2NormalizationFixture;
}

describe('Timeline v2 runtime normalization', () => {
  const fixture = loadFixture();

  it('uses the versioned cross-runtime fixture', () => {
    expect(fixture.version).toBe(1);
  });

  for (const sample of fixture.success_cases) {
    it(`matches shared fixture: ${sample.name}`, () => {
      const before = JSON.parse(JSON.stringify(sample.input)) as TimelineV2Document;
      const normalized = normalizeTimelineV2EvaluationInputs(sample.input);
      expect(sample.input).toEqual(before);
      expect(JSON.parse(JSON.stringify(normalized))).toEqual(sample.expected);
    });
  }

  for (const sample of fixture.error_cases) {
    it(`fails closed: ${sample.name}`, () => {
      try {
        normalizeTimelineV2EvaluationInputs(sample.input);
        throw new Error('normalizer unexpectedly succeeded');
      } catch (error) {
        expect(error).toBeInstanceOf(TimelineV2RuntimeError);
        const runtimeError = error as TimelineV2RuntimeError;
        expect(runtimeError.code).toBe(TIMELINE_V2_RUNTIME_INVALID);
        expect(runtimeError.path).toBe(sample.path);
      }
    });
  }
});
