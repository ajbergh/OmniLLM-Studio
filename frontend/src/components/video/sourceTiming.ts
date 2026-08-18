import { sourceTimeMs } from '../../video/renderContract';
import { sourceTimeAtFrameMs } from '../../video/renderContractEvaluation';

export type TimelineSourceAddress =
  | { kind: 'frame'; frameIndex: number; fps: number }
  | { kind: 'time'; timelineMs: number };

/** True when playhead time still represents the exact frame address supplied by a deterministic caller. */
export function frameAddressMatchesTimelineMs(
  frameIndex: number,
  fps: number,
  timelineMs: number,
): boolean {
  if (frameIndex < 0 || fps <= 0 || !Number.isFinite(timelineMs)) return false;
  return Math.abs(timelineMs - (Math.trunc(frameIndex) * 1000) / Math.trunc(fps)) <= 1e-6;
}

/**
 * Resolve media source time from the authoritative timeline address.
 *
 * Deterministic render/capture callers must pass an output-frame address so
 * source time is derived directly from rational frame identity. Interactive,
 * free-running playback uses timeline time and intentionally remains
 * sub-frame/responsive rather than quantizing the user's playhead to a frame.
 */
export function sourceTimeForAddressMs(
  address: TimelineSourceAddress,
  clipStartMs: number,
  trimInMs: number,
  playbackRate: number,
): number {
  if (address.kind === 'frame') {
    return sourceTimeAtFrameMs(
      address.frameIndex,
      address.fps,
      clipStartMs,
      trimInMs,
      playbackRate,
    );
  }
  return sourceTimeMs(address.timelineMs, clipStartMs, trimInMs, playbackRate);
}

/**
 * Deterministic frame capture needs a materially tighter seek tolerance than
 * interactive scrub/playback. Otherwise adjacent high-FPS frames can reuse a
 * stale media frame because the legacy 50 ms paused-scrub tolerance is wider
 * than an entire output frame.
 */
export function mediaSeekToleranceSeconds(address: TimelineSourceAddress): number {
  return address.kind === 'frame' ? 0.0005 : 0.05;
}
