import type {
  VideoSceneCamera,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineKeyframe,
  VideoTimelineTrack,
  VideoTimelineTransform,
  VideoTimelineTransition,
} from '../types/video';
import type {
  RenderContractMetadata,
  TimelineV2Clip,
  TimelineV2Document,
  TimelineV2Keyframe,
  TimelineV2Track,
  TimelineV2Transform,
  TimelineV2Transition,
} from './renderContractTypes';

const supportedTransformKeys = new Set([
  'x', 'y', 'z', 'scale', 'scale_x', 'scale_y', 'rotation', 'rotation_x', 'rotation_y', 'rotation_z',
  'opacity', 'anchor_x', 'anchor_y', 'perspective', 'crop',
]);
const supportedCropKeys = new Set(['top', 'right', 'bottom', 'left']);
const supportedTransitionPlacements = new Set(['in', 'out', 'between']);
const visualTransformDefaults: TimelineV2Transform = { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };

export class RenderContractAdapterError extends Error {
  readonly code: string;
  readonly path: string;
  readonly remediation: string;

  constructor(code: string, path: string, message: string, remediation: string) {
    super(path ? `${path}: ${message}` : message);
    this.name = 'RenderContractAdapterError';
    this.code = code;
    this.path = path;
    this.remediation = remediation;
  }
}

/**
 * Adapts an editor-valid Timeline v1 document into the canonical Timeline v2
 * projection without mutating the caller. Defaults mirror backend v1
 * normalization where the editor can legitimately carry an omitted value.
 */
export function adaptTimelineV1ToV2(document: VideoTimelineDocument): TimelineV2Document {
  if (document.version > 1) {
    throw new RenderContractAdapterError(
      'V1_VERSION_UNSUPPORTED',
      'version',
      `timeline version ${document.version} cannot be adapted by the v1 adapter`,
      'use a contract adapter for the authored timeline version',
    );
  }

  const source = cloneJson(document);
  const tracks = source.tracks.map((track, trackIndex) => adaptTrack(source, track, trackIndex));
  const maxClipEnd = Math.max(0, ...tracks.flatMap((track) => track.clips.map((clip) => clip.start_ms + clip.duration_ms)));
  const durationMs = source.duration_ms > 0 ? Math.max(source.duration_ms, maxClipEnd) : Math.max(maxClipEnd, 30_000);

  return {
    version: 2,
    canvas: { ...source.canvas },
    duration_ms: durationMs,
    tracks,
    markers: (source.markers ?? []).map((marker) => ({ ...marker })),
    scenes: source.scenes?.map((scene) => ({
      id: scene.id,
      name: scene.name,
      start_ms: scene.start_ms,
      duration_ms: scene.duration_ms,
      camera: scene.camera ? adaptCamera(scene.camera) : undefined,
      effects: scene.effects?.map((effect) => ({ ...effect, params: cloneMetadata(effect.params) })),
      metadata: cloneMetadataOptional(scene.metadata),
    })),
    working_color_space: 'srgb',
    metadata: cloneMetadata(source.metadata),
  };
}

function adaptTrack(document: VideoTimelineDocument, track: VideoTimelineTrack, trackIndex: number): TimelineV2Track {
  return {
    id: track.id,
    type: track.type,
    name: track.name,
    locked: track.locked,
    muted: track.muted,
    solo: track.solo,
    visible: track.visible,
    height: track.height,
    clips: track.clips.map((clip, clipIndex) => adaptClip(document, track, clip, `tracks[${trackIndex}].clips[${clipIndex}]`)),
  };
}

function adaptClip(document: VideoTimelineDocument, track: VideoTimelineTrack, clip: VideoTimelineClip, path: string): TimelineV2Clip {
  const playbackRate = clip.playback_rate && clip.playback_rate > 0 ? clip.playback_rate : 1;
  const consumedSourceMs = Math.max(1, Math.round(clip.duration_ms * playbackRate));
  const adapted: TimelineV2Clip = {
    id: clip.id,
    asset_id: clip.asset_id,
    start_ms: clip.start_ms,
    duration_ms: clip.duration_ms,
    trim_in_ms: clip.trim_in_ms,
    trim_out_ms: clip.trim_in_ms + consumedSourceMs,
    playback_rate: playbackRate,
    z_index: clip.z_index,
    group_id: clip.group_id,
    template_slot: clip.template_slot,
    muted: clip.muted,
    audio_only: clip.audio_only,
    transform: adaptTransform(clip.transform, path, track.type),
    volume: clip.volume,
    fade_in_ms: clip.fade_in_ms,
    fade_out_ms: clip.fade_out_ms,
    text: clip.text ? { ...clip.text, params: cloneMetadataOptional(clip.text.params) } : undefined,
    shape: clip.shape ? { ...clip.shape } : undefined,
    cursor: clip.cursor ? {
      ...clip.cursor,
      scale: normalizeCursorScale(clip.cursor.scale),
      events: clip.cursor.events?.map((event) => ({ ...event })),
    } : undefined,
    effects: (clip.effects ?? []).map((effect) => ({ ...effect, params: cloneMetadata(effect.params) })),
    transitions: adaptTransitions(document, track, clip, path),
    keyframes: (clip.keyframes ?? []).map(adaptKeyframe),
    animation_blocks: clip.animation_blocks?.map((block) => ({
      ...block,
      params: cloneMetadataOptional(block.params),
      generated_keyframe_ids: [...block.generated_keyframe_ids],
    })),
    metadata: cloneMetadataOptional(clip.metadata),
  };
  if (clip.asset_id && track.type !== 'audio' && track.type !== 'music' && !clip.audio_only) {
    adapted.media_fit = 'contain';
  }
  return adapted;
}

