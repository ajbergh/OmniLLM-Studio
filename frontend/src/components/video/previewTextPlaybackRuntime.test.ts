import { afterEach, describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  previewTextPlaybackPlanIdentity,
  previewTextPlaybackPlanKey,
  previewTextPlaybackStructuralDeferredReason,
  publishPreviewTextPlaybackRuntime,
  resetPreviewTextPlaybackRuntimeForTests,
  resolvePreviewTextPlaybackRuntime,
  type PreviewTextPlaybackLayer,
} from './previewTextPlaybackRuntime';

function textLayer(
  id = 'title',
  resourceId: string | undefined = 'playback-font-v1',
  fontAssetId = 'font-asset',
): PreviewTextPlaybackLayer {
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

afterEach(() => resetPreviewTextPlaybackRuntimeForTests());

describe('normal playback text runtime', () => {
  it('builds stable topology identity with exact frame execution keys', () => {
    const layers = [textLayer()];
    const identity = previewTextPlaybackPlanIdentity(layers);
    expect(identity).toContain('playback-font-v1');
    expect(identity).toContain('font-asset');
    expect(previewTextPlaybackPlanKey(12, layers)).toBe(`12|${identity}`);
    expect(previewTextPlaybackPlanKey(13, layers)).toBe(`13|${identity}`);
  });

  it('keeps family-name-only and missing font assets explicitly deferred', () => {
    expect(previewTextPlaybackStructuralDeferredReason([textLayer('family', undefined)]))
      .toBe('family:resource-font-required');
    expect(resolvePreviewTextPlaybackRuntime(5, [textLayer('family', undefined)])).toEqual({
      ready: false,
      deferredReason: 'text-playback-runtime-deferred:family:resource-font-required',
    });
    expect(previewTextPlaybackStructuralDeferredReason([textLayer('missing', 'font-v1', '')]))
      .toBe('missing:font-asset-unavailable');
  });

  it('requires one successful runtime proof and keeps identical text topology warm across frames', () => {
    const layers = [textLayer()];
    expect(resolvePreviewTextPlaybackRuntime(9, layers)).toEqual({
      ready: false,
      deferredReason: 'text-playback-runtime-not-ready',
    });

    const planIdentity = previewTextPlaybackPlanIdentity(layers);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 9,
      planKey: previewTextPlaybackPlanKey(9, layers),
      planIdentity,
      status: 'ready',
    });
    expect(resolvePreviewTextPlaybackRuntime(9, layers)).toEqual({ ready: true });
    expect(resolvePreviewTextPlaybackRuntime(10, layers)).toEqual({ ready: true });
  });

  it('revokes warm readiness when canonical text or font identity changes', () => {
    const original = [textLayer()];
    const planIdentity = previewTextPlaybackPlanIdentity(original);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 7,
      planKey: previewTextPlaybackPlanKey(7, original),
      planIdentity,
      status: 'ready',
    });

    const changedText = [textLayer()];
    changedText[0].canonicalState = {
      ...changedText[0].canonicalState,
      text: { ...changedText[0].canonicalState?.text, text: 'Changed title' },
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text'>;
    expect(resolvePreviewTextPlaybackRuntime(8, changedText).deferredReason)
      .toBe('text-playback-runtime-not-ready');
    expect(resolvePreviewTextPlaybackRuntime(8, [textLayer('title', 'playback-font-v1', 'font-asset-2')]).deferredReason)
      .toBe('text-playback-runtime-not-ready');
  });

  it('retains pending, deferred, and failed readiness semantics', () => {
    const layers = [textLayer()];
    const planIdentity = previewTextPlaybackPlanIdentity(layers);
    const planKey = previewTextPlaybackPlanKey(9, layers);

    publishPreviewTextPlaybackRuntime({ frameIndex: 9, planKey, planIdentity, status: 'pending' });
    expect(resolvePreviewTextPlaybackRuntime(9, layers).deferredReason)
      .toBe('text-playback-runtime-not-ready');

    publishPreviewTextPlaybackRuntime({
      frameIndex: 9,
      planKey,
      planIdentity,
      status: 'deferred',
      reason: 'title:text-layout-not-ready',
    });
    expect(resolvePreviewTextPlaybackRuntime(9, layers).deferredReason)
      .toBe('text-playback-runtime-deferred:title:text-layout-not-ready');

    publishPreviewTextPlaybackRuntime({
      frameIndex: 9,
      planKey,
      planIdentity,
      status: 'failed',
      reason: 'title:font-face-load-failed',
    });
    expect(resolvePreviewTextPlaybackRuntime(9, layers).deferredReason)
      .toBe('text-playback-runtime-failed:title:font-face-load-failed');
  });
});
