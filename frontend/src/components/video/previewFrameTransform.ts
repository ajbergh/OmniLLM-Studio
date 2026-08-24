import type { VideoTimelineClip, VideoTimelineTransform } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { evaluateClipProperty } from '../../video/renderContractProperties';

export interface ResolvedPreviewFrameTransform {
  transform: VideoTimelineTransform;
  /** Canonical FrameState opacity already includes clip fade-in/fade-out. */
  opacityIncludesClipFades: boolean;
}

/**
 * Resolve the transform values consumed by the existing preview painter.
 *
 * Explicit frame-addressed canonical state owns deterministic transform and
 * opacity decisions. Interactive/free-running playback keeps the established
 * property evaluator. An in-flight direct-manipulation gesture deliberately
 * bypasses canonical state so the existing responsive live overlay remains the
 * interaction authority until the gesture commits.
 */
export function resolvePreviewFrameTransform(
  clip: VideoTimelineClip,
  clipTimeMs: number,
  canonicalState: Pick<CanonicalFrameLayerState, 'transform'> | undefined,
  hasLiveOverride: boolean,
): ResolvedPreviewFrameTransform {
  if (canonicalState && !hasLiveOverride) {
    const canonical = canonicalState.transform;
    return {
      transform: {
        x: canonical.x,
        y: canonical.y,
        z: canonical.z,
        scale: clip.transform?.scale ?? 1,
        scale_x: canonical.scale_x,
        scale_y: canonical.scale_y,
        rotation: canonical.rotation_z,
        rotation_x: canonical.rotation_x,
        rotation_y: canonical.rotation_y,
        rotation_z: canonical.rotation_z,
        opacity: canonical.opacity,
        anchor_x: canonical.anchor_x,
        anchor_y: canonical.anchor_y,
        ...(canonical.perspective !== undefined ? { perspective: canonical.perspective } : {}),
        ...(canonical.crop ? { crop: { ...canonical.crop } } : {}),
      },
      opacityIncludesClipFades: true,
    };
  }

  const transform: VideoTimelineTransform = {
    x: 0,
    y: 0,
    scale: 1,
    rotation: 0,
    opacity: 1,
    ...(clip.transform || {}),
  };
  for (const property of ['x', 'y', 'z', 'scale', 'scale_x', 'scale_y', 'rotation', 'rotation_x', 'rotation_y', 'rotation_z', 'opacity'] as const) {
    transform[property] = evaluateClipProperty(clip, property, clipTimeMs);
  }
  return { transform, opacityIncludesClipFades: false };
}
