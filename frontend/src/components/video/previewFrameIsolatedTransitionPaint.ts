import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_DIP_BLACK,
  TRANSITION_PAINT_PAIR_SLIDE,
  TRANSITION_PAINT_PAIR_WIPE,
  TRANSITION_PAINT_PAIR_ZOOM,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';

export interface OrderedCanonicalPreviewLayer {
  clipId: string;
  canonicalState?: Pick<CanonicalFrameLayerState, 'clip_id' | 'transition_paint'>;
}

export interface IsolatedTransitionLayerPaint {
  clipId: string;
  opacityMultiplier: number;
  offsetXFraction: number;
  offsetYFraction: number;
  scaleMultiplier: number;
  clipPath?: string;
  additive: boolean;
}

export type IsolatedTransitionPaintMode = 'none' | 'canonical-isolated' | 'canonical-deferred';

export interface ResolvedIsolatedTransitionPaint {
  mode: IsolatedTransitionPaintMode;
  transitionId?: string;
  composition?: string;
  insertionIndex?: number;
  blackWeight: number;
  layers: IsolatedTransitionLayerPaint[];
  deferredReason?: string;
}

const PAIR_COMPOSITIONS = new Set<string>([
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_PAIR_SLIDE,
  TRANSITION_PAINT_PAIR_WIPE,
  TRANSITION_PAINT_PAIR_ZOOM,
]);

const IDENTITY_LAYER = {
  opacityMultiplier: 1,
  offsetXFraction: 0,
  offsetYFraction: 0,
  scaleMultiplier: 1,
  additive: false,
};

/**
 * Resolve frame-level transition paint that needs an isolated composition
 * surface. Pair members must be adjacent in canonical visual order; otherwise
 * collapsing them into one surface would reorder an unrelated layer, so the
 * consumer explicitly defers instead of guessing a stacking position.
 *
 * Weighted blends use additive isolated surfaces (`plus-lighter` in the DOM
 * consumer) so canonical outgoing/incoming/black weights remain arithmetic
 * contributions rather than source-over alpha attenuation.
 */
