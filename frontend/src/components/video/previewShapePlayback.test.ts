import { describe, expect, it } from 'vitest';
import type { CanonicalEvaluatedShapeState } from '../../video/renderContractShape';
import type { VideoTimelineShape } from '../../types/video';
import {
  parsePreviewShapeRasterColor,
  previewShapePlaybackStructuralDeferredReason,
  type PreviewShapePlaybackContext,
  type PreviewShapePlaybackLayer,
} from './previewShapePlayback';

const canonicalShape: CanonicalEvaluatedShapeState = {
  contract_version: 'shape-state-v1',
  kind: 'rounded_rectangle',
  width: 240,
  height: 120,
  fill: 'rgba(10,20,30,0.5)',
  stroke: '#f59e0b',
  stroke_width: 8,
  blur_radius: 12,
  corner_radius: 24,
};

const context: PreviewShapePlaybackContext = {
  canvasWidth: 640,
  canvasHeight: 360,
  scenes: [],
};

function layer(shape: VideoTimelineShape = {
  kind: 'rounded_rectangle',
  width: 240,
  height: 120,
  fill: 'rgba(10,20,30,0.5)',
  stroke: '#f59e0b',
  stroke_width: 8,
  corner_radius: 24,
}): PreviewShapePlaybackLayer {
  return {
    clip: {
      id: 'shape',
      start_ms: 1000,
      duration_ms: 1200,
      shape,
      transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
      effects: [],
      transitions: [],
      keyframes: [],
      animation_blocks: [],
    },
    canonicalState: { shape: canonicalShape },
  };
}

describe('rounded rectangle normal-playback admission', () => {
  it('admits the export-proven static 2D subset', () => {
    expect(previewShapePlaybackStructuralDeferredReason(layer(), context)).toBeUndefined();
  });

  it.each([
    ['wrong kind', (value: PreviewShapePlaybackLayer) => { value.clip.shape = { ...value.clip.shape!, kind: 'ellipse' }; }, 'shape:shape-kind-unsupported'],
    ['asset', (value: PreviewShapePlaybackLayer) => { value.clip.asset_id = 'asset'; }, 'shape:standalone-shape-required'],
    ['text', (value: PreviewShapePlaybackLayer) => { value.clip.text = {}; }, 'shape:standalone-shape-required'],
    ['cursor', (value: PreviewShapePlaybackLayer) => { value.clip.cursor = {}; }, 'shape:standalone-shape-required'],
    ['audio only', (value: PreviewShapePlaybackLayer) => { value.clip.audio_only = true; }, 'shape:standalone-shape-required'],
    ['duration', (value: PreviewShapePlaybackLayer) => { value.clip.duration_ms = 0; }, 'shape:duration-invalid'],
    ['fade', (value: PreviewShapePlaybackLayer) => { value.clip.fade_in_ms = 50; }, 'shape:fade-unsupported'],
    ['transition', (value: PreviewShapePlaybackLayer) => { value.clip.transitions = [{}]; }, 'shape:transition-parent-unsupported'],
    ['animation', (value: PreviewShapePlaybackLayer) => { value.clip.animation_blocks = [{}]; }, 'shape:animation-parent-unsupported'],
    ['keyframe', (value: PreviewShapePlaybackLayer) => { value.clip.keyframes = [{}]; }, 'shape:keyframe-parent-unsupported'],
    ['effect', (value: PreviewShapePlaybackLayer) => { value.clip.effects = [{ enabled: true }]; }, 'shape:effect-parent-unsupported'],
    ['crop', (value: PreviewShapePlaybackLayer) => { value.clip.transform = { ...value.clip.transform!, crop: { top: 0, right: 0, bottom: 0, left: 0 } }; }, 'shape:parent-transform-unsupported'],
    ['nonuniform scale', (value: PreviewShapePlaybackLayer) => { value.clip.transform = { ...value.clip.transform!, scale_x: 1, scale_y: 1.1 }; }, 'shape:parent-transform-unsupported'],
    ['3d', (value: PreviewShapePlaybackLayer) => { value.clip.transform = { ...value.clip.transform!, rotation_x: 5 }; }, 'shape:parent-transform-unsupported'],
    ['opacity', (value: PreviewShapePlaybackLayer) => { value.clip.transform = { ...value.clip.transform!, opacity: 1.2 }; }, 'shape:parent-transform-unsupported'],
  ])('fails closed for %s', (_name, edit, reason) => {
    const value = layer();
    edit(value);
    expect(previewShapePlaybackStructuralDeferredReason(value, context)).toBe(reason);
  });

  it('fails closed for overlapping scene camera', () => {
    expect(previewShapePlaybackStructuralDeferredReason(layer(), {
      ...context,
      scenes: [{ start_ms: 1500, duration_ms: 500, camera: { x: 1 } }],
    })).toBe('shape:scene-camera-unsupported');
  });

  it('requires matching canonical shape-state-v1', () => {
    const missing = layer();
    missing.canonicalState = {};
    expect(previewShapePlaybackStructuralDeferredReason(missing, context)).toBe('shape:canonical-shape-state-unavailable');

    const mismatch = layer();
    mismatch.canonicalState = { shape: { ...canonicalShape, kind: 'ellipse' } };
    expect(previewShapePlaybackStructuralDeferredReason(mismatch, context)).toBe('shape:canonical-shape-contract-mismatch');
  });

  it('rejects raster geometry outside the canvas', () => {
    const value = layer();
    value.canonicalState = { shape: { ...canonicalShape, width: 641 } };
    expect(previewShapePlaybackStructuralDeferredReason(value, context)).toBe('shape:shape-raster-out-of-bounds');
  });

  it('mirrors the Go raster color grammar', () => {
    for (const value of ['transparent', '', '#abc', '#abcd', '#112233', '#11223344', 'rgb(1, 2, 3)', 'rgba(1,2,3,0.5)']) {
      expect(parsePreviewShapeRasterColor(value), value).toBe(true);
    }
    for (const value of ['red', 'hsl(1,2%,3%)', 'rgb(256,2,3)', 'rgba(1,2,3,2)', 'rgb(1%,2%,3%)']) {
      expect(parsePreviewShapeRasterColor(value), value).toBe(false);
    }
  });

  it('checks canonical fill and stroke, not authored aliases', () => {
    const fill = layer();
    fill.canonicalState = { shape: { ...canonicalShape, fill: 'red' } };
    expect(previewShapePlaybackStructuralDeferredReason(fill, context)).toBe('shape:fill-color-unsupported');

    const stroke = layer();
    stroke.canonicalState = { shape: { ...canonicalShape, stroke: 'red' } };
    expect(previewShapePlaybackStructuralDeferredReason(stroke, context)).toBe('shape:stroke-color-unsupported');
  });
});
