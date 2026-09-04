import { endFrame, startFrame } from '../../video/renderContract';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type {
  VideoTimelineClip,
  VideoTimelineScene,
  VideoTimelineTrack,
  VideoTimelineTransform,
} from '../../types/video';

export const PREVIEW_CURSOR_MAX_EXACT_FPS = 999;
export const PREVIEW_CURSOR_MAX_SEGMENTS_PER_CLIP = 300;

const CURSOR_POINTER_BASE_SIZE = 64;
const CURSOR_HIGHLIGHT_RADIUS_FACTOR = 1.1;
const CURSOR_CLICK_RING_RADIUS_FACTOR = 1.3;
const CURSOR_PARENT_EPSILON = 1e-9;
const VISUAL_KEYFRAME_PROPERTIES = new Set([
  'x', 'y', 'z',
  'scale', 'scale_x', 'scale_y',
  'rotation', 'rotation_x', 'rotation_y', 'rotation_z',
  'opacity',
]);
const SUPPORTED_TRANSFORM_KEYS = new Set([
  'x', 'y', 'scale', 'scale_x', 'scale_y', 'rotation', 'rotation_z', 'opacity', 'crop',
  'z', 'rotation_x', 'rotation_y', 'anchor_x', 'anchor_y', 'perspective',
]);

export interface PreviewCursorPlaybackContext {
  fps: number;
  canvasWidth: number;
  canvasHeight: number;
  scenes: readonly Pick<VideoTimelineScene, 'start_ms' | 'duration_ms' | 'camera'>[];
}

export type PreviewCursorPlaybackLayer = {
  clip: Pick<VideoTimelineClip, 'id'> & Partial<Pick<
    VideoTimelineClip,
    | 'asset_id'
    | 'start_ms'
    | 'duration_ms'
    | 'audio_only'
    | 'text'
    | 'shape'
    | 'cursor'
    | 'transform'
    | 'fade_in_ms'
    | 'fade_out_ms'
    | 'effects'
    | 'transitions'
    | 'keyframes'
    | 'animation_blocks'
  >>;
  track?: Pick<VideoTimelineTrack, 'clips'>;
  asset?: { mime_type: string };
  canonicalState?: Pick<CanonicalFrameLayerState, 'cursor'>;
};

/** True only when authored cursor metadata has at least one sampled event. */
export function hasPreviewCursorPlaybackMetadata(layer: PreviewCursorPlaybackLayer): boolean {
  return Boolean(layer.clip.cursor?.events?.length);
}

/**
 * Return the first structural reason that prevents normal playback from claiming
 * canonical cursor authority. The checks deliberately mirror the supported
 * FidelityRenderer cursor-raster subset rather than the broader editor schema:
 * one media owner, <=999 fps and <=300 exact frame segments, static 2D parent,
 * no fades/effects/transitions/animation/visual keyframes, no overlapping
 * same-track sibling, no overlapping scene camera, and bounded cursor raster.
 *
 * This is a browser admission rule only. It never mutates cursor-state-v1 or
 * persisted timeline data.
 */
export function previewCursorPlaybackStructuralDeferredReason(
  layer: PreviewCursorPlaybackLayer,
  context: PreviewCursorPlaybackContext,
): string | undefined {
  const { clip } = layer;
  const cursor = clip.cursor;
  if (!cursor?.events?.length) return undefined;

  if (!layer.asset || (!layer.asset.mime_type.startsWith('video/') && !layer.asset.mime_type.startsWith('image/'))) {
    return `${clip.id}:media-owner-required`;
  }
  if (!clip.asset_id || clip.text || clip.shape || clip.audio_only) return `${clip.id}:media-owner-required`;
  if (!Number.isFinite(context.fps) || context.fps <= 0 || context.fps > PREVIEW_CURSOR_MAX_EXACT_FPS) {
    return `${clip.id}:fps-out-of-range`;
  }
  const startMS = clip.start_ms ?? 0;
  const durationMS = clip.duration_ms ?? 0;
  if (!Number.isFinite(startMS) || !Number.isFinite(durationMS) || durationMS <= 0) {
    return `${clip.id}:duration-invalid`;
  }
  const segments = endFrame(startMS + durationMS, context.fps) - startFrame(startMS, context.fps);
  if (segments <= 0 || segments > PREVIEW_CURSOR_MAX_SEGMENTS_PER_CLIP) {
    return `${clip.id}:segment-bound-${segments}`;
  }
  if (cursor.visible === false) return `${clip.id}:hidden-cursor-not-export-proven`;
  if (cursor.smoothing === true) return `${clip.id}:smoothing-unsupported`;
  if ((clip.fade_in_ms ?? 0) > 0 || (clip.fade_out_ms ?? 0) > 0) return `${clip.id}:fade-unsupported`;
  if ((clip.transitions?.length ?? 0) > 0) return `${clip.id}:transition-parent-unsupported`;
  if ((clip.animation_blocks?.length ?? 0) > 0) return `${clip.id}:animation-parent-unsupported`;
  if (clip.effects?.some((effect) => effect.enabled)) return `${clip.id}:effect-parent-unsupported`;
  if (clip.keyframes?.some((keyframe) => VISUAL_KEYFRAME_PROPERTIES.has(keyframe.property.trim().toLowerCase()))) {
    return `${clip.id}:animated-parent-unsupported`;
  }
  if (!cursorParentTransformSupported(clip.transform)) return `${clip.id}:parent-transform-unsupported`;
  if (!layer.track) return `${clip.id}:track-unavailable`;
  if (hasOverlappingSibling(clip, layer.track.clips)) return `${clip.id}:same-track-overlap-unsupported`;
  if (hasOverlappingSceneCamera(clip, context.scenes)) return `${clip.id}:scene-camera-unsupported`;

  const scale = cursor.scale && cursor.scale > 0 ? cursor.scale : 1;
  if (!Number.isFinite(scale)) return `${clip.id}:cursor-scale-invalid`;
  if (!cursorRasterFitsCanvas(
    context.canvasWidth,
    context.canvasHeight,
    scale,
    cursor.highlight === true,
    cursor.click_rings === true,
  )) {
    return `${clip.id}:cursor-raster-out-of-bounds`;
  }
  if (!layer.canonicalState?.cursor) return `${clip.id}:canonical-cursor-state-unavailable`;
  return undefined;
}

