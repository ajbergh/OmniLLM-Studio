import { describe, expect, it } from 'vitest';
import { evaluateClipEffectStackAtTime, evaluateSceneEffectStack } from '../src/video/renderContractEffects';
import { evaluateVisualFrameState } from '../src/video/renderContractFrameState';
import type { TimelineV2Clip, TimelineV2Document, TimelineV2Scene } from '../src/video/renderContractTypes';

function clipWithEffects(): TimelineV2Clip {
  return {
    id: 'clip', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
    effects: [
      { id: 'bright', type: 'brightness', enabled: true, params: {} },
      { id: 'disabled', type: 'blur', enabled: false, params: { amount: 20 } },
      { id: 'contrast', type: 'contrast', enabled: true, params: { amount: 2.5 } },
    ],
    keyframes: [
      { id: 'type0', property: 'effect.brightness.amount', time_ms: 0, value: 0.5 },
      { id: 'type1', property: 'effect.brightness.amount', time_ms: 1000, value: 0.7 },
      { id: 'id0', property: 'effect.bright.amount', time_ms: 0, value: 1.1 },
      { id: 'id1', property: 'effect.bright.amount', time_ms: 1000, value: 1.9 },
      { id: 'contrast0', property: 'effect.contrast.amount', time_ms: 0, value: 1 },
      { id: 'contrast1', property: 'effect.contrast.amount', time_ms: 1000, value: 3 },
    ],
  };
}

describe('canonical effect stack', () => {
  it('preserves authored order while omitting disabled effects and prefers id automation', () => {
    expect(evaluateClipEffectStackAtTime(clipWithEffects(), 500)).toEqual([
      {
        contract_version: 'effect-state-v1', id: 'bright', type: 'brightness', scope: 'clip', order: 0,
        params: { amount: 1.5 },
      },
      {
        contract_version: 'effect-state-v1', id: 'contrast', type: 'contrast', scope: 'clip', order: 2,
        params: { amount: 2 },
      },
    ]);
  });

  it('applies scene defaults and omits disabled scene effects', () => {
    const scene: TimelineV2Scene = {
      id: 'scene', name: 'Scene', start_ms: 0, duration_ms: 1000,
      effects: [
        { id: 'vignette', type: 'vignette', enabled: true, params: {} },
        { id: 'off', type: 'grayscale', enabled: false, params: {} },
      ],
    };
    expect(evaluateSceneEffectStack(scene)).toEqual([
      {
        contract_version: 'effect-state-v1', id: 'vignette', type: 'vignette', scope: 'scene', order: 0,
        params: { amount: 0.4 },
      },
    ]);
  });

  it('fails closed on unknown parameters and undefined amount automation', () => {
    const unknown = clipWithEffects();
    unknown.effects = [{ id: 'blur', type: 'blur', enabled: true, params: { mystery: 1 } }];
    unknown.keyframes = [];
    expect(() => evaluateClipEffectStackAtTime(unknown, 0)).toThrow(/unsupported parameter/);

    const undefinedAmount = clipWithEffects();
    undefinedAmount.effects = [{ id: 'gray', type: 'grayscale', enabled: true, params: {} }];
    undefinedAmount.keyframes = [{ id: 'amount', property: 'effect.gray.amount', time_ms: 0, value: 1 }];
    expect(() => evaluateClipEffectStackAtTime(undefinedAmount, 0)).toThrow(/does not define canonical amount automation/);
  });

  it('projects clip and scene effects into authoritative FrameState', () => {
    const document: TimelineV2Document = {
      version: 2,
      canvas: { width: 640, height: 360, fps: 30, background: '#000000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'track', type: 'layer', name: 'Layer', locked: false, muted: false, visible: true,
        clips: [{
          id: 'clip', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
          effects: [{ id: 'bright', type: 'brightness', enabled: true, params: {} }], keyframes: [],
        }],
      }],
      scenes: [{
        id: 'scene', name: 'Scene', start_ms: 0, duration_ms: 1000,
        effects: [{ id: 'vignette', type: 'vignette', enabled: true, params: {} }],
      }],
    };
    const state = evaluateVisualFrameState(document, 0);
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.scene_effects?.map((effect) => effect.id)).toEqual(['vignette']);
    expect(state.layers[0].effects?.map((effect) => effect.id)).toEqual(['bright']);
  });
});