function adaptTransitions(
  document: VideoTimelineDocument,
  track: VideoTimelineTrack,
  clip: VideoTimelineClip,
  path: string,
): TimelineV2Transition[] | undefined {
  void track;
  const transitions = clip.transitions ?? [];
  if (transitions.length === 0) return undefined;
  return transitions.map((transition: VideoTimelineTransition, transitionIndex) => {
    const transitionPath = `${path}.transitions[${transitionIndex}]`;
    const placement = transition.placement?.trim().toLowerCase() ?? '';
    if (!placement) {
      throw new RenderContractAdapterError(
        'V1_TRANSITION_PLACEMENT_AMBIGUOUS', transitionPath,
        'v1 transition placement is not explicit enough for Timeline v2',
        'choose explicit in, out, or between placement in the editor before canonical rendering',
      );
    }
    if (!supportedTransitionPlacements.has(placement)) {
      throw new RenderContractAdapterError(
        'V1_TRANSITION_PLACEMENT_INVALID', `${transitionPath}.placement`,
        `unsupported transition placement ${JSON.stringify(transition.placement)}`,
        'use in, out, or between',
      );
    }
    validateTransitionPaintCombination(transition.type, placement, transitionPath);
    const peerClipId = transition.peer_clip_id?.trim() ?? '';
    if (placement === 'between') {
      if (!peerClipId) {
        throw new RenderContractAdapterError(
          'V1_TRANSITION_PEER_INVALID', `${transitionPath}.peer_clip_id`,
          'between transitions require an explicit peer clip',
          'choose an overlapping peer clip in the editor',
        );
      }
      const peerLocation = findTimelineClipById(document, peerClipId);
      if (!peerLocation || peerLocation.clip.id === clip.id) {
        throw new RenderContractAdapterError(
          'V1_TRANSITION_PEER_INVALID', `${transitionPath}.peer_clip_id`,
          `transition peer ${JSON.stringify(peerClipId)} is missing or invalid`,
          'choose a different existing overlapping clip',
        );
      }
      if (!isVisualTransitionPeer(peerLocation.track, peerLocation.clip)) {
        throw new RenderContractAdapterError(
          'V1_TRANSITION_PEER_INVALID', `${transitionPath}.peer_clip_id`,
          `transition peer ${JSON.stringify(peerClipId)} is not a visible visual clip`,
          'choose a visible non-audio peer clip',
        );
      }
      if (peerLocation.clip.start_ms === clip.start_ms) {
        throw new RenderContractAdapterError(
          'V1_TRANSITION_PEER_INVALID', `${transitionPath}.peer_clip_id`,
          'between transition owner and peer must have distinct start times',
          'choose a peer with a distinct authored start time',
        );
      }
      const overlap = timelineTransitionOverlapMs(clip, peerLocation.clip);
      if (overlap < transition.duration_ms) {
        throw new RenderContractAdapterError(
          'V1_TRANSITION_OVERLAP_INVALID', `${transitionPath}.duration_ms`,
          `between transition needs ${transition.duration_ms}ms overlap but only ${overlap}ms is authored`,
          'shorten the transition or increase real clip overlap',
        );
      }
    } else if (peerClipId) {
      throw new RenderContractAdapterError(
        'V1_TRANSITION_PEER_INVALID', `${transitionPath}.peer_clip_id`,
        'in/out transitions must not declare a peer',
        'clear peer_clip_id or choose between placement',
      );
    }
    return {
      id: transition.id,
      type: transition.type,
      duration_ms: transition.duration_ms,
      direction: transition.direction,
      placement: placement as TimelineV2Transition['placement'],
      ...(peerClipId ? { peer_clip_id: peerClipId } : {}),
    };
  });
}

