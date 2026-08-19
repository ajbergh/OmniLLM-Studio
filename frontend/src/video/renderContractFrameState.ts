import { endFrame } from './renderContract';
import { activeClipsAtFrame } from './renderContractEvaluation';
import { evaluateMediaGeometry, type CanonicalMediaGeometry } from './renderContractMediaGeometry';
import { normalizeTimelineV2EvaluationInputs } from './renderContractNormalize';
import {
  evaluatePerspectiveProjection,
  type CanonicalPerspectiveProjection,
} from './renderContractPerspectiveProjection';
import { evaluateCameraProperty, evaluateClipProperty } from './renderContractProperties';
import {
  evaluateTransitionPaint,
  supportsTransitionPaint,
  type CanonicalTransitionPaint,
} from './renderContractTransitionPaint';
import {
  evaluateClipTransitionsAtFrameNormalized,
  type CanonicalTransitionState,
} from './renderContractTransitions';
import type {
  TimelineV2Canvas,
  TimelineV2Clip,
  TimelineV2ContentBounds,
  TimelineV2Crop,
  TimelineV2Document,
  TimelineV2Scene,
  TimelineV2Track,
} from './renderContractTypes';

export const VISUAL_FRAME_STATE_CONTRACT_V1 = 'visual-frame-state-v1' as const;
export type Matrix4 = [number, number, number, number, number, number, number, number, number, number, number, number, number, number, number, number];

export interface CanonicalRationalTime { numerator: number; denominator: number; }
export interface CanonicalEvaluatedCamera {
  x: number; y: number; z: number;
  rotation_x: number; rotation_y: number; rotation_z: number;
  field_of_view: number; focus_depth: number; perspective_distance: number;
}
export interface CanonicalEvaluatedTransform {
  x: number; y: number; z: number;
  scale_x: number; scale_y: number;
  rotation_x: number; rotation_y: number; rotation_z: number;
  opacity: number; anchor_x: number; anchor_y: number;
  perspective?: number; crop?: TimelineV2Crop;
}
export interface CanonicalFrameLayerState {
  track_index: number; clip_index: number; track_id: string; clip_id: string; z_index: number;
  start_frame: number; end_frame: number; source_time_ms: number; media_fit?: string;
  content_bounds?: TimelineV2ContentBounds;
  media_geometry?: CanonicalMediaGeometry;
  transform: CanonicalEvaluatedTransform;
  view_transform: CanonicalEvaluatedTransform;
  model_matrix: Matrix4;
  perspective_projection: CanonicalPerspectiveProjection;
  transitions?: CanonicalTransitionState[];
  transition_paint?: CanonicalTransitionPaint[];
  unresolved: string[];
  authoritative: boolean;
}
export interface CanonicalVisualFrameState {
  contract_version: typeof VISUAL_FRAME_STATE_CONTRACT_V1;
  frame_index: number;
  frame_time: CanonicalRationalTime;
  canvas: TimelineV2Canvas;
  active_scene_id?: string;
  camera: CanonicalEvaluatedCamera;
  layers: CanonicalFrameLayerState[];
  unresolved: string[];
  authoritative: boolean;
}

/** Exact frame presentation time relative to an authored millisecond origin. */
export function frameRelativeMilliseconds(frameIndex: number, fps: number, originMs: number): number {
  const frame = Math.max(0, Math.trunc(frameIndex));
  const rate = Math.max(1, Math.trunc(fps));
  return (frame * 1000 - originMs * rate) / rate;
}

