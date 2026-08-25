import type { VideoTimelineDocument, VideoTimelineTransition } from '../../../types/video';

export const MIN_TRANSITION_DURATION_MS = 100;

export interface TransitionPeerOption {
  id: string;
  label: string;
  overlap_ms: number;
  track_index: number;
  clip_index: number;
}

type TransitionPlacement = NonNullable<VideoTimelineTransition['placement']>;

export function transitionPlacementSupported(type: VideoTimelineTransition['type'], placement: TransitionPlacement): boolean {
  if (type === 'fade') return placement !== 'between';
  if (type === 'crossfade') return placement === 'between';
  return true;
}

export function visualTransitionOwnerEligible(document: VideoTimelineDocument, ownerClipId: string): boolean {
  for (const track of document.tracks) {
    const clip = track.clips.find((candidate) => candidate.id === ownerClipId);
    if (!clip) continue;
    return track.visible && track.type !== 'audio' && track.type !== 'music' && !clip.audio_only;
  }
  return false;
}

/** Return real overlapping visual peers; no adjacency or hidden handles are inferred. */
export function transitionPeerOptions(document: VideoTimelineDocument, ownerClipId: string): TransitionPeerOption[] {
  let owner: { start_ms: number; duration_ms: number } | undefined;
  for (const track of document.tracks) {
    const clip = track.clips.find((candidate) => candidate.id === ownerClipId);
    if (!clip) continue;
    if (!track.visible || track.type === 'audio' || track.type === 'music' || clip.audio_only) return [];
    owner = clip;
    break;
  }
  if (!owner) return [];

  const result: TransitionPeerOption[] = [];
  document.tracks.forEach((track, trackIndex) => {
    if (!track.visible || track.type === 'audio' || track.type === 'music') return;
    track.clips.forEach((candidate, clipIndex) => {
      if (candidate.id === ownerClipId || candidate.audio_only || candidate.start_ms === owner!.start_ms) return;
      const overlapMs = Math.max(
        0,
        Math.min(owner!.start_ms + owner!.duration_ms, candidate.start_ms + candidate.duration_ms)
          - Math.max(owner!.start_ms, candidate.start_ms),
      );
      if (overlapMs < MIN_TRANSITION_DURATION_MS) return;
      result.push({
        id: candidate.id,
        label: `${track.name || `Track ${trackIndex + 1}`} · ${candidate.id} · ${overlapMs}ms overlap`,
        overlap_ms: overlapMs,
        track_index: trackIndex,
        clip_index: clipIndex,
      });
    });
  });
  return result;
}

export function clampTransitionDurationToPeer(requestedMs: number, peer: TransitionPeerOption): number {
  const requested = Number.isFinite(requestedMs) ? Math.round(requestedMs) : MIN_TRANSITION_DURATION_MS;
  return Math.max(MIN_TRANSITION_DURATION_MS, Math.min(peer.overlap_ms, requested));
}
