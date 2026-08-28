import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  resolvePreviewWeightedPairRasterSourceCapability,
  type PreviewWeightedPairRasterSourceSupported,
} from './previewFrameWeightedPairRaster';

export const PREVIEW_PIXELATE_BACKDROP_PLAN_V1 = 'preview-pixelate-backdrop-plan-v1' as const;

export type PreviewPixelateBackdropRuntimeRequirement =
  | 'decoded-frame-ready'
  | 'opaque-region-proof';

export type PreviewPixelateBackdropDeferredReason =
  | 'multiple-pixelate-layers-deferred'
  | 'pixelate-state-not-authoritative'
  | 'pixelate-state-unresolved'
  | 'pixelate-text-deferred'
  | 'pixelate-cursor-deferred'
  | 'pixelate-effects-deferred'
  | 'pixelate-transition-deferred'
  | 'pixelate-transform-deferred'
  | 'pixelate-camera-transform-deferred'
  | 'backdrop-layer-count-deferred'
  | 'backdrop-opacity-deferred'
  | 'backdrop-transition-deferred'
  | 'backdrop-rotation-deferred'
  | 'backdrop-camera-transform-deferred';

export interface PreviewPixelateBackdropLayer {
  clip: { id: string };
  asset?: { mime_type: string };
  canonicalState?: CanonicalFrameLayerState;
}

export interface PreviewPixelateBackdropReady<T extends PreviewPixelateBackdropLayer> {
  contract_version: typeof PREVIEW_PIXELATE_BACKDROP_PLAN_V1;
  mode: 'canonical-ready';
  target: T;
  backdrop: T;
  rasterSource: PreviewWeightedPairRasterSourceSupported;
  runtimeRequirements: readonly PreviewPixelateBackdropRuntimeRequirement[];
  deferredReasons: [];
}

export interface PreviewPixelateBackdropNone<T extends PreviewPixelateBackdropLayer> {
  contract_version: typeof PREVIEW_PIXELATE_BACKDROP_PLAN_V1;
  mode: 'canonical-none';
  layers: readonly T[];
  deferredReasons: [];
}

export interface PreviewPixelateBackdropLegacy<T extends PreviewPixelateBackdropLayer> {
  contract_version: typeof PREVIEW_PIXELATE_BACKDROP_PLAN_V1;
  mode: 'legacy';
  layers: readonly T[];
  deferredReasons: [];
}

export interface PreviewPixelateBackdropDeferred<T extends PreviewPixelateBackdropLayer> {
  contract_version: typeof PREVIEW_PIXELATE_BACKDROP_PLAN_V1;
  mode: 'canonical-deferred';
  target?: T;
  backdrop?: T;
  deferredReasons: string[];
}

export type PreviewPixelateBackdropPlan<T extends PreviewPixelateBackdropLayer> =
  | PreviewPixelateBackdropReady<T>
  | PreviewPixelateBackdropNone<T>
  | PreviewPixelateBackdropLegacy<T>
  | PreviewPixelateBackdropDeferred<T>;

const RUNTIME_REQUIREMENTS: readonly PreviewPixelateBackdropRuntimeRequirement[] = [
  'decoded-frame-ready',
  'opaque-region-proof',
];

/**
 * Plan the first exact pixelate backdrop surface without broadening today's
 * raster eligibility. Layers are bottom-to-top, matching canonical preview
 * stack order.
 *
 * V1 deliberately admits only one pixelate region over exactly one clean media
 * layer. The structural plan is separate from runtime decoded-pixel evidence:
 * a consumer must still prove that the media frame is ready and that the
 * sampled region is opaque before replacing the CSS approximation.
 */