/** Renderer-independent visual FrameState. Unimplemented paint families are explicit. */
export function evaluateVisualFrameState(document: TimelineV2Document, frameIndex: number): CanonicalVisualFrameState {
  const normalized = normalizeTimelineV2EvaluationInputs(document);
  const fps = normalized.canvas.fps;
  const frame = Math.trunc(frameIndex);
  if (frame < 0 || frame >= endFrame(normalized.duration_ms, fps)) {
    throw new Error(`frame index ${frame} is outside timeline frame range`);
  }
  const scene = sceneAtFramePresentation(normalized.scenes ?? [], frame, fps);
  const camera = evaluateFrameCamera(scene, frame, fps, normalized.canvas.height);
  const state: CanonicalVisualFrameState = {
    contract_version: VISUAL_FRAME_STATE_CONTRACT_V1,
    frame_index: frame,
    frame_time: { numerator: frame, denominator: fps },
    canvas: normalized.canvas,
    ...(scene ? { active_scene_id: scene.id } : {}),
    camera,
    layers: [],
    unresolved: scene?.effects?.length ? ['scene_effects'] : [],
    authoritative: false,
  };
  for (const active of activeClipsAtFrame(normalized, frame)) {
    const track = normalized.tracks[active.track_index];
    const clip = track.clips[active.clip_index];
    if (!track.visible || clip.audio_only || track.type === 'audio' || track.type === 'music') continue;
    const transitions = evaluateClipTransitionsAtFrameNormalized(
      normalized,
      active.track_index,
      active.clip_index,
      frame,
    );
    const layer = evaluateFrameLayer(normalized.canvas, track, clip, active, camera, transitions, frame);
    state.layers.push(layer);
    state.unresolved.push(...layer.unresolved.map((entry) => `${clip.id}:${entry}`));
  }
  state.unresolved = uniqueStrings(state.unresolved);
  state.authoritative = state.unresolved.length === 0;
  return state;
}

function evaluateFrameCamera(scene: TimelineV2Scene | undefined, frameIndex: number, fps: number, canvasHeight: number): CanonicalEvaluatedCamera {
  const sceneTimeMs = frameRelativeMilliseconds(frameIndex, fps, scene?.start_ms ?? 0);
  const camera = scene?.camera;
  const fieldOfView = clamp(evaluateCameraProperty(camera, 'field_of_view', sceneTimeMs), 1, 179);
  return {
    x: evaluateCameraProperty(camera, 'x', sceneTimeMs),
    y: evaluateCameraProperty(camera, 'y', sceneTimeMs),
    z: evaluateCameraProperty(camera, 'z', sceneTimeMs),
    rotation_x: evaluateCameraProperty(camera, 'rotation_x', sceneTimeMs),
    rotation_y: evaluateCameraProperty(camera, 'rotation_y', sceneTimeMs),
    rotation_z: evaluateCameraProperty(camera, 'rotation_z', sceneTimeMs),
    field_of_view: fieldOfView,
    focus_depth: evaluateCameraProperty(camera, 'focus_depth', sceneTimeMs),
    perspective_distance: camera
      ? Math.max(1, canvasHeight) / (2 * Math.tan(fieldOfView * Math.PI / 360))
      : 1200,
  };
}

type ActiveLayerIdentity = ReturnType<typeof activeClipsAtFrame>[number];

