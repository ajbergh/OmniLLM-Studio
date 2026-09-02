import { afterEach, describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';
import {
  clearPreviewWeightedPlaybackRuntime,
  previewWeightedPlaybackPlanIdentity,
  previewWeightedPlaybackPlanKey,
  previewWeightedPlaybackRuntimeRevision,
  publishPreviewWeightedPlaybackRuntime,
  resetPreviewWeightedPlaybackRuntimeForTests,
  resolvePreviewWeightedPlaybackRuntime,
} from './previewWeightedPlaybackRuntime';

type Layer = {
  clip: { id: string };
  canonicalState?: CanonicalFrameLayerState;
};

function weightedPlan(transitionId = 'crossfade'): PreviewTransitionPairPlan<Layer> {
  const lower = { clip: { id: 'lower' } } as Layer;
  const upper = { clip: { id: 'upper' } } as Layer;
  return {
    mode: 'canonical-weighted-deferred',
    deferredReasons: [],
    weightedRasterDeferredReasons: [],
    slots: [{
      kind: 'pair',
      lower,
      upper,
      surface: {
        transition_id: transitionId,
        owner_clip_id: 'lower',
        peer_clip_id: 'upper',
        outgoing_clip_id: 'lower',
        incoming_clip_id: 'upper',
        lower_clip_id: 'lower',
        upper_clip_id: 'upper',
        lower_layer_index: 0,
        upper_layer_index: 1,
      } as never,
      paint: {} as never,
      pixel: {} as never,
      execution: 'weighted-canvas-deferred',
      layerPaintByClipId: new Map(),
      weightedRasterSource: { supported: true } as never,
    }],
  };
}

afterEach(() => resetPreviewWeightedPlaybackRuntimeForTests());

describe('weighted playback runtime registry', () => {
  it('keys execution by exact frame while retaining stable pair topology identity', () => {
    const plan = weightedPlan();
    const key = previewWeightedPlaybackPlanKey(42, plan);
    expect(key).toBe('42|crossfade:lower>upper');
    expect(previewWeightedPlaybackPlanIdentity(plan)).toBe('crossfade:lower>upper');
    expect(previewWeightedPlaybackPlanKey(43, plan)).not.toBe(key);
    expect(previewWeightedPlaybackPlanKey(42, weightedPlan('zoom'))).not.toBe(key);
  });

  it('keeps ready source/runtime capability warm across frames of the same pair topology', () => {
    const plan = weightedPlan();
    const planIdentity = previewWeightedPlaybackPlanIdentity(plan);
    const planKey = previewWeightedPlaybackPlanKey(42, plan);
    publishPreviewWeightedPlaybackRuntime({ frameIndex: 42, planKey, planIdentity, status: 'pending' });
    expect(resolvePreviewWeightedPlaybackRuntime(42, plan)).toEqual({
      ready: false,
      deferredReason: 'transition-weighted-runtime-not-ready',
    });

    publishPreviewWeightedPlaybackRuntime({ frameIndex: 42, planKey, planIdentity, status: 'ready' });
    expect(resolvePreviewWeightedPlaybackRuntime(42, plan)).toEqual({ ready: true });
    expect(resolvePreviewWeightedPlaybackRuntime(43, plan)).toEqual({ ready: true });
    expect(resolvePreviewWeightedPlaybackRuntime(42, weightedPlan('zoom'))).toEqual({
      ready: false,
      deferredReason: 'transition-weighted-runtime-not-ready',
    });
  });

  it('preserves deferred and failed renderer reasons', () => {
    const plan = weightedPlan();
    const planIdentity = previewWeightedPlaybackPlanIdentity(plan);
    const planKey = previewWeightedPlaybackPlanKey(42, plan);
    publishPreviewWeightedPlaybackRuntime({
      frameIndex: 42,
      planKey,
      planIdentity,
      status: 'deferred',
      reason: 'upper:decoder-budget-poster',
    });
    expect(resolvePreviewWeightedPlaybackRuntime(42, plan).deferredReason)
      .toBe('transition-weighted-runtime-deferred:upper:decoder-budget-poster');

    publishPreviewWeightedPlaybackRuntime({
      frameIndex: 42,
      planKey,
      planIdentity,
      status: 'failed',
      reason: 'weighted-canvas-readback-failed',
    });
    expect(resolvePreviewWeightedPlaybackRuntime(42, plan).deferredReason)
      .toBe('transition-weighted-runtime-failed:weighted-canvas-readback-failed');
  });

  it('notifies only when runtime state actually changes', () => {
    const plan = weightedPlan();
    const planIdentity = previewWeightedPlaybackPlanIdentity(plan);
    const planKey = previewWeightedPlaybackPlanKey(42, plan);
    const before = previewWeightedPlaybackRuntimeRevision();
    publishPreviewWeightedPlaybackRuntime({ frameIndex: 42, planKey, planIdentity, status: 'pending' });
    const afterPending = previewWeightedPlaybackRuntimeRevision();
    expect(afterPending).toBe(before + 1);
    publishPreviewWeightedPlaybackRuntime({ frameIndex: 42, planKey, planIdentity, status: 'pending' });
    expect(previewWeightedPlaybackRuntimeRevision()).toBe(afterPending);
    clearPreviewWeightedPlaybackRuntime();
    expect(previewWeightedPlaybackRuntimeRevision()).toBe(afterPending + 1);
  });
});
