import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  TRANSITION_PAINT_PAIR_ZOOM,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import type {
  PreviewTransitionPairPlan,
  PreviewTransitionPairSlot,
} from './previewFrameTransitionPairs';

export interface PreviewCanvasMediaLayer {
  clip: { id: string };
  asset?: { mime_type: string };
  canonicalState?: CanonicalFrameLayerState;
}

export interface PreviewWeightedPairCanvasLayer extends PreviewCanvasMediaLayer {}

export interface PreviewCanvasMediaRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PreviewCanvasMediaLayerPlan {
  source: PreviewCanvasMediaRect;
  destination: PreviewCanvasMediaRect;
  clip: PreviewCanvasMediaRect;
  originX: number;
  originY: number;
  translateX: number;
  translateY: number;
  rotationRadians: number;
  scaleX: number;
  scaleY: number;
  opacity: number;
}

export type PreviewWeightedPairCanvasRect = PreviewCanvasMediaRect;
export type PreviewWeightedPairCanvasLayerPlan = PreviewCanvasMediaLayerPlan;

/**
 * Admit weighted Canvas execution only for a complete deterministic frame that
 * has one clean all-weighted interpretation. Canonical pair validity and media
 * raster-source capability have already been resolved by the pair planner;
 * runtime decoder/poster/readiness checks remain separate in the React consumer.
 */
export function shouldConsumePreviewFrameWeightedPairs<T extends PreviewWeightedPairCanvasLayer>(
  plan: PreviewTransitionPairPlan<T>,
): boolean {
  if (plan.mode !== 'canonical-weighted-deferred'
    || plan.deferredReasons.length > 0
    || plan.weightedRasterDeferredReasons.length > 0) {
    return false;
  }

  let pairCount = 0;
  for (const slot of plan.slots) {
    if (slot.kind !== 'pair') continue;
    pairCount += 1;
    if (slot.execution !== 'weighted-canvas-deferred' || !slot.weightedRasterSource?.supported) return false;
    const owner = slot.surface.owner_clip_id === slot.lower.clip.id ? slot.lower : slot.upper;
    const peer = slot.surface.peer_clip_id === slot.lower.clip.id ? slot.lower : slot.upper;
    const ownerPaints = owner.canonicalState?.transition_paint ?? [];
    const peerPaints = peer.canonicalState?.transition_paint ?? [];
    if (ownerPaints.length !== 1 || ownerPaints[0].transition_id !== slot.surface.transition_id) return false;
    if (peerPaints.length !== 0) return false;
  }
  return pairCount > 0;
}

/** Return every clip id that must have a real decoded source, never a proxy poster. */
export function weightedPairCanvasClipIds<T extends PreviewWeightedPairCanvasLayer>(
  plan: PreviewTransitionPairPlan<T>,
): string[] {
  const result: string[] = [];
  for (const slot of plan.slots) {
    if (slot.kind !== 'pair' || slot.execution !== 'weighted-canvas-deferred') continue;
    result.push(slot.lower.clip.id, slot.upper.clip.id);
  }
  return [...new Set(result)];
}

/**
 * Resolve one canonical media input into exact Canvas source/destination
 * geometry plus the established canonical 2D preview transform. Callers may
 * supply one additional positive layer-scale multiplier when a composition
 * contract (for example pair zoom) owns that scale outside FrameState.
 */
