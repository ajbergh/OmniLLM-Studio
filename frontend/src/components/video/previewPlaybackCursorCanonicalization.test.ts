import { afterEach, describe, expect, it } from 'vitest';
import { CURSOR_STATE_CONTRACT_V1 } from '../../video/renderContractCursor';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { VideoTimelineClip } from '../../types/video';
import { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';
import type { PreviewCursorPlaybackContext } from './previewCursorPlayback';
import {
  previewTextPlaybackPlanIdentity,
  previewTextPlaybackPlanKey,
  publishPreviewTextPlaybackRuntime,
  resetPreviewTextPlaybackRuntimeForTests,
} from './previewTextPlaybackRuntime';

type Layer = Parameters<typeof resolvePreviewPlaybackCanonicalization>[2][number];
type Plan = NonNullable<Parameters<typeof resolvePreviewPlaybackCanonicalization>[3]>;

function cursorOwner(overrides: Partial<VideoTimelineClip> = {}): VideoTimelineClip {
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

function cursorLayer(owner = cursorOwner()): Layer {
  return {
    clip: owner,
    track: { clips: [owner] },
    asset: { mime_type: 'video/mp4' },
    canonicalState: {
      authoritative: true,
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
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text' | 'cursor'>,
  } as Layer;
}

function textLayer(): Layer {
  return {
    clip: { id: 'title', text: {} },
    fontAsset: { id: 'font-asset', kind: 'font' },
    canonicalState: {
      authoritative: true,
      text: {
        contract_version: 'text-state-v1',
        text: 'Cursor plus text',
        font_family: 'DejaVu Sans',
        font_family_source: 'authored',
        font_resource_id: 'playback-font-v1',
        font_face_source: 'packaged-resource',
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
    } as Pick<CanonicalFrameLayerState, 'authoritative' | 'text' | 'cursor'>,
  } as Layer;
}

function context(): PreviewCursorPlaybackContext {
  return { fps: 30, canvasWidth: 640, canvasHeight: 360, scenes: [] };
}

function plan(): Plan {
  return { mode: 'canonical-none', slots: [], deferredReasons: [], weightedRasterDeferredReasons: [] };
}

afterEach(() => resetPreviewTextPlaybackRuntimeForTests());

describe('cursor whole-frame canonical playback admission', () => {
  it('admits an exact static-2D cursor owner', () => {
    expect(resolvePreviewPlaybackCanonicalization(
      6,
      { authoritative: true },
      [cursorLayer()],
      plan(),
      context(),
    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 6 });
  });

  it('fails the complete frame closed when cursor export support is structurally deferred', () => {
    const owner = cursorOwner({ fade_in_ms: 100 });
    expect(resolvePreviewPlaybackCanonicalization(
      6,
      { authoritative: true },
      [cursorLayer(owner)],
      plan(),
      context(),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'cursor-playback-deferred:cursor-owner:fade-unsupported',
    });
  });

  it('keeps cursor plus resource text frame-atomic until text runtime is ready', () => {
    const layers = [cursorLayer(), textLayer()];
    expect(resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      layers,
      plan(),
      context(),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'text-playback-runtime-not-ready',
    });

    const identity = previewTextPlaybackPlanIdentity(layers);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 8,
      planKey: previewTextPlaybackPlanKey(8, layers),
      planIdentity: identity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      layers,
      plan(),
      context(),
    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 8 });
  });

  it('does not partially promote ready text when the cursor parent is unsupported', () => {
    const owner = cursorOwner({
      keyframes: [{ id: 'cursor-x', property: 'x', time_ms: 0, value: 20 }],
    });
    const layers = [cursorLayer(owner), textLayer()];
    const identity = previewTextPlaybackPlanIdentity(layers);
    publishPreviewTextPlaybackRuntime({
      frameIndex: 8,
      planKey: previewTextPlaybackPlanKey(8, layers),
      planIdentity: identity,
      status: 'ready',
    });
    expect(resolvePreviewPlaybackCanonicalization(
      8,
      { authoritative: true },
      layers,
      plan(),
      context(),
    )).toEqual({
      mode: 'legacy-time-fallback',
      canonicalFrame: null,
      deferredReason: 'cursor-playback-deferred:cursor-owner:animated-parent-unsupported',
    });
  });
});
