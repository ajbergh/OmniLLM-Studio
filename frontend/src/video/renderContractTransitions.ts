import { endFrame, startFrame } from './renderContract';
import { normalizeTimelineV2EvaluationInputs } from './renderContractNormalize';
import type { TimelineV2Clip, TimelineV2Document, TimelineV2Transition } from './renderContractTypes';

export const TRANSITION_STATE_CONTRACT_V1 = 'transition-state-v1' as const;

export interface CanonicalTransitionState {
  contract_version: typeof TRANSITION_STATE_CONTRACT_V1;
  id: string;
  type: string;
  placement: string;
  direction?: string;
  peer_clip_id?: string;
  peer_track_index?: number;
  peer_clip_index?: number;
  role: 'incoming' | 'outgoing';
  peer_role?: 'incoming' | 'outgoing';
  start_frame: number;
  end_frame: number;
  progress: number;
  active: boolean;
}

/**
 * Evaluate exact-frame transition placement and peer ownership without
 * defining type-specific paint. Between transitions require real authored
 * overlap; no hidden source handles or adjacency are inferred.
 */
export function evaluateClipTransitionsAtFrame(
  document: TimelineV2Document,
  trackIndex: number,
  clipIndex: number,
  frameIndex: number,
): CanonicalTransitionState[] {
  const normalized = normalizeTimelineV2EvaluationInputs(document);
  return evaluateClipTransitionsAtFrameNormalized(normalized, trackIndex, clipIndex, frameIndex);
}

export function evaluateClipTransitionsAtFrameNormalized(
  document: TimelineV2Document,
  trackIndex: number,
  clipIndex: number,
  frameIndex: number,
): CanonicalTransitionState[] {
  const track = document.tracks[trackIndex];
  const owner = track?.clips[clipIndex];
  if (!owner) throw new Error('transition owner index is outside timeline bounds');
  if (frameIndex < 0 || frameIndex >= endFrame(document.duration_ms, document.canvas.fps)) {
    throw new Error(`frame index ${frameIndex} is outside timeline frame range`);
  }

  const seen = new Set<string>();
  return (owner.transitions ?? []).map((transition, transitionIndex) => {
    const path = `tracks[${trackIndex}].clips[${clipIndex}].transitions[${transitionIndex}]`;
    const id = transition.id.trim();
    if (!id) throw new Error(`${path}.id: transition id must not be empty`);
    if (seen.has(id)) throw new Error(`${path}.id: duplicate transition id ${JSON.stringify(id)}`);
    seen.add(id);
    if (transition.duration_ms < 1) throw new Error(`${path}.duration_ms: transition duration must be positive`);

    const state: CanonicalTransitionState = {
      contract_version: TRANSITION_STATE_CONTRACT_V1,
      id,
      type: transition.type.trim().toLowerCase(),
      placement: transition.placement.trim().toLowerCase(),
      ...(normalizedDirection(transition) ? { direction: normalizedDirection(transition) } : {}),
      role: 'incoming',
      start_frame: 0,
      end_frame: 0,
      progress: 0,
      active: false,
    };

    let startMs = 0;
    let endMs = 0;
    switch (state.placement) {
      case 'in':
        if (transition.peer_clip_id?.trim()) throw new Error(`${path}.peer_clip_id: in transitions must not declare a peer`);
        startMs = owner.start_ms;
        endMs = Math.min(owner.start_ms + transition.duration_ms, owner.start_ms + owner.duration_ms);
        state.role = 'incoming';
        break;
      case 'out':
        if (transition.peer_clip_id?.trim()) throw new Error(`${path}.peer_clip_id: out transitions must not declare a peer`);
        endMs = owner.start_ms + owner.duration_ms;
        startMs = Math.max(owner.start_ms, endMs - transition.duration_ms);
        state.role = 'outgoing';
        break;
      case 'between': {
        const peerId = transition.peer_clip_id?.trim() ?? '';
        if (!peerId) throw new Error(`${path}.peer_clip_id: between transitions require an explicit peer`);
        if (peerId === owner.id) throw new Error(`${path}.peer_clip_id: transition peer must differ from owner`);
        const peer = findPeer(document, peerId);
        if (!peer) throw new Error(`${path}.peer_clip_id: peer clip ${JSON.stringify(peerId)} does not exist`);
        if (peer.clip.start_ms === owner.start_ms) {
          throw new Error(`${path}.peer_clip_id: between transition owner and peer must have distinct start times`);
        }
        const overlapStart = Math.max(owner.start_ms, peer.clip.start_ms);
        const overlapEnd = Math.min(owner.start_ms + owner.duration_ms, peer.clip.start_ms + peer.clip.duration_ms);
        const overlapDuration = overlapEnd - overlapStart;
        if (overlapDuration < transition.duration_ms) {
          throw new Error(`${path}.duration_ms: between transition requires at least ${transition.duration_ms}ms of real owner/peer overlap`);
        }
        startMs = overlapStart;
        endMs = overlapStart + transition.duration_ms;
        state.peer_clip_id = peerId;
        state.peer_track_index = peer.trackIndex;
        state.peer_clip_index = peer.clipIndex;
        if (owner.start_ms < peer.clip.start_ms) {
          state.role = 'outgoing';
          state.peer_role = 'incoming';
        } else {
          state.role = 'incoming';
          state.peer_role = 'outgoing';
        }
        break;
      }
      default:
        throw new Error(`${path}.placement: unsupported transition placement ${JSON.stringify(transition.placement)}`);
    }

    if (endMs <= startMs) throw new Error(`${path}.duration_ms: transition window is empty`);
    state.start_frame = startFrame(startMs, document.canvas.fps);
    state.end_frame = endFrame(endMs, document.canvas.fps);
    state.active = state.start_frame <= frameIndex && frameIndex < state.end_frame;
    const presentationMs = (frameIndex * 1000) / document.canvas.fps;
    state.progress = clamp01((presentationMs - startMs) / (endMs - startMs));
    return state;
  });
}

function findPeer(document: TimelineV2Document, peerId: string): { trackIndex: number; clipIndex: number; clip: TimelineV2Clip } | undefined {
  for (let trackIndex = 0; trackIndex < document.tracks.length; trackIndex += 1) {
    const clips = document.tracks[trackIndex].clips;
    for (let clipIndex = 0; clipIndex < clips.length; clipIndex += 1) {
      if (clips[clipIndex].id === peerId) return { trackIndex, clipIndex, clip: clips[clipIndex] };
    }
  }
  return undefined;
}

function normalizedDirection(transition: TimelineV2Transition): string {
  const direction = transition.direction?.trim().toLowerCase() ?? '';
  if (!direction && (transition.type === 'slide' || transition.type === 'wipe')) return 'left';
  return direction;
}

function clamp01(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  if (value >= 1) return 1;
  return value;
}
