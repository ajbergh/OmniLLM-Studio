import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../../video/renderContractMediaGeometry';

export type PreviewWeightedPairRasterSourceKind = 'image' | 'video';

export type PreviewWeightedPairRasterDeferredReason =
  | 'canonical-state-missing'
  | 'canonical-state-not-authoritative'
  | 'canonical-state-unresolved'
  | 'text-raster-deferred'
  | 'shape-raster-deferred'
  | 'cursor-raster-deferred'
  | 'media-asset-missing'
  | 'media-mime-unsupported'
  | 'media-geometry-missing'
  | 'clip-effects-raster-deferred'
  | 'three-dimensional-transform-raster-deferred';

export interface PreviewWeightedPairRasterLayer {
  clip: { id: string };
  asset?: { mime_type: string };
  canonicalState?: CanonicalFrameLayerState;
}

export interface PreviewWeightedPairRasterSourceSupported {
  supported: true;
  clipId: string;
  kind: PreviewWeightedPairRasterSourceKind;
}

export interface PreviewWeightedPairRasterSourceDeferred {
  supported: false;
  clipId: string;
  reason: PreviewWeightedPairRasterDeferredReason;
}

export type PreviewWeightedPairRasterSourceCapability =
  | PreviewWeightedPairRasterSourceSupported
  | PreviewWeightedPairRasterSourceDeferred;

export interface PreviewWeightedPairRasterPairSupported {
  supported: true;
  lower: PreviewWeightedPairRasterSourceSupported;
  upper: PreviewWeightedPairRasterSourceSupported;
}

export interface PreviewWeightedPairRasterPairDeferred {
  supported: false;
  lower: PreviewWeightedPairRasterSourceCapability;
  upper: PreviewWeightedPairRasterSourceCapability;
  reasons: string[];
}

export type PreviewWeightedPairRasterPairCapability =
  | PreviewWeightedPairRasterPairSupported
  | PreviewWeightedPairRasterPairDeferred;

/**
 * Classify whether one canonical preview layer can provide an exact media
 * raster source to a future weighted Canvas pair compositor.
 *
 * This is deliberately stricter than "visible visual clip". Text, shapes,
 * cursor overlays, clip effects, unresolved canonical state, and 3D transforms
 * remain deferred until their Canvas painters have explicit canonical parity.
 * A supported result means only that the layer has an eligible canonical media
 * source; runtime decoded-frame availability is a separate consumer gate.
 */
export function resolvePreviewWeightedPairRasterSourceCapability(
  layer: PreviewWeightedPairRasterLayer,
): PreviewWeightedPairRasterSourceCapability {
  const clipId = layer.clip.id;
  const state = layer.canonicalState;
  if (!state) return deferred(clipId, 'canonical-state-missing');
  if (!state.authoritative) return deferred(clipId, 'canonical-state-not-authoritative');
  if ((state.unresolved ?? []).length > 0) return deferred(clipId, 'canonical-state-unresolved');
  if (state.text) return deferred(clipId, 'text-raster-deferred');
  if (state.shape) return deferred(clipId, 'shape-raster-deferred');
  if (state.cursor) return deferred(clipId, 'cursor-raster-deferred');
  if ((state.effects ?? []).length > 0) return deferred(clipId, 'clip-effects-raster-deferred');
  if (hasThreeDimensionalTransform(state)) {
    return deferred(clipId, 'three-dimensional-transform-raster-deferred');
  }

  const mime = layer.asset?.mime_type?.trim().toLowerCase() ?? '';
  if (!mime) return deferred(clipId, 'media-asset-missing');
  const kind: PreviewWeightedPairRasterSourceKind | null = mime.startsWith('image/')
    ? 'image'
    : mime.startsWith('video/')
      ? 'video'
      : null;
  if (!kind) return deferred(clipId, 'media-mime-unsupported');

  if (state.media_geometry?.contract_version !== MEDIA_GEOMETRY_CONTRACT_V1) {
    return deferred(clipId, 'media-geometry-missing');
  }

  return { supported: true, clipId, kind };
}

/** Resolve both pair inputs without hiding which input blocks rasterization. */
export function resolvePreviewWeightedPairRasterPairCapability(
  lower: PreviewWeightedPairRasterLayer,
  upper: PreviewWeightedPairRasterLayer,
): PreviewWeightedPairRasterPairCapability {
  const lowerCapability = resolvePreviewWeightedPairRasterSourceCapability(lower);
  const upperCapability = resolvePreviewWeightedPairRasterSourceCapability(upper);
  if (lowerCapability.supported && upperCapability.supported) {
    return { supported: true, lower: lowerCapability, upper: upperCapability };
  }
  const reasons: string[] = [];
  if (!lowerCapability.supported) reasons.push(`${lowerCapability.clipId}:${lowerCapability.reason}`);
  if (!upperCapability.supported) reasons.push(`${upperCapability.clipId}:${upperCapability.reason}`);
  return {
    supported: false,
    lower: lowerCapability,
    upper: upperCapability,
    reasons,
  };
}

function hasThreeDimensionalTransform(state: CanonicalFrameLayerState): boolean {
  const transform = state.view_transform;
  return transform.z !== 0
    || transform.rotation_x !== 0
    || transform.rotation_y !== 0
    || transform.perspective !== undefined;
}

function deferred(
  clipId: string,
  reason: PreviewWeightedPairRasterDeferredReason,
): PreviewWeightedPairRasterSourceDeferred {
  return { supported: false, clipId, reason };
}
