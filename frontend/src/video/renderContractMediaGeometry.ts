import type {
  TimelineV2Canvas,
  TimelineV2Clip,
  TimelineV2ContentBounds,
  TimelineV2Crop,
} from './renderContractTypes';

export const MEDIA_GEOMETRY_CONTRACT_V1 = 'media-geometry-v1' as const;

export interface CanonicalMediaGeometry {
  contract_version: typeof MEDIA_GEOMETRY_CONTRACT_V1;
  fit: 'contain' | 'cover' | 'fill' | 'none';
  viewport_bounds: TimelineV2ContentBounds;
  source_bounds: TimelineV2ContentBounds;
  visible_source_bounds: TimelineV2ContentBounds;
  painted_bounds: TimelineV2ContentBounds;
  clip_bounds: TimelineV2ContentBounds;
  scale_x: number;
  scale_y: number;
}

/**
 * Resolve renderer-independent media placement. Source/content bounds are
 * required because the source aspect ratio is semantic input and must not be
 * inferred from the output canvas.
 */
export function evaluateMediaGeometry(canvas: TimelineV2Canvas, clip: TimelineV2Clip): CanonicalMediaGeometry {
  if (canvas.width < 1 || canvas.height < 1) throw new Error('media geometry requires a positive canvas');
  if (!clip.content_bounds) throw new Error('media geometry requires explicit content_bounds');
  const source = { ...clip.content_bounds };
  validateGeometryBounds(source);
  const fit = (clip.media_fit || 'contain').trim().toLowerCase();
  if (fit !== 'contain' && fit !== 'cover' && fit !== 'fill' && fit !== 'none') {
    throw new Error(`unsupported media_fit ${JSON.stringify(clip.media_fit)}`);
  }
  const visible = clip.mask_source_crop ? cropBounds(source, clip.mask_source_crop) : source;
  const viewport: TimelineV2ContentBounds = { x: 0, y: 0, width: canvas.width, height: canvas.height };
  let scaleX = 1;
  let scaleY = 1;
  if (fit === 'contain') {
    const scale = Math.min(viewport.width / visible.width, viewport.height / visible.height);
    scaleX = scale;
    scaleY = scale;
  } else if (fit === 'cover') {
    const scale = Math.max(viewport.width / visible.width, viewport.height / visible.height);
    scaleX = scale;
    scaleY = scale;
  } else if (fit === 'fill') {
    scaleX = viewport.width / visible.width;
    scaleY = viewport.height / visible.height;
  }
  const paintedWidth = visible.width * scaleX;
  const paintedHeight = visible.height * scaleY;
  const painted: TimelineV2ContentBounds = {
    x: (viewport.width - paintedWidth) / 2,
    y: (viewport.height - paintedHeight) / 2,
    width: paintedWidth,
    height: paintedHeight,
  };
  const clipBounds = clip.transform?.crop ? cropBounds(viewport, clip.transform.crop) : viewport;
  return {
    contract_version: MEDIA_GEOMETRY_CONTRACT_V1,
    fit,
    viewport_bounds: viewport,
    source_bounds: source,
    visible_source_bounds: visible,
    painted_bounds: painted,
    clip_bounds: clipBounds,
    scale_x: scaleX,
    scale_y: scaleY,
  };
}

function validateGeometryBounds(bounds: TimelineV2ContentBounds): void {
  for (const value of [bounds.x, bounds.y, bounds.width, bounds.height]) {
    if (!Number.isFinite(value)) throw new Error('content_bounds: bounds must be finite');
  }
  if (bounds.width <= 0 || bounds.height <= 0) throw new Error('content_bounds: width and height must be positive');
}

function cropBounds(bounds: TimelineV2ContentBounds, crop: TimelineV2Crop): TimelineV2ContentBounds {
  for (const value of [crop.top, crop.right, crop.bottom, crop.left]) {
    if (!Number.isFinite(value) || value < 0 || value > 1) {
      throw new Error('crop edges must be finite and between 0 and 1');
    }
  }
  if (crop.left + crop.right >= 1 || crop.top + crop.bottom >= 1) {
    throw new Error('crop edges must leave a positive visible area');
  }
  return {
    x: bounds.x + bounds.width * crop.left,
    y: bounds.y + bounds.height * crop.top,
    width: bounds.width * (1 - crop.left - crop.right),
    height: bounds.height * (1 - crop.top - crop.bottom),
  };
}
