import type {
  TimelineV2Document,
  TimelineV2Track,
  TimelineV2Transform,
} from './renderContractTypes';

export const TIMELINE_V2_RUNTIME_INVALID = 'TIMELINE_V2_RUNTIME_INVALID';

const trackTypes = new Set(['layer', 'video', 'image', 'audio', 'music', 'text', 'caption', 'shape', 'callout']);
const mediaFits = new Set(['contain', 'cover', 'fill', 'none']);
const transformDefaults: TimelineV2Transform = { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };

export class TimelineV2RuntimeError extends Error {
  readonly code: string;
  readonly path: string;
  readonly remediation: string;

  constructor(path: string, message: string, remediation: string) {
    super(path ? `${path}: ${message}` : message);
    this.name = 'TimelineV2RuntimeError';
    this.code = TIMELINE_V2_RUNTIME_INVALID;
    this.path = path;
    this.remediation = remediation;
  }
}

/**
 * Normalize the Timeline v2 fields required by canonical frame activity and
 * source-time evaluation. Feature-specific text/shape/effect/transition/camera
 * semantics remain owned by the evaluator slices that implement them.
 */
export function normalizeTimelineV2EvaluationInputs(document: TimelineV2Document): TimelineV2Document {
  const normalized = cloneJson(document);
  if (normalized.version !== 2) throw runtimeError('version', 'version must be 2', 'adapt or upgrade the timeline to Timeline v2 before evaluation');
  if (normalized.canvas.width < 1) throw runtimeError('canvas.width', 'width must be at least 1', 'provide a positive canvas width');
  if (normalized.canvas.height < 1) throw runtimeError('canvas.height', 'height must be at least 1', 'provide a positive canvas height');
  if (!Number.isInteger(normalized.canvas.fps) || normalized.canvas.fps < 1 || normalized.canvas.fps > 120) {
    throw runtimeError('canvas.fps', 'fps must be an integer between 1 and 120', 'choose a supported integer output frame rate');
  }
  if (normalized.canvas.background.trim() === '') throw runtimeError('canvas.background', 'background must not be empty', 'provide an explicit canvas background');
  if (!Number.isInteger(normalized.duration_ms) || normalized.duration_ms < 0) throw runtimeError('duration_ms', 'duration_ms must be a non-negative integer', 'provide a non-negative timeline duration');
  normalized.working_color_space ??= 'srgb';
  if (normalized.working_color_space !== 'srgb') throw runtimeError('working_color_space', `working color space ${JSON.stringify(normalized.working_color_space)} is unsupported`, 'use srgb until another canonical working color space is versioned');
  normalized.metadata ??= {};
  normalized.tracks ??= [];
  normalized.markers ??= [];

  const trackIds = new Set<string>();
  const clipIds = new Set<string>();
  let maxClipEnd = 0;
  normalized.tracks.forEach((track, trackIndex) => {
    const trackPath = `tracks[${trackIndex}]`;
    track.id = track.id.trim();
    if (!track.id) throw runtimeError(`${trackPath}.id`, 'track id must not be empty', 'provide a stable track id');
    if (trackIds.has(track.id)) throw runtimeError(`${trackPath}.id`, `duplicate track id ${JSON.stringify(track.id)}`, 'use a unique track id');
    trackIds.add(track.id);
    track.type = track.type.trim().toLowerCase() as TimelineV2Track['type'];
    if (!trackTypes.has(track.type)) throw runtimeError(`${trackPath}.type`, `unsupported track type ${JSON.stringify(track.type)}`, 'use a Timeline v2 track type');
    if (track.height !== undefined && (!Number.isInteger(track.height) || track.height < 32 || track.height > 160)) {
      throw runtimeError(`${trackPath}.height`, 'track height must be an integer between 32 and 160', 'choose a supported track height');
    }
    track.clips ??= [];
    track.clips.forEach((clip, clipIndex) => {
      const clipPath = `${trackPath}.clips[${clipIndex}]`;
      clip.id = clip.id.trim();
      if (!clip.id) throw runtimeError(`${clipPath}.id`, 'clip id must not be empty', 'provide a stable clip id');
      if (clipIds.has(clip.id)) throw runtimeError(`${clipPath}.id`, `duplicate clip id ${JSON.stringify(clip.id)}`, 'use a globally unique clip id');
      clipIds.add(clip.id);
      if (!Number.isInteger(clip.start_ms) || clip.start_ms < 0) throw runtimeError(`${clipPath}.start_ms`, 'start_ms must be a non-negative integer', 'place the clip at or after timeline time zero');
      if (!Number.isInteger(clip.duration_ms) || clip.duration_ms < 1) throw runtimeError(`${clipPath}.duration_ms`, 'duration_ms must be an integer of at least 1', 'provide a positive output-timeline duration');
      if (!Number.isInteger(clip.trim_in_ms) || clip.trim_in_ms < 0) {
        throw runtimeError(`${clipPath}.trim_in_ms`, 'trim_in_ms must be a non-negative integer', 'provide a non-negative source trim-in point');
      }
      if (!Number.isInteger(clip.trim_out_ms) || clip.trim_out_ms < 0) {
        throw runtimeError(`${clipPath}.trim_out_ms`, 'trim_out_ms must be a non-negative integer', 'provide a non-negative source trim-out point');
      }

      const playbackRate = clip.playback_rate ?? 1;
      if (!Number.isFinite(playbackRate) || playbackRate < 0.25 || playbackRate > 4) {
        throw runtimeError(`${clipPath}.playback_rate`, 'playback_rate must be finite and between 0.25 and 4', 'choose a supported constant playback rate');
      }
      clip.playback_rate = playbackRate;
      clip.trim_out_ms = clip.trim_in_ms + sourceDurationForTimelineV2(clip.duration_ms, playbackRate);

      const visual = track.type !== 'audio' && track.type !== 'music' && !clip.audio_only;
      if (visual) {
        clip.transform = normalizeTransform(clip.transform, `${clipPath}.transform`);
        if (clip.asset_id && !clip.media_fit) clip.media_fit = 'contain';
      }
      if (clip.media_fit && !mediaFits.has(clip.media_fit)) {
        throw runtimeError(`${clipPath}.media_fit`, `unsupported media_fit ${JSON.stringify(clip.media_fit)}`, 'use contain, cover, fill, or none');
      }
      clip.effects ??= [];
      clip.keyframes ??= [];
      maxClipEnd = Math.max(maxClipEnd, clip.start_ms + clip.duration_ms);
    });
  });
  normalized.duration_ms = Math.max(normalized.duration_ms, maxClipEnd);
  return normalized;
}

