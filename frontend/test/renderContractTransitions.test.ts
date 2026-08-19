import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateClipTransitionsAtFrame,
  TRANSITION_STATE_CONTRACT_V1,
  type CanonicalTransitionState,
} from '../src/video/renderContractTransitions';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

interface ExpectedState {
  id: string;
  placement: string;
  role: string;
  peer_role?: string;
  peer_clip_id?: string;
  peer_track_index?: number;
  peer_clip_index?: number;
  direction?: string;
  start_frame: number;
  end_frame: number;
  progress: number;
  active: boolean;
}
interface Fixture {
  version: number;
  document: TimelineV2Document;
  cases: Array<{ name: string; frame_index: number; expected: ExpectedState[] }>;
}

const fixtureURL = new URL('../../video-renderer/test/fixtures/transition-state-v1.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8')) as Fixture;

function expectState(state: CanonicalTransitionState, expected: ExpectedState): void {
  expect(state.contract_version).toBe(TRANSITION_STATE_CONTRACT_V1);
  expect(state.id).toBe(expected.id);
  expect(state.placement).toBe(expected.placement);
  expect(state.role).toBe(expected.role);
  expect(state.peer_role).toBe(expected.peer_role);
  expect(state.peer_clip_id).toBe(expected.peer_clip_id);
  expect(state.peer_track_index).toBe(expected.peer_track_index);
  expect(state.peer_clip_index).toBe(expected.peer_clip_index);
  expect(state.direction).toBe(expected.direction);
  expect(state.start_frame).toBe(expected.start_frame);
  expect(state.end_frame).toBe(expected.end_frame);
  expect(state.progress).toBeCloseTo(expected.progress, 12);
  expect(state.active).toBe(expected.active);
}

describe('canonical transition state', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const states = evaluateClipTransitionsAtFrame(fixture.document, 0, 0, sample.frame_index);
      expect(states).toHaveLength(sample.expected.length);
      states.forEach((state, index) => expectState(state, sample.expected[index]));
    }
  });

  it('fails closed when a between peer does not exist', () => {
    const doc = structuredClone(fixture.document);
    doc.tracks[0].clips[0].transitions![1].peer_clip_id = 'missing';
    expect(() => evaluateClipTransitionsAtFrame(doc, 0, 0, 55)).toThrow(/does not exist/);
  });

  it('fails closed when authored overlap is shorter than the transition', () => {
    const doc = structuredClone(fixture.document);
    doc.tracks[1].clips[0].start_ms = 650;
    expect(() => evaluateClipTransitionsAtFrame(doc, 0, 0, 65)).toThrow(/real owner\/peer overlap/);
  });

  it('fails closed on unknown runtime transition semantics', () => {
    const unknownType = structuredClone(fixture.document);
    unknownType.tracks[0].clips[0].transitions![0].type = 'future-transition' as never;
    expect(() => evaluateClipTransitionsAtFrame(unknownType, 0, 0, 15)).toThrow(/unsupported transition type/);

    const unknownDirection = structuredClone(fixture.document);
    unknownDirection.tracks[0].clips[0].transitions![2].direction = 'diagonal' as never;
    expect(() => evaluateClipTransitionsAtFrame(unknownDirection, 0, 0, 65)).toThrow(/unsupported transition direction/);
  });
});
