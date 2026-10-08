import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { VideoTimelineShape, VideoTimelineTransform } from '../../types/video';
import {
  previewShapePlaybackStructuralDeferredReason,
  shapeRasterColorSupported,
  type PreviewShapePlaybackContext,
  type PreviewShapePlaybackLayer,
} from './previewShapePlayback';

const context: PreviewShapePlaybackContext = {
  canvasWidth: 640,
  canvasHeight: 360,
  scenes: [],
};

function roundedShape(): VideoTimelineShape {
  return {
    kind: 'rounded_rectangle',
    width: 240,
    height: 120,
    fill: 'rgba(10,20,30,0.5)',
    stroke: '#f59e0b',
    stroke_width: 8,
    corner_radius: 24,
  };
}

function transform(): VideoTimelineTransform {
  return { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };
}

function layer(overrides: Partial<PreviewShapePlaybackLayer['clip']> = {}): PreviewShapePlaybackLayer {
  return {
    clip: {
      id: 'shape',
      start_ms: 1000,
      duration_ms: 1200,
      shape: roundedShape(),
      transform: transform(),
      effects: [],
      keyframes: [],
      ...overrides,
    },
    canonicalState: {
      shape: {
        contract_version: 'shape-state-v1',
        kind: 'rounded_rectangle',
        width: 240,
        height: 120,
        fill: 'rgba(10,20,30,0.5)',
        stroke: '#f59e0b',
        stroke_width: 8,
        blur_radius: 12,
        corner_radius: 24,
      },
    } as Pick<CanonicalFrameLayerState, 'shape'>,
  };
}

describe('rounded rectangle normal-playback admission', () => {
  it('admits the exact export-proven standalone static 2D subset', () => {
    expect(previewShapePlaybackStructuralDeferredReason(layer(), context)).toBeUndefined();
  });

  it.each([
    ['unsupported shape kind', { shape: { ...roundedShape(), kind: 'ellipse' as const } }, 'shape:shape-kind-unsupported'],
    ['mixed media owner', { asset_id: 'asset-1' }, 'shape:standalone-shape-required'],
    ['fade', { fade_in_ms: 100 }, 'shape:fade-unsupported'],
    ['transition', { transitions: [{}] }, 'shape:transition-parent-unsupported'],
    ['animation', { animation_blocks: [{}] }, 'shape:animation-parent-unsupported'],
    ['effect', { effects: [{ enabled: true }] }, 'shape:effect-parent-unsupported'],
    ['keyframe', { keyframes: [{}] }, 'shape:keyframe-parent-unsupported'],
    ['crop', { transform: { ...transform(), crop: { top: 0.1, right: 0, bottom: 0, left: 0 } } }, 'shape:crop-unsupported'],
    ['3D parent', { transform: { ...transform(), rotation_x: 2 } }, 'shape:parent-transform-unsupported'],
    ['non-uniform parent scale', { transform: { ...transform(), scale_x: 1, scale_y: 1.1 } }, 'shape:parent-transform-unsupported'],
    ['invalid parent opacity', { transform: { ...transform(), opacity: 1.1 } }, 'shape:parent-opacity-unsupported'],
  ])('fails closed for %s', (_label, overrides, reason) => {
    expect(previewShapePlaybackStructuralDeferredReason(layer(overrides), context)).toBe(reason);
  });

  it('fails closed for any overlapping scene camera', () => {
    expect(previewShapePlaybackStructuralDeferredReason(layer(), {
      ...context,
      scenes: [{ start_ms: 1500, duration_ms: 200, camera: { x: 0 } }],
    })).toBe('shape:scene-camera-unsupported');
  });

  it('requires matching canonical shape state and canvas-contained geometry', () => {
    const missing = layer();
    delete missing.canonicalState;
    expect(previewShapePlaybackStructuralDeferredReason(missing, context)).toBe('shape:canonical-shape-state-unavailable');

    const oversized = layer();
    oversized.canonicalState = {
      shape: { ...oversized.canonicalState!.shape!, width: 641 },
    } as Pick<CanonicalFrameLayerState, 'shape'>;
    expect(previewShapePlaybackStructuralDeferredReason(oversized, context)).toBe('shape:shape-raster-out-of-bounds');
  });

  it('matches the renderer color grammar rather than accepting arbitrary CSS', () => {
    for (const value of ['transparent', '#abc', '#abcd', '#aabbcc', '#aabbccdd', 'rgb(1, 2, 3)', 'rgba(10,20,30,0.5)']) {
      expect(shapeRasterColorSupported(value), value).toBe(true);
    }
    for (const value of ['red', 'hsl(30 100% 50%)', 'rgb(0 0 0)', 'rgba(0,0,0,50%)', '#12', 'rgba(0,0,0,2)']) {
      expect(shapeRasterColorSupported(value), value).toBe(false);
    }
  });
});
