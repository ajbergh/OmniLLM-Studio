import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { SHAPE_STATE_CONTRACT_V1 } from '../../video/renderContractShape';
import type {
  VideoTimelineScene,
  VideoTimelineShape,
  VideoTimelineTransform,
} from '../../types/video';

const SHAPE_PARENT_EPSILON = 1e-9;
const SUPPORTED_TRANSFORM_KEYS = new Set([
  'x', 'y', 'scale', 'scale_x', 'scale_y', 'rotation', 'rotation_z', 'opacity', 'crop',
  'z', 'rotation_x', 'rotation_y', 'anchor_x', 'anchor_y', 'perspective',
]);

export interface PreviewShapePlaybackContext {
  canvasWidth: number;
  canvasHeight: number;
  scenes: readonly Pick<VideoTimelineScene, 'start_ms' | 'duration_ms' | 'camera'>[];
}

/** Structural clip shape used only by the normal-playback admission classifier. */
export interface PreviewShapePlaybackClip {
  id: string;
  asset_id?: string;
  start_ms?: number;
  duration_ms?: number;
  audio_only?: boolean;
  text?: unknown;
  shape?: VideoTimelineShape;
  cursor?: unknown;
  transform?: VideoTimelineTransform;
  fade_in_ms?: number;
  fade_out_ms?: number;
  effects?: readonly { enabled?: boolean }[];
  transitions?: readonly unknown[];
  keyframes?: readonly unknown[];
  animation_blocks?: readonly unknown[];
}

export type PreviewShapePlaybackLayer = {
  clip: PreviewShapePlaybackClip;
  canonicalState?: Pick<CanonicalFrameLayerState, 'shape'>;
};

/** True for authored shape metadata, including kinds that remain compatibility-only. */
export function hasPreviewShapePlaybackMetadata(layer: PreviewShapePlaybackLayer): boolean {
  return Boolean(layer.clip.shape);
}

/**
 * Return the first structural reason that prevents normal playback from claiming
 * canonical rounded-rectangle authority. This intentionally mirrors
 * canonicalRoundedRectangleRasterClip in the FidelityRenderer: only a standalone
 * static-2D rounded_rectangle with no crop/fades/effects/transitions/keyframes/
 * animation, no scene-camera overlap, a uniform positive parent scale, bounded
 * opacity, canvas-contained canonical geometry, and renderer-supported colors is
 * admitted. All other annotation kinds remain on the compatibility painter.
 *
 * The classifier never mutates authored timeline data or canonical FrameState.
 */
export function previewShapePlaybackStructuralDeferredReason(
  layer: PreviewShapePlaybackLayer,
  context: PreviewShapePlaybackContext,
): string | undefined {
  const { clip } = layer;
  const shape = clip.shape;
  if (!shape) return undefined;

  if (shape.kind.trim().toLowerCase() !== 'rounded_rectangle') return `${clip.id}:shape-kind-unsupported`;
  if (clip.asset_id || clip.text || clip.cursor || clip.audio_only) return `${clip.id}:standalone-shape-required`;

  const durationMS = clip.duration_ms ?? 0;
  if (!Number.isFinite(durationMS) || durationMS <= 0) return `${clip.id}:duration-invalid`;
  if ((clip.fade_in_ms ?? 0) > 0 || (clip.fade_out_ms ?? 0) > 0) return `${clip.id}:fade-unsupported`;
  if ((clip.transitions?.length ?? 0) > 0) return `${clip.id}:transition-parent-unsupported`;
  if ((clip.animation_blocks?.length ?? 0) > 0) return `${clip.id}:animation-parent-unsupported`;
  if (clip.effects?.some((effect) => effect.enabled === true)) return `${clip.id}:effect-parent-unsupported`;
  if ((clip.keyframes?.length ?? 0) > 0) return `${clip.id}:keyframe-parent-unsupported`;
  if (hasOverlappingSceneCamera(clip, context.scenes)) return `${clip.id}:scene-camera-unsupported`;

  const parentReason = shapeParentTransformDeferredReason(clip.transform);
  if (parentReason) return `${clip.id}:${parentReason}`;

  const canonicalShape = layer.canonicalState?.shape;
  if (!canonicalShape) return `${clip.id}:canonical-shape-state-unavailable`;
  if (canonicalShape.contract_version !== SHAPE_STATE_CONTRACT_V1 || canonicalShape.kind !== 'rounded_rectangle') {
    return `${clip.id}:canonical-shape-contract-mismatch`;
  }
  if (!Number.isFinite(context.canvasWidth) || !Number.isFinite(context.canvasHeight)
    || context.canvasWidth < 2 || context.canvasHeight < 2) {
    return `${clip.id}:canvas-invalid`;
  }
  if (canonicalShape.width > context.canvasWidth || canonicalShape.height > context.canvasHeight) {
    return `${clip.id}:shape-raster-out-of-bounds`;
  }
  if (!shapeRasterColorSupported(canonicalShape.fill)) return `${clip.id}:fill-color-unsupported`;
  if (canonicalShape.stroke.trim() !== '' && !shapeRasterColorSupported(canonicalShape.stroke)) {
    return `${clip.id}:stroke-color-unsupported`;
  }
  return undefined;
}

