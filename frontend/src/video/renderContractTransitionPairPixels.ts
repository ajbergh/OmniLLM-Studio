import {
  TRANSITION_PAINT_CONTRACT_V1,
  type CanonicalTransitionPaint,
} from './renderContractTransitionPaint';
import type { CanonicalTransitionPairSurface } from './renderContractTransitionPairSurfaces';

export const TRANSITION_PAIR_PIXEL_COMPOSITION_V1 = 'transition-pair-pixel-composition-v1' as const;
export const TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB = 'srgb' as const;
export const TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB = 'linear-srgb' as const;
export const TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB = 'srgb' as const;
export const TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1 = 'iec-61966-2-1-srgb' as const;
export const TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT = 'straight' as const;
export const TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED = 'premultiplied' as const;
export const TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT = 'straight' as const;
export const TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER = 'unit-interval-before-output-transfer' as const;
export const TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM = 'weighted-sum' as const;
export const TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK = 'source-over-stack' as const;
export const TRANSITION_PAIR_PIXEL_BLACK_OPAQUE = 'opaque-linear-black' as const;

export interface CanonicalTransitionPairPixelComposition {
  contract_version: typeof TRANSITION_PAIR_PIXEL_COMPOSITION_V1;
  transition_id: string;
  composition: string;
  input_color_encoding: typeof TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB;
  working_color_space: typeof TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB;
  output_color_encoding: typeof TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB;
  transfer_function: typeof TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1;
  input_alpha: typeof TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT;
  accumulator_alpha: typeof TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED;
  output_alpha: typeof TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT;
  clamp_policy: typeof TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER;
  blend_operator:
    | typeof TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM
    | typeof TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK;
  lower_clip_id: string;
  upper_clip_id: string;
  outgoing_clip_id: string;
  incoming_clip_id: string;
  outgoing_weight?: number;
  incoming_weight?: number;
  black_weight?: number;
  black_source?: typeof TRANSITION_PAIR_PIXEL_BLACK_OPAQUE;
  stack_bottom_clip_id?: string;
  stack_top_clip_id?: string;
}

/**
 * Bind exact renderer-independent pair-pixel semantics to a stack-safe pair
 * surface and its canonical transition paint.
 *
 * RGB inputs are decoded with the IEC 61966-2-1 sRGB transfer function,
 * composition runs in linear sRGB with premultiplied accumulation, values are
 * clamped to the unit interval, and RGB is encoded back to sRGB after straight
 * output alpha is recovered. Crossfade, pair zoom, and between dip-to-black
 * apply canonical weights exactly once. Slide and wipe preserve canonical
 * lower/upper source-over order and add no transition weight.
 */