function cursorParentTransformSupported(transform: VideoTimelineTransform | undefined): boolean {
  if (!transform) return true;
  for (const [key, value] of Object.entries(transform as Record<string, unknown>)) {
    if (!SUPPORTED_TRANSFORM_KEYS.has(key)) return false;
    if (key === 'crop') continue;
    if (typeof value !== 'number' || !Number.isFinite(value)) return false;
    if (['z', 'rotation_x', 'rotation_y', 'anchor_x', 'anchor_y', 'perspective'].includes(key)
      && Math.abs(value) > CURSOR_PARENT_EPSILON) return false;
  }
  const scale = transform.scale ?? 1;
  const scaleX = transform.scale_x ?? scale;
  const scaleY = transform.scale_y ?? scale;
  return Number.isFinite(scaleX)
    && Number.isFinite(scaleY)
    && Math.abs(scaleX - scaleY) <= CURSOR_PARENT_EPSILON;
}

function hasOverlappingSibling(
  clip: PreviewCursorPlaybackLayer['clip'],
  siblings: readonly VideoTimelineClip[],
): boolean {
  const start = clip.start_ms ?? 0;
  const duration = clip.duration_ms ?? 0;
  const end = start + duration;
  return siblings.some((sibling) => sibling.id !== clip.id
    && sibling.duration_ms > 0
    && !sibling.audio_only
    && sibling.start_ms < end
    && sibling.start_ms + sibling.duration_ms > start);
}

function hasOverlappingSceneCamera(
  clip: PreviewCursorPlaybackLayer['clip'],
  scenes: readonly Pick<VideoTimelineScene, 'start_ms' | 'duration_ms' | 'camera'>[],
): boolean {
  const start = clip.start_ms ?? 0;
  const end = start + (clip.duration_ms ?? 0);
  return scenes.some((scene) => Boolean(scene.camera)
    && scene.duration_ms > 0
    && scene.start_ms < end
    && scene.start_ms + scene.duration_ms > start);
}

function cursorRasterFitsCanvas(
  width: number,
  height: number,
  scale: number,
  highlight: boolean,
  clickRings: boolean,
): boolean {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width < 2 || height < 2 || scale <= 0) return false;
  const size = CURSOR_POINTER_BASE_SIZE * scale;
  let negativeExtent = 0;
  let positiveX = 50 * scale;
  let positiveY = 58 * scale;
  if (highlight) {
    const radius = CURSOR_HIGHLIGHT_RADIUS_FACTOR * size;
    negativeExtent = Math.max(negativeExtent, radius);
    positiveX = Math.max(positiveX, radius);
    positiveY = Math.max(positiveY, radius);
  }
  if (clickRings) {
    const radius = CURSOR_CLICK_RING_RADIUS_FACTOR * size;
    negativeExtent = Math.max(negativeExtent, radius);
    positiveX = Math.max(positiveX, radius);
    positiveY = Math.max(positiveY, radius);
  }
  const centerX = width / 2;
  const centerY = height / 2;
  return negativeExtent + 2 <= centerX
    && negativeExtent + 2 <= centerY
    && positiveX + 2 <= width - centerX
    && positiveY + 2 <= height - centerY;
}
