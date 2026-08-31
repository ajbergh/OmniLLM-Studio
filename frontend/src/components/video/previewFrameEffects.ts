import type { VideoTimelineEffect } from '../../types/video';
import {
  EFFECT_STATE_CONTRACT_V1,
  type CanonicalEffectScope,
  type CanonicalEvaluatedEffectState,
} from '../../video/renderContractEffects';
import type {
  CanonicalFrameLayerState,
  CanonicalVisualFrameState,
} from '../../video/renderContractFrameState';
import { composePreviewFilter, effectDefinition } from './effects/effectRegistry';

export type PreviewFrameEffectMode = 'canonical-frame' | 'legacy-authored';

export interface ResolvedPreviewFrameEffectPaint {
  filter?: string;
  mode: PreviewFrameEffectMode;
}

type CanonicalEffectState = Pick<CanonicalFrameLayerState, 'effects'>;
type CanonicalSceneEffectState = Pick<CanonicalVisualFrameState, 'scene_effects'>;

/**
 * Resolve the CSS-compatible portion of one deterministic canonical clip effect
 * stack. Canonical FrameState owns enabled-state, authored order, defaults,
 * parameter clamping, and exact-frame amount automation. Effects that the
 * existing CSS preview cannot paint remain intentionally absent from `filter`;
 * this consumer does not invent new effect-rendering semantics.
 *
 * The presence of canonical layer state is authoritative even when `effects`
 * is omitted, because omission means the canonical enabled stack is empty.
 * Legacy authored effects are consulted only when canonical FrameState itself
 * is unavailable through the explicit fail-closed projection fallback.
 */
export function resolvePreviewFrameEffectPaint(
  canonicalState: CanonicalEffectState | undefined,
  legacyEffects: VideoTimelineEffect[] | undefined,
): ResolvedPreviewFrameEffectPaint {
  if (!canonicalState) {
    return {
      filter: composePreviewFilter(legacyEffects),
      mode: 'legacy-authored',
    };
  }

  return {
    filter: composeCanonicalPreviewFilter(canonicalState.effects),
    mode: 'canonical-frame',
  };
}

/**
 * Resolve the existing stage-level CSS scene-effect painter from already
 * evaluated visual-frame-state-v1. Canonical omission is authoritative zero
 * enabled scene effects; authored scene effects remain the explicit fallback path
 * only when the top-level canonical frame itself is unavailable.
 */
export function resolvePreviewFrameSceneEffectPaint(
  canonicalState: CanonicalSceneEffectState | undefined,
  legacyEffects: VideoTimelineEffect[] | undefined,
): ResolvedPreviewFrameEffectPaint {
  if (!canonicalState) {
    return {
      filter: composePreviewFilter(legacyEffects),
      mode: 'legacy-authored',
    };
  }

  return {
    filter: composeCanonicalPreviewSceneFilter(canonicalState.scene_effects),
    mode: 'canonical-frame',
  };
}

/** Compose the existing CSS preview filter from evaluated clip effect-state-v1. */
export function composeCanonicalPreviewFilter(
  effects: readonly CanonicalEvaluatedEffectState[] | undefined,
): string | undefined {
  return composeCanonicalPreviewFilterForScope(effects, 'clip');
}

/** Compose the existing whole-program CSS filter from evaluated scene effect-state-v1. */
export function composeCanonicalPreviewSceneFilter(
  effects: readonly CanonicalEvaluatedEffectState[] | undefined,
): string | undefined {
  return composeCanonicalPreviewFilterForScope(effects, 'scene');
}

function composeCanonicalPreviewFilterForScope(
  effects: readonly CanonicalEvaluatedEffectState[] | undefined,
  scope: CanonicalEffectScope,
): string | undefined {
  if (!effects || effects.length === 0) return undefined;

  const ordered = [...effects].sort((left, right) => left.order - right.order);
  const seenOrders = new Set<number>();
  const parts: string[] = [];

  for (const effect of ordered) {
    validateCanonicalEffect(effect, scope, seenOrders);
    const definition = effectDefinition(effect.type);
    if (!definition) {
      throw new Error(`canonical preview effect type ${JSON.stringify(effect.type)} is not registered`);
    }
    const filter = definition.previewFilter(effect.params);
    if (filter) parts.push(filter);
  }

  return parts.length > 0 ? parts.join(' ') : undefined;
}

function validateCanonicalEffect(
  effect: CanonicalEvaluatedEffectState,
  scope: CanonicalEffectScope,
  seenOrders: Set<number>,
): void {
  if (effect.contract_version !== EFFECT_STATE_CONTRACT_V1) {
    throw new Error(`canonical preview effects require ${EFFECT_STATE_CONTRACT_V1}`);
  }
  if (effect.scope !== scope) {
    throw new Error(`canonical preview ${scope} effect ${JSON.stringify(effect.id)} has invalid scope ${JSON.stringify(effect.scope)}`);
  }
  if (!effect.id.trim() || !effect.type.trim()) {
    throw new Error(`canonical preview ${scope} effects require non-empty id and type`);
  }
  if (!Number.isInteger(effect.order) || effect.order < 0 || seenOrders.has(effect.order)) {
    throw new Error(`canonical preview ${scope} effect ${JSON.stringify(effect.id)} has invalid or duplicate order ${effect.order}`);
  }
  seenOrders.add(effect.order);
}