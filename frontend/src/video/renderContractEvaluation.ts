import { activeAtFrame, endFrame, startFrame } from './renderContract';
import type { TimelineV2Document } from './renderContractTypes';

export interface CanonicalFrameRange {
  start_frame: number;
  end_frame: number;
}

export interface CanonicalActiveClip {
  track_index: number;
  clip_index: number;
  track_id: string;
  clip_id: string;
  z_index: number;
  start_frame: number;
  end_frame: number;
  source_time_ms: number;
}

/** Map an explicit authored millisecond interval to a half-open frame range. */
export function frameRangeFromMs(startMs: number, endMs: number, fps: number): CanonicalFrameRange {
  if (fps <= 0 || endMs <= startMs) return { start_frame: 0, end_frame: 0 };
  const normalizedStart = Math.max(0, startMs);
  if (endMs < 0) return { start_frame: 0, end_frame: 0 };
  const normalizedFps = Math.max(1, Math.trunc(fps));
  const start = startFrame(normalizedStart, normalizedFps);
  const end = Math.max(start, endFrame(endMs, normalizedFps));
  return { start_frame: start, end_frame: end };
}

export function frameRangeContains(range: CanonicalFrameRange, frameIndex: number): boolean {
  return frameIndex >= range.start_frame && frameIndex < range.end_frame;
}

/**
 * Derive source time directly from output-frame identity without first rounding
 * the frame to integer milliseconds.
 */
export function sourceTimeAtFrameMs(
  frameIndex: number,
  fps: number,
  clipStartMs: number,
  trimInMs: number,
  playbackRate: number,
): number {
  const rate = playbackRate === 0 ? 1 : playbackRate;
  if (frameIndex < 0 || fps <= 0) return trimInMs;
  const normalizedFrame = Math.max(0, Math.trunc(frameIndex));
  const normalizedFps = Math.max(1, Math.trunc(fps));
  const elapsedNumerator = normalizedFrame * 1000 - clipStartMs * normalizedFps;
  if (elapsedNumerator <= 0) return trimInMs;
  return trimInMs + (elapsedNumerator * rate) / normalizedFps;
}

/**
 * Return temporally active clips in canonical stable order: track array index,
 * z_index (default 0), then clip array index. Visibility/mute/solo remain later
 * composition/audio decisions rather than temporal-activity rules.
 */
export function activeClipsAtFrame(document: TimelineV2Document, frameIndex: number): CanonicalActiveClip[] {
  const fps = Math.trunc(document.canvas.fps);
  const normalizedFrame = Math.trunc(frameIndex);
  if (normalizedFrame < 0 || fps <= 0) return [];
  const result: CanonicalActiveClip[] = [];
  for (const [trackIndex, track] of document.tracks.entries()) {
    for (const [clipIndex, clip] of track.clips.entries()) {
      if (!activeAtFrame(normalizedFrame, clip.start_ms, clip.duration_ms, fps)) continue;
      result.push({
        track_index: trackIndex,
        clip_index: clipIndex,
        track_id: track.id,
        clip_id: clip.id,
        z_index: clip.z_index ?? 0,
        start_frame: startFrame(clip.start_ms, fps),
        end_frame: endFrame(clip.start_ms + clip.duration_ms, fps),
        source_time_ms: sourceTimeAtFrameMs(
          normalizedFrame,
          fps,
          clip.start_ms,
          clip.trim_in_ms,
          clip.playback_rate ?? 1,
        ),
      });
    }
  }
  return result.sort((left, right) => (
    left.track_index - right.track_index
    || left.z_index - right.z_index
    || left.clip_index - right.clip_index
  ));
}