export function planPreviewPixelateBackdrop<T extends PreviewPixelateBackdropLayer>(
  frameIndex: number | null,
  layers: readonly T[],
): PreviewPixelateBackdropPlan<T> {
  if (frameIndex === null || layers.some((layer) => !layer.canonicalState)) {
    return {
      contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
      mode: 'legacy',
      layers,
      deferredReasons: [],
    };
  }

  const pixelateIndices = layers.flatMap((layer, index) => (
    layer.canonicalState?.shape?.kind === 'pixelate' ? [index] : []
  ));
  if (pixelateIndices.length === 0) {
    return {
      contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
      mode: 'canonical-none',
      layers,
      deferredReasons: [],
    };
  }
  if (pixelateIndices.length > 1) {
    return deferred<T>(undefined, undefined, ['multiple-pixelate-layers-deferred']);
  }

  const targetIndex = pixelateIndices[0];
  const target = layers[targetIndex];
  const targetState = target.canonicalState as CanonicalFrameLayerState;
  const targetReasons = targetDeferredReasons(targetState);

  const lowerLayers = layers.slice(0, targetIndex);
  if (lowerLayers.length !== 1) {
    return deferred(target, undefined, [...targetReasons, 'backdrop-layer-count-deferred']);
  }

  const backdrop = lowerLayers[0];
  const backdropState = backdrop.canonicalState as CanonicalFrameLayerState;
  const backdropReasons: string[] = [];
  const rasterSource = resolvePreviewWeightedPairRasterSourceCapability(backdrop);
  if (!rasterSource.supported) {
    backdropReasons.push(`backdrop:${rasterSource.reason}`);
  }
  if (backdropState.view_transform.opacity !== 1) {
    backdropReasons.push('backdrop-opacity-deferred');
  }
  if ((backdropState.transitions ?? []).length > 0 || (backdropState.transition_paint ?? []).length > 0) {
    backdropReasons.push('backdrop-transition-deferred');
  }
  if (backdropState.view_transform.rotation_z !== 0) {
    backdropReasons.push('backdrop-rotation-deferred');
  }
  if (!samePlanarPlacement(backdropState.transform, backdropState.view_transform)) {
    backdropReasons.push('backdrop-camera-transform-deferred');
  }

  const reasons = uniqueStrings([...targetReasons, ...backdropReasons]);
  if (reasons.length > 0 || !rasterSource.supported) {
    return deferred(target, backdrop, reasons);
  }

  return {
    contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
    mode: 'canonical-ready',
    target,
    backdrop,
    rasterSource,
    runtimeRequirements: RUNTIME_REQUIREMENTS,
    deferredReasons: [],
  };
}

function targetDeferredReasons(state: CanonicalFrameLayerState): string[] {
  const reasons: PreviewPixelateBackdropDeferredReason[] = [];
  if (!state.authoritative) reasons.push('pixelate-state-not-authoritative');
  if ((state.unresolved ?? []).length > 0) reasons.push('pixelate-state-unresolved');
  if (state.text) reasons.push('pixelate-text-deferred');
  if (state.cursor) reasons.push('pixelate-cursor-deferred');
  if ((state.effects ?? []).length > 0) reasons.push('pixelate-effects-deferred');
  if ((state.transitions ?? []).length > 0 || (state.transition_paint ?? []).length > 0) {
    reasons.push('pixelate-transition-deferred');
  }
  if (!simplePixelateRegionTransform(state.view_transform)) {
    reasons.push('pixelate-transform-deferred');
  }
  if (!samePlanarPlacement(state.transform, state.view_transform)) {
    reasons.push('pixelate-camera-transform-deferred');
  }
  return uniqueStrings(reasons);
}

function simplePixelateRegionTransform(transform: CanonicalFrameLayerState['view_transform']): boolean {
  return Number.isFinite(transform.x)
    && Number.isFinite(transform.y)
    && Number.isFinite(transform.scale_x)
    && Number.isFinite(transform.scale_y)
    && transform.scale_x > 0
    && transform.scale_y > 0
    && transform.scale_x === transform.scale_y
    && transform.z === 0
    && transform.rotation_x === 0
    && transform.rotation_y === 0
    && transform.rotation_z === 0
    && transform.opacity === 1
    && transform.anchor_x === 0
    && transform.anchor_y === 0
    && transform.perspective === undefined
    && transform.crop === undefined;
}

function samePlanarPlacement(
  authored: CanonicalFrameLayerState['transform'],
  view: CanonicalFrameLayerState['view_transform'],
): boolean {
  return authored.x === view.x
    && authored.y === view.y
    && authored.z === view.z
    && authored.scale_x === view.scale_x
    && authored.scale_y === view.scale_y
    && authored.rotation_x === view.rotation_x
    && authored.rotation_y === view.rotation_y
    && authored.rotation_z === view.rotation_z
    && authored.opacity === view.opacity
    && authored.anchor_x === view.anchor_x
    && authored.anchor_y === view.anchor_y;
}

function deferred<T extends PreviewPixelateBackdropLayer>(
  target: T | undefined,
  backdrop: T | undefined,
  deferredReasons: string[],
): PreviewPixelateBackdropDeferred<T> {
  return {
    contract_version: PREVIEW_PIXELATE_BACKDROP_PLAN_V1,
    mode: 'canonical-deferred',
    ...(target ? { target } : {}),
    ...(backdrop ? { backdrop } : {}),
    deferredReasons: uniqueStrings(deferredReasons),
  };
}

function uniqueStrings(values: readonly string[]): string[] {
  return [...new Set(values)];
}
