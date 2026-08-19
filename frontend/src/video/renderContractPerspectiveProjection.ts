import type { Matrix4 } from './renderContractFrameState';

export const PERSPECTIVE_PROJECTION_CONTRACT_V1 = 'perspective-projection-v1' as const;

export interface PerspectiveProjectionCamera {
  perspective_distance: number;
}

export interface PerspectiveProjectionTransform {
  z: number;
  perspective?: number;
}

export interface CanonicalPerspectiveProjection {
  contract_version: typeof PERSPECTIVE_PROJECTION_CONTRACT_V1;
  distance: number;
  source: 'camera' | 'clip';
  origin_w: number;
  matrix: Matrix4;
}

/**
 * Resolve the canonical CSS-style homogeneous perspective matrix after the
 * camera-relative layer model transform. Positive clip perspective overrides
 * camera distance; zero/omitted clip perspective inherits the camera value.
 */
export function evaluatePerspectiveProjection(
  camera: PerspectiveProjectionCamera,
  view: PerspectiveProjectionTransform,
): CanonicalPerspectiveProjection {
  const usesClip = view.perspective !== undefined && view.perspective !== 0;
  const distance = usesClip ? (view.perspective as number) : camera.perspective_distance;
  if (!Number.isFinite(distance) || distance <= 0) {
    throw new Error('perspective distance must be finite and positive');
  }
  if (!Number.isFinite(view.z)) {
    throw new Error('camera-relative z must be finite');
  }
  const matrix: Matrix4 = [
    1, 0, 0, 0,
    0, 1, 0, 0,
    0, 0, 1, 0,
    0, 0, -1 / distance, 1,
  ];
  return {
    contract_version: PERSPECTIVE_PROJECTION_CONTRACT_V1,
    distance,
    source: usesClip ? 'clip' : 'camera',
    origin_w: 1 - view.z / distance,
    matrix,
  };
}