function evaluateFrameLayer(
  canvas: TimelineV2Canvas,
  track: TimelineV2Track,
  clip: TimelineV2Clip,
  active: ActiveLayerIdentity,
  camera: CanonicalEvaluatedCamera,
  transitions: CanonicalTransitionState[],
  frameIndex: number,
): CanonicalFrameLayerState {
  const clipTimeMs = frameRelativeMilliseconds(frameIndex, canvas.fps, clip.start_ms);
  const transform: CanonicalEvaluatedTransform = {
    x: evaluateClipProperty(clip, 'x', clipTimeMs),
    y: evaluateClipProperty(clip, 'y', clipTimeMs),
    z: evaluateClipProperty(clip, 'z', clipTimeMs),
    scale_x: evaluateClipProperty(clip, 'scale_x', clipTimeMs),
    scale_y: evaluateClipProperty(clip, 'scale_y', clipTimeMs),
    rotation_x: evaluateClipProperty(clip, 'rotation_x', clipTimeMs),
    rotation_y: evaluateClipProperty(clip, 'rotation_y', clipTimeMs),
    rotation_z: evaluateClipProperty(clip, 'rotation_z', clipTimeMs),
    opacity: evaluateClipProperty(clip, 'opacity', clipTimeMs) * fadeFactorAtTime(clip, clipTimeMs),
    anchor_x: clip.transform?.anchor_x ?? 0,
    anchor_y: clip.transform?.anchor_y ?? 0,
    ...(clip.transform?.perspective !== undefined ? { perspective: clip.transform.perspective } : {}),
    ...(clip.transform?.crop ? { crop: { ...clip.transform.crop } } : {}),
  };
  const view: CanonicalEvaluatedTransform = {
    ...transform,
    x: transform.x - camera.x,
    y: transform.y - camera.y,
    z: transform.z - camera.z,
    rotation_x: transform.rotation_x - camera.rotation_x,
    rotation_y: transform.rotation_y - camera.rotation_y,
    rotation_z: transform.rotation_z - camera.rotation_z,
  };
  const contentBounds = effectiveContentBounds(clip);
  const unresolved = unresolvedLayerFeatures(clip, contentBounds);
  const transitionPaint: CanonicalTransitionPaint[] = [];
  for (const transition of transitions) {
    if (!transition.active) continue;
    if (!supportsTransitionPaint(transition.type)) {
      unresolved.push(`transition_paint:${transition.id}`);
      continue;
    }
    const paint = evaluateTransitionPaint(clip.id, transition);
    if (paint) transitionPaint.push(paint);
  }
  const mediaGeometry = clip.asset_id && clip.content_bounds ? evaluateMediaGeometry(canvas, clip) : undefined;
  let anchorOffsetX = 0;
  let anchorOffsetY = 0;
  if (mediaGeometry) {
    anchorOffsetX = transform.anchor_x * mediaGeometry.painted_bounds.width / Math.max(1, canvas.width);
    anchorOffsetY = transform.anchor_y * mediaGeometry.painted_bounds.height / Math.max(1, canvas.height);
  } else if (contentBounds) {
    anchorOffsetX = transform.anchor_x * contentBounds.width / Math.max(1, canvas.width);
    anchorOffsetY = transform.anchor_y * contentBounds.height / Math.max(1, canvas.height);
  } else if (transform.anchor_x !== 0 || transform.anchor_y !== 0) {
    unresolved.push('content_bounds_for_anchor');
  }
  const perspectiveProjection = evaluatePerspectiveProjection(camera, view);
  const normalizedUnresolved = uniqueStrings(unresolved);
  return {
    track_index: active.track_index,
    clip_index: active.clip_index,
    track_id: track.id,
    clip_id: clip.id,
    z_index: active.z_index,
    start_frame: active.start_frame,
    end_frame: active.end_frame,
    source_time_ms: active.source_time_ms,
    ...(clip.media_fit ? { media_fit: clip.media_fit } : {}),
    ...(contentBounds ? { content_bounds: contentBounds } : {}),
    ...(mediaGeometry ? { media_geometry: mediaGeometry } : {}),
    transform,
    view_transform: view,
    model_matrix: composeModelMatrix(view, anchorOffsetX, anchorOffsetY),
    perspective_projection: perspectiveProjection,
    ...(transitions.length > 0 ? { transitions } : {}),
    ...(transitionPaint.length > 0 ? { transition_paint: transitionPaint } : {}),
    unresolved: normalizedUnresolved,
    authoritative: normalizedUnresolved.length === 0,
  };
}

function sceneAtFramePresentation(scenes: TimelineV2Scene[], frameIndex: number, fps: number): TimelineV2Scene | undefined {
  const presentation = frameIndex * 1000;
  return scenes.find((scene) => scene.start_ms * fps <= presentation && presentation < (scene.start_ms + scene.duration_ms) * fps);
}

