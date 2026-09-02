import { afterEach, describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';
import { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';
import {
  previewTextPlaybackPlanIdentity,
  previewTextPlaybackPlanKey,
  publishPreviewTextPlaybackRuntime,
  resetPreviewTextPlaybackRuntimeForTests,
} from './previewTextPlaybackRuntime';
import {
  previewWeightedPlaybackPlanIdentity,
  previewWeightedPlaybackPlanKey,
  publishPreviewWeightedPlaybackRuntime,
  resetPreviewWeightedPlaybackRuntimeForTests,
} from './previewWeightedPlaybackRuntime';

type Layer = Parameters<typeof resolvePreviewPlaybackCanonicalization>[2][number];
type Plan = NonNullable<Parameters<typeof resolvePreviewPlaybackCanonicalization>[3]>;

function mediaLayer(id = 'media', mimeType = 'video/mp4'): Layer {
  return {
    clip: { id },
    asset: { mime_type: mimeType },
    canonicalState: { authoritative: true } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text'>,
  };
}

function textLayer(
  id = 'title',
  resourceId: string | undefined = 'playback-font-v1',
  fontAssetId = 'font-asset',
): Layer {
  return {
    clip: { id, text: {} },
    ...(fontAssetId ? { fontAsset: { id: fontAssetId, kind: 'font' } } : {}),
    canonicalState: {
      authoritative: true,
      text: {
        contract_version: 'text-state-v1',
        text: 'Playback title',
        font_family: 'DejaVu Sans',
        font_family_source: 'authored',
        ...(resourceId ? { font_resource_id: resourceId } : {}),
        font_face_source: resourceId ? 'packaged-resource' : 'family-name-only',
        font_size: 36,
        font_weight: '700',
        color: '#ffffff',
        stroke_width: 0,
        text_align: 'center',
        vertical_align: 'middle',
        line_height_mode: 'normal',
        letter_spacing: 0,
        border_radius: 0,
        padding: { top: 0, right: 0, bottom: 0, left: 0 },
      },
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text'>,
  };
}

function plan(
  mode: PreviewTransitionPairPlan<Layer>['mode'] = 'canonical-none',
  deferredReasons: string[] = [],
  weightedRasterDeferredReasons: string[] = [],
): Plan {
  return {
    mode,
    slots: [],
    deferredReasons,
    weightedRasterDeferredReasons,
  };
}

function weightedPlan(transitionId = 'weighted-crossfade'): Plan {
  const lower = mediaLayer('lower');
  const upper = mediaLayer('upper', 'image/png');
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

afterEach(() => {
  resetPreviewTextPlaybackRuntimeForTests();
  resetPreviewWeightedPlaybackRuntimeForTests();
});

describe('normal playback canonicalization gate', () => {
  it('leaves a non-playing address on the legacy time path', () => {
    expect(resolvePreviewPlaybackCanonicalization(null, undefined, [], null)).toEqual({
      mode: 'legacy-time',
      canonicalFrame: null,
    });
  });

  it('admits an authoritative media-only frame with no transition debt', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      37,
      { authoritative: true },
      [mediaLayer()],
      plan(),
    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 37 });
  });

  it('admits a clean source-over transition plan for media-only layers', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      12,
      { authoritative: true },
      [mediaLayer('lower'), mediaLayer('upper', 'image/png')],
      plan('canonical-source-over'),
    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 12 });
  });

  it('fails the whole visual frame closed when canonical frame authority is unavailable', () => {
    expect(resolvePreviewPlaybackCanonicalization(5, undefined, [mediaLayer()], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-frame-state-unavailable',
    });
    expect(resolvePreviewPlaybackCanonicalization(5, { authoritative: false }, [mediaLayer()], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'canonical-frame-state-nonauthoritative',
    });
  });

  it('fails closed for missing or non-authoritative canonical layer state', () => {
    const missing = mediaLayer();
    delete missing.canonicalState;
    expect(resolvePreviewPlaybackCanonicalization(5, { authoritative: true }, [missing], plan()).deferredReason)
      .toBe('canonical-layer-state-unavailable:media');

    const nonauthoritative = mediaLayer();
    nonauthoritative.canonicalState = { authoritative: false };
    expect(resolvePreviewPlaybackCanonicalization(5, { authoritative: true }, [nonauthoritative], plan()).deferredReason)
      .toBe('canonical-layer-state-nonauthoritative:media');
  });

  it('fails resource-backed text closed until exact font/layout runtime readiness is proven', () => {
    const layers = [textLayer()];
    expect(resolvePreviewPlaybackCanonicalization(8, { authoritative: true }, layers, plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-not-ready',
    });

    const planIdentity = previewTextPlaybackPlanIdentity(layers);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 8,
      planKey: previewTextPlaybackPlanKey(8, layers),
      planIdentity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(8, { authoritative: true }, layers, plan())).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 8,
    });
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, layers, plan())).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 9,
    });
  });

  it('keeps family-name-only text explicitly fail-closed', () => {
    const layer = textLayer('family-only', '', '');
    expect(resolvePreviewPlaybackCanonicalization(8, { authoritative: true }, [layer], plan())).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-deferred:family-only:resource-font-required',
    });
  });

  it('admits a media plus standalone text frame only after text readiness', () => {
    const layers = [mediaLayer('background'), textLayer('title')];
    expect(resolvePreviewPlaybackCanonicalization(11, { authoritative: true }, layers, plan()).deferredReason)
      .toBe('text-playback-runtime-not-ready');

    const planIdentity = previewTextPlaybackPlanIdentity(layers);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 11,
      planKey: previewTextPlaybackPlanKey(11, layers),
      planIdentity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(11, { authoritative: true }, layers, plan()))
      .toEqual({ mode: 'canonical-playback', canonicalFrame: 11 });
  });

  it.each([
    ['shape', { ...mediaLayer(), clip: { id: 'shape', shape: {} } }],
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

  it('fails weighted composition closed until the pair topology is proven ready, then stays warm across frames', () => {
    const transitionPlan = weightedPlan();
    const layers = [mediaLayer('lower'), mediaLayer('upper', 'image/png')];
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-weighted-runtime-not-ready',
    });

    const planKey = previewWeightedPlaybackPlanKey(9, transitionPlan);
    const planIdentity = previewWeightedPlaybackPlanIdentity(transitionPlan);
    publishPreviewWeightedPlaybackRuntime({ frameIndex: 9, planKey, planIdentity, status: 'ready' });
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 9,
    });
    expect(resolvePreviewPlaybackCanonicalization(10, { authoritative: true }, layers, transitionPlan)).toEqual({
      mode: 'canonical-playback',
      canonicalFrame: 10,
    });
    expect(resolvePreviewPlaybackCanonicalization(
      10,
      { authoritative: true },
      layers,
      weightedPlan('weighted-zoom'),
    ).deferredReason).toBe('transition-weighted-runtime-not-ready');
  });

  it('retains weighted raster-source and renderer-runtime deferral reasons', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [mediaLayer()],
      plan('canonical-weighted-deferred', [], ['weighted-crossfade:lower:unsupported-raster-source']),
    ).deferredReason).toBe(
      'transition-weighted-raster-deferred:weighted-crossfade:lower:unsupported-raster-source',
    );

    const transitionPlan = weightedPlan();
    const planKey = previewWeightedPlaybackPlanKey(9, transitionPlan);
    const planIdentity = previewWeightedPlaybackPlanIdentity(transitionPlan);
    publishPreviewWeightedPlaybackRuntime({
      frameIndex: 9,
      planKey,
      planIdentity,
      status: 'deferred',
      reason: 'upper:decoder-budget-poster',
    });
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [mediaLayer('lower'), mediaLayer('upper', 'image/png')],
      transitionPlan,
    ).deferredReason).toBe(
      'transition-weighted-runtime-deferred:upper:decoder-budget-poster',
    );
  });

  it('retains explicit canonical transition deferral reasons before runtime admission', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [mediaLayer()],
      plan('canonical-weighted-deferred', ['transition-1:overlap-not-adjacent']),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-deferred:transition-1:overlap-not-adjacent',
    });
  });
});