export function evaluateTransitionPairPixelComposition(
  surface: CanonicalTransitionPairSurface,
  paint: CanonicalTransitionPaint,
): CanonicalTransitionPairPixelComposition {
  const transitionId = surface.transition_id.trim();
  if (!transitionId) throw new Error('transition pair pixel composition requires a transition id');
  if (paint.contract_version !== TRANSITION_PAINT_CONTRACT_V1) {
    throw new Error(`transition pair pixel composition requires ${TRANSITION_PAINT_CONTRACT_V1} paint`);
  }
  if (paint.transition_id.trim() !== transitionId) {
    throw new Error('transition pair pixel composition paint transition id must match surface');
  }
  const composition = surface.composition.trim();
  if (paint.composition.trim() !== composition) {
    throw new Error('transition pair pixel composition paint composition must match surface');
  }

  const outgoing = surface.outgoing_clip_id.trim();
  const incoming = surface.incoming_clip_id.trim();
  if ((paint.outgoing_clip_id?.trim() ?? '') !== outgoing
    || (paint.incoming_clip_id?.trim() ?? '') !== incoming) {
    throw new Error('transition pair pixel composition paint inputs must match surface');
  }

  const lower = surface.lower_clip_id.trim();
  const upper = surface.upper_clip_id.trim();
  if (!lower || !upper || lower === upper) {
    throw new Error('transition pair pixel composition requires distinct lower and upper clips');
  }
  if (!pairIdsMatchOwnerPeer(
    surface.owner_clip_id.trim(),
    surface.peer_clip_id.trim(),
    outgoing,
    incoming,
  )) {
    throw new Error('transition pair pixel composition surface inputs must be owner and peer');
  }
  if (!pairPixelIdsMatch(lower, upper, outgoing, incoming)) {
    throw new Error('transition pair pixel composition lower/upper clips must be the outgoing/incoming pair');
  }

  const result: CanonicalTransitionPairPixelComposition = {
    contract_version: TRANSITION_PAIR_PIXEL_COMPOSITION_V1,
    transition_id: transitionId,
    composition,
    input_color_encoding: TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB,
    working_color_space: TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB,
    output_color_encoding: TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB,
    transfer_function: TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1,
    input_alpha: TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT,
    accumulator_alpha: TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED,
    output_alpha: TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT,
    clamp_policy: TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER,
    blend_operator: TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK,
    lower_clip_id: lower,
    upper_clip_id: upper,
    outgoing_clip_id: outgoing,
    incoming_clip_id: incoming,
  };

  switch (composition) {
    case 'pair-crossfade':
    case 'pair-zoom': {
      const [outgoingWeight, incomingWeight] = requirePairWeights(paint);
      return {
        ...result,
        blend_operator: TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM,
        outgoing_weight: outgoingWeight,
        incoming_weight: incomingWeight,
      };
    }

    case 'dip-to-black': {
      const [outgoingWeight, incomingWeight, blackWeight] = requirePairBlackWeights(paint);
      return {
        ...result,
        blend_operator: TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM,
        outgoing_weight: outgoingWeight,
        incoming_weight: incomingWeight,
        black_weight: blackWeight,
        black_source: TRANSITION_PAIR_PIXEL_BLACK_OPAQUE,
      };
    }

    case 'pair-slide':
    case 'pair-wipe':
      if (paint.outgoing_weight !== undefined
        || paint.incoming_weight !== undefined
        || paint.black_weight !== undefined) {
        throw new Error(`transition ${JSON.stringify(transitionId)} source-over pair paint must not carry pair weights`);
      }
      return {
        ...result,
        blend_operator: TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK,
        stack_bottom_clip_id: lower,
        stack_top_clip_id: upper,
      };

    default:
      throw new Error(`transition ${JSON.stringify(transitionId)} composition ${JSON.stringify(composition)} does not have pair-pixel semantics`);
  }
}

function requirePairWeights(paint: CanonicalTransitionPaint): [number, number] {
  if (paint.outgoing_weight === undefined || paint.incoming_weight === undefined) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} weighted pair paint requires outgoing and incoming weights`);
  }
  if (paint.black_weight !== undefined) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} weighted pair paint must not carry black weight`);
  }
  validateWeight(paint.transition_id, 'outgoing', paint.outgoing_weight);
  validateWeight(paint.transition_id, 'incoming', paint.incoming_weight);
  if (!unitSum(paint.outgoing_weight + paint.incoming_weight)) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} pair weights must sum to 1`);
  }
  return [paint.outgoing_weight, paint.incoming_weight];
}

function requirePairBlackWeights(paint: CanonicalTransitionPaint): [number, number, number] {
  if (paint.outgoing_weight === undefined
    || paint.incoming_weight === undefined
    || paint.black_weight === undefined) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} dip-to-black pair paint requires outgoing, incoming, and black weights`);
  }
  validateWeight(paint.transition_id, 'outgoing', paint.outgoing_weight);
  validateWeight(paint.transition_id, 'incoming', paint.incoming_weight);
  validateWeight(paint.transition_id, 'black', paint.black_weight);
  if (!unitSum(paint.outgoing_weight + paint.incoming_weight + paint.black_weight)) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} dip-to-black weights must sum to 1`);
  }
  return [paint.outgoing_weight, paint.incoming_weight, paint.black_weight];
}

function validateWeight(transitionId: string, label: string, weight: number): void {
  if (!Number.isFinite(weight) || weight < 0 || weight > 1) {
    throw new Error(`transition ${JSON.stringify(transitionId)} ${label} weight must be finite and within [0,1]`);
  }
}

function pairIdsMatchOwnerPeer(
  owner: string,
  peer: string,
  outgoing: string,
  incoming: string,
): boolean {
  if (!owner || !peer || owner === peer || outgoing === incoming) return false;
  return (outgoing === owner && incoming === peer) || (outgoing === peer && incoming === owner);
}

function pairPixelIdsMatch(lower: string, upper: string, outgoing: string, incoming: string): boolean {
  return (lower === outgoing && upper === incoming) || (lower === incoming && upper === outgoing);
}

function unitSum(value: number): boolean {
  return Math.abs(value - 1) <= 1e-9;
}
