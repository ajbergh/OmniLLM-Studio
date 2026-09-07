import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';

const playbackContext = {
  fps: 30,
  canvasWidth: 640,
  canvasHeight: 360,
  scenes: [],
};

function roundedLayer(kind: 'rounded_rectangle' | 'ellipse' = 'rounded_rectangle') {
  return {
    clip: {
      id: 'shape',
      start_ms: 0,
      duration_ms: 1200,
      trim_in_ms: 0,
      trim_out_ms: 1200,
      shape: {
        kind,
        width: 240,
        height: 120,
        fill: 'rgba(10,20,30,0.5)',
        stroke: '#f59e0b',
        stroke_width: 8,
        corner_radius: 24,
      },
      transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
      effects: [],
      keyframes: [],
    },
    canonicalState: {
      authoritative: true,
      shape: {
        contract_version: 'shape-state-v1',
        kind,
        width: 240,
        height: 120,
        fill: 'rgba(10,20,30,0.5)',
        stroke: '#f59e0b',
        stroke_width: 8,
        blur_radius: 12,
        corner_radius: 24,
      },
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'shape'>,
  };
}

const transitionPlan = {
  mode: 'canonical-none' as const,
  slots: [],
  deferredReasons: [],
  weightedRasterDeferredReasons: [],
};

describe('normal playback shape canonicalization', () => {
  it('admits the export-proven rounded rectangle subset', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [roundedLayer()],
      transitionPlan,
      playbackContext,
    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 9 });
  });

  it('keeps unsupported annotation kinds on the whole-frame fallback', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [roundedLayer('ellipse')],
      transitionPlan,
      playbackContext,
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'shape-playback-deferred:shape:shape-kind-unsupported',
    });
  });

  it('revokes a mixed frame atomically when the rounded rectangle parent is unsupported', () => {
    const shape = roundedLayer();
    shape.clip.fade_in_ms = 100;
    const media = {
      clip: { id: 'media' },
      asset: { mime_type: 'image/png' },
      canonicalState: { authoritative: true },
    };
    expect(resolvePreviewPlaybackCanonicalization(
      9,
      { authoritative: true },
      [media, shape],
      transitionPlan,
      playbackContext,
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'shape-playback-deferred:shape:fade-unsupported',
    });
  });
});