export function resolvePreviewFrameIsolatedTransitionPaint(
  orderedLayers: readonly OrderedCanonicalPreviewLayer[],
): ResolvedIsolatedTransitionPaint {
  const candidates: CanonicalTransitionPaint[] = [];
  for (const layer of orderedLayers) {
    for (const paint of layer.canonicalState?.transition_paint ?? []) {
      if (PAIR_COMPOSITIONS.has(paint.composition) || paint.composition === TRANSITION_PAINT_DIP_BLACK) {
        candidates.push(paint);
      }
    }
  }
  if (candidates.length === 0) return { mode: 'none', blackWeight: 0, layers: [] };
  if (candidates.length !== 1) {
    return deferred(candidates[0], 'multiple-isolated-transitions');
  }

  const paint = candidates[0];
  validatePaintBase(paint);

  if (paint.composition === TRANSITION_PAINT_DIP_BLACK && paint.placement !== 'between') {
    return resolveOneSidedDip(orderedLayers, paint);
  }

  const outgoingId = requireClipId(paint.outgoing_clip_id, 'outgoing_clip_id', paint);
  const incomingId = requireClipId(paint.incoming_clip_id, 'incoming_clip_id', paint);
  if (outgoingId === incomingId) throw invalidPaint(paint, 'outgoing and incoming clips must be distinct');
  const outgoingIndex = orderedLayers.findIndex((layer) => layer.clipId === outgoingId);
  const incomingIndex = orderedLayers.findIndex((layer) => layer.clipId === incomingId);
  if (outgoingIndex < 0 || incomingIndex < 0) {
    return deferred(paint, 'pair-member-not-renderable');
  }
  if (Math.abs(outgoingIndex - incomingIndex) !== 1) {
    return deferred(paint, 'pair-members-not-adjacent');
  }

  const base: ResolvedIsolatedTransitionPaint = {
    mode: 'canonical-isolated',
    transitionId: paint.transition_id,
    composition: paint.composition,
    insertionIndex: Math.max(outgoingIndex, incomingIndex),
    blackWeight: 0,
    layers: [],
  };

  switch (paint.composition) {
    case TRANSITION_PAINT_CROSSFADE: {
      const outgoingWeight = requireUnit(paint.outgoing_weight, 'outgoing_weight', paint);
      const incomingWeight = requireUnit(paint.incoming_weight, 'incoming_weight', paint);
      requireWeightSum(paint, outgoingWeight + incomingWeight);
      base.layers = [
        weightedLayer(outgoingId, outgoingWeight),
        weightedLayer(incomingId, incomingWeight),
      ];
      return base;
    }

    case TRANSITION_PAINT_PAIR_SLIDE:
      if (paint.translation_space !== TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION) {
        throw invalidPaint(paint, `translation_space must be ${TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION}`);
      }
      base.layers = [
        translatedLayer(outgoingId, paint.outgoing_offset_x, paint.outgoing_offset_y, paint),
        translatedLayer(incomingId, paint.incoming_offset_x, paint.incoming_offset_y, paint),
      ];
      return base;

    case TRANSITION_PAINT_PAIR_WIPE:
      if (paint.clip_space !== TRANSITION_PAINT_CLIP_LAYER_FRACTION) {
        throw invalidPaint(paint, `clip_space must be ${TRANSITION_PAINT_CLIP_LAYER_FRACTION}`);
      }
      base.layers = [
        { clipId: outgoingId, ...IDENTITY_LAYER },
        {
          clipId: incomingId,
          ...IDENTITY_LAYER,
          clipPath: insetPath(
            paint.incoming_clip_top,
            paint.incoming_clip_right,
            paint.incoming_clip_bottom,
            paint.incoming_clip_left,
            paint,
          ),
        },
      ];
      return base;

    case TRANSITION_PAINT_PAIR_ZOOM: {
      if (paint.scale_space !== TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER) {
        throw invalidPaint(paint, `scale_space must be ${TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER}`);
      }
      const outgoingWeight = requireUnit(paint.outgoing_weight, 'outgoing_weight', paint);
      const incomingWeight = requireUnit(paint.incoming_weight, 'incoming_weight', paint);
      requireWeightSum(paint, outgoingWeight + incomingWeight);
      base.layers = [
        weightedLayer(outgoingId, outgoingWeight, requireNonNegative(paint.outgoing_scale, 'outgoing_scale', paint)),
        weightedLayer(incomingId, incomingWeight, requireNonNegative(paint.incoming_scale, 'incoming_scale', paint)),
      ];
      return base;
    }

    case TRANSITION_PAINT_DIP_BLACK: {
      const outgoingWeight = requireUnit(paint.outgoing_weight, 'outgoing_weight', paint);
      const incomingWeight = requireUnit(paint.incoming_weight, 'incoming_weight', paint);
      const blackWeight = requireUnit(paint.black_weight, 'black_weight', paint);
      requireWeightSum(paint, outgoingWeight + incomingWeight + blackWeight);
      base.blackWeight = blackWeight;
      base.layers = [
        weightedLayer(outgoingId, outgoingWeight),
        weightedLayer(incomingId, incomingWeight),
      ];
      return base;
    }

    default:
      return deferred(paint, `unsupported-isolated-composition:${paint.composition}`);
  }
}

