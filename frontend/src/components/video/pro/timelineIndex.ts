import type { VideoAsset, VideoTimelineClip, VideoTimelineDocument, VideoTimelineTrack } from '../../../types/video';

export interface IndexedTimelineClip { clip: VideoTimelineClip; track: VideoTimelineTrack; trackIndex: number; asset?: VideoAsset; endMs: number }
export interface TimelineIntervalIndex { clips: IndexedTimelineClip[]; starts: number[]; assetById: Map<string, VideoAsset> }

export function buildTimelineIntervalIndex(document: VideoTimelineDocument | null, assets: VideoAsset[]): TimelineIntervalIndex {
  const assetById = new Map(assets.map((asset) => [asset.id, asset]));
  const clips: IndexedTimelineClip[] = [];
  for (const [trackIndex, track] of (document?.tracks || []).entries()) {
    for (const clip of track.clips) clips.push({ clip, track, trackIndex, asset: clip.asset_id ? assetById.get(clip.asset_id) : undefined, endMs: clip.start_ms + clip.duration_ms });
  }
  clips.sort((a, b) => a.clip.start_ms - b.clip.start_ms || a.endMs - b.endMs);
  return { clips, starts: clips.map((item) => item.clip.start_ms), assetById };
}

export function queryActiveClips(index: TimelineIntervalIndex, timeMs: number): IndexedTimelineClip[] {
  let low = 0; let high = index.starts.length;
  while (low < high) { const mid = (low + high) >>> 1; if (index.starts[mid] <= timeMs) low = mid + 1; else high = mid; }
  const result: IndexedTimelineClip[] = [];
  for (let i = low - 1; i >= 0; i -= 1) {
    const item = index.clips[i];
    if (item.endMs > timeMs) result.push(item);
    if (timeMs - item.clip.start_ms > 6 * 60 * 60 * 1000 && result.length > 0) break;
  }
  return result;
}

export function visibleClips(clips: VideoTimelineClip[], startMs: number, endMs: number, overscanMs = 1000): VideoTimelineClip[] {
  const min = Math.max(0, startMs - overscanMs); const max = endMs + overscanMs;
  return clips.filter((clip) => clip.start_ms < max && clip.start_ms + clip.duration_ms > min);
}

export function applyDecoderBudget<T extends IndexedTimelineClip>(items: T[], limit: number): { mounted: T[]; posters: T[] } {
  const videos = items.filter((item) => item.asset?.mime_type.startsWith('video/')).sort((a, b) => b.trackIndex - a.trackIndex || (b.clip.z_index || 0) - (a.clip.z_index || 0));
  const mountedIds = new Set(videos.slice(0, Math.max(1, limit)).map((item) => item.clip.id));
  return { mounted: items.filter((item) => !item.asset?.mime_type.startsWith('video/') || mountedIds.has(item.clip.id)), posters: videos.filter((item) => !mountedIds.has(item.clip.id)) };
}
