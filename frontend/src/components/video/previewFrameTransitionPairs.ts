import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_PAIR_SLIDE,
  TRANSITION_PAINT_PAIR_WIPE,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import {
  evaluateTransitionPairPixelComposition,
  TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK,
  type CanonicalTransitionPairPixelComposition,
} from '../../video/renderContractTransitionPairPixels';
import {
  evaluateTransitionPairSurfacePlan,
  type CanonicalTransitionPairSurface,
} from '../../video/renderContractTransitionPairSurfaces';

export type PreviewTransitionPairExecution = 'source-over-dom' | 'weighted-canvas-deferred';

export interface PreviewTransitionPairLayerPaint {
  offsetXFraction: number;
  offsetYFraction: number;
  scaleMultiplier: number;
  clipPath?: string;
}

export interface PreviewTransitionPairSlot<T> {
  kind: 'pair';
  lower: T;
  upper: T;
  surface: CanonicalTransitionPairSurface;
  paint: CanonicalTransitionPaint;
  pixel: CanonicalTransitionPairPixelComposition;
  execution: PreviewTransitionPairExecution;
  layerPaintByClipId: ReadonlyMap<string, PreviewTransitionPairLayerPaint>;
}

export interface PreviewTransitionSingleSlot<T> {
  kind: 'single';
  layer: T;
}

export interface PreviewTransitionPairPlan<T> {
  mode: 'legacy' | 'canonical-none' | 'canonical-source-over' | 'canonical-weighted-deferred' | 'canonical-mixed';
  slots: Array<PreviewTransitionSingleSlot<T> | PreviewTransitionPairSlot<T>>;
  deferredReasons: string[];
}

const IDENTITY_LAYER_PAINT: PreviewTransitionPairLayerPaint = {
  offsetXFraction: 0,
  offsetYFraction: 0,
  scaleMultiplier: 1,
};

type PairablePreviewLayer = {
  clip: { id: string };
  canonicalState?: CanonicalFrameLayerState;
};

/**
 * Convert the canonical bottom-to-top preview layer list into replacement slots
 * without changing unrelated layer order.
 *
 * Source-over pair slide/wipe can be executed by the DOM painter because the
 * pair-pixel contract adds no weights. Weighted crossfade/zoom/dip remain
 * explicitly deferred until the Canvas path can obey linear-sRGB premultiplied
 * accumulation; callers must not approximate those families with CSS opacity.
 */
export function planPreviewFrameTransitionPairs<T extends PairablePreviewLayer>(
  frameIndex: number | null,
  layers: readonly T[],
): PreviewTransitionPairPlan<T> {
  if (frameIndex === null || layers.some((layer) => !layer.canonicalState)) {
    return {
      mode: 'legacy',
      slots: layers.map((layer) => ({ kind: 'single', layer })),
      deferredReasons: [],
    };
  }

  const canonicalLayers = layers.map((layer) => layer.canonicalState as CanonicalFrameLayerState);
  const surfacePlan = evaluateTransitionPairSurfacePlan({
    contract_version: 'visual-frame-state-v1',
    frame_index: frameIndex,
    layers: canonicalLayers,
    authoritative: canonicalLayers.every((layer) => layer.authoritative),
  });
  const deferredReasons = surfacePlan.deferred.map((entry) => `${entry.transition_id}:${entry.reason}`);
  const pairByLowerIndex = new Map<number, PreviewTransitionPairSlot<T>>();

  for (const surface of surfacePlan.surfaces) {
    const ownerState = canonicalLayers.find((layer) => layer.clip_id === surface.owner_clip_id);
    const paint = ownerState?.transition_paint?.find((candidate) => candidate.transition_id === surface.transition_id);
    if (!paint) throw new Error(`preview transition pair ${JSON.stringify(surface.transition_id)} is missing owner paint`);
    const pixel = evaluateTransitionPairPixelComposition(surface, paint);
    const lower = layers[surface.lower_layer_index];
    const upper = layers[surface.upper_layer_index];
    if (!lower || !upper
      || lower.clip.id !== surface.lower_clip_id
      || upper.clip.id !== surface.upper_clip_id) {
      throw new Error(`preview transition pair ${JSON.stringify(surface.transition_id)} slot identity drift`);
    }
    const execution: PreviewTransitionPairExecution = pixel.blend_operator === TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK
      ? 'source-over-dom'
      : 'weighted-canvas-deferred';
    pairByLowerIndex.set(surface.lower_layer_index, {
      kind: 'pair',
      lower,
      upper,
      surface,
      paint,
      pixel,
      execution,
      layerPaintByClipId: resolvePairLayerPaint(surface, paint, execution),
    });
  }

  const slots: PreviewTransitionPairPlan<T>['slots'] = [];
  for (let index = 0; index < layers.length; index += 1) {
    const pair = pairByLowerIndex.get(index);
    if (pair) {
      slots.push(pair);
      index += 1;
      continue;
    }
    slots.push({ kind: 'single', layer: layers[index] });
  }

  const pairSlots = slots.filter((slot): slot is PreviewTransitionPairSlot<T> => slot.kind === 'pair');
  if (pairSlots.length === 0) {
    return {
      mode: 'canonical-none',
      slots,
      deferredReasons,
    };
  }
  const sourceOver = pairSlots.some((slot) => slot.execution === 'source-over-dom');
  const weighted = pairSlots.some((slot) => slot.execution === 'weighted-canvas-deferred');
  return {
    mode: sourceOver && weighted
      ? 'canonical-mixed'
      : sourceOver
        ? 'canonical-source-over'
        : 'canonical-weighted-deferred',
    slots,
    deferredReasons,
  };
}

