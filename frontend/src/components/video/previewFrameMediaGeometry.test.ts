import { describe, expect, it } from 'vitest';
import type { VideoAsset, VideoTimelineDocument } from '../../types/video';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { CanonicalMediaGeometry } from '../../video/renderContractMediaGeometry';
import { buildTimelineIntervalIndex, queryActiveClipsAtFrame } from './pro/timelineIndex';
import {
  canonicalPreviewMediaClipPath,
  canonicalPreviewMediaElementStyle,
  resolveCanonicalPreviewMediaGeometry,
} from './previewFrameMediaGeometry';

const portraitGeometry: CanonicalMediaGeometry = {
  contract_version: 'media-geometry-v1',
  fit: 'contain',
  viewport_bounds: { x: 0, y: 0, width: 1280, height: 720 },
  source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
  visible_source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
  painted_bounds: { x: 460, y: 0, width: 360, height: 720 },
  clip_bounds: { x: 64, y: 72, width: 1088, height: 576 },
  scale_x: 0.36,
  scale_y: 0.36,
};

function state(geometry: CanonicalMediaGeometry | null = portraitGeometry): CanonicalFrameLayerState {
  return {
    media_geometry: geometry ?? undefined,
  } as CanonicalFrameLayerState;
}

function portraitAsset(): VideoAsset {
  return {
    id: 'asset-portrait',
    kind: 'image',
    source_type: 'upload',
    file_name: 'portrait.png',
    file_path: '/tmp/portrait.png',
    mime_type: 'image/png',
    size_bytes: 128,
    width: 1000,
    height: 2000,
    created_at: '2026-08-24T00:00:00Z',
  };
}

function document(): VideoTimelineDocument {
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
        id: 'clip-portrait',
        asset_id: 'asset-portrait',
        start_ms: 0,
        duration_ms: 1000,
        trim_in_ms: 0,
        trim_out_ms: 1000,
        transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
        effects: [],
        transitions: [],
        keyframes: [],
      }],
    }],
  };
}

describe('canonical preview media geometry', () => {
  it('consumes real persisted portrait bounds through canonical FrameState', () => {
    const entry = queryActiveClipsAtFrame(
      buildTimelineIntervalIndex(document(), [portraitAsset()]),
      15,
      30,
    )[0];
    const canonicalState = entry.canonicalState;
    if (!canonicalState) throw new Error('expected canonical frame state');

    expect(canonicalState.media_geometry?.painted_bounds).toEqual({
      x: 460,
      y: 0,
      width: 360,
      height: 720,
    });
    expect(resolveCanonicalPreviewMediaGeometry(15, canonicalState, true, false, false))
      .toBe(canonicalState.media_geometry);
  });

  it('uses canonical geometry only for an admitted non-interactive visual frame', () => {
    expect(resolveCanonicalPreviewMediaGeometry(null, state(), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), false, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), true, true, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(), true, false, true)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(12, state(null), true, false, false)).toBeNull();
  });

  it('fails closed when canonical bounds or scales are invalid', () => {
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      painted_bounds: { ...portraitGeometry.painted_bounds, width: 0 },
    }), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      scale_x: Number.NaN,
    }), true, false, false)).toBeNull();
    expect(resolveCanonicalPreviewMediaGeometry(1, state({
      ...portraitGeometry,
      clip_bounds: { x: -1, y: 0, width: 1280, height: 720 },
    }), true, false, false)).toBeNull();
  });

  it('maps canonical painted bounds into preview CSS pixels without object-fit reinterpretation', () => {
    expect(canonicalPreviewMediaElementStyle(portraitGeometry, 0.5)).toEqual({
      position: 'absolute',
      left: 230,
      top: 0,
      width: 180,
      height: 360,
      maxWidth: 'none',
      maxHeight: 'none',
      objectFit: 'fill',
    });
  });

  it('maps canonical canvas clip bounds to a full-stage inset path', () => {
    expect(canonicalPreviewMediaClipPath(portraitGeometry, 0.5)).toBe('inset(36px 64px 36px 32px)');
  });
});
