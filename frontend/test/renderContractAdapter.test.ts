import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument } from '../src/types/video';
import {
  adaptTimelineV1ToV2,
  RenderContractAdapterError,
} from '../src/video/renderContractAdapter';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

interface AdapterFixture {
  version: number;
  success_cases: Array<{
    name: string;
    input: VideoTimelineDocument;
    expected: TimelineV2Document;
  }>;
  error_cases: Array<{
    name: string;
    input: VideoTimelineDocument;
    code: string;
    path: string;
  }>;
}

function loadFixture(): AdapterFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/v1-canonical-adapter-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as AdapterFixture;
}

function jsonShape<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

describe('Timeline v1 canonical adapter', () => {
  const fixture = loadFixture();

  it('uses the versioned cross-runtime fixture', () => {
    expect(fixture.version).toBe(1);
  });

  for (const sample of fixture.success_cases) {
    it(`matches shared fixture: ${sample.name}`, () => {
      const before = jsonShape(sample.input);
      const adapted = adaptTimelineV1ToV2(sample.input);

      expect(sample.input).toEqual(before);
      expect(jsonShape(adapted)).toEqual(sample.expected);

      if (sample.input.metadata.nested && adapted.metadata.nested) {
        const adaptedNested = adapted.metadata.nested as Record<string, unknown>;
        adaptedNested.value = 99;
        expect(sample.input.metadata.nested).toEqual({ value: 1 });
      }
    });
  }

  for (const sample of fixture.error_cases) {
    it(`fails closed: ${sample.name}`, () => {
      try {
        adaptTimelineV1ToV2(sample.input);
        throw new Error('adapter unexpectedly succeeded');
      } catch (error) {
        expect(error).toBeInstanceOf(RenderContractAdapterError);
        const adapterError = error as RenderContractAdapterError;
        expect(adapterError.code).toBe(sample.code);
        expect(adapterError.path).toBe(sample.path);
      }
    });
  }
});
