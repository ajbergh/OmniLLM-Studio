import type { CSSProperties } from 'react';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  MEDIA_GEOMETRY_CONTRACT_V1,
  type CanonicalMediaGeometry,
} from '../../video/renderContractMediaGeometry';
import type { TimelineV2ContentBounds } from '../../video/renderContractTypes';

/**
 * Resolve canonical media geometry only for deterministic, non-interactive
 * preview frames. Free-running playback and direct-manipulation/crop gestures
 * intentionally retain the established browser painter until those paths are
 * canonicalized separately.
 */
export function resolveCanonicalPreviewMediaGeometry(
  deterministicFrame: number | null,
  state: CanonicalFrameLayerState | undefined,
  isMedia: boolean,
  hasLiveOverride: boolean,
  inCropEdit: boolean,
): CanonicalMediaGeometry | null {
  if (deterministicFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;
  const geometry = state?.media_geometry;
  if (!geometry || geometry.contract_version !== MEDIA_GEOMETRY_CONTRACT_V1) return null;
  if (!isPositiveBounds(geometry.viewport_bounds)
    || !isPositiveBounds(geometry.source_bounds)
    || !isPositiveBounds(geometry.visible_source_bounds)
    || !isPositiveBounds(geometry.painted_bounds)
    || !isPositiveBounds(geometry.clip_bounds)) return null;
  if (!Number.isFinite(geometry.scale_x) || geometry.scale_x <= 0
    || !Number.isFinite(geometry.scale_y) || geometry.scale_y <= 0) return null;
  if (!containsBounds(geometry.source_bounds, geometry.visible_source_bounds)) return null;
  if (!containsBounds(geometry.viewport_bounds, geometry.clip_bounds)) return null;
  return geometry;
}

/** Position decoded media into the canonical canvas-space painted rectangle. */
export function canonicalPreviewMediaElementStyle(
  geometry: CanonicalMediaGeometry,
  stageScale: number,
): CSSProperties {
  const scale = safeStageScale(stageScale);
  return {
    position: 'absolute',
    left: geometry.painted_bounds.x * scale,
    top: geometry.painted_bounds.y * scale,
    width: geometry.painted_bounds.width * scale,
    height: geometry.painted_bounds.height * scale,
    maxWidth: 'none',
    maxHeight: 'none',
    objectFit: 'fill',
  };
}

/**
 * Convert canonical canvas-space clip bounds into an inset path relative to
 * the full-stage viewport. The surrounding viewport should also use
 * `overflow: hidden` so cover/none paint outside the canvas is clipped before
 * the layer transform is applied.
 */
export function canonicalPreviewMediaClipPath(
  geometry: CanonicalMediaGeometry,
  stageScale: number,
): string {
  const scale = safeStageScale(stageScale);
  const viewport = geometry.viewport_bounds;
  const clip = geometry.clip_bounds;
  const top = (clip.y - viewport.y) * scale;
  const left = (clip.x - viewport.x) * scale;
  const right = (viewport.x + viewport.width - clip.x - clip.width) * scale;
  const bottom = (viewport.y + viewport.height - clip.y - clip.height) * scale;
  return `inset(${cleanZero(top)}px ${cleanZero(right)}px ${cleanZero(bottom)}px ${cleanZero(left)}px)`;
}

function isPositiveBounds(bounds: TimelineV2ContentBounds): boolean {
  return [bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite)
    && bounds.width > 0
    && bounds.height > 0;
}

function containsBounds(outer: TimelineV2ContentBounds, inner: TimelineV2ContentBounds): boolean {
  const epsilon = 1e-9;
  return inner.x + epsilon >= outer.x
    && inner.y + epsilon >= outer.y
    && inner.x + inner.width <= outer.x + outer.width + epsilon
    && inner.y + inner.height <= outer.y + outer.height + epsilon;
}

function safeStageScale(stageScale: number): number {
  return Number.isFinite(stageScale) && stageScale > 0 ? stageScale : 1;
}

function cleanZero(value: number): number {
  return Math.abs(value) < 1e-9 ? 0 : value;
}