function resolveOneSidedDip(
  orderedLayers: readonly OrderedCanonicalPreviewLayer[],
  paint: CanonicalTransitionPaint,
): ResolvedIsolatedTransitionPaint {
  const clipId = paint.placement === 'in'
    ? requireClipId(paint.incoming_clip_id, 'incoming_clip_id', paint)
    : requireClipId(paint.outgoing_clip_id, 'outgoing_clip_id', paint);
  const clipIndex = orderedLayers.findIndex((layer) => layer.clipId === clipId);
  if (clipIndex < 0) return deferred(paint, 'dip-owner-not-renderable');
  const layerWeight = requireUnit(
    paint.placement === 'in' ? paint.incoming_weight : paint.outgoing_weight,
    paint.placement === 'in' ? 'incoming_weight' : 'outgoing_weight',
    paint,
  );
  const blackWeight = requireUnit(paint.black_weight, 'black_weight', paint);
  requireWeightSum(paint, layerWeight + blackWeight);
  return {
    mode: 'canonical-isolated',
    transitionId: paint.transition_id,
    composition: paint.composition,
    insertionIndex: clipIndex,
    blackWeight,
    layers: [weightedLayer(clipId, layerWeight)],
  };
}

function weightedLayer(clipId: string, weight: number, scaleMultiplier = 1): IsolatedTransitionLayerPaint {
  return {
    clipId,
    ...IDENTITY_LAYER,
    opacityMultiplier: weight,
    scaleMultiplier,
    additive: true,
  };
}

function translatedLayer(
  clipId: string,
  x: number | undefined,
  y: number | undefined,
  paint: CanonicalTransitionPaint,
): IsolatedTransitionLayerPaint {
  return {
    clipId,
    ...IDENTITY_LAYER,
    offsetXFraction: requireFinite(x, 'pair offset x', paint),
    offsetYFraction: requireFinite(y, 'pair offset y', paint),
  };
}

function insetPath(
  top: number | undefined,
  right: number | undefined,
  bottom: number | undefined,
  left: number | undefined,
  paint: CanonicalTransitionPaint,
): string {
  return `inset(${requireUnit(top, 'incoming_clip_top', paint) * 100}% ${requireUnit(right, 'incoming_clip_right', paint) * 100}% ${requireUnit(bottom, 'incoming_clip_bottom', paint) * 100}% ${requireUnit(left, 'incoming_clip_left', paint) * 100}%)`;
}

function validatePaintBase(paint: CanonicalTransitionPaint): void {
  if (paint.contract_version !== TRANSITION_PAINT_CONTRACT_V1) throw invalidPaint(paint, `contract_version must be ${TRANSITION_PAINT_CONTRACT_V1}`);
  if (!paint.transition_id.trim()) throw invalidPaint(paint, 'transition_id must not be empty');
}

function requireClipId(value: string | undefined, field: string, paint: CanonicalTransitionPaint): string {
  const resolved = value?.trim() ?? '';
  if (!resolved) throw invalidPaint(paint, `${field} must not be empty`);
  return resolved;
}

function requireFinite(value: number | undefined, field: string, paint: CanonicalTransitionPaint): number {
  if (value === undefined || !Number.isFinite(value)) throw invalidPaint(paint, `${field} must be finite`);
  return value;
}

function requireUnit(value: number | undefined, field: string, paint: CanonicalTransitionPaint): number {
  const resolved = requireFinite(value, field, paint);
  if (resolved < 0 || resolved > 1) throw invalidPaint(paint, `${field} must be within [0,1]`);
  return resolved;
}

function requireNonNegative(value: number | undefined, field: string, paint: CanonicalTransitionPaint): number {
  const resolved = requireFinite(value, field, paint);
  if (resolved < 0) throw invalidPaint(paint, `${field} must be non-negative`);
  return resolved;
}

function requireWeightSum(paint: CanonicalTransitionPaint, sum: number): void {
  if (Math.abs(sum - 1) > 1e-9) throw invalidPaint(paint, `isolated contribution weights must sum to 1; got ${sum}`);
}

function deferred(paint: CanonicalTransitionPaint | undefined, reason: string): ResolvedIsolatedTransitionPaint {
  return {
    mode: 'canonical-deferred',
    transitionId: paint?.transition_id,
    composition: paint?.composition,
    blackWeight: 0,
    layers: [],
    deferredReason: reason,
  };
}

function invalidPaint(paint: CanonicalTransitionPaint, detail: string): Error {
  return new Error(`canonical isolated transition ${JSON.stringify(paint.transition_id)}: ${detail}`);
}
