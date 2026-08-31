import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';
import { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';

type Layer = Parameters<typeof resolvePreviewPlaybackCanonicalization>[2][number];
type Plan = Parameters<typeof resolvePreviewPlaybackCanonicalization>[3];

function mediaLayer(id = 'media', mimeType = 'video/mp4'): Layer {
  return {
    clip: { id },
    asset: { mime_type: mimeType },
    canonicalState: { authoritative: true } as Pick<CanonicalFrameLayerState, 'authoritative'>,
  };
}

function plan(
  mode: PreviewTransitionPairPlan<Layer>['mode'] = 'canonical-none',
  deferredReasons: string[] = [],
): Plan {
  return { mode, deferredReasons };
}

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

  it.each([
    ['text', { ...mediaLayer(), clip: { id: 'text', text: {} } }],
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

  it.each([
    ['legacy', 'transition-plan-legacy'],
    ['canonical-weighted-deferred', 'transition-plan-weighted-deferred'],
    ['canonical-mixed', 'transition-plan-mixed'],
  ] as const)('fails closed for %s transition composition', (mode, reason) => {
    expect(resolvePreviewPlaybackCanonicalization(9, { authoritative: true }, [mediaLayer()], plan(mode))).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: reason,
    });
  });

  it('retains explicit canonical transition deferral reasons', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [mediaLayer()],
      plan('canonical-none', ['transition-1:overlap-not-adjacent']),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'transition-deferred:transition-1:overlap-not-adjacent',
    });
  });
});
