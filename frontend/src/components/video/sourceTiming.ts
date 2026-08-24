import { sourceTimeMs } from '../../video/renderContract';
import { sourceTimeAtFrameMs } from '../../video/renderContractEvaluation';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';

export type TimelineSourceAddress =
  | { kind: 'frame'; frameIndex: number; fps: number }
  | { kind: 'time'; timelineMs: number };

type CanonicalSourceTimeState = Pick<CanonicalFrameLayerState, 'source_time_ms'>;

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
 * Deterministic callers without evaluated FrameState derive source time from
 * rational frame identity. Interactive free-running playback uses timeline
 * time and intentionally remains sub-frame/responsive rather than quantizing
 * the user's playhead to an output frame.
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
 * Resolve source time for visual preview media.
 *
 * A successful canonical frame-addressed preview projection already contains
 * the evaluated source time and therefore owns that deterministic decision.
 * Free-running playback and explicit compatibility/fail-closed fallback keep
 * using the established address evaluator. Audio intentionally does not pass a
 * visual FrameState here; its timing migrates with AudioGraph consumption.
 */
export function sourceTimeForPreviewMediaMs(
  address: TimelineSourceAddress,
  canonicalState: CanonicalSourceTimeState | undefined,
  clipStartMs: number,
  trimInMs: number,
  playbackRate: number,
): number {
  if (address.kind === 'frame' && canonicalState) {
    return canonicalState.source_time_ms;
  }
  return sourceTimeForAddressMs(address, clipStartMs, trimInMs, playbackRate);
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
