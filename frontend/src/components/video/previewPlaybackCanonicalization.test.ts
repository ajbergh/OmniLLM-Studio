import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  clearPreviewTextPlaybackRuntime,
  previewTextPlaybackPlanKey,
  publishPreviewTextPlaybackRuntime,
} from './previewTextPlaybackRuntime';
import {
  clearPreviewWeightedPlaybackRuntime,
  previewWeightedPlaybackPlanKey,
  publishPreviewWeightedPlaybackRuntime,
} from './previewWeightedPlaybackRuntime';
import { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';

function mediaLayer(id = 'media', mime = 'video/mp4') {
  return {
    clip: { id },
    asset: { mime_type: mime },
    canonicalState: { authoritative: true } as Pick<CanonicalFrameLayerState, 'authoritative'>,
  };
}

function textLayer(id = 'title') {
  return {
    clip: { id, text: { text: 'hello' } },
    fontAsset: { id: 'font-asset', kind: 'font' },
    canonicalState: {
      authoritative: true,
      text: {
        contract_version: 'text-state-v1',
        text: 'hello',
        font_family: 'DejaVu Sans',
        font_family_source: 'authored',
        font_face_source: 'family-name-only',
        font_size: 36,
        font_weight: '700',
        color: '#ffffff',
        background: '#111827cc',
        stroke_width: 0,
        text_align: 'center',
        vertical_align: 'middle',
        line_height_mode: 'multiplier',
        letter_spacing: 0.5,
        border_radius: 8,
        padding: { top: 8, right: 18, bottom: 8, left: 18 },
        line_height: 1.2,
        font_resource_id: 'playback-font-v1',
      },
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text'>,
  };
}

function plan(mode: 'legacy' | 'canonical-none' | 'canonical-source-over' | 'canonical-mixed' | 'canonical-weighted-deferred' = 'canonical-none') {
  return {
    mode,
    slots: [],
    deferredReasons: [],
    weightedRasterDeferredReasons: [],
  };
}

beforeEach(() => {
  clearPreviewTextPlaybackRuntime();
  clearPreviewWeightedPlaybackRuntime();
});

afterEach(() => {
  clearPreviewTextPlaybackRuntime();
  clearPreviewWeightedPlaybackRuntime();
});

describe('normal playback canonicalization', () => {
  it('stays in legacy time when no playback frame exists', () => {
    expect(resolvePreviewPlaybackCanonicalization(null, undefined, [], null)).toEqual({
      mode: 'legacy-time',
      canonicalFrame: null,
    });
  });

  it('fails closed when frame state is unavailable or non-authoritative', () => {
    expect(resolvePreviewPlaybackCanonicalization(8, undefined, [mediaLayer()], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-frame-state-unavailable',
    });
    expect(resolvePreviewPlaybackCanonicalization(8, { authoritative: false }, [mediaLayer()], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-frame-state-nonauthoritative',
    });
  });

  it('fails closed when a canonical visual layer is missing or non-authoritative', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      [{ ...mediaLayer(), canonicalState: undefined }],
      plan(),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-layer-state-unavailable:media',
    });
    expect(resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      [{ ...mediaLayer(), canonicalState: { authoritative: false } }],
      plan(),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-layer-state-nonauthoritative:media',
    });
  });

  it.each(['canonical-none', 'canonical-source-over'] as const)('admits supported media for %s composition', (mode) => {
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], plan(mode))).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 9,
    });
  });

  it('admits ready resource-backed text without changing authored text semantics', () => {
    const layers = [textLayer()];
    const textPlanIdentity = 'text-plan-ready';
    publishPreviewTextPlaybackRuntime({
      frameIndex: 12,
      planKey: previewTextPlaybackPlanKey(12, layers),
      planIdentity: textPlanIdentity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(12, { authoritative: true }, layers, plan())).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 12,
    });
  });

  it('fails closed until resource-backed text runtime is ready', () => {
    const layers = [textLayer()];
    expect(resolvePreviewPlaybackCanonicalization(12, { authoritative: true }, layers, plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-deferred:title:font-face-not-ready',
    });
  });

  it('keeps family-name-only text on the time-domain fallback', () => {
    const layer = textLayer();
    layer.canonicalState.text = { ...layer.canonicalState.text, font_resource_id: undefined };
    expect(resolvePreviewPlaybackCanonicalization(12, { authoritative: true }, [layer], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-deferred:title:resource-font-required',
    });
  });

  it('keeps text font-load failures on the time-domain fallback', () => {
    const layers = [textLayer()];
    publishPreviewTextPlaybackRuntime({
      frameIndex: 12,
      planKey: previewTextPlaybackPlanKey(12, layers),
      planIdentity: 'text-plan-failed',
      status: 'failed',
      reason: 'title:font-face-load-failed',
    });
    expect(resolvePreviewPlaybackCanonicalization(12, { authoritative: true }, layers, plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-failed:title:font-face-load-failed',
    });
  });

  it('admits supported text and weighted surfaces only when both are ready', () => {
    const layers = [textLayer(), mediaLayer('lower'), mediaLayer('upper')];
    const transitionPlan = {
      mode: 'canonical-weighted-deferred' as const,
      slots: [
        {
          kind: 'pair' as const,
          pair: {
            id: 'pair',
            kind: 'crossfade' as const,
            outClipId: 'lower',
            inClipId: 'upper',
          },
        },
      ],
      deferredReasons: [],
      weightedRasterDeferredReasons: [],
    };
    const textPlanIdentity = 'text-plan-mixed';
    const weightedPlanIdentity = 'weighted-plan-mixed';

    publishPreviewWeightedPlaybackRuntime({
      frameIndex: 14,
      planKey: previewWeightedPlaybackPlanKey(14, transitionPlan),
      planIdentity: weightedPlanIdentity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(14, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-deferred:title:font-face-not-ready',
    });

    publishPreviewTextPlaybackRuntime({
      frameIndex: 14,
      planKey: previewTextPlaybackPlanKey(14, layers),
      planIdentity: textPlanIdentity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(14, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 14,
    });

    publishPreviewTextPlaybackRuntime({
      frameIndex: 14,
      planKey: previewTextPlaybackPlanKey(14, layers),
      planIdentity: textPlanIdentity,
      status: 'failed',
      reason: 'title:font-face-load-failed',
    });
    expect(resolvePreviewPlaybackCanonicalization(14, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-failed:title:font-face-load-failed',
    });

    publishPreviewTextPlaybackRuntime({
      frameIndex: 14,
      planKey: previewTextPlaybackPlanKey(14, layers),
      planIdentity: textPlanIdentity,
      status: 'ready',
    });
    publishPreviewWeightedPlaybackRuntime({
      frameIndex: 14,
      planKey: previewWeightedPlaybackPlanKey(14, transitionPlan),
      planIdentity: weightedPlanIdentity,
      status: 'deferred',
      reason: 'lower:decoder-budget-poster',
    });
    expect(resolvePreviewPlaybackCanonicalization(14, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-weighted-runtime-deferred:lower:decoder-budget-poster',
    });
  });

  it.each([
    ['cursor', { ...mediaLayer(), clip: { id: 'cursor', cursor: {} } }],
    ['missing-asset', { ...mediaLayer(), clip: { id: 'missing-asset' }, asset: undefined }],
    ['audio-asset', mediaLayer('audio-asset', 'audio/wav')],
  ])('keeps unsupported %s painters on the time-domain fallback', (_label, layer) => {
    const result = resolvePreviewPlaybackCanonicalization(8, { authoritative: true }, [layer], plan());
    expect(result.mode).toBe('legacy-time-fallback');
    expect(result.canonicalFrame).toBeNull();
    expect(result.deferredReason).toBe(`unsupported-playback-painter:${layer.clip.id}`);
  });

  it('keeps mixed supported text plus unsupported painter frames on one deterministic fallback', () => {
    const result = resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      [textLayer(), { ...mediaLayer(), clip: { id: 'cursor', cursor: {} } }],
      plan(),
    );
    expect(result).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'unsupported-playback-painter:cursor',
    });
  });

  it.each([
    ['legacy', 'transition-plan-legacy'],
    ['canonical-mixed', 'transition-plan-mixed'],
  ] as const)('fails closed for %s transition composition', (mode, reason) => {
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], plan(mode))).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: reason,
    });
  });

  it('fails closed for transition deferrals', () => {
    const transitionPlan = { ...plan('canonical-source-over'), deferredReasons: ['pair:peer-missing'] };
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-deferred:pair:peer-missing',
    });
  });

  it('fails closed for weighted raster deferrals before runtime readiness', () => {
    const transitionPlan = {
      ...plan('canonical-weighted-deferred'),
      weightedRasterDeferredReasons: ['pair:unsupported-source'],
    };
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-weighted-raster-deferred:pair:unsupported-source',
    });
  });

  it('fails closed until the weighted playback runtime is ready', () => {
    const transitionPlan = {
      mode: 'canonical-weighted-deferred' as const,
      slots: [
        {
          kind: 'pair' as const,
          pair: {
            id: 'pair',
            kind: 'crossfade' as const,
            outClipId: 'media',
            inClipId: 'upper',
          },
        },
      ],
      deferredReasons: [],
      weightedRasterDeferredReasons: [],
    };
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-weighted-runtime-not-ready',
    });
  });
});
