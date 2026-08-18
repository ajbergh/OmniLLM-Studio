import type {
  VideoAsset,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
} from '../../../types/video';
import { activeAtFrame } from '../../../video/renderContract';
import { compareCanonicalClipOrder } from '../../../video/renderContractEvaluation';

export interface IndexedTimelineClip {
  clip: VideoTimelineClip;
  track: VideoTimelineTrack;
  trackIndex: number;
  /** Original clip array index, retained independently of interval sorting. */
  clipIndex: number;
  asset?: VideoAsset;
  /** Cached interval end. Optional so preview-layer projections remain assignable. */
  endMs?: number;
}

export interface TimelineIntervalIndex {
  clips: IndexedTimelineClip[];
  starts: number[];
  /** Maximum clip end observed from index 0 through each position. */
  prefixMaxEnd: number[];
  assetById: Map<string, VideoAsset>;
}

function upperBound(values: number[], target: number): number {
  let low = 0;
  let high = values.length;
  while (low < high) {
    const middle = (low + high) >>> 1;
    if (values[middle] <= target) low = middle + 1;
    else high = middle;
  }
  return low;
}

function clipEnd(item: IndexedTimelineClip): number {
  return item.endMs ?? item.clip.start_ms + item.clip.duration_ms;
}

export function compareIndexedTimelineClipOrder(
  left: IndexedTimelineClip,
  right: IndexedTimelineClip,
): number {
  return compareCanonicalClipOrder(
    { track_index: left.trackIndex, clip_index: left.clipIndex, z_index: left.clip.z_index ?? 0 },
    { track_index: right.trackIndex, clip_index: right.clipIndex, z_index: right.clip.z_index ?? 0 },
  );
}

export function buildTimelineIntervalIndex(
  document: VideoTimelineDocument | null,
  assets: VideoAsset[],
): TimelineIntervalIndex {
  const assetById = new Map(assets.map((asset) => [asset.id, asset]));
  const clips: IndexedTimelineClip[] = [];

  for (const [trackIndex, track] of (document?.tracks ?? []).entries()) {
    for (const [clipIndex, clip] of track.clips.entries()) {
      clips.push({
        clip,
        track,
        trackIndex,
        clipIndex,
        asset: clip.asset_id ? assetById.get(clip.asset_id) : undefined,
        endMs: clip.start_ms + clip.duration_ms,
      });
    }
  }

  // Temporal ordering powers interval lookup only. clipIndex is retained so
  // query results can be restored to canonical composition order afterwards.
  clips.sort((left, right) => (
    left.clip.start_ms - right.clip.start_ms
    || clipEnd(left) - clipEnd(right)
    || left.trackIndex - right.trackIndex
    || left.clipIndex - right.clipIndex
  ));
  const starts = clips.map((item) => item.clip.start_ms);
  const prefixMaxEnd: number[] = [];
  let maxEnd = Number.NEGATIVE_INFINITY;
  for (const item of clips) {
    maxEnd = Math.max(maxEnd, clipEnd(item));
    prefixMaxEnd.push(maxEnd);
  }

  return { clips, starts, prefixMaxEnd, assetById };
}

/**
 * Return clips active at timeMs. The prefix maximum lets the backward scan stop
 * as soon as no earlier interval can overlap the query, avoiding a full scan on
 * long projects containing short clips. Candidates are restored to canonical
 * composition order after lookup so interval-index temporal sorting never leaks
 * into preview layer semantics.
 */
export function queryActiveClips(
  index: TimelineIntervalIndex,
  timeMs: number,
): IndexedTimelineClip[] {
  const endExclusive = upperBound(index.starts, timeMs);
  const result: IndexedTimelineClip[] = [];

  for (let position = endExclusive - 1; position >= 0; position -= 1) {
    if (index.prefixMaxEnd[position] <= timeMs) break;
    const item = index.clips[position];
    if (clipEnd(item) > timeMs) result.push(item);
  }

  return result.sort(compareIndexedTimelineClipOrder);
}

/**
 * Return clips overlapping one canonical output frame. Deterministic capture is
 * intentionally frame-addressed: a clip authored partway into a frame belongs
 * to that frame under floor-start/ceil-end semantics even when a millisecond
 * point query at the frame's start would miss it. This path is reserved for
 * explicit frame-addressed evaluation; interactive playback keeps the indexed
 * point query above.
 */
export function queryActiveClipsAtFrame(
  index: TimelineIntervalIndex,
  frameIndex: number,
  fps: number,
): IndexedTimelineClip[] {
  const normalizedFrame = Math.trunc(frameIndex);
  const normalizedFPS = Math.trunc(fps);
  if (normalizedFrame < 0 || normalizedFPS <= 0) return [];
  return index.clips
    .filter((item) => activeAtFrame(
      normalizedFrame,
      item.clip.start_ms,
      item.clip.duration_ms,
      normalizedFPS,
    ))
    .sort(compareIndexedTimelineClipOrder);
}

/** Return clips intersecting the visible timeline window, with overscan. */
export function visibleClips(
  clips: VideoTimelineClip[],
  startMs: number,
  endMs: number,
  overscanMs = 1000,
): VideoTimelineClip[] {
  const minimum = Math.max(0, startMs - overscanMs);
  const maximum = Math.max(minimum, endMs + overscanMs);
  const ordered = [...clips].sort((left, right) => left.start_ms - right.start_ms);
  const starts = ordered.map((clip) => clip.start_ms);
  const candidateEnd = upperBound(starts, maximum - Number.EPSILON);
  const result: VideoTimelineClip[] = [];

  for (let index = 0; index < candidateEnd; index += 1) {
    const clip = ordered[index];
    if (clip.start_ms + clip.duration_ms > minimum) result.push(clip);
  }
  return result;
}

/**
 * Limit simultaneously mounted video decoders. The selected video is promoted
 * ahead of ordinary z-order candidates so direct manipulation never degrades
 * into a non-interactive poster frame. Equal-priority candidates retain
 * deterministic reverse-canonical order so visually higher clips win decoders.
 */
export function applyDecoderBudget<T extends IndexedTimelineClip>(
  items: T[],
  limit: number,
  preferredClipId?: string | null,
): { mounted: T[]; posters: T[] } {
  const videos = items
    .filter((item) => item.asset?.mime_type.startsWith('video/'))
    .sort((left, right) => (
      Number(right.clip.id === preferredClipId) - Number(left.clip.id === preferredClipId)
      || -compareIndexedTimelineClipOrder(left, right)
    ));
  const mountedIds = new Set(
    videos.slice(0, Math.max(1, limit)).map((item) => item.clip.id),
  );
  return {
    mounted: items.filter((item) => (
      !item.asset?.mime_type.startsWith('video/') || mountedIds.has(item.clip.id)
    )),
    posters: videos.filter((item) => !mountedIds.has(item.clip.id)),
  };
}
