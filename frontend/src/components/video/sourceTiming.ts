import { sourceTimeMs, startFrame } from '../../video/renderContract';
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
 * Return the canonical output-frame identity containing a free-running visual
 * playhead. The UI/audio clock remains continuous; only visual evaluation is
 * projected into the same integer frame domain used by export. Invalid or
 * negative playheads fail closed so callers can retain their time-domain path.
 */
export function playbackVisualFrameIndex(timelineMs: number, fps: number): number | null {
  const normalizedFPS = Math.trunc(fps);
  if (!Number.isFinite(timelineMs) || timelineMs < 0 || !Number.isFinite(fps) || normalizedFPS <= 0) return null;
  return startFrame(timelineMs, normalizedFPS);
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
 * the evaluated source time and therefore owns that visual decision, including
 * free-running playback after it has been projected into an authoritative output
 * frame. Explicit compatibility/fail-closed fallback keeps the established time
 * evaluator. Audio intentionally receives a separate time-domain address until
 * AudioGraph consumption lands.
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
 * Convert an exact canonical source-time boundary into a browser video seek
 * target that is safely inside the requested frame interval. JavaScript's
 * nearest Float64 representation of a rational PTS can sit infinitesimally
 * below that boundary (for example 59/30), and containers such as WebM can
 * quantize frame timestamps to milliseconds (for example 1.967 s). A seek that
 * remains below that quantized PTS can legitimately present the prior frame.
 *
 * The nudge is browser-consumer policy only: it never changes canonical source
 * time, never applies to free-running playback/audio, remains strictly below
 * the 0.5 ms deterministic presentation tolerance, and is capped to a small
 * fraction of the output-frame interval. Presentation identity is still keyed
 * to canonical source time rather than treating project FPS as source FPS.
 */
export function deterministicVideoSeekTargetSeconds(
  address: TimelineSourceAddress,
  canonicalTargetSeconds: number,
): number {
  if (address.kind !== 'frame' || !Number.isFinite(canonicalTargetSeconds) || canonicalTargetSeconds < 0) {
    return canonicalTargetSeconds;
  }
  const fps = Math.max(1, Math.trunc(address.fps));
  // 0.49 ms clears a 1 ms container timestamp rounded to the nearest tick at
  // 30/60/120 fps while staying below the strict 0.5 ms presentation tolerance.
  const nudgeSeconds = Math.min(0.00049, 1 / (fps * 16));
  return canonicalTargetSeconds + nudgeSeconds;
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
