import { describe, expect, it } from 'vitest';
import type { VideoTimelineDocument, VideoTimelineTransform } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import { resolvePreviewFrameViewTransform, type PreviewCameraTransform } from './previewFrameViewTransform';

const transform: VideoTimelineTransform = {
  x: 80,
  y: -20,
  z: 30,
  scale: 1,
  rotation: 14,
  rotation_x: 7,
  rotation_y: -9,
  rotation_z: 14,
  opacity: 1,
};

const camera: PreviewCameraTransform = {
  x: 25,
  y: 5,
  z: -10,
  rotation_x: 2,
  rotation_y: -3,
  rotation_z: 4,
};

const canonicalView: CanonicalFrameLayerState['view_transform'] = {
  x: 41,
  y: -17,
  z: 63,
  scale_x: 1.2,
  scale_y: 0.8,
  rotation_x: 11,
  rotation_y: -13,
  rotation_z: 29,
  opacity: 0.7,
  anchor_x: 0,
  anchor_y: 0,
};

describe('preview FrameState view transform resolver', () => {
  it('consumes camera-relative view state projected from a real Timeline v1 frame', () => {
    const document: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 1280, height: 720, fps: 30, background: '#000000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      scenes: [{
        id: 'scene-1',
        name: 'Camera scene',
        start_ms: 0,
        duration_ms: 1000,
        camera: {
          x: 25,
          y: 5,
          z: -10,
          rotation_x: 2,
          rotation_y: -3,
          rotation_z: 4,
        },
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
          transform,
          effects: [],
          transitions: [],
          keyframes: [],
        }],
      }],
    };
    const entry = queryActiveClipsAtFrame(buildTimelineIntervalIndex(document, []), 15, 30)[0];

    expect(entry.canonicalState?.view_transform).toMatchObject({
      x: 55,
      y: -25,
      z: 40,
      rotation_x: 5,
      rotation_y: -6,
      rotation_z: 10,
    });
    expect(resolvePreviewFrameViewTransform(
      entry.clip.transform || {},
      entry.canonicalState,
      // Deliberately wrong local camera proves canonical view state owns the
      // deterministic path rather than being subtracted again here.
      { x: 999, y: 999, z: 999, rotation_x: 99, rotation_y: 99, rotation_z: 99 },
      false,
    )).toEqual({
      x: 55,
      y: -25,
      z: 40,
      rotation_x: 5,
      rotation_y: -6,
      rotation_z: 10,
    });
  });

  it('uses the canonical camera-relative view transform for deterministic state', () => {
    expect(resolvePreviewFrameViewTransform(transform, { view_transform: canonicalView }, camera, false)).toEqual({
      x: 41,
      y: -17,
      z: 63,
      rotation_x: 11,
      rotation_y: -13,
      rotation_z: 29,
    });
  });

  it('keeps free-running and compatibility fallback on local camera subtraction', () => {
    expect(resolvePreviewFrameViewTransform(transform, undefined, camera, false)).toEqual({
      x: 55,
      y: -25,
      z: 40,
      rotation_x: 5,
      rotation_y: -6,
      rotation_z: 10,
    });
  });

  it('lets a live direct-manipulation gesture bypass stale canonical view state', () => {
    expect(resolvePreviewFrameViewTransform(transform, { view_transform: canonicalView }, camera, true)).toEqual({
      x: 55,
      y: -25,
      z: 40,
      rotation_x: 5,
      rotation_y: -6,
      rotation_z: 10,
    });
  });

  it('preserves exact zero camera-relative values instead of falling through', () => {
    const zero = { ...canonicalView, x: 0, y: 0, z: 0, rotation_x: 0, rotation_y: 0, rotation_z: 0 };
    expect(resolvePreviewFrameViewTransform(transform, { view_transform: zero }, camera, false)).toEqual({
      x: 0,
      y: 0,
      z: 0,
      rotation_x: 0,
      rotation_y: 0,
      rotation_z: 0,
    });
  });
});
