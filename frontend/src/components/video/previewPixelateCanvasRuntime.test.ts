import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { PERSPECTIVE_PROJECTION_CONTRACT_V1 } from '../../video/renderContractPerspectiveProjection';
import { SHAPE_STATE_CONTRACT_V1 } from '../../video/renderContractShape';
import {
  PREVIEW_PIXELATE_CANVAS_REGION_V1,
  previewPixelateRegionIsOpaque,
  resolvePreviewPixelateCanvasRegion,
} from './previewPixelateCanvasRuntime';

const identityMatrix: CanonicalFrameLayerState['model_matrix'] = [
  1, 0, 0, 0,
  0, 1, 0, 0,
  0, 0, 1, 0,
  0, 0, 0, 1,
];

function pixelateState(overrides: Partial<CanonicalFrameLayerState['view_transform']> = {}): CanonicalFrameLayerState {
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
    ...overrides,
  };
  return {
    track_index: 0,
    clip_index: 0,
    track_id: 'track',
    clip_id: 'pixelate',
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
    shape: {
      contract_version: SHAPE_STATE_CONTRACT_V1,
      kind: 'pixelate',
      width: 403,
      height: 307,
      fill: 'transparent',
      stroke: '#f59e0b',
      stroke_width: 6,
      blur_radius: 20,
      corner_radius: 0,
    },
    authoritative: true,
    unresolved: [],
  };
}

describe('resolvePreviewPixelateCanvasRegion', () => {
  it('matches FFmpeg integer placement and non-divisible raster dimensions', () => {
    const region = resolvePreviewPixelateCanvasRegion(
      pixelateState({ x: 17.9, y: -8.9 }),
      800,
      600,
    );

    expect(region).toEqual({
      contract_version: PREVIEW_PIXELATE_CANVAS_REGION_V1,
      x: 215,
      y: 138,
      width: 403,
      height: 307,
      raster: {
        contract_version: 'preview-pixelate-raster-v1',
        width: 403,
        height: 307,
        block_size: 20,
        downsample_width: 20,
        downsample_height: 15,
      },
    });
  });

  it('matches FFmpeg half-up dimensions for admitted uniform scalar scale', () => {
    const region = resolvePreviewPixelateCanvasRegion(
      pixelateState({ scale_x: 1.25, scale_y: 1.25 }),
      800,
      600,
    );

    expect(region.width).toBe(504);
    expect(region.height).toBe(384);
    expect(region.x).toBe(148);
    expect(region.y).toBe(108);
    expect(region.raster.downsample_width).toBe(25);
    expect(region.raster.downsample_height).toBe(19);
  });

  it('clamps an oversized or translated region to the same frame bounds as FFmpeg crop', () => {
    const state = pixelateState({ x: 400, y: -400, scale_x: 3, scale_y: 3 });
    if (!state.shape) throw new Error('expected shape');
    state.shape.width = 900;
    state.shape.height = 700;
    const region = resolvePreviewPixelateCanvasRegion(state, 800, 600);
    expect(region).toMatchObject({ x: 0, y: 0, width: 800, height: 600 });
  });

  it('rejects nonuniform runtime scale even if a caller bypasses structural admission', () => {
    expect(() => resolvePreviewPixelateCanvasRegion(
      pixelateState({ scale_x: 1.2, scale_y: 1.1 }),
      800,
      600,
    )).toThrow('finite positive uniform scale');
  });
});

describe('previewPixelateRegionIsOpaque', () => {
  it('accepts only regions whose every straight-alpha byte is 255', () => {
    expect(previewPixelateRegionIsOpaque(new Uint8ClampedArray([
      10, 20, 30, 255,
      40, 50, 60, 255,
    ]))).toBe(true);
    expect(previewPixelateRegionIsOpaque(new Uint8ClampedArray([
      10, 20, 30, 255,
      40, 50, 60, 254,
    ]))).toBe(false);
  });

  it('rejects empty or malformed RGBA buffers instead of inventing opacity', () => {
    expect(() => previewPixelateRegionIsOpaque(new Uint8ClampedArray())).toThrow('non-empty RGBA');
    expect(() => previewPixelateRegionIsOpaque(new Uint8ClampedArray([1, 2, 3]))).toThrow('non-empty RGBA');
  });
});
