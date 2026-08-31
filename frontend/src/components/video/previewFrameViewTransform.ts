import type { VideoTimelineTransform } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';

export interface PreviewCameraTransform {
  x: number;
  y: number;
  z: number;
  rotation_x: number;
  rotation_y: number;
  rotation_z: number;
}

export interface ResolvedPreviewViewTransform {
  x: number;
  y: number;
  z: number;
  rotation_x: number;
  rotation_y: number;
  rotation_z: number;
}

/**
 * Resolve the camera-relative transform consumed by the current preview painter.
 *
 * Canonical visual frames already contain `view_transform`, so the preview must
 * not independently subtract/interpolate camera state again, including admitted
 * media-only normal playback. Established local camera subtraction remains the
 * compatibility path for unavailable projection and live direct manipulation.
 */
export function resolvePreviewFrameViewTransform(
  transform: VideoTimelineTransform,
  canonicalState: Pick<CanonicalFrameLayerState, 'view_transform'> | undefined,
  camera: PreviewCameraTransform,
  hasLiveOverride: boolean,
): ResolvedPreviewViewTransform {
  if (canonicalState && !hasLiveOverride) {
    const view = canonicalState.view_transform;
    return {
      x: view.x,
      y: view.y,
      z: view.z,
      rotation_x: view.rotation_x,
      rotation_y: view.rotation_y,
      rotation_z: view.rotation_z,
    };
  }

  return {
    x: (transform.x ?? 0) - camera.x,
    y: (transform.y ?? 0) - camera.y,
    z: (transform.z ?? 0) - camera.z,
    rotation_x: (transform.rotation_x ?? 0) - camera.rotation_x,
    rotation_y: (transform.rotation_y ?? 0) - camera.rotation_y,
    rotation_z: (transform.rotation_z ?? transform.rotation ?? 0) - camera.rotation_z,
  };
}
