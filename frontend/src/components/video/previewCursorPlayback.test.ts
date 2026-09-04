import { describe, expect, it } from 'vitest';
import { CURSOR_STATE_CONTRACT_V1, CURSOR_STATE_CONTRACT_V2 } from '../../video/renderContractCursor';
import type { VideoTimelineClip, VideoTimelineScene, VideoTimelineTrack } from '../../types/video';
import {
  previewCursorPlaybackStructuralDeferredReason,
  type PreviewCursorPlaybackContext,
  type PreviewCursorPlaybackLayer,
} from './previewCursorPlayback';

function clip(overrides: Partial<VideoTimelineClip> = {}): VideoTimelineClip {
  return {
    id: 'cursor-owner',
    asset_id: 'asset-video',
    start_ms: 0,
    duration_ms: 1200,
    trim_in_ms: 0,
    trim_out_ms: 1200,
    transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
    cursor: {
      visible: true,
      scale: 1,
      highlight: true,
      click_rings: true,
      events: [
        { time_ms: 0, x: 100, y: 100 },
        { time_ms: 600, x: 320, y: 180, click: true },
        { time_ms: 1199, x: 540, y: 260 },
      ],
    },
    effects: [],
    transitions: [],
    keyframes: [],
    animation_blocks: [],
    ...overrides,
  };
}

function layer(
  owner = clip(),
  siblings: VideoTimelineClip[] = [],
): PreviewCursorPlaybackLayer {
  const track: Pick<VideoTimelineTrack, 'clips'> = { clips: [owner, ...siblings] };
  return {
    clip: owner,
    track,
    asset: { mime_type: 'video/mp4' },
    canonicalState: {
      cursor: {
        contract_version: CURSOR_STATE_CONTRACT_V1,
        visible: true,
        scale: 1,
        highlight: true,
        click_rings: true,
        x: 160,
        y: 120,
        click: false,
      },
    },
  };
}

function context(overrides: Partial<PreviewCursorPlaybackContext> = {}): PreviewCursorPlaybackContext {
  return {
    fps: 30,
    canvasWidth: 640,
    canvasHeight: 360,
    scenes: [],
    ...overrides,
  };
}

describe('cursor normal-playback export-proven subset', () => {
  it('admits the static 2D media-owner subset', () => {
    expect(previewCursorPlaybackStructuralDeferredReason(layer(), context())).toBeUndefined();
  });


  it('admits smoothing when the exact cursor-state-v2 sample is available', () => {
    const owner = clip({ cursor: { ...clip().cursor!, smoothing: true } });
    const smoothed = layer(owner);
    smoothed.canonicalState = {
      cursor: {
        contract_version: CURSOR_STATE_CONTRACT_V2,
        visible: true,
        scale: 1,
        highlight: true,
        click_rings: true,
        x: 160,
        y: 120,
        click: false,
      },
    };
    expect(previewCursorPlaybackStructuralDeferredReason(smoothed, context())).toBeUndefined();
  });

  it('fails closed when authored smoothing and computed cursor contract versions disagree', () => {
    const smoothedOwner = clip({ cursor: { ...clip().cursor!, smoothing: true } });
    const smoothedWithV1 = layer(smoothedOwner);
    expect(previewCursorPlaybackStructuralDeferredReason(smoothedWithV1, context()))
      .toBe('cursor-owner:canonical-cursor-contract-mismatch');

    const linearWithV2 = layer();
    linearWithV2.canonicalState = {
      cursor: {
        contract_version: CURSOR_STATE_CONTRACT_V2,
        visible: true,
        scale: 1,
        highlight: true,
        click_rings: true,
        x: 160,
        y: 120,
        click: false,
      },
    };
    expect(previewCursorPlaybackStructuralDeferredReason(linearWithV2, context()))
      .toBe('cursor-owner:canonical-cursor-contract-mismatch');
  });

  it('retains the renderer exact-frame and expansion bounds', () => {
    expect(previewCursorPlaybackStructuralDeferredReason(layer(), context({ fps: 1000 })))
      .toBe('cursor-owner:fps-out-of-range');
    expect(previewCursorPlaybackStructuralDeferredReason(
      layer(clip({ duration_ms: 11000, trim_out_ms: 11000 })),
      context(),
    )).toBe('cursor-owner:segment-bound-330');
  });

  it.each([
    ['fade', clip({ fade_in_ms: 100 }), 'cursor-owner:fade-unsupported'],
    ['transition', clip({ transitions: [{ id: 't', type: 'crossfade', duration_ms: 300 }] }), 'cursor-owner:transition-parent-unsupported'],
    ['animation', clip({ animation_blocks: [{ id: 'a', block_key: 'move', family: 'during', start_ms: 0, duration_ms: 300, generated_keyframe_ids: [] }] }), 'cursor-owner:animation-parent-unsupported'],
    ['effect', clip({ effects: [{ id: 'fx', type: 'blur', enabled: true, params: {} }] }), 'cursor-owner:effect-parent-unsupported'],
    ['keyframe', clip({ keyframes: [{ id: 'k', property: 'x', time_ms: 0, value: 10 }] }), 'cursor-owner:animated-parent-unsupported'],
  ])('fails closed for %s cursor parents', (_label, owner, expected) => {
    expect(previewCursorPlaybackStructuralDeferredReason(layer(owner), context())).toBe(expected);
  });

  it('rejects non-uniform and 3D parent transforms', () => {
    expect(previewCursorPlaybackStructuralDeferredReason(
      layer(clip({ transform: { x: 0, y: 0, scale: 1, scale_x: 1, scale_y: 1.1, rotation: 0, opacity: 1 } })),
      context(),
    )).toBe('cursor-owner:parent-transform-unsupported');
    expect(previewCursorPlaybackStructuralDeferredReason(
      layer(clip({ transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1, rotation_y: 5 } })),
      context(),
    )).toBe('cursor-owner:parent-transform-unsupported');
  });

  it('rejects same-track overlap and overlapping scene camera ownership', () => {
    const sibling = clip({ id: 'sibling', start_ms: 600, duration_ms: 800, cursor: undefined });
    expect(previewCursorPlaybackStructuralDeferredReason(layer(clip(), [sibling]), context()))
      .toBe('cursor-owner:same-track-overlap-unsupported');

    const scene: Pick<VideoTimelineScene, 'start_ms' | 'duration_ms' | 'camera'> = {
      start_ms: 0,
      duration_ms: 2000,
      camera: { x: 0 },
    };
    expect(previewCursorPlaybackStructuralDeferredReason(layer(), context({ scenes: [scene] })))
      .toBe('cursor-owner:scene-camera-unsupported');
  });

  it('rejects cursor rasters that the export path cannot bound inside the canvas', () => {
    expect(previewCursorPlaybackStructuralDeferredReason(
      layer(),
      context({ canvasWidth: 120, canvasHeight: 120 }),
    )).toBe('cursor-owner:cursor-raster-out-of-bounds');
  });

  it('requires the exact canonical cursor sample before granting playback authority', () => {
    const missing = layer();
    missing.canonicalState = {};
    expect(previewCursorPlaybackStructuralDeferredReason(missing, context()))
      .toBe('cursor-owner:canonical-cursor-state-unavailable');
  });
});
