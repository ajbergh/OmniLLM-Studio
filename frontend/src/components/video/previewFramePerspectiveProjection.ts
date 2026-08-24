import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';

type CanonicalPerspectiveState = Pick<CanonicalFrameLayerState, 'perspective_projection'>;

/**
 * Decide whether one deterministic preview frame can leave the legacy shared
 * stage perspective and consume canonical per-layer projection instead.
 *
 * Canonical preview evaluation is all-or-fallback for visual layers, but the
 * explicit all-layer check keeps this consumer fail-closed if that invariant is
 * ever relaxed. Any in-flight direct-manipulation gesture also keeps the whole
 * frame on the established shared-stage path so interaction geometry cannot mix
 * projection contexts mid-gesture.
 */
export function shouldUseCanonicalPreviewPerspective(
  deterministicFrame: number | null,
  canonicalStates: Array<CanonicalPerspectiveState | undefined>,
  hasLiveOverride: boolean,
): boolean {
  if (deterministicFrame === null || hasLiveOverride) return false;
  return canonicalStates.every((state) => {
    const distance = state?.perspective_projection.distance;
    return typeof distance === 'number' && Number.isFinite(distance) && distance > 0;
  });
}

/** Return the canonical canvas-pixel perspective distance for one layer. */
export function resolveCanonicalPreviewPerspectiveDistance(
  canonicalState: CanonicalPerspectiveState | undefined,
  useCanonicalPerspective: boolean,
): number | null {
  if (!useCanonicalPerspective || !canonicalState) return null;
  const distance = canonicalState.perspective_projection.distance;
  return Number.isFinite(distance) && distance > 0 ? distance : null;
}

/**
 * Scale a canonical canvas-pixel perspective distance into preview CSS pixels.
 * CSS perspective values below 1px compute as 1px, so make that browser floor
 * explicit rather than introducing the legacy stage's much larger 100px clamp.
 */
export function canonicalPreviewPerspectiveCSSPixels(distance: number, stageScale: number): number {
  if (!Number.isFinite(distance) || distance <= 0) {
    throw new Error('canonical perspective distance must be finite and positive');
  }
  if (!Number.isFinite(stageScale) || stageScale < 0) {
    throw new Error('preview stage scale must be finite and non-negative');
  }
  return Math.max(1, distance * stageScale);
}
