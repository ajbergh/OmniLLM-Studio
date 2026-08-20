import { samplePropertyKeyframes } from './renderContractProperties';
import type { RenderContractMetadata, TimelineV2Clip, TimelineV2Effect, TimelineV2Scene } from './renderContractTypes';

export const EFFECT_STATE_CONTRACT_V1 = 'effect-state-v1' as const;
export type CanonicalEffectScope = 'clip' | 'scene';

interface EffectParamSpec { defaultValue: number; min: number; max: number; }

const canonicalEffectParams: Record<string, Record<string, EffectParamSpec>> = {
  brightness: { amount: { defaultValue: 1.1, min: 0, max: 2 } },
  contrast: { amount: { defaultValue: 1.2, min: 0, max: 3 } },
  saturation: { amount: { defaultValue: 1.3, min: 0, max: 3 } },
  blur: { amount: { defaultValue: 6, min: 0, max: 30 } },
  grayscale: {},
  sharpen: { amount: { defaultValue: 1, min: 0, max: 3 } },
  vignette: { amount: { defaultValue: 0.4, min: 0, max: 1 } },
  shadow: {},
  background_blur: { amount: { defaultValue: 10, min: 0, max: 30 } },
  chroma_key: {
    similarity: { defaultValue: 0.3, min: 0.01, max: 1 },
    blend: { defaultValue: 0.05, min: 0, max: 0.5 },
  },
  film_grain: { amount: { defaultValue: 8, min: 0, max: 40 } },
  bloom: { amount: { defaultValue: 0.25, min: 0, max: 1 } },
  color_grade: { amount: { defaultValue: 1.08, min: 0.5, max: 2 } },
  edge_fade: { amount: { defaultValue: 0.35, min: 0, max: 1 } },
  rgb_split: { amount: { defaultValue: 3, min: 0, max: 20 } },
  ghost_trail: { amount: { defaultValue: 3, min: 2, max: 5 } },
  motion_blur: { amount: { defaultValue: 0.5, min: 0, max: 1 } },
  depth_of_field: { amount: { defaultValue: 2, min: 0, max: 12 } },
  rack_focus: { amount: { defaultValue: 2, min: 0, max: 12 } },
};

export interface CanonicalEvaluatedEffectState {
  contract_version: typeof EFFECT_STATE_CONTRACT_V1;
  id: string;
  type: string;
  scope: CanonicalEffectScope;
  order: number;
  params: RenderContractMetadata;
}

/** Enabled clip effects in authored order, with exact-time amount automation. */
export function evaluateClipEffectStackAtTime(clip: TimelineV2Clip, clipTimeMs: number): CanonicalEvaluatedEffectState[] {
  return evaluateEffectStack(clip.effects, clip.keyframes, clipTimeMs, 'clip');
}

/** Enabled scene effects in authored order. Timeline v2 has no scene-effect keyframes. */
export function evaluateSceneEffectStack(scene: TimelineV2Scene | undefined): CanonicalEvaluatedEffectState[] {
  return evaluateEffectStack(scene?.effects ?? [], [], undefined, 'scene');
}

function evaluateEffectStack(
  effects: readonly TimelineV2Effect[],
  keyframes: TimelineV2Clip['keyframes'],
  timeMs: number | undefined,
  scope: CanonicalEffectScope,
): CanonicalEvaluatedEffectState[] {
  const out: CanonicalEvaluatedEffectState[] = [];
  effects.forEach((effect, order) => {
    if (!effect.enabled) return;
    const id = effect.id.trim();
    const type = effect.type.trim().toLowerCase();
    const specs = canonicalEffectParams[type];
    if (!specs) throw new Error(`unsupported canonical effect type ${JSON.stringify(type)}`);
    if (!id) throw new Error(`canonical ${scope} effect at order ${order} has empty id`);
    const params = canonicalizeEffectParams(type, effect.params, specs);
    if (timeMs !== undefined) {
      const amountSpec = specs.amount;
      const idProperty = `effect.${id.toLowerCase()}.amount`;
      const typeProperty = `effect.${type}.amount`;
      const idAmount = samplePropertyKeyframes(keyframes, idProperty, timeMs);
      const typeAmount = idAmount === null ? samplePropertyKeyframes(keyframes, typeProperty, timeMs) : null;
      const amount = idAmount ?? typeAmount;
      if (amount !== null) {
        if (!amountSpec) throw new Error(`effect type ${JSON.stringify(type)} does not define canonical amount automation`);
        if (!Number.isFinite(amount)) throw new Error('animated amount must be finite');
        params.amount = clamp(amount, amountSpec.min, amountSpec.max);
      }
    }
    out.push({
      contract_version: EFFECT_STATE_CONTRACT_V1,
      id,
      type,
      scope,
      order,
      params,
    });
  });
  return out;
}

function canonicalizeEffectParams(
  type: string,
  authored: RenderContractMetadata,
  specs: Record<string, EffectParamSpec>,
): RenderContractMetadata {
  const params: RenderContractMetadata = Object.fromEntries(
    Object.entries(specs).map(([key, spec]) => [key, spec.defaultValue]),
  );
  if (type === 'chroma_key') params.color = '#00FF00';
  for (const [rawKey, raw] of Object.entries(authored ?? {})) {
    const key = rawKey.trim().toLowerCase();
    if (key === 'color' && type === 'chroma_key') {
      if (typeof raw !== 'string' || !raw.trim()) throw new Error(`parameter ${JSON.stringify(rawKey)} must be a non-empty string`);
      params.color = raw.trim();
      continue;
    }
    const spec = specs[key];
    if (!spec) throw new Error(`unsupported parameter ${JSON.stringify(rawKey)} for effect type ${JSON.stringify(type)}`);
    const value = typeof raw === 'number' ? raw : Number.NaN;
    if (!Number.isFinite(value)) throw new Error(`parameter ${JSON.stringify(rawKey)} must be a finite number`);
    params[key] = clamp(value, spec.min, spec.max);
  }
  return params;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}
