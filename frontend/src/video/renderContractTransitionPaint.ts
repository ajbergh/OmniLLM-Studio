import { easeProgress } from './renderContract';
import {
  TRANSITION_STATE_CONTRACT_V1,
  type CanonicalTransitionState,
} from './renderContractTransitions';

export const TRANSITION_PAINT_CONTRACT_V1 = 'transition-paint-v1' as const;
export const TRANSITION_PAINT_OWNER_ALPHA = 'owner-opacity' as const;
export const TRANSITION_PAINT_CROSSFADE = 'pair-crossfade' as const;
export const TRANSITION_PAINT_DIP_BLACK = 'dip-to-black' as const;
export const TRANSITION_PAINT_OWNER_TRANSLATE = 'owner-translate' as const;
export const TRANSITION_PAINT_PAIR_SLIDE = 'pair-slide' as const;
export const TRANSITION_PAINT_OWNER_WIPE = 'owner-wipe' as const;
export const TRANSITION_PAINT_PAIR_WIPE = 'pair-wipe' as const;
export const TRANSITION_PAINT_OWNER_ZOOM = 'owner-zoom' as const;
export const TRANSITION_PAINT_PAIR_ZOOM = 'pair-zoom' as const;
export const TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER = 'layer-multiplier' as const;
export const TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION = 'canvas-fraction' as const;
export const TRANSITION_PAINT_CLIP_LAYER_FRACTION = 'layer-fraction' as const;

/**
 * Report whether transition-paint-v1 defines paint semantics for this type.
 * FrameState callers use this to retain explicit unresolved debt for valid
 * transition families whose paint is intentionally not canonical yet.
 */
export function supportsTransitionPaint(transitionType: string): boolean {
  switch (transitionType.trim().toLowerCase()) {
    case 'fade':
    case 'crossfade':
    case 'dip_to_black':
    case 'slide':
    case 'wipe':
    case 'zoom':
      return true;
    default:
      return false;
  }
}

export interface CanonicalTransitionPaint {
  contract_version: typeof TRANSITION_PAINT_CONTRACT_V1;
  transition_id: string;
  type: string;
  placement: string;
  composition:
    | typeof TRANSITION_PAINT_OWNER_ALPHA
    | typeof TRANSITION_PAINT_CROSSFADE
    | typeof TRANSITION_PAINT_DIP_BLACK
    | typeof TRANSITION_PAINT_OWNER_TRANSLATE
    | typeof TRANSITION_PAINT_PAIR_SLIDE
    | typeof TRANSITION_PAINT_OWNER_WIPE
    | typeof TRANSITION_PAINT_PAIR_WIPE
    | typeof TRANSITION_PAINT_OWNER_ZOOM
    | typeof TRANSITION_PAINT_PAIR_ZOOM;
  owner_clip_id: string;
  peer_clip_id?: string;
  progress: number;
  owner_opacity?: number;
  outgoing_clip_id?: string;
  incoming_clip_id?: string;
  outgoing_weight?: number;
  incoming_weight?: number;
  black_weight?: number;
  translation_space?: typeof TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION;
  owner_offset_x?: number;
  owner_offset_y?: number;
  outgoing_offset_x?: number;
  outgoing_offset_y?: number;
  incoming_offset_x?: number;
  incoming_offset_y?: number;
  clip_space?: typeof TRANSITION_PAINT_CLIP_LAYER_FRACTION;
  owner_clip_top?: number;
  owner_clip_right?: number;
  owner_clip_bottom?: number;
  owner_clip_left?: number;
  incoming_clip_top?: number;
  incoming_clip_right?: number;
  incoming_clip_bottom?: number;
  incoming_clip_left?: number;
  scale_space?: typeof TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER;
  owner_scale?: number;
  outgoing_scale?: number;
  incoming_scale?: number;
}

