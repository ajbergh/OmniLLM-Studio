import { curveProgress } from './renderContract';
import type { TimelineV2Camera, TimelineV2Clip, TimelineV2MotionCurve } from './renderContractTypes';

/** Minimal renderer-independent keyframe shape used by canonical interpolation. */
export interface CanonicalPropertyKeyframe {
  property: string;
  time_ms: number;
  value: number;
  easing?: string;
  curve?: TimelineV2MotionCurve;
}

export const canonicalClipProperties = [
  'x', 'y', 'z',
  'scale', 'scale_x', 'scale_y',
  'rotation', 'rotation_x', 'rotation_y', 'rotation_z',
  'opacity', 'volume',
] as const;
export type CanonicalClipProperty = typeof canonicalClipProperties[number];

export const canonicalCameraProperties = [
  'x', 'y', 'z',
  'rotation_x', 'rotation_y', 'rotation_z',
  'field_of_view', 'focus_depth',
] as const;
export type CanonicalCameraProperty = typeof canonicalCameraProperties[number];

const clipPropertySet = new Set<string>(canonicalClipProperties);
const cameraPropertySet = new Set<string>(canonicalCameraProperties);

function normalizePropertyName(property: string): string {
  return property.trim().toLowerCase();
}

/**
 * Sample one numeric property at clip/scene-relative time. Property matching
 * is trimmed and case-insensitive. Values hold before/after the authored range,
 * and each segment uses the LATER keyframe's curve/easing. Original array order
 * breaks equal-time ties explicitly so runtimes do not depend on sort stability.
 */
export function samplePropertyKeyframes(
  keyframes: readonly CanonicalPropertyKeyframe[] | undefined,
  property: string,
  timeMs: number,
): number | null {
  const normalizedProperty = normalizePropertyName(property);
  if (!normalizedProperty) return null;
  const points = (keyframes ?? [])
    .map((keyframe, index) => ({ keyframe, index }))
    .filter(({ keyframe }) => normalizePropertyName(keyframe.property) === normalizedProperty)
    .sort((left, right) => left.keyframe.time_ms - right.keyframe.time_ms || left.index - right.index)
    .map(({ keyframe }) => keyframe);
  if (points.length === 0) return null;
  if (timeMs <= points[0].time_ms) return points[0].value;
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1];
    const next = points[index];
    if (timeMs <= next.time_ms) {
      const span = next.time_ms - previous.time_ms;
      const progress = span > 0 ? (timeMs - previous.time_ms) / span : 1;
      const eased = curveProgress(progress, next.curve, next.easing);
      return previous.value + (next.value - previous.value) * eased;
    }
  }
  return points[points.length - 1].value;
}

/** Resolve a supported clip property from its static/default value + keyframes. */
export function evaluateClipProperty(
  clip: Pick<TimelineV2Clip, 'transform' | 'volume' | 'keyframes'>,
  property: string,
  timeMs: number,
): number {
  const normalized = normalizePropertyName(property);
  const base = clipPropertyBaseValue(clip, normalized);
  return samplePropertyKeyframes(clip.keyframes, normalized, timeMs) ?? base;
}

/**
 * Return v1-preview-compatible static/default values. Axis scale falls back to
 * uniform scale and rotation_z falls back to legacy 2D rotation. Matrix/anchor
 * composition remains owned by the later canonical transform evaluator.
 */
export function clipPropertyBaseValue(
  clip: Pick<TimelineV2Clip, 'transform' | 'volume'>,
  property: string,
): number {
  const normalized = normalizePropertyName(property);
  if (!clipPropertySet.has(normalized)) throw unsupportedProperty('clip', normalized);
  const transform = clip.transform;
  switch (normalized) {
    case 'x': return transform?.x ?? 0;
    case 'y': return transform?.y ?? 0;
    case 'z': return transform?.z ?? 0;
    case 'scale': return transform?.scale ?? 1;
    case 'scale_x': return transform?.scale_x ?? transform?.scale ?? 1;
    case 'scale_y': return transform?.scale_y ?? transform?.scale ?? 1;
    case 'rotation': return transform?.rotation ?? 0;
    case 'rotation_x': return transform?.rotation_x ?? 0;
    case 'rotation_y': return transform?.rotation_y ?? 0;
    case 'rotation_z': return transform?.rotation_z ?? transform?.rotation ?? 0;
    case 'opacity': return transform?.opacity ?? 1;
    case 'volume': return clip.volume ?? 1;
    default: throw unsupportedProperty('clip', normalized);
  }
}

/** Resolve a supported camera property from its static/default value + keyframes. */
export function evaluateCameraProperty(
  camera: TimelineV2Camera | undefined,
  property: string,
  timeMs: number,
): number {
  const normalized = normalizePropertyName(property);
  const base = cameraPropertyBaseValue(camera, normalized);
  return samplePropertyKeyframes(camera?.keyframes, normalized, timeMs) ?? base;
}

/** field_of_view defaults to 50 degrees; other supported camera values to zero. */
export function cameraPropertyBaseValue(camera: TimelineV2Camera | undefined, property: string): number {
  const normalized = normalizePropertyName(property);
  if (!cameraPropertySet.has(normalized)) throw unsupportedProperty('camera', normalized);
  switch (normalized) {
    case 'x': return camera?.x ?? 0;
    case 'y': return camera?.y ?? 0;
    case 'z': return camera?.z ?? 0;
    case 'rotation_x': return camera?.rotation_x ?? 0;
    case 'rotation_y': return camera?.rotation_y ?? 0;
    case 'rotation_z': return camera?.rotation_z ?? 0;
    case 'field_of_view': return camera?.field_of_view ?? 50;
    case 'focus_depth': return camera?.focus_depth ?? 0;
    default: throw unsupportedProperty('camera', normalized);
  }
}

function unsupportedProperty(scope: 'clip' | 'camera', property: string): Error {
  return new Error(`unsupported canonical ${scope} property ${JSON.stringify(property)}`);
}
