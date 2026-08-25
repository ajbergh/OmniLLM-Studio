import type { CanonicalFrameLayerState, CanonicalVisualFrameState } from './renderContractFrameState';
import {
  TRANSITION_PAINT_CONTRACT_V1,
  type CanonicalTransitionPaint,
} from './renderContractTransitionPaint';

export const TRANSITION_PAIR_SURFACE_PLAN_V1 = 'transition-pair-surface-plan-v1' as const;

export type TransitionPairSurfaceDeferredReason =
  | 'frame-non-authoritative'
  | 'pair-input-missing'
  | 'pair-input-non-authoritative'
  | 'pair-inputs-not-adjacent';

export interface CanonicalTransitionPairSurface {
  transition_id: string;
  composition: string;
  owner_clip_id: string;
  peer_clip_id: string;
  outgoing_clip_id: string;
  incoming_clip_id: string;
  lower_clip_id: string;
  upper_clip_id: string;
  lower_layer_index: number;
  upper_layer_index: number;
  replacement_layer_index: number;
}

export interface DeferredTransitionPairSurface {
  transition_id: string;
  owner_clip_id: string;
  peer_clip_id: string;
  outgoing_clip_id: string;
  incoming_clip_id: string;
  reason: TransitionPairSurfaceDeferredReason;
}

export interface CanonicalTransitionPairSurfacePlan {
  contract_version: typeof TRANSITION_PAIR_SURFACE_PLAN_V1;
  frame_index: number;
  surfaces: CanonicalTransitionPairSurface[];
  deferred: DeferredTransitionPairSurface[];
  authoritative: boolean;
}

/** Only the canonical FrameState fields used by pair-surface planning. */
export type TransitionPairSurfaceFrameInput = Pick<
  CanonicalVisualFrameState,
  'contract_version' | 'frame_index' | 'layers' | 'authoritative'
>;

/**
 * Resolve pair-transition surface placement without defining browser paint.
 *
 * A true two-input surface can replace two canonical layers without changing
 * any unrelated layer ordering only when those inputs are adjacent in the
 * already-evaluated bottom-to-top layer list. Non-adjacent, missing, or
 * non-authoritative inputs remain explicitly deferred rather than being
 * regrouped or independently opacity-scaled.
 */
export function evaluateTransitionPairSurfacePlan(
  frame: TransitionPairSurfaceFrameInput,
): CanonicalTransitionPairSurfacePlan {
  if (frame.contract_version !== 'visual-frame-state-v1') {
    throw new Error(`transition pair surfaces require visual-frame-state-v1 input`);
  }

  const plan: CanonicalTransitionPairSurfacePlan = {
    contract_version: TRANSITION_PAIR_SURFACE_PLAN_V1,
    frame_index: frame.frame_index,
    surfaces: [],
    deferred: [],
    authoritative: frame.authoritative,
  };

  const indexByClip = new Map<string, number>();
  frame.layers.forEach((layer, index) => {
    const clipId = layer.clip_id.trim();
    if (!clipId) throw new Error(`layers[${index}].clip_id: pair surface clip id must not be empty`);
    if (indexByClip.has(clipId)) throw new Error(`layers[${index}].clip_id: duplicate active clip id ${JSON.stringify(clipId)}`);
    indexByClip.set(clipId, index);
  });

  const claimed = new Set<string>();
  for (const [ownerIndex, ownerLayer] of frame.layers.entries()) {
    for (const paint of ownerLayer.transition_paint ?? []) {
      if (!isPairSurfacePaint(paint)) continue;
      validatePairPaintOwner(ownerLayer, paint, ownerIndex);
      const outgoing = paint.outgoing_clip_id?.trim() ?? '';
      const incoming = paint.incoming_clip_id?.trim() ?? '';
      const peer = paint.peer_clip_id?.trim() ?? '';
      if (!outgoing || !incoming || !peer || outgoing === incoming) {
        throw new Error(`layers[${ownerIndex}].transition_paint: pair paint requires distinct outgoing/incoming clips and a peer`);
      }
      const expectedPair = new Set([ownerLayer.clip_id, peer]);
      if (!expectedPair.has(outgoing) || !expectedPair.has(incoming) || expectedPair.size !== 2) {
        throw new Error(`layers[${ownerIndex}].transition_paint: pair paint inputs must be owner and peer`);
      }
      if (claimed.has(outgoing) || claimed.has(incoming)) {
        throw new Error(`layers[${ownerIndex}].transition_paint: a clip cannot participate in multiple pair surfaces at one frame`);
      }

      const base = {
        transition_id: paint.transition_id,
        owner_clip_id: ownerLayer.clip_id,
        peer_clip_id: peer,
        outgoing_clip_id: outgoing,
        incoming_clip_id: incoming,
      };
      const outgoingIndex = indexByClip.get(outgoing);
      const incomingIndex = indexByClip.get(incoming);
      if (outgoingIndex === undefined || incomingIndex === undefined) {
        plan.deferred.push({ ...base, reason: 'pair-input-missing' });
        plan.authoritative = false;
        continue;
      }
      if (!frame.authoritative) {
        plan.deferred.push({ ...base, reason: 'frame-non-authoritative' });
        plan.authoritative = false;
        continue;
      }
      if (!frame.layers[outgoingIndex].authoritative || !frame.layers[incomingIndex].authoritative) {
        plan.deferred.push({ ...base, reason: 'pair-input-non-authoritative' });
        plan.authoritative = false;
        continue;
      }

      const lowerLayerIndex = Math.min(outgoingIndex, incomingIndex);
      const upperLayerIndex = Math.max(outgoingIndex, incomingIndex);
      if (upperLayerIndex !== lowerLayerIndex + 1) {
        plan.deferred.push({ ...base, reason: 'pair-inputs-not-adjacent' });
        plan.authoritative = false;
        continue;
      }

      claimed.add(outgoing);
      claimed.add(incoming);
      plan.surfaces.push({
        ...base,
        composition: paint.composition,
        lower_clip_id: frame.layers[lowerLayerIndex].clip_id,
        upper_clip_id: frame.layers[upperLayerIndex].clip_id,
        lower_layer_index: lowerLayerIndex,
        upper_layer_index: upperLayerIndex,
        replacement_layer_index: lowerLayerIndex,
      });
    }
  }
  return plan;
}

function isPairSurfacePaint(paint: CanonicalTransitionPaint): boolean {
  if (paint.contract_version !== TRANSITION_PAINT_CONTRACT_V1) {
    throw new Error(`transition pair surfaces require transition-paint-v1 instructions`);
  }
  if (paint.composition === 'pair-crossfade' || paint.composition === 'pair-slide'
    || paint.composition === 'pair-wipe' || paint.composition === 'pair-zoom') return true;
  return paint.composition === 'dip-to-black'
    && Boolean(paint.outgoing_clip_id?.trim())
    && Boolean(paint.incoming_clip_id?.trim());
}

function validatePairPaintOwner(
  ownerLayer: CanonicalFrameLayerState,
  paint: CanonicalTransitionPaint,
  ownerIndex: number,
): void {
  if (!paint.transition_id.trim()) throw new Error(`layers[${ownerIndex}].transition_paint: transition id must not be empty`);
  if (paint.owner_clip_id.trim() !== ownerLayer.clip_id.trim()) {
    throw new Error(`layers[${ownerIndex}].transition_paint: owner clip id must match containing layer`);
  }
}
