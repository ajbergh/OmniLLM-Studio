import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../../video/renderContractMediaGeometry';
import {
  resolvePreviewWeightedPairRasterPairCapability,
  resolvePreviewWeightedPairRasterSourceCapability,
} from './previewFrameWeightedPairRaster';

function state(overrides: Partial<CanonicalFrameLayerState> = {}): CanonicalFrameLayerState {
  return {
    clip_id: 'clip-a',
    authoritative: true,
    unresolved: [],
    media_geometry: {
      contract_version: MEDIA_GEOMETRY_CONTRACT_V1,
      fit: 'contain',
      viewport_bounds: { x: 0, y: 0, width: 640, height: 360 },
      source_bounds: { x: 0, y: 0, width: 640, height: 360 },
      visible_source_bounds: { x: 0, y: 0, width: 640, height: 360 },
      painted_bounds: { x: 0, y: 0, width: 640, height: 360 },
      clip_bounds: { x: 0, y: 0, width: 640, height: 360 },
      scale_x: 1,
      scale_y: 1,
    },
    view_transform: {
      x: 0,
      y: 0,
      z: 0,
      scale_x: 1,
      scale_y: 1,
      rotation_x: 0,
      rotation_y: 0,
      rotation_z: 0,
      opacity: 1,
      anchor_x: 0,
      anchor_y: 0,
    },
    ...overrides,
  } as CanonicalFrameLayerState;
}

function layer(
  clipId: string,
  mimeType = 'image/png',
  overrides: Partial<CanonicalFrameLayerState> = {},
) {
  return {
    clip: { id: clipId },
    asset: { mime_type: mimeType },
    canonicalState: state({ clip_id: clipId, ...overrides }),
  };
}

describe('resolvePreviewWeightedPairRasterSourceCapability', () => {
  it('admits authoritative 2D image and video sources with canonical media geometry', () => {
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('image', 'image/png'))).toEqual({
      supported: true,
      clipId: 'image',
      kind: 'image',
    });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('video', 'video/mp4'))).toEqual({
      supported: true,
      clipId: 'video',
      kind: 'video',
    });
  });

  it('fails closed for canonical text, shape, cursor, and clip-effect paint', () => {
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('text', 'image/png', {
      text: { contract_version: 'text-state-v1' } as CanonicalFrameLayerState['text'],
    }))).toMatchObject({ supported: false, reason: 'text-raster-deferred' });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('shape', 'image/png', {
      shape: { contract_version: 'shape-state-v1' } as CanonicalFrameLayerState['shape'],
    }))).toMatchObject({ supported: false, reason: 'shape-raster-deferred' });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('cursor', 'image/png', {
      cursor: { contract_version: 'cursor-state-v1' } as CanonicalFrameLayerState['cursor'],
    }))).toMatchObject({ supported: false, reason: 'cursor-raster-deferred' });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('effect', 'image/png', {
      effects: [{
        contract_version: 'effect-state-v1',
      } as NonNullable<CanonicalFrameLayerState['effects']>[number]],
    }))).toMatchObject({ supported: false, reason: 'clip-effects-raster-deferred' });
  });

  it('fails closed for unresolved state, missing geometry, non-media assets, and 3D transforms', () => {
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('unresolved', 'image/png', {
      unresolved: ['media_geometry:content_bounds'],
    }))).toMatchObject({ supported: false, reason: 'canonical-state-unresolved' });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('geometry', 'image/png', {
      media_geometry: undefined,
    }))).toMatchObject({ supported: false, reason: 'media-geometry-missing' });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('audio', 'audio/wav'))).toMatchObject({
      supported: false,
      reason: 'media-mime-unsupported',
    });
    expect(resolvePreviewWeightedPairRasterSourceCapability(layer('3d', 'image/png', {
      view_transform: {
        ...state().view_transform,
        rotation_y: 15,
      },
    }))).toMatchObject({ supported: false, reason: 'three-dimensional-transform-raster-deferred' });
  });
});

describe('resolvePreviewWeightedPairRasterPairCapability', () => {
  it('requires both pair inputs and exposes every blocking input reason', () => {
    expect(resolvePreviewWeightedPairRasterPairCapability(
      layer('lower', 'image/png'),
      layer('upper', 'video/mp4'),
    )).toMatchObject({
      supported: true,
      lower: { kind: 'image' },
      upper: { kind: 'video' },
    });

    const deferred = resolvePreviewWeightedPairRasterPairCapability(
      layer('lower', 'image/png', { shape: { contract_version: 'shape-state-v1' } as CanonicalFrameLayerState['shape'] }),
      layer('upper', 'video/mp4', { media_geometry: undefined }),
    );
    expect(deferred.supported).toBe(false);
    if (deferred.supported) return;
    expect(deferred.reasons).toEqual([
      'lower:shape-raster-deferred',
      'upper:media-geometry-missing',
    ]);
  });
});