function resolvePairLayerPaint(
  surface: CanonicalTransitionPairSurface,
  paint: CanonicalTransitionPaint,
  execution: PreviewTransitionPairExecution,
): ReadonlyMap<string, PreviewTransitionPairLayerPaint> {
  const result = new Map<string, PreviewTransitionPairLayerPaint>([
    [surface.lower_clip_id, { ...IDENTITY_LAYER_PAINT }],
    [surface.upper_clip_id, { ...IDENTITY_LAYER_PAINT }],
  ]);
  if (execution !== 'source-over-dom') return result;

  if (paint.composition === TRANSITION_PAINT_PAIR_SLIDE) {
    if (paint.translation_space !== TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION) {
      throw new Error(`preview transition pair ${JSON.stringify(surface.transition_id)} slide translation space is invalid`);
    }
    result.set(surface.outgoing_clip_id, {
      ...IDENTITY_LAYER_PAINT,
      offsetXFraction: requireFinite(paint.outgoing_offset_x, 'outgoing_offset_x', surface.transition_id),
      offsetYFraction: requireFinite(paint.outgoing_offset_y, 'outgoing_offset_y', surface.transition_id),
    });
    result.set(surface.incoming_clip_id, {
      ...IDENTITY_LAYER_PAINT,
      offsetXFraction: requireFinite(paint.incoming_offset_x, 'incoming_offset_x', surface.transition_id),
      offsetYFraction: requireFinite(paint.incoming_offset_y, 'incoming_offset_y', surface.transition_id),
    });
    return result;
  }

  if (paint.composition === TRANSITION_PAINT_PAIR_WIPE) {
    if (paint.clip_space !== TRANSITION_PAINT_CLIP_LAYER_FRACTION) {
      throw new Error(`preview transition pair ${JSON.stringify(surface.transition_id)} wipe clip space is invalid`);
    }
    const top = requireUnit(paint.incoming_clip_top, 'incoming_clip_top', surface.transition_id);
    const right = requireUnit(paint.incoming_clip_right, 'incoming_clip_right', surface.transition_id);
    const bottom = requireUnit(paint.incoming_clip_bottom, 'incoming_clip_bottom', surface.transition_id);
    const left = requireUnit(paint.incoming_clip_left, 'incoming_clip_left', surface.transition_id);
    result.set(surface.incoming_clip_id, {
      ...IDENTITY_LAYER_PAINT,
      clipPath: `inset(${top * 100}% ${right * 100}% ${bottom * 100}% ${left * 100}%)`,
    });
    return result;
  }

  throw new Error(`preview transition pair ${JSON.stringify(surface.transition_id)} source-over execution requires slide or wipe paint`);
}

function requireFinite(value: number | undefined, field: string, transitionId: string): number {
  if (value === undefined || !Number.isFinite(value)) {
    throw new Error(`preview transition pair ${JSON.stringify(transitionId)} ${field} must be finite`);
  }
  return value;
}

function requireUnit(value: number | undefined, field: string, transitionId: string): number {
  const resolved = requireFinite(value, field, transitionId);
  if (resolved < 0 || resolved > 1) {
    throw new Error(`preview transition pair ${JSON.stringify(transitionId)} ${field} must be within [0,1]`);
  }
  return resolved;
}