/**
 * Convert canonical transition timing/ownership state into renderer-neutral
 * paint state. Fade is one-sided opacity, crossfade is a true pair blend,
 * dip-to-black has an explicit black contribution, slide uses normalized
 * canvas-fraction translation, wipe clips the isolated layer surface, and zoom
 * uses a continuous layer-scale multiplier. Direction names the entry/reveal
 * edge where applicable.
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
  const transitionId = state.id.trim();
  if (!transitionId) throw new Error('transition paint transition id must not be empty');
  if (!state.active) return undefined;
  if (!Number.isFinite(state.progress) || state.progress < 0 || state.progress > 1) {
    throw new Error(`transition ${JSON.stringify(transitionId)} progress must be finite and within [0,1]`);
  }

  const type = state.type.trim().toLowerCase();
  const placement = state.placement.trim().toLowerCase();
  const peer = state.peer_clip_id?.trim() ?? '';
  const base = {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: transitionId,
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

    case 'slide': {
      const [entryX, entryY] = transitionSlideEntryVector(state.direction);
      const exitX = -entryX;
      const exitY = -entryY;
      const translation_space = TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION;
      if (placement === 'in') {
        if (state.role !== 'incoming' || peer) {
          throw invalidTransitionPaintState(state, 'slide-in requires an incoming owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_TRANSLATE,
          translation_space,
          owner_offset_x: signedZero(entryX * (1 - state.progress)),
          owner_offset_y: signedZero(entryY * (1 - state.progress)),
        };
      }
      if (placement === 'out') {
        if (state.role !== 'outgoing' || peer) {
          throw invalidTransitionPaintState(state, 'slide-out requires an outgoing owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_TRANSLATE,
          translation_space,
          owner_offset_x: signedZero(exitX * state.progress),
          owner_offset_y: signedZero(exitY * state.progress),
        };
      }
      if (placement === 'between') {
        if (!peer) throw invalidTransitionPaintState(state, 'slide between requires an explicit peer');
        const pair = transitionPaintPairRoles(owner, state);
        return {
          ...base,
          composition: TRANSITION_PAINT_PAIR_SLIDE,
          translation_space,
          outgoing_clip_id: pair.outgoingClipId,
          incoming_clip_id: pair.incomingClipId,
          outgoing_offset_x: signedZero(exitX * state.progress),
          outgoing_offset_y: signedZero(exitY * state.progress),
          incoming_offset_x: signedZero(entryX * (1 - state.progress)),
          incoming_offset_y: signedZero(entryY * (1 - state.progress)),
        };
      }
      throw invalidTransitionPaintState(state, 'slide requires in, out, or between placement');
    }

    case 'wipe': {
      const clip_space = TRANSITION_PAINT_CLIP_LAYER_FRACTION;
      if (placement === 'in') {
        if (state.role !== 'incoming' || peer) {
          throw invalidTransitionPaintState(state, 'wipe-in requires an incoming owner with no peer');
        }
        const clip = transitionWipeInsets(state.direction, state.progress, false);
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_WIPE,
          clip_space,
          owner_clip_top: clip.top,
          owner_clip_right: clip.right,
          owner_clip_bottom: clip.bottom,
          owner_clip_left: clip.left,
        };
      }
      if (placement === 'out') {
        if (state.role !== 'outgoing' || peer) {
          throw invalidTransitionPaintState(state, 'wipe-out requires an outgoing owner with no peer');
        }
        const clip = transitionWipeInsets(state.direction, state.progress, true);
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_WIPE,
          clip_space,
          owner_clip_top: clip.top,
          owner_clip_right: clip.right,
          owner_clip_bottom: clip.bottom,
          owner_clip_left: clip.left,
        };
      }
      if (placement === 'between') {
        if (!peer) throw invalidTransitionPaintState(state, 'wipe between requires an explicit peer');
        const pair = transitionPaintPairRoles(owner, state);
        const clip = transitionWipeInsets(state.direction, state.progress, false);
        return {
          ...base,
          composition: TRANSITION_PAINT_PAIR_WIPE,
          clip_space,
          outgoing_clip_id: pair.outgoingClipId,
          incoming_clip_id: pair.incomingClipId,
          incoming_clip_top: clip.top,
          incoming_clip_right: clip.right,
          incoming_clip_bottom: clip.bottom,
          incoming_clip_left: clip.left,
        };
      }
      throw invalidTransitionPaintState(state, 'wipe requires in, out, or between placement');
    }

    case 'zoom': {
      const scale_space = TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER;
      if (placement === 'in') {
        if (state.role !== 'incoming' || peer) {
          throw invalidTransitionPaintState(state, 'zoom-in requires an incoming owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_ZOOM,
          scale_space,
          owner_opacity: clamp01(state.progress),
          owner_scale: transitionZoomScale(state.progress),
        };
      }
      if (placement === 'out') {
        if (state.role !== 'outgoing' || peer) {
          throw invalidTransitionPaintState(state, 'zoom-out requires an outgoing owner with no peer');
        }
        return {
          ...base,
          composition: TRANSITION_PAINT_OWNER_ZOOM,
          scale_space,
          owner_opacity: clamp01(1 - state.progress),
          owner_scale: transitionZoomScale(1 - state.progress),
        };
      }
      if (placement === 'between') {
        if (!peer) throw invalidTransitionPaintState(state, 'zoom between requires an explicit peer');
        const pair = transitionPaintPairRoles(owner, state);
        return {
          ...base,
          composition: TRANSITION_PAINT_PAIR_ZOOM,
          scale_space,
          outgoing_clip_id: pair.outgoingClipId,
          incoming_clip_id: pair.incomingClipId,
          outgoing_weight: clamp01(1 - state.progress),
          incoming_weight: clamp01(state.progress),
          outgoing_scale: transitionZoomScale(1 - state.progress),
          incoming_scale: transitionZoomScale(state.progress),
        };
      }
      throw invalidTransitionPaintState(state, 'zoom requires in, out, or between placement');
    }

    default:
      throw new Error(`transition ${JSON.stringify(transitionId)} type ${JSON.stringify(state.type)} does not yet have canonical paint semantics`);
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

function transitionSlideEntryVector(direction: string | undefined): [number, number] {
  switch ((direction ?? '').trim().toLowerCase()) {
    case 'left': return [-1, 0];
    case 'right': return [1, 0];
    case 'up': return [0, -1];
    case 'down': return [0, 1];
    default: throw new Error('slide requires direction left, right, up, or down');
  }
}

function transitionWipeInsets(
  direction: string | undefined,
  progress: number,
  outgoing: boolean,
): { top: number; right: number; bottom: number; left: number } {
  const hidden = outgoing ? progress : 1 - progress;
  let top = 0;
  let right = 0;
  let bottom = 0;
  let left = 0;
  switch ((direction ?? '').trim().toLowerCase()) {
    case 'left':
      if (outgoing) left = hidden; else right = hidden;
      break;
    case 'right':
      if (outgoing) right = hidden; else left = hidden;
      break;
    case 'up':
      if (outgoing) top = hidden; else bottom = hidden;
      break;
    case 'down':
      if (outgoing) bottom = hidden; else top = hidden;
      break;
    default:
      throw new Error('wipe requires direction left, right, up, or down');
  }
  return { top: signedZero(top), right: signedZero(right), bottom: signedZero(bottom), left: signedZero(left) };
}

function transitionZoomScale(progress: number): number {
  return 0.82 + 0.18 * easeProgress(progress, 'ease-out');
}

function invalidTransitionPaintState(state: CanonicalTransitionState, message: string): Error {
  return new Error(`transition ${JSON.stringify(state.id.trim())}: ${message}`);
}

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value));
}

function signedZero(value: number): number {
  return Object.is(value, -0) ? 0 : value;
}
