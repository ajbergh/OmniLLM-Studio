import type {
  VideoAsset,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
} from '../../../types/video';

export interface IndexedTimelineClip {
  clip: VideoTimelineClip;
  track: VideoTimelineTrack;
  trackIndex: number;
  asset?: VideoAsset;
  endMs: number;
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

export function buildTimelineIntervalIndex(
  document: VideoTimelineDocument | null,
  assets: VideoAsset[],
): TimelineIntervalIndex {
  const assetById = new Map(assets.map((asset) => [asset.id, asset]));
  const clips: IndexedTimelineClip[] = [];

  for (const [trackIndex, track] of (document?.tracks ?? []).entries()) {
    for (const clip of track.clips) {
      clips.push({
        clip,
        track,
        trackIndex,
        asset: clip.asset_id ? assetById.get(clip.asset_id) : undefined,
        endMs: clip.start_ms + clip.duration_ms,
      });
    }
  }

  clips.sort((left, right) => (
    left.clip.start_ms - right.clip.start_ms
    || left.endMs - right.endMs
    || left.trackIndex - right.trackIndex
  ));
  const starts = clips.map((item) => item.clip.start_ms);
  const prefixMaxEnd: number[] = [];
  let maxEnd = Number.NEGATIVE_INFINITY;
  for (const item of clips) {
    maxEnd = Math.max(maxEnd, item.endMs);
    prefixMaxEnd.push(maxEnd);
  }

  return { clips, starts, prefixMaxEnd, assetById };
}

/**
 * Return clips active at timeMs. The prefix maximum lets the backward scan stop
 * as soon as no earlier interval can overlap the query, avoiding a full scan on
 * long projects containing short clips.
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
    if (item.endMs > timeMs) result.push(item);
  }

  return result.reverse();
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

export function applyDecoderBudget<T extends IndexedTimelineClip>(
  items: T[],
  limit: number,
): { mounted: T[]; posters: T[] } {
  const videos = items
    .filter((item) => item.asset?.mime_type.startsWith('video/'))
    .sort((left, right) => (
      right.trackIndex - left.trackIndex
      || (right.clip.z_index ?? 0) - (left.clip.z_index ?? 0)
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
