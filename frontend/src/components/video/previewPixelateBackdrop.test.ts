import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../../video/renderContractMediaGeometry';
import { PERSPECTIVE_PROJECTION_CONTRACT_V1 } from '../../video/renderContractPerspectiveProjection';
import { SHAPE_STATE_CONTRACT_V1 } from '../../video/renderContractShape';
import {
  PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
  planPreviewPixelateBackdrop,
} from './previewPixelateBackdrop';

const identityMatrix: CanonicalFrameLayerState['model_matrix'] = [
  1, 0, 0, 0,
  0, 1, 0, 0,
  0, 0, 1, 0,
  0, 0, 0, 1,
];

const transform: CanonicalFrameLayerState['transform'] = {
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
};

function state(overrides: Partial<CanonicalFrameLayerState> = {}): CanonicalFrameLayerState {
  return {
    track_index: 0,
    clip_index: 0,
    track_id: 'track',
    clip_id: 'clip',
    z_index: 0,
    start_frame: 0,
    end_frame: 30,
    source_time_ms: 0,
    transform: { ...transform },
    view_transform: { ...transform },
    model_matrix: [...identityMatrix],
    perspective_projection: {
      contract_version: PERSPECTIVE_PROJECTION_CONTRACT_V1,
      distance: 1200,
      source: 'camera',
      origin_w: 1,
      matrix: [...identityMatrix],
    },
    unresolved: [],
    authoritative: true,
    ...overrides,
  };
}

function mediaState(overrides: Partial<CanonicalFrameLayerState> = {}): CanonicalFrameLayerState {
  return state({
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
    ...overrides,
  });
}

function pixelateState(overrides: Partial<CanonicalFrameLayerState> = {}): CanonicalFrameLayerState {
  return state({
    shape: {
      contract_version: SHAPE_STATE_CONTRACT_V1,
      kind: 'pixelate',
      width: 160,
      height: 90,
      fill: 'transparent',
      stroke: '#f59e0b',
      stroke_width: 6,
      blur_radius: 12,
      corner_radius: 0,
    },
    ...overrides,
  });
}

function layer(
  clipId: string,
  canonicalState: CanonicalFrameLayerState | undefined,
  mimeType?: string,
) {
  return {
    clip: { id: clipId },
    ...(mimeType ? { asset: { mime_type: mimeType } } : {}),
    ...(canonicalState ? { canonicalState: { ...canonicalState, clip_id: clipId } } : {}),
  };
}

describe('planPreviewPixelateBackdrop', () => {
  it('admits exactly one clean media backdrop beneath one simple pixelate region', () => {
    const plan = planPreviewPixelateBackdrop(12, [
      layer('backdrop', mediaState(), 'image/png'),
      layer('pixelate', pixelateState()),
    ]);

    expect(plan).toMatchObject({
      contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
      mode: 'canonical-ready',
      target: { clip: { id: 'pixelate' } },
      backdrop: { clip: { id: 'backdrop' } },
      rasterSource: { supported: true, clipId: 'backdrop', kind: 'image' },
      runtimeRequirements: ['decoded-frame-ready', 'opaque-region-proof'],
      deferredReasons: [],
    });
  });

  it('returns legacy or canonical-none without inventing a pixelate surface', () => {
    const clean = layer('backdrop', mediaState(), 'video/mp4');
    expect(planPreviewPixelateBackdrop(null, [clean])).toMatchObject({ mode: 'legacy' });
    expect(planPreviewPixelateBackdrop(0, [layer('missing', undefined, 'video/mp4')])).toMatchObject({ mode: 'legacy' });
    expect(planPreviewPixelateBackdrop(0, [clean])).toEqual({
      contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
      mode: 'canonical-none',
      layers: [clean],
      deferredReasons: [],
    });
  });

  it('fails closed when more than one pixelate region is active', () => {
    const plan = planPreviewPixelateBackdrop(0, [
      layer('backdrop', mediaState(), 'video/mp4'),
      layer('pixelate-a', pixelateState()),
      layer('pixelate-b', pixelateState()),
    ]);
    expect(plan).toMatchObject({
      mode: 'canonical-deferred',
      deferredReasons: ['multiple-pixelate-layers-deferred'],
    });
  });

  it('keeps complex pixelate-region paint and transforms explicitly deferred', () => {
    const authored = { ...transform, x: 20 };
    const view = { ...authored, x: 10, rotation_z: 5, opacity: 0.8 };
    const plan = planPreviewPixelateBackdrop(0, [
      layer('backdrop', mediaState(), 'video/mp4'),
      layer('pixelate', pixelateState({
        authoritative: false,
        unresolved: ['shape:test'],
        text: { contract_version: 'text-state-v1' } as CanonicalFrameLayerState['text'],
        cursor: { contract_version: 'cursor-state-v1' } as CanonicalFrameLayerState['cursor'],
        effects: [{ contract_version: 'effect-state-v1' } as NonNullable<CanonicalFrameLayerState['effects']>[number]],
        transitions: [{ contract_version: 'transition-state-v1' } as NonNullable<CanonicalFrameLayerState['transitions']>[number]],
        transform: authored,
        view_transform: view,
      })),
    ]);

    expect(plan).toMatchObject({ mode: 'canonical-deferred' });
    if (plan.mode !== 'canonical-deferred') return;
    expect(plan.deferredReasons).toEqual([
      'pixelate-state-not-authoritative',
      'pixelate-state-unresolved',
      'pixelate-text-deferred',
      'pixelate-cursor-deferred',
      'pixelate-effects-deferred',
      'pixelate-transition-deferred',
      'pixelate-transform-deferred',
      'pixelate-camera-transform-deferred',
    ]);
  });

  it('requires exactly one lower visual layer before runtime raster acquisition', () => {
    expect(planPreviewPixelateBackdrop(0, [layer('pixelate', pixelateState())])).toMatchObject({
      mode: 'canonical-deferred',
      deferredReasons: ['backdrop-layer-count-deferred'],
    });
    expect(planPreviewPixelateBackdrop(0, [
      layer('lower-a', mediaState(), 'image/png'),
      layer('lower-b', mediaState(), 'video/mp4'),
      layer('pixelate', pixelateState()),
    ])).toMatchObject({
      mode: 'canonical-deferred',
      deferredReasons: ['backdrop-layer-count-deferred'],
    });
  });

  it('surfaces media-source, opacity, transition, rotation, and camera blockers independently', () => {
    const authored = { ...transform, rotation_z: 8 };
    const view = { ...authored, x: -5, opacity: 0.5 };
    const plan = planPreviewPixelateBackdrop(0, [
      layer('backdrop', mediaState({
        transform: authored,
        view_transform: view,
        transitions: [{ contract_version: 'transition-state-v1' } as NonNullable<CanonicalFrameLayerState['transitions']>[number]],
      }), 'audio/wav'),
      layer('pixelate', pixelateState()),
    ]);

    expect(plan).toMatchObject({ mode: 'canonical-deferred' });
    if (plan.mode !== 'canonical-deferred') return;
    expect(plan.deferredReasons).toEqual([
      'backdrop:media-mime-unsupported',
      'backdrop-opacity-deferred',
      'backdrop-transition-deferred',
      'backdrop-rotation-deferred',
      'backdrop-camera-transform-deferred',
    ]);
  });
});