function shapeParentTransformDeferredReason(transform: VideoTimelineTransform | undefined): string | undefined {
  if (!transform) return undefined;
  const values = transform as unknown as Record<string, unknown>;
  for (const [key, raw] of Object.entries(values)) {
    if (!SUPPORTED_TRANSFORM_KEYS.has(key)) return 'parent-transform-unsupported';
    if (key === 'crop') {
      if (raw !== undefined && raw !== null) return 'crop-unsupported';
      continue;
    }
    if (typeof raw !== 'number' || !Number.isFinite(raw)) return 'parent-transform-unsupported';
    if (['z', 'rotation_x', 'rotation_y', 'anchor_x', 'anchor_y', 'perspective'].includes(key)
      && Math.abs(raw) > SHAPE_PARENT_EPSILON) return 'parent-transform-unsupported';
  }

  const scale = transform.scale ?? 1;
  const scaleX = transform.scale_x ?? scale;
  const scaleY = transform.scale_y ?? scale;
  const opacity = transform.opacity ?? 1;
  if (!Number.isFinite(scaleX) || !Number.isFinite(scaleY)
    || Math.abs(scaleX - scaleY) > SHAPE_PARENT_EPSILON
    || scaleX <= 0) return 'parent-transform-unsupported';
  if (!Number.isFinite(opacity) || opacity < 0 || opacity > 1) return 'parent-opacity-unsupported';
  return undefined;
}

function hasOverlappingSceneCamera(
  clip: PreviewShapePlaybackClip,
  scenes: readonly Pick<VideoTimelineScene, 'start_ms' | 'duration_ms' | 'camera'>[],
): boolean {
  const start = clip.start_ms ?? 0;
  const end = start + (clip.duration_ms ?? 0);
  return scenes.some((scene) => Boolean(scene.camera)
    && scene.duration_ms > 0
    && scene.start_ms < end
    && scene.start_ms + scene.duration_ms > start);
}

/** Match backend parseShapeRasterColor without accepting broader browser CSS. */
export function shapeRasterColorSupported(value: string): boolean {
  const source = value.trim().toLowerCase();
  if (source === '' || source === 'transparent') return true;
  if (source.startsWith('#')) {
    const hex = source.slice(1);
    return [3, 4, 6, 8].includes(hex.length) && /^[0-9a-f]+$/.test(hex);
  }
  const match = source.match(/^(rgba?)\((.*)\)$/);
  if (!match) return false;
  const parts = match[2].split(',').map((part) => part.trim());
  if (parts.length !== 3 && parts.length !== 4) return false;
  for (let index = 0; index < 3; index += 1) {
    const channel = Number(parts[index]);
    if (!Number.isFinite(channel) || channel < 0 || channel > 255) return false;
  }
  if (parts.length === 4) {
    const alpha = Number(parts[3]);
    if (!Number.isFinite(alpha) || alpha < 0 || alpha > 1) return false;
  }
  return true;
}