function effectiveContentBounds(clip: TimelineV2Clip): TimelineV2ContentBounds | undefined {
  if (clip.content_bounds) return { ...clip.content_bounds };
  if (clip.shape) return { x: 0, y: 0, width: clip.shape.width && clip.shape.width > 0 ? clip.shape.width : 320, height: clip.shape.height && clip.shape.height > 0 ? clip.shape.height : 180 };
  if (clip.text?.box_width && clip.text.box_height) return { x: 0, y: 0, width: clip.text.box_width, height: clip.text.box_height };
  return undefined;
}

function unresolvedLayerFeatures(clip: TimelineV2Clip, contentBounds: TimelineV2ContentBounds | undefined): string[] {
  const unresolved: string[] = [];
  if (clip.effects.length > 0) unresolved.push('effects');
  if (clip.text) unresolved.push('text');
  if (clip.shape) unresolved.push('shape');
  if (clip.cursor) unresolved.push('cursor');
  if (clip.asset_id && !contentBounds) unresolved.push('media_geometry:content_bounds');
  return unresolved;
}

function fadeFactorAtTime(clip: TimelineV2Clip, clipTimeMs: number): number {
  const remaining = clip.duration_ms - clipTimeMs;
  let factor = 1;
  if ((clip.fade_in_ms ?? 0) > 0) factor = Math.min(factor, clipTimeMs / (clip.fade_in_ms as number));
  if ((clip.fade_out_ms ?? 0) > 0) factor = Math.min(factor, remaining / (clip.fade_out_ms as number));
  return clamp(factor, 0, 1);
}

function composeModelMatrix(transform: CanonicalEvaluatedTransform, anchorX: number, anchorY: number): Matrix4 {
  let matrix = translationMatrix(transform.x, transform.y, transform.z);
  matrix = multiplyMatrix(matrix, translationMatrix(anchorX, anchorY, 0));
  matrix = multiplyMatrix(matrix, rotationXMatrix(transform.rotation_x));
  matrix = multiplyMatrix(matrix, rotationYMatrix(transform.rotation_y));
  matrix = multiplyMatrix(matrix, rotationZMatrix(transform.rotation_z));
  matrix = multiplyMatrix(matrix, scaleMatrix(transform.scale_x, transform.scale_y, 1));
  return multiplyMatrix(matrix, translationMatrix(-anchorX, -anchorY, 0));
}

function identityMatrix(): Matrix4 {
  return [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1];
}
function multiplyMatrix(left: Matrix4, right: Matrix4): Matrix4 {
  const out = Array(16).fill(0) as number[];
  for (let row = 0; row < 4; row += 1) for (let col = 0; col < 4; col += 1) for (let k = 0; k < 4; k += 1) out[row * 4 + col] += left[row * 4 + k] * right[k * 4 + col];
  return out as Matrix4;
}
function translationMatrix(x: number, y: number, z: number): Matrix4 {
  const matrix = identityMatrix(); matrix[3] = x; matrix[7] = y; matrix[11] = z; return matrix;
}
function scaleMatrix(x: number, y: number, z: number): Matrix4 {
  const matrix = identityMatrix(); matrix[0] = x; matrix[5] = y; matrix[10] = z; return matrix;
}
function rotationXMatrix(degrees: number): Matrix4 {
  const r = degrees * Math.PI / 180; const c = Math.cos(r); const s = Math.sin(r);
  return [1, 0, 0, 0, 0, c, -s, 0, 0, s, c, 0, 0, 0, 0, 1];
}
function rotationYMatrix(degrees: number): Matrix4 {
  const r = degrees * Math.PI / 180; const c = Math.cos(r); const s = Math.sin(r);
  return [c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1];
}
function rotationZMatrix(degrees: number): Matrix4 {
  const r = degrees * Math.PI / 180; const c = Math.cos(r); const s = Math.sin(r);
  return [c, -s, 0, 0, s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1];
}
function uniqueStrings(values: string[]): string[] { return [...new Set(values.filter(Boolean))].sort(); }
function clamp(value: number, min: number, max: number): number { return Math.max(min, Math.min(max, value)); }
