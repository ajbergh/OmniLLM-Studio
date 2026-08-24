import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { CanonicalMediaGeometry } from '../../video/renderContractMediaGeometry';
import {
  canonicalPreviewMediaClipPath,
  canonicalPreviewMediaElementStyle,
  resolveCanonicalPreviewMediaGeometry,
} from './previewFrameMediaGeometry';

const portraitGeometry: CanonicalMediaGeometry = {
  contract_version: 'media-geometry-v1',
  fit: 'contain',
  viewport_bounds: { x: 0, y: 0, width: 1280, height: 720 },
  source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
  visible_source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
  painted_bounds: { x: 460, y: 0, width: 360, height: 720 },
  clip_bounds: { x: 64, y: 72, width: 1088, height: 576 },
  scale_x: 0.36,
  scale_y: 0.36,
};

function state(geometry: CanonicalMediaGeometry = portraitGeometry): CanonicalFrameLayerState {
  return {
    media_geometry: geometry,
  } as CanonicalFrameLayerState;
}

describe('canonical preview media geometry', () => {
  it('uses valid canonical geometry only for deterministic non-interactive media', () => {
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), true, false, false)).toBe(portraitGeometry);
    expect(resolveCanonicalPreviewMediaGeometry(null, state(), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), false, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), true, true, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), true, false, true)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, {} as CanonicalFrameLayerState, true, false, false)).toBeNull();
  });

  it('fails closed when canonical bounds or scales are invalid', () => {
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      painted_bounds: { ...portraitGeometry.painted_bounds, width: 0 },
    }), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      scale_x: Number.NaN,
    }), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      clip_bounds: { x: -1, y: 0, width: 1280, height: 720 },
    }), true, false, false)).toBeNull();
  });

  it('maps canonical painted bounds into preview CSS pixels without object-fit reinterpretation', () => {
    expect(canonicalPreviewMediaElementStyle(portraitGeometry, 0.5)).toEqual({
      position: 'absolute',
      left: 230,
      top: 0,
      width: 180,
      height: 360,
      maxWidth: 'none',
      maxHeight: 'none',
      objectFit: 'fill',
    });
  });

  it('maps canonical canvas clip bounds to a full-stage inset path', () => {
    expect(canonicalPreviewMediaClipPath(portraitGeometry, 0.5)).toBe('inset(36px 64px 36px 32px)');
  });
});