export function resolvePreviewCanvasMediaLayerPlan<T extends PreviewCanvasMediaLayer>(
  layer: T,
  intrinsicWidth: number,
  intrinsicHeight: number,
  scaleMultiplier = 1,
): PreviewCanvasMediaLayerPlan {
  const state = layer.canonicalState;
  const geometry = state?.media_geometry;
  if (!state || !geometry) throw new Error('canonical Canvas media layer requires canonical media geometry');
  if (!Number.isFinite(intrinsicWidth) || intrinsicWidth <= 0
    || !Number.isFinite(intrinsicHeight) || intrinsicHeight <= 0) {
    throw new Error('canonical Canvas media layer requires positive intrinsic source dimensions');
  }
  if (!Number.isFinite(scaleMultiplier) || scaleMultiplier <= 0) {
    throw new Error('canonical Canvas media layer scale multiplier must be finite and positive');
  }

  const source = geometry.source_bounds;
  const visible = geometry.visible_source_bounds;
  const viewport = geometry.viewport_bounds;
  const painted = geometry.painted_bounds;
  const clip = geometry.clip_bounds;
  const sourceRect = {
    x: ((visible.x - source.x) / source.width) * intrinsicWidth,
    y: ((visible.y - source.y) / source.height) * intrinsicHeight,
    width: (visible.width / source.width) * intrinsicWidth,
    height: (visible.height / source.height) * intrinsicHeight,
  };
  for (const value of [
    sourceRect.x,
    sourceRect.y,
    sourceRect.width,
    sourceRect.height,
    painted.x,
    painted.y,
    painted.width,
    painted.height,
    clip.x,
    clip.y,
    clip.width,
    clip.height,
  ]) {
    if (!Number.isFinite(value)) throw new Error('canonical Canvas media geometry must be finite');
  }
  if (sourceRect.width <= 0 || sourceRect.height <= 0 || painted.width <= 0 || painted.height <= 0) {
    throw new Error('canonical Canvas media geometry must retain positive source and destination area');
  }

  const view = state.view_transform;
  const transform = state.transform;
  const opacity = Math.max(0, Math.min(1, transform.opacity));
  return {
    source: sourceRect,
    destination: { ...painted },
    clip: { ...clip },
    originX: viewport.x + viewport.width / 2 + transform.anchor_x,
    originY: viewport.y + viewport.height / 2 + transform.anchor_y,
    translateX: view.x,
    translateY: view.y,
    rotationRadians: view.rotation_z * Math.PI / 180,
    scaleX: view.scale_x * scaleMultiplier,
    scaleY: view.scale_y * scaleMultiplier,
    opacity,
  };
}

/** Apply one resolved canonical media-layer plan to a 2D context. */
export function paintPreviewCanvasMediaLayer(
  context: CanvasRenderingContext2D,
  source: CanvasImageSource,
  plan: PreviewCanvasMediaLayerPlan,
): void {
  context.save();
  try {
    context.globalAlpha = plan.opacity;
    context.translate(plan.translateX, plan.translateY);
    context.translate(plan.originX, plan.originY);
    context.rotate(plan.rotationRadians);
    context.scale(plan.scaleX, plan.scaleY);
    context.translate(-plan.originX, -plan.originY);
    context.beginPath();
    context.rect(plan.clip.x, plan.clip.y, plan.clip.width, plan.clip.height);
    context.clip();
    context.drawImage(
      source,
      plan.source.x,
      plan.source.y,
      plan.source.width,
      plan.source.height,
      plan.destination.x,
      plan.destination.y,
      plan.destination.width,
      plan.destination.height,
    );
  } finally {
    context.restore();
  }
}

/**
 * Resolve one pair input. Transition weights remain excluded because #277's
 * linear-sRGB kernel applies them exactly once after isolated rasterization;
 * pair zoom contributes only its layer-scale multiplier here.
 */
export function resolvePreviewWeightedPairCanvasLayerPlan<T extends PreviewWeightedPairCanvasLayer>(
  slot: PreviewTransitionPairSlot<T>,
  layer: T,
  intrinsicWidth: number,
  intrinsicHeight: number,
): PreviewWeightedPairCanvasLayerPlan {
  return resolvePreviewCanvasMediaLayerPlan(
    layer,
    intrinsicWidth,
    intrinsicHeight,
    weightedPairScaleForClip(slot.paint, layer.clip.id),
  );
}

/** Backward-compatible weighted-pair painter over the shared canonical media painter. */
export function paintPreviewWeightedPairCanvasLayer(
  context: CanvasRenderingContext2D,
  source: CanvasImageSource,
  plan: PreviewWeightedPairCanvasLayerPlan,
): void {
  paintPreviewCanvasMediaLayer(context, source, plan);
}

function weightedPairScaleForClip(paint: CanonicalTransitionPaint, clipId: string): number {
  if (paint.composition !== TRANSITION_PAINT_PAIR_ZOOM) return 1;
  if (paint.scale_space !== TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} pair zoom requires layer-multiplier scale space`);
  }
  let value: number | undefined;
  if (clipId === paint.outgoing_clip_id) value = paint.outgoing_scale;
  else if (clipId === paint.incoming_clip_id) value = paint.incoming_scale;
  else throw new Error(`transition ${JSON.stringify(paint.transition_id)} pair zoom does not contain clip ${JSON.stringify(clipId)}`);
  if (value === undefined || !Number.isFinite(value) || value <= 0) {
    throw new Error(`transition ${JSON.stringify(paint.transition_id)} pair zoom scale must be finite and positive`);
  }
  return value;
}
