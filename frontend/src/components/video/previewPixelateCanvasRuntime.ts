import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  resolvePreviewPixelateRasterPlan,
  type PreviewPixelateRasterPlan,
} from './previewPixelateRaster';

export const PREVIEW_PIXELATE_CANVAS_REGION_V1 = 'preview-pixelate-canvas-region-v1' as const;

export interface PreviewPixelateCanvasRegion {
  contract_version: typeof PREVIEW_PIXELATE_CANVAS_REGION_V1;
  x: number;
  y: number;
  width: number;
  height: number;
  raster: PreviewPixelateRasterPlan;
}

/**
 * Resolve the exact static FFmpeg pixelate crop/overlay rectangle in canonical
 * canvas pixels. The structural backdrop planner must run first: this helper
 * intentionally mirrors the admitted legacy region path's integer rounding,
 * center-relative placement, frame-bound clamping, and scalar uniform scale.
 */
export function resolvePreviewPixelateCanvasRegion(
  state: CanonicalFrameLayerState,
  canvasWidth: number,
  canvasHeight: number,
): PreviewPixelateCanvasRegion {
  requirePositiveInteger('canvas width', canvasWidth);
  requirePositiveInteger('canvas height', canvasHeight);
  const shape = state.shape;
  if (!shape || shape.kind !== 'pixelate') {
    throw new Error('preview pixelate Canvas region requires canonical pixelate shape state');
  }
  requirePositiveInteger('shape width', shape.width);
  requirePositiveInteger('shape height', shape.height);

  const view = state.view_transform;
  if (!Number.isFinite(view.x) || !Number.isFinite(view.y)) {
    throw new Error('preview pixelate Canvas region position must be finite');
  }
  if (!Number.isFinite(view.scale_x) || !Number.isFinite(view.scale_y)
    || view.scale_x <= 0 || view.scale_y <= 0 || view.scale_x !== view.scale_y) {
    throw new Error('preview pixelate Canvas region requires finite positive uniform scale');
  }

  let width = shape.width;
  let height = shape.height;
  if (view.scale_x !== 1) width = Math.max(2, Math.floor(shape.width * view.scale_x + 0.5));
  if (view.scale_y !== 1) height = Math.max(2, Math.floor(shape.height * view.scale_y + 0.5));
  width = Math.min(width, canvasWidth);
  height = Math.min(height, canvasHeight);

  let x = Math.floor((canvasWidth - width) / 2) + Math.trunc(view.x);
  let y = Math.floor((canvasHeight - height) / 2) + Math.trunc(view.y);
  x = Math.max(0, Math.min(x, canvasWidth - width));
  y = Math.max(0, Math.min(y, canvasHeight - height));

  return {
    contract_version: PREVIEW_PIXELATE_CANVAS_REGION_V1,
    x,
    y,
    width,
    height,
    raster: resolvePreviewPixelateRasterPlan(width, height, shape.blur_radius),
  };
}

/** Prove every decoded pixel in one candidate backdrop region has straight alpha 255. */
export function previewPixelateRegionIsOpaque(rgba: Uint8ClampedArray): boolean {
  if (rgba.length === 0 || rgba.length % 4 !== 0) {
    throw new Error('preview pixelate opacity proof requires a non-empty RGBA byte buffer');
  }
  for (let index = 3; index < rgba.length; index += 4) {
    if (rgba[index] !== 255) return false;
  }
  return true;
}

function requirePositiveInteger(name: string, value: number): void {
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`preview pixelate ${name} must be a positive integer`);
  }
}
