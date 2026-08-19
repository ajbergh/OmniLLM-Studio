import {
  TRANSITION_STATE_CONTRACT_V1,
  type CanonicalTransitionState,
} from './renderContractTransitions';

export const TRANSITION_PAINT_CONTRACT_V1 = 'transition-paint-v1' as const;
export const TRANSITION_PAINT_OWNER_ALPHA = 'owner-opacity' as const;
export const TRANSITION_PAINT_CROSSFADE = 'pair-crossfade' as const;
export const TRANSITION_PAINT_DIP_BLACK = 'dip-to-black' as const;

export interface CanonicalTransitionPaint {
  contract_version: typeof TRANSITION_PAINT_CONTRACT_V1;
  transition_id: string;
  type: string;
  placement: string;
  composition:
    | typeof TRANSITION_PAINT_OWNER_ALPHA
    | typeof TRANSITION_PAINT_CROSSFADE
    | typeof TRANSITION_PAINT_DIP_BLACK;
  owner_clip_id: string;
  peer_clip_id?: string;
  progress: number;
  owner_opacity?: number;
  outgoing_clip_id?: string;
  incoming_clip_id?: string;
  outgoing_weight?: number;
  incoming_weight?: number;
  black_weight?: number;
}

/**
 * Convert canonical transition timing/ownership state into renderer-neutral
 * paint weights. Inactive transitions produce no paint. Fade is one-sided
 * in/out opacity, crossfade is a true between-clip blend, and dip-to-black
 * uses outgoing/black/incoming weights around a black midpoint.
 */
export function evaluateTransitionPaint(
  ownerClipId: string,
  state: CanonicalTransitionState,
): CanonicalTransitionPaint | undefined {
  const owner = ownerClipId.trim();
  if (!owner) throw new Error('transition paint owner clip id must not be empty');
  if (state.contract_version !== TRANSITION_STATE_CONTRACT_V1) {
    throw new Error(`transition paint requires ${TRANSITION_STATE_CONTRACT_V1} input`);
  }
  if (!state.active) return undefined;
  if (!Number.isFinite(state.progress) || state.progress < 0 || state.progress > 1) {
    throw new Error(`transition ${JSON.stringify(state.id)} progress must be finite and within [0,1]`);
  }

  const type = state.type.trim().toLowerCase();
  const placement = state.placement.trim().toLowerCase();
  const peer = state.peer_clip_id?.trim() ?? '';
  const base = {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: state.id,
    type,
    placement,
    owner_clip_id: owner,
    ...(peer ? { peer_clip_id: peer } : {}),
    progress: state.progress,
  };

  switch (type) {
    case 'fade':
      if (placement === 'in') {
        if (state.role !== 'incoming' || peer) {
          throw invalidTransitionPaintState(state, 'fade-in requires an incoming owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_ALPHA,
          owner_opacity: clamp01(state.progress),
        };
      }
      if (placement === 'out') {
        if (state.role !== 'outgoing' || peer) {
          throw invalidTransitionPaintState(state, 'fade-out requires an outgoing owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_ALPHA,
          owner_opacity: clamp01(1 - state.progress),
        };
      }
      throw invalidTransitionPaintState(state, 'fade supports only in or out placement; use crossfade for a two-clip blend');

    case 'crossfade': {
      if (placement !== 'between' || !peer) {
        throw invalidTransitionPaintState(state, 'crossfade requires between placement with an explicit peer');
      }
      const pair = transitionPaintPairRoles(owner, state);
      return {
        ...base,
        composition: TRANSITION_PAINT_CROSSFADE,
        outgoing_clip_id: pair.outgoingClipId,
        incoming_clip_id: pair.incomingClipId,
        outgoing_weight: clamp01(1 - state.progress),
        incoming_weight: clamp01(state.progress),
      };
    }

    case 'dip_to_black':
      if (placement === 'in') {
        if (state.role !== 'incoming' || peer) {
          throw invalidTransitionPaintState(state, 'dip-to-black in requires an incoming owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_DIP_BLACK,
          incoming_clip_id: owner,
          incoming_weight: clamp01(state.progress),
          black_weight: clamp01(1 - state.progress),
        };
      }
      if (placement === 'out') {
        if (state.role !== 'outgoing' || peer) {
          throw invalidTransitionPaintState(state, 'dip-to-black out requires an outgoing owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_DIP_BLACK,
          outgoing_clip_id: owner,
          outgoing_weight: clamp01(1 - state.progress),
          black_weight: clamp01(state.progress),
        };
      }
      if (placement === 'between') {
        if (!peer) throw invalidTransitionPaintState(state, 'dip-to-black between requires an explicit peer');
        const pair = transitionPaintPairRoles(owner, state);
        if (state.progress < 0.5) {
          return {
            ...base,
            composition: TRANSITION_PAINT_DIP_BLACK,
            outgoing_clip_id: pair.outgoingClipId,
            incoming_clip_id: pair.incomingClipId,
            outgoing_weight: clamp01(1 - 2 * state.progress),
            incoming_weight: 0,
            black_weight: clamp01(2 * state.progress),
          };
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_DIP_BLACK,
          outgoing_clip_id: pair.outgoingClipId,
          incoming_clip_id: pair.incomingClipId,
          outgoing_weight: 0,
          incoming_weight: clamp01(2 * state.progress - 1),
          black_weight: clamp01(2 * (1 - state.progress)),
        };
      }
      throw invalidTransitionPaintState(state, 'dip-to-black requires in, out, or between placement');

    default:
      throw new Error(`transition ${JSON.stringify(state.id)} type ${JSON.stringify(state.type)} does not yet have canonical paint semantics`);
  }
}

function transitionPaintPairRoles(
  ownerClipId: string,
  state: CanonicalTransitionState,
): { outgoingClipId: string; incomingClipId: string } {
  const peer = state.peer_clip_id?.trim() ?? '';
  if (!peer || peer === ownerClipId) {
    throw invalidTransitionPaintState(state, 'between paint requires a distinct peer');
  }
  if (state.role === 'outgoing' && state.peer_role === 'incoming') {
    return { outgoingClipId: ownerClipId, incomingClipId: peer };
  }
  if (state.role === 'incoming' && state.peer_role === 'outgoing') {
    return { outgoingClipId: peer, incomingClipId: ownerClipId };
  }
  throw invalidTransitionPaintState(state, 'between paint requires complementary outgoing/incoming roles');
}

function invalidTransitionPaintState(state: CanonicalTransitionState, message: string): Error {
  return new Error(`transition ${JSON.stringify(state.id)}: ${message}`);
}

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value));
}