function findTimelineClipById(document: VideoTimelineDocument, clipId: string): { track: VideoTimelineTrack; clip: VideoTimelineClip } | undefined {
  for (const track of document.tracks) {
    const clip = track.clips.find((candidate) => candidate.id === clipId);
    if (clip) return { track, clip };
  }
  return undefined;
}

function isVisualTransitionPeer(track: VideoTimelineTrack, clip: VideoTimelineClip): boolean {
  return track.visible && track.type !== 'audio' && track.type !== 'music' && !clip.audio_only;
}

function validateTransitionPaintCombination(type: string, placement: string, path: string): void {
  const normalizedType = type.trim().toLowerCase();
  if (normalizedType === 'fade' && placement === 'between') {
    throw new RenderContractAdapterError(
      'V1_TRANSITION_COMBINATION_INVALID', `${path}.placement`,
      'fade supports only in or out placement',
      'use in/out placement or choose crossfade for a two-clip blend',
    );
  }
  if (normalizedType === 'crossfade' && placement !== 'between') {
    throw new RenderContractAdapterError(
      'V1_TRANSITION_COMBINATION_INVALID', `${path}.placement`,
      'crossfade requires between placement',
      'choose between placement and an overlapping visual peer',
    );
  }
}

function timelineTransitionOverlapMs(left: VideoTimelineClip, right: VideoTimelineClip): number {
  return Math.max(0, Math.min(left.start_ms + left.duration_ms, right.start_ms + right.duration_ms) - Math.max(left.start_ms, right.start_ms));
}

function adaptTransform(
  transform: VideoTimelineTransform | undefined,
  path: string,
  trackType: VideoTimelineTrack['type'],
): TimelineV2Transform | undefined {
  const visual = trackType !== 'audio' && trackType !== 'music';
  if (!transform && !visual) return undefined;
  const raw = (transform ?? {}) as Record<string, unknown>;
  for (const [key, value] of Object.entries(raw)) {
    if (!supportedTransformKeys.has(key)) {
      throw unsupportedTransform(`${path}.transform.${key}`, key);
    }
    if (key === 'crop') {
      validateCrop(value, `${path}.transform.crop`);
    } else {
      validateFiniteNumber(value, `${path}.transform.${key}`);
    }
  }
  return {
    ...(visual ? visualTransformDefaults : {}),
    ...transform,
    crop: transform?.crop ? { ...transform.crop } : undefined,
  };
}

function adaptCamera(camera: VideoSceneCamera) {
  const fieldOfView = Math.min(179, Math.max(1, camera.field_of_view && camera.field_of_view > 0 ? camera.field_of_view : 50));
  return {
    x: camera.x ?? 0,
    y: camera.y ?? 0,
    z: camera.z ?? 0,
    rotation_x: camera.rotation_x ?? 0,
    rotation_y: camera.rotation_y ?? 0,
    rotation_z: camera.rotation_z ?? 0,
    field_of_view: fieldOfView,
    focus_depth: camera.focus_depth ?? 0,
    keyframes: camera.keyframes?.map(adaptKeyframe),
  };
}

function adaptKeyframe(keyframe: VideoTimelineKeyframe): TimelineV2Keyframe {
  return { ...keyframe, curve: keyframe.curve ? { ...keyframe.curve } : undefined };
}

function validateCrop(value: unknown, path: string): void {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new RenderContractAdapterError('V1_TRANSFORM_VALUE_INVALID', path, 'v1 crop must be an object', 'replace crop with top/right/bottom/left numeric fields');
  }
  for (const [key, side] of Object.entries(value as Record<string, unknown>)) {
    if (!supportedCropKeys.has(key)) throw unsupportedTransform(`${path}.${key}`, key);
    validateFiniteNumber(side, `${path}.${key}`);
  }
}

function validateFiniteNumber(value: unknown, path: string): void {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new RenderContractAdapterError('V1_TRANSFORM_VALUE_INVALID', path, 'v1 transform value must be a finite number', 'replace the value with a finite numeric transform parameter');
  }
}

function unsupportedTransform(path: string, key: string): RenderContractAdapterError {
  return new RenderContractAdapterError('V1_TRANSFORM_FIELD_UNSUPPORTED', path, `v1 transform field ${JSON.stringify(key)} has no canonical Timeline v2 semantics`, 'remove the unsupported field or define its Timeline v2 semantics before rendering');
}

function normalizeCursorScale(value: number | undefined): number | undefined {
  if (value === undefined) return undefined;
  if (value <= 0) return 1;
  return Math.min(4, Math.max(0.25, value));
}

function cloneMetadata(value: Record<string, unknown> | undefined): RenderContractMetadata {
  return value ? cloneJson(value) : {};
}

function cloneMetadataOptional(value: Record<string, unknown> | undefined): RenderContractMetadata | undefined {
  return value ? cloneJson(value) : undefined;
}

function cloneJson<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}
