import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_OWNER_ALPHA,
  TRANSITION_PAINT_OWNER_TRANSLATE,
  TRANSITION_PAINT_OWNER_WIPE,
  TRANSITION_PAINT_OWNER_ZOOM,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';

export type PreviewFrameOwnerTransitionMode = 'canonical-frame' | 'canonical-deferred' | 'legacy-none';

export interface ResolvedPreviewFrameOwnerTransitionPaint {
  mode: PreviewFrameOwnerTransitionMode;
  opacityMultiplier: number;
  offsetXFraction: number;
  offsetYFraction: number;
  scaleMultiplier: number;
  clipPath?: string;
  deferredComposition?: string;
}

type CanonicalTransitionLayerState = Pick<CanonicalFrameLayerState, 'clip_id' | 'transition_paint'>;

const IDENTITY: Omit<ResolvedPreviewFrameOwnerTransitionPaint, 'mode'> = {
  opacityMultiplier: 1,
  offsetXFraction: 0,
  offsetYFraction: 0,
  scaleMultiplier: 1,
};

/**
 * Consume only transition-paint-v1 instructions that operate on one isolated
 * owner layer. Pair transitions and dip-to-black require a frame-level
 * composition surface, so this resolver reports them as deferred rather than
 * approximating a two-input blend with independent CSS opacity.
 *
 * Canonical omission is authoritative: an evaluated layer with no active
 * transition_paint returns identity canonical paint. Live manipulation stays on
 * the established interactive path; admitted media-only normal-playback frames
 * may consume this same canonical paint contract.
 */
export function resolvePreviewFrameOwnerTransitionPaint(
  canonicalState: CanonicalTransitionLayerState | undefined,
  hasLiveOverride: boolean,
): ResolvedPreviewFrameOwnerTransitionPaint {
  if (!canonicalState || hasLiveOverride) return { ...IDENTITY, mode: 'legacy-none' };

  const paints = canonicalState.transition_paint ?? [];
  if (paints.length === 0) return { ...IDENTITY, mode: 'canonical-frame' };
  if (paints.length !== 1) {
    return { ...IDENTITY, mode: 'canonical-deferred', deferredComposition: 'multiple-active-transitions' };
  }

  const paint = paints[0];
  validateOwnerPaintIdentity(canonicalState.clip_id, paint);

  switch (paint.composition) {
    case TRANSITION_PAINT_OWNER_ALPHA:
      return {
        ...IDENTITY,
        mode: 'canonical-frame',
        opacityMultiplier: requireUnit(paint.owner_opacity, 'owner_opacity', paint),
      };

    case TRANSITION_PAINT_OWNER_TRANSLATE:
      if (paint.translation_space !== TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION) {
        throw invalidPaint(paint, `translation_space must be ${TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION}`);
      }
      return {
        ...IDENTITY,
        mode: 'canonical-frame',
        offsetXFraction: requireFinite(paint.owner_offset_x, 'owner_offset_x', paint),
        offsetYFraction: requireFinite(paint.owner_offset_y, 'owner_offset_y', paint),
      };

    case TRANSITION_PAINT_OWNER_WIPE: {
      if (paint.clip_space !== TRANSITION_PAINT_CLIP_LAYER_FRACTION) {
        throw invalidPaint(paint, `clip_space must be ${TRANSITION_PAINT_CLIP_LAYER_FRACTION}`);
      }
      const top = requireUnit(paint.owner_clip_top, 'owner_clip_top', paint);
      const right = requireUnit(paint.owner_clip_right, 'owner_clip_right', paint);
      const bottom = requireUnit(paint.owner_clip_bottom, 'owner_clip_bottom', paint);
      const left = requireUnit(paint.owner_clip_left, 'owner_clip_left', paint);
      return {
        ...IDENTITY,
        mode: 'canonical-frame',
        clipPath: `inset(${top * 100}% ${right * 100}% ${bottom * 100}% ${left * 100}%)`,
      };
    }

    case TRANSITION_PAINT_OWNER_ZOOM:
      if (paint.scale_space !== TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER) {
        throw invalidPaint(paint, `scale_space must be ${TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER}`);
      }
      return {
        ...IDENTITY,
        mode: 'canonical-frame',
        opacityMultiplier: requireUnit(paint.owner_opacity, 'owner_opacity', paint),
        scaleMultiplier: requireNonNegativeFinite(paint.owner_scale, 'owner_scale', paint),
      };

    default:
      return {
        ...IDENTITY,
        mode: 'canonical-deferred',
        deferredComposition: paint.composition,
      };
  }
}

function validateOwnerPaintIdentity(ownerClipId: string, paint: CanonicalTransitionPaint): void {
  if (paint.contract_version !== TRANSITION_PAINT_CONTRACT_V1) {
    throw invalidPaint(paint, `contract_version must be ${TRANSITION_PAINT_CONTRACT_V1}`);
  }
  if (!paint.transition_id.trim()) throw invalidPaint(paint, 'transition_id must not be empty');
  if (paint.owner_clip_id !== ownerClipId) {
    throw invalidPaint(paint, `owner_clip_id ${JSON.stringify(paint.owner_clip_id)} does not match layer ${JSON.stringify(ownerClipId)}`);
  }
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

function requireNonNegativeFinite(value: number | undefined, field: string, paint: CanonicalTransitionPaint): number {
  const resolved = requireFinite(value, field, paint);
  if (resolved < 0) throw invalidPaint(paint, `${field} must be non-negative`);
  return resolved;
}

function invalidPaint(paint: CanonicalTransitionPaint, detail: string): Error {
  return new Error(`canonical preview transition ${JSON.stringify(paint.transition_id)}: ${detail}`);
}
