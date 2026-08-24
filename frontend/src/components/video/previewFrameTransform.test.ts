import { describe, expect, it } from 'vitest';
import type { VideoTimelineClip, VideoTimelineDocument } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import { resolvePreviewFrameTransform } from './previewFrameTransform';

function clip(): VideoTimelineClip {
  return {
    id: 'clip-1',
    start_ms: 0,
    duration_ms: 1000,
    trim_in_ms: 0,
    trim_out_ms: 1000,
    transform: { x: 10, y: 20, scale: 1, rotation: 5, opacity: 0.8 },
    effects: [],
    keyframes: [
      { id: 'x-0', property: 'x', time_ms: 0, value: 10, easing: 'linear' },
      { id: 'x-1', property: 'x', time_ms: 1000, value: 110, easing: 'linear' },
    ],
  };
}

const canonicalTransform: CanonicalFrameLayerState['transform'] = {
  x: 75,
  y: -12,
  z: 18,
  scale_x: 1.25,
  scale_y: 0.75,
  rotation_x: 4,
  rotation_y: -7,
  rotation_z: 33,
  opacity: 0.42,
  anchor_x: 12,
  anchor_y: -8,
  crop: { top: 0.1, right: 0.2, bottom: 0.15, left: 0.05 },
};

describe('preview FrameState transform resolver', () => {
  it('consumes transform and already-faded opacity projected from a transition-free Timeline v1 frame', () => {
    const projectedClip = clip();
    projectedClip.fade_in_ms = 1000;
    const document: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 100, height: 100, fps: 30, background: '#000000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'track-1',
        type: 'layer',
        name: 'Layer 1',
        locked: false,
        muted: false,
        visible: true,
        clips: [projectedClip],
      }],
    };
    const entry = queryActiveClipsAtFrame(buildTimelineIntervalIndex(document, []), 15, 30)[0];

    expect(entry.canonicalState).toBeDefined();
    expect(entry.canonicalState?.transform.opacity).toBeCloseTo(0.4, 9);
    const result = resolvePreviewFrameTransform(entry.clip, 500, entry.canonicalState, false);
    expect(result.opacityIncludesClipFades).toBe(true);
    expect(result.transform.x).toBe(entry.canonicalState?.transform.x);
    expect(result.transform.opacity).toBe(entry.canonicalState?.transform.opacity);
  });

  it('uses exact canonical transform and already-faded opacity for deterministic state', () => {
    const result = resolvePreviewFrameTransform(clip(), 500, { transform: canonicalTransform }, false);

    expect(result.opacityIncludesClipFades).toBe(true);
    expect(result.transform).toMatchObject({
      x: 75,
      y: -12,
      z: 18,
      scale_x: 1.25,
      scale_y: 0.75,
      rotation_x: 4,
      rotation_y: -7,
      rotation: 33,
      rotation_z: 33,
      opacity: 0.42,
      anchor_x: 12,
      anchor_y: -8,
    });
    expect(result.transform.crop).toEqual(canonicalTransform.crop);
    expect(result.transform.crop).not.toBe(canonicalTransform.crop);
  });

  it('preserves exact canonical zero opacity as an owned faded result', () => {
    const result = resolvePreviewFrameTransform(
      clip(),
      500,
      { transform: { ...canonicalTransform, opacity: 0 } },
      false,
    );

    expect(result.opacityIncludesClipFades).toBe(true);
    expect(result.transform.opacity).toBe(0);
  });

  it('preserves exact canonical zero axis scale through the legacy truthy fallback', () => {
    const zeroX = resolvePreviewFrameTransform(
      clip(),
      500,
      { transform: { ...canonicalTransform, scale_x: 0 } },
      false,
    );
    expect(zeroX.transform.scale_x).toBe(0);
    expect(zeroX.transform.scale_y).toBe(0.75);
    expect(zeroX.transform.scale).toBe(0);

    const zeroY = resolvePreviewFrameTransform(
      clip(),
      500,
      { transform: { ...canonicalTransform, scale_y: 0 } },
      false,
    );
    expect(zeroY.transform.scale_x).toBe(1.25);
    expect(zeroY.transform.scale_y).toBe(0);
    expect(zeroY.transform.scale).toBe(0);
  });

  it('keeps free-running/fallback transform evaluation on the established property path', () => {
    const result = resolvePreviewFrameTransform(clip(), 500, undefined, false);

    expect(result.opacityIncludesClipFades).toBe(false);
    expect(result.transform.x).toBe(60);
    expect(result.transform.y).toBe(20);
    expect(result.transform.rotation).toBe(5);
    expect(result.transform.opacity).toBe(0.8);
  });

  it('lets an in-flight direct-manipulation gesture bypass canonical state', () => {
    const result = resolvePreviewFrameTransform(clip(), 500, { transform: canonicalTransform }, true);

    expect(result.opacityIncludesClipFades).toBe(false);
    expect(result.transform.x).toBe(60);
    expect(result.transform.x).not.toBe(canonicalTransform.x);
  });
});
