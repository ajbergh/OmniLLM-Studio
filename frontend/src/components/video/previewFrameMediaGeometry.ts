import type { CSSProperties } from 'react';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  MEDIA_GEOMETRY_CONTRACT_V1,
  type CanonicalMediaGeometry,
} from '../../video/renderContractMediaGeometry';
import type { TimelineV2ContentBounds } from '../../video/renderContractTypes';

type CanonicalMediaGeometryState = Pick<CanonicalFrameLayerState, 'media_geometry'>;

/**
 * Resolve canonical media placement for an admitted canonical visual frame.
 * Deterministic capture and media-only normal playback may both consume it;
 * direct manipulation/crop editing retain the established browser painter.
 */
export function resolveCanonicalPreviewMediaGeometry(
  canonicalFrame: number | null,
  state: CanonicalMediaGeometryState | undefined,
  isMedia: boolean,
  hasLiveOverride: boolean,
  inCropEdit: boolean,
): CanonicalMediaGeometry | null {
  if (canonicalFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;
  const geometry = state?.media_geometry;
  if (!geometry || geometry.contract_version !== MEDIA_GEOMETRY_CONTRACT_V1) return null;
  return validCanonicalMediaGeometry(geometry) ? geometry : null;
}

/**
 * Place a replaced media element directly from canonical canvas-space painted
 * bounds. The browser must not re-run contain/cover semantics after FrameState
 * has already decided the output rectangle.
 */
export function canonicalPreviewMediaElementStyle(
  geometry: CanonicalMediaGeometry,
  stageScale: number,
): CSSProperties {
  validateStageScale(stageScale);
  const viewport = geometry.viewport_bounds;
  const bounds = geometry.painted_bounds;
  return {
    position: 'absolute',
    left: (bounds.x - viewport.x) * stageScale,
    top: (bounds.y - viewport.y) * stageScale,
    width: bounds.width * stageScale,
    height: bounds.height * stageScale,
    maxWidth: 'none',
    maxHeight: 'none',
    objectFit: 'fill',
  };
}

/**
 * Express canonical output crop as a full-stage clip path. `clip_bounds` are in
 * canvas coordinates, so this deliberately avoids element-relative percentage
 * crop semantics when canonical media geometry is active.
 */
export function canonicalPreviewMediaClipPath(
  geometry: CanonicalMediaGeometry,
  stageScale: number,
): string {
  validateStageScale(stageScale);
  const viewport = geometry.viewport_bounds;
  const clip = geometry.clip_bounds;
  const top = (clip.y - viewport.y) * stageScale;
  const left = (clip.x - viewport.x) * stageScale;
  const right = (viewport.x + viewport.width - clip.x - clip.width) * stageScale;
  const bottom = (viewport.y + viewport.height - clip.y - clip.height) * stageScale;
  return `inset(${top}px ${right}px ${bottom}px ${left}px)`;
}

function validCanonicalMediaGeometry(geometry: CanonicalMediaGeometry): boolean {
  if (!positiveBounds(geometry.viewport_bounds)
    || !positiveBounds(geometry.source_bounds)
    || !positiveBounds(geometry.visible_source_bounds)
    || !positiveBounds(geometry.painted_bounds)
    || !positiveBounds(geometry.clip_bounds)) {
    return false;
  }
  if (!Number.isFinite(geometry.scale_x) || geometry.scale_x <= 0
    || !Number.isFinite(geometry.scale_y) || geometry.scale_y <= 0) {
    return false;
  }
  return containsBounds(geometry.source_bounds, geometry.visible_source_bounds)
    && containsBounds(geometry.viewport_bounds, geometry.clip_bounds);
}

function positiveBounds(bounds: TimelineV2ContentBounds): boolean {
  return [bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite)
    && bounds.width > 0
    && bounds.height > 0;
}

function containsBounds(outer: TimelineV2ContentBounds, inner: TimelineV2ContentBounds): boolean {
  const epsilon = 1e-9;
  return inner.x >= outer.x - epsilon
    && inner.y >= outer.y - epsilon
    && inner.x + inner.width <= outer.x + outer.width + epsilon
    && inner.y + inner.height <= outer.y + outer.height + epsilon;
}

function validateStageScale(stageScale: number): void {
  if (!Number.isFinite(stageScale) || stageScale < 0) {
    throw new Error('preview stage scale must be finite and non-negative');
  }
}
