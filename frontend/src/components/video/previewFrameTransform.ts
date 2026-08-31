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
 * Frame-addressed canonical state owns transform and opacity decisions for both
 * deterministic capture and admitted media-only normal-playback frames. Missing
 * canonical projection retains the established time-domain property evaluator.
 * An in-flight direct-manipulation gesture deliberately bypasses canonical state
 * so the responsive live overlay remains the interaction authority until commit.
 */
export function resolvePreviewFrameTransform(
  clip: VideoTimelineClip,
  clipTimeMs: number,
  canonicalState: Pick<CanonicalFrameLayerState, 'transform'> | undefined,
  hasLiveOverride: boolean,
): ResolvedPreviewFrameTransform {
  if (canonicalState && !hasLiveOverride) {
    const canonical = canonicalState.transform;
    // The legacy CSS painter still uses `scale_x || scale` / `scale_y || scale`.
    // Preserve an exact canonical zero axis until that painter is migrated to
    // nullish axis selection; non-zero canonical axes remain authoritative.
    const compatibilityScale = canonical.scale_x === 0 || canonical.scale_y === 0
      ? 0
      : (clip.transform?.scale ?? 1);
    return {
      transform: {
        x: canonical.x,
        y: canonical.y,
        z: canonical.z,
        scale: compatibilityScale,
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
