import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument } from '../src/types/video';
import {
  evaluateVisualFrameStateDiagnostic,
  VISUAL_FRAME_STATE_DIAGNOSTIC_V1,
} from '../src/video/renderContractFrameStateDiagnostics';

function compatibleTimeline(): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 640, height: 360, fps: 30, background: '#000000' },
    duration_ms: 1000,
    tracks: [{
      id: 'track-1',
      type: 'layer',
      name: 'Layer 1',
      locked: false,
      muted: false,
      visible: true,
      clips: [{
        id: 'clip-1',
        asset_id: 'asset-1',
        start_ms: 0,
        duration_ms: 1000,
        trim_in_ms: 100,
        trim_out_ms: 1100,
        playback_rate: 1,
        transform: { x: 12, y: -4, scale: 1, rotation: 0, opacity: 1 },
        effects: [],
        transitions: [],
        keyframes: [],
      }],
    }],
    markers: [],
    metadata: {},
  };
}

describe('visual FrameState diagnostic envelope', () => {
  it('returns canonical state when v1 semantics are representable', () => {
    const result = evaluateVisualFrameStateDiagnostic(compatibleTimeline(), 3);
    expect(result.contract_version).toBe(VISUAL_FRAME_STATE_DIAGNOSTIC_V1);
    expect(result.frame_index).toBe(3);
    expect(result.available).toBe(true);
    expect(result.error).toBeUndefined();
    expect(result.state?.frame_index).toBe(3);
    expect(result.state?.layers).toHaveLength(1);
    expect(result.state?.layers[0].source_time_ms).toBeCloseTo(200, 9);
    expect(result.state?.layers[0].transform.x).toBe(12);
  });

  it('preserves fail-closed transition ambiguity as structured unavailability', () => {
    const timeline = compatibleTimeline();
    timeline.tracks[0].clips[0].transitions = [{
      id: 'transition-1',
      type: 'fade',
      duration_ms: 100,
    }];

    const result = evaluateVisualFrameStateDiagnostic(timeline, 0);
    expect(result.available).toBe(false);
    expect(result.state).toBeUndefined();
    expect(result.error?.code).toBe('V1_TRANSITION_PLACEMENT_AMBIGUOUS');
    expect(result.error?.path).toContain('transitions[0]');
    expect(result.error?.remediation).toBeTruthy();
  });

  it('reports evaluator failures without throwing out of the diagnostic surface', () => {
    const result = evaluateVisualFrameStateDiagnostic(compatibleTimeline(), 9999);
    expect(result.available).toBe(false);
    expect(result.error?.code).toBe('FRAME_STATE_EVALUATION_FAILED');
    expect(result.error?.message).toContain('outside timeline frame range');
  });

  it('accepts an editor-verified font resource without claiming packaged-face provenance', () => {
    const timeline = compatibleTimeline();
    delete timeline.tracks[0].clips[0].asset_id;
    timeline.tracks[0].clips[0].text = {
      text: 'Title',
      font_family: 'Inter',
      font_resource_id: 'inter-400-normal',
    };

    const result = evaluateVisualFrameStateDiagnostic(timeline, 0, {
      availableFontResourceIDs: new Set(['inter-400-normal']),
    });

    expect(result.available).toBe(true);
    expect(result.state?.layers[0].text?.font_resource_id).toBe('inter-400-normal');
    expect(result.state?.layers[0].text?.font_face_source).toBe('family-name-only');
  });

  it('keeps font resource references fail closed without an editor resource context', () => {
    const timeline = compatibleTimeline();
    delete timeline.tracks[0].clips[0].asset_id;
    timeline.tracks[0].clips[0].text = {
      text: 'Title',
      font_family: 'Inter',
      font_resource_id: 'inter-400-normal',
    };

    const result = evaluateVisualFrameStateDiagnostic(timeline, 0);

    expect(result.available).toBe(false);
    expect(result.error?.code).toBe('FRAME_STATE_EVALUATION_FAILED');
    expect(result.error?.message).toContain('manifest does not package');
  });
});