function normalizeTransform(transform: TimelineV2Transform | undefined, path: string): TimelineV2Transform {
  const normalized: TimelineV2Transform = { ...transformDefaults, ...(transform ?? {}) };
  const numericFields: Array<[string, number | undefined]> = [
    ['x', normalized.x], ['y', normalized.y], ['z', normalized.z], ['scale', normalized.scale],
    ['scale_x', normalized.scale_x], ['scale_y', normalized.scale_y], ['rotation', normalized.rotation],
    ['rotation_x', normalized.rotation_x], ['rotation_y', normalized.rotation_y], ['rotation_z', normalized.rotation_z],
    ['opacity', normalized.opacity], ['anchor_x', normalized.anchor_x], ['anchor_y', normalized.anchor_y], ['perspective', normalized.perspective],
  ];
  for (const [name, value] of numericFields) {
    if (value !== undefined && !Number.isFinite(value)) throw runtimeError(`${path}.${name}`, 'transform value must be finite', 'provide a finite numeric transform value');
  }
  if (normalized.opacity !== undefined && (normalized.opacity < 0 || normalized.opacity > 1)) {
    throw runtimeError(`${path}.opacity`, 'opacity must be between 0 and 1', 'provide normalized opacity');
  }
  if (normalized.crop) {
    for (const [name, value] of Object.entries(normalized.crop)) {
      if (!Number.isFinite(value) || value < 0 || value > 1) throw runtimeError(`${path}.crop.${name}`, 'crop value must be finite and between 0 and 1', 'provide a normalized crop edge');
    }
    normalized.crop = { ...normalized.crop };
  }
  return normalized;
}

function sourceDurationForTimelineV2(durationMs: number, playbackRate: number): number {
  return Math.max(1, Math.round(durationMs * playbackRate));
}

function runtimeError(path: string, message: string, remediation: string): TimelineV2RuntimeError {
  return new TimelineV2RuntimeError(path, message, remediation);
}

function cloneJson<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}
