import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument, VideoTimelineTransform } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import {
  canonicalPreviewPerspectiveCSSPixels,
  resolveCanonicalPreviewPerspectiveDistance,
  shouldUseCanonicalPreviewPerspective,
} from './previewFramePerspectiveProjection';

const baseTransform: VideoTimelineTransform = {
  x: 0,
  y: 0,
  z: 100,
  scale: 1,
  rotation: 0,
  rotation_x: 0,
  rotation_y: 0,
  rotation_z: 0,
  opacity: 1,
};

function perspectiveState(
  distance: number,
  source: 'camera' | 'clip' = 'camera',
): Pick<CanonicalFrameLayerState, 'perspective_projection'> {
  return {
    perspective_projection: {
      contract_version: 'perspective-projection-v1',
      distance,
      source,
      origin_w: 1 - 100 / distance,
      matrix: [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        0, 0, -1 / distance, 1,
      ],
    },
  };
}

describe('preview FrameState perspective projection resolver', () => {
  it('consumes a real clip-specific canonical perspective override', () => {
    const document: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 1280, height: 720, fps: 30, background: '#000000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      scenes: [{
        id: 'scene-1',
        name: 'Perspective scene',
        start_ms: 0,
        duration_ms: 1000,
        camera: { field_of_view: 60 },
      }],
      tracks: [{
        id: 'track-1',
        type: 'layer',
        name: 'Layer 1',
        locked: false,
        muted: false,
        visible: true,
        clips: [{
          id: 'clip-1',
          start_ms: 0,
          duration_ms: 1000,
          trim_in_ms: 0,
          trim_out_ms: 1000,
          transform: { ...baseTransform, perspective: 500 },
          effects: [],
          transitions: [],
          keyframes: [],
        }],
      }],
    };

    const entry = queryActiveClipsAtFrame(buildTimelineIntervalIndex(document, []), 15, 30)[0];
    const canonicalState = entry.canonicalState;
    if (!canonicalState) throw new Error('expected canonical frame state');

    expect(canonicalState.perspective_projection.source).toBe('clip');
    expect(canonicalState.perspective_projection.distance).toBe(500);
    expect(shouldUseCanonicalPreviewPerspective(15, [canonicalState], false)).toBe(true);
    expect(resolveCanonicalPreviewPerspectiveDistance(canonicalState, true)).toBe(500);
  });

  it('requires every deterministic visual layer to have a valid canonical projection', () => {
    expect(shouldUseCanonicalPreviewPerspective(15, [perspectiveState(1000), perspectiveState(500, 'clip')], false)).toBe(true);
    expect(shouldUseCanonicalPreviewPerspective(15, [perspectiveState(1000), undefined], false)).toBe(false);
    expect(shouldUseCanonicalPreviewPerspective(15, [perspectiveState(Number.NaN)], false)).toBe(false);
  });

  it('keeps free-running playback and live interaction on the shared stage perspective', () => {
    expect(shouldUseCanonicalPreviewPerspective(null, [perspectiveState(1000)], false)).toBe(false);
    expect(shouldUseCanonicalPreviewPerspective(15, [perspectiveState(1000)], true)).toBe(false);
    expect(resolveCanonicalPreviewPerspectiveDistance(perspectiveState(1000), false)).toBeNull();
  });

  it('scales canonical canvas-pixel distance without the legacy 100px clamp', () => {
    expect(canonicalPreviewPerspectiveCSSPixels(500, 0.5)).toBe(250);
    expect(canonicalPreviewPerspectiveCSSPixels(2, 0.25)).toBe(1);
    expect(() => canonicalPreviewPerspectiveCSSPixels(0, 1)).toThrow(/finite and positive/);
  });
});
