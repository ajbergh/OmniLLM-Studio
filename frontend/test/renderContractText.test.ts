import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateTextState, TEXT_STATE_CONTRACT_V1, type CanonicalEvaluatedTextState } from '../src/video/renderContractText';
import type { TimelineV2Text } from '../src/video/renderContractTypes';

interface TextStateFixture {
  version: number;
  canvas_height: number;
  default_case: { input: TimelineV2Text; expected: CanonicalEvaluatedTextState };
  authored_case: { input: TimelineV2Text; expected: CanonicalEvaluatedTextState };
}

function loadFixture(): TextStateFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/text-state-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as TextStateFixture;
}

describe('canonical text state', () => {
  const fixture = loadFixture();

  it('uses the versioned shared fixture', () => {
    expect(fixture.version).toBe(1);
  });

  it.each([
    ['defaults', fixture.default_case],
    ['authored', fixture.authored_case],
  ] as const)('matches shared %s semantics', (_name, sample) => {
    const state = evaluateTextState(sample.input, fixture.canvas_height);
    expect(state?.contract_version).toBe(TEXT_STATE_CONTRACT_V1);
    expect(state).toEqual(sample.expected);
  });

  it('fails closed on invalid authoring', () => {
    expect(() => evaluateTextState({ text: 'x', text_align: 'justify' as never }, 360)).toThrow(/text_align/);
    expect(() => evaluateTextState({ text: 'x', vertical_align: 'baseline' as never }, 360)).toThrow(/vertical_align/);
    expect(() => evaluateTextState({ text: 'x', padding_left: -1 }, 360)).toThrow(/padding_left/);
    expect(() => evaluateTextState({ text: 'x', letter_spacing: Number.NaN }, 360)).toThrow(/letter_spacing/);
    expect(() => evaluateTextState({ text: 'x', box_width: 0 }, 360)).toThrow(/box_width/);
  });

  it('does not interpret extension params as canonical styling', () => {
    const state = evaluateTextState({ text: 'x', params: { font_size: 999, unknown: true } }, 360);
    expect(state?.font_size).toBe(20);
  });

  it('serializes font-face provenance without claiming a packaged face', () => {
    expect(evaluateTextState({ text: 'x' }, 360)?.font_face_source).toBe('composition-default');
    expect(evaluateTextState({ text: 'x', font_family: 'Inter' }, 360)?.font_face_source).toBe('family-name-only');
    const withResource = evaluateTextState({ text: 'x', font_family: 'Inter', font_resource_id: 'inter-400-normal' }, 360);
    expect(withResource?.font_resource_id).toBe('inter-400-normal');
    // Only the manifest-level FrameState entry point can promote a face to
    // packaged-resource; the pure evaluator never claims one.
    expect(withResource?.font_face_source).toBe('family-name-only');
  });

  it('rejects surrounding whitespace in font_resource_id', () => {
    expect(() => evaluateTextState({ text: 'x', font_resource_id: ' inter-400-normal ' }, 360)).toThrow(/font_resource_id/);
  });
});
