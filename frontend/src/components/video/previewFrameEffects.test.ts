import { describe, expect, it } from 'vitest';
import type { VideoAsset, VideoTimelineDocument, VideoTimelineEffect } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import {
  composeCanonicalPreviewFilter,
  resolvePreviewFrameEffectPaint,
} from './previewFrameEffects';

function imageAsset(): VideoAsset {
  return {
    id: 'asset-image',
    kind: 'image',
    source_type: 'upload',
    file_name: 'image.png',
    file_path: '/tmp/image.png',
    mime_type: 'image/png',
    size_bytes: 128,
    width: 1280,
    height: 720,
    created_at: '2026-08-24T00:00:00Z',
  };
}

function documentWithAnimatedEffect(): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 1280, height: 720, fps: 30, background: '#000000' },
    duration_ms: 1000,
    markers: [],
    metadata: {},
    tracks: [{
      id: 'track-1',
      type: 'layer',
      name: 'Layer 1',
      locked: false,
      muted: false,
      visible: true,
      clips: [{
        id: 'clip-image',
        asset_id: 'asset-image',
        start_ms: 0,
        duration_ms: 1000,
        trim_in_ms: 0,
        trim_out_ms: 1000,
        transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
        effects: [{ id: 'bright', type: 'brightness', enabled: true, params: { amount: 1.1 } }],
        transitions: [],
        keyframes: [
          { id: 'bright-start', property: 'effect.bright.amount', time_ms: 0, value: 1, easing: 'linear' },
          { id: 'bright-end', property: 'effect.bright.amount', time_ms: 1000, value: 2, easing: 'linear' },
        ],
      }],
    }],
  };
}

const legacyBrightness: VideoTimelineEffect[] = [
  { id: 'bright', type: 'brightness', enabled: true, params: { amount: 1.1 } },
];

describe('canonical preview clip effects', () => {
  it('consumes exact-frame canonical effect automation instead of authored params', () => {
    const entry = queryActiveClipsAtFrame(
      buildTimelineIntervalIndex(documentWithAnimatedEffect(), [imageAsset()]),
      15,
      30,
    )[0];
    const canonicalState = entry.canonicalState;
    if (!canonicalState) throw new Error('expected canonical frame state');

    expect(canonicalState.effects).toEqual([
      expect.objectContaining({
        contract_version: 'effect-state-v1',
        id: 'bright',
        type: 'brightness',
        scope: 'clip',
        order: 0,
        params: { amount: 1.5 },
      }),
    ]);
    expect(resolvePreviewFrameEffectPaint(canonicalState, entry.clip.effects)).toEqual({
      filter: 'brightness(1.5)',
      mode: 'canonical-frame',
    });
  });

  it('falls back to authored preview effects only when canonical state is unavailable', () => {
    expect(resolvePreviewFrameEffectPaint(undefined, legacyBrightness)).toEqual({
      filter: 'brightness(1.1)',
      mode: 'legacy-authored',
    });
  });

  it('treats an omitted canonical effect stack as authoritative zero enabled effects', () => {
    const canonicalState = { effects: undefined } as Pick<CanonicalFrameLayerState, 'effects'>;
    expect(resolvePreviewFrameEffectPaint(canonicalState, legacyBrightness)).toEqual({
      filter: undefined,
      mode: 'canonical-frame',
    });
  });

  it('keeps canonical but CSS-unrenderable effects out of the filter without legacy reinterpretation', () => {
    const canonicalState = {
      effects: [{
        contract_version: 'effect-state-v1',
        id: 'sharp',
        type: 'sharpen',
        scope: 'clip',
        order: 0,
        params: { amount: 2 },
      }],
    } as Pick<CanonicalFrameLayerState, 'effects'>;
    expect(resolvePreviewFrameEffectPaint(canonicalState, legacyBrightness)).toEqual({
      filter: undefined,
      mode: 'canonical-frame',
    });
  });

  it('fails closed on malformed canonical effect identity or ordering', () => {
    expect(() => composeCanonicalPreviewFilter([{
      contract_version: 'effect-state-v1',
      id: 'scene-effect',
      type: 'brightness',
      scope: 'scene',
      order: 0,
      params: { amount: 1.2 },
    }])).toThrow(/invalid scope/);

    expect(() => composeCanonicalPreviewFilter([
      {
        contract_version: 'effect-state-v1', id: 'one', type: 'brightness', scope: 'clip', order: 0, params: { amount: 1.1 },
      },
      {
        contract_version: 'effect-state-v1', id: 'two', type: 'contrast', scope: 'clip', order: 0, params: { amount: 1.2 },
      },
    ])).toThrow(/invalid or duplicate order/);
  });
});
