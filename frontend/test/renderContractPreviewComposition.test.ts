import { describe, expect, it } from 'vitest';
import type {
  VideoAsset,
  VideoTimelineClip,
  VideoTimelineDocument,
} from '../src/types/video';
import {
  evaluateCanonicalPreviewCompositionFrame,
  PREVIEW_COMPOSITION_FRAME_V1,
} from '../src/video/renderContractPreviewComposition';

function clip(id: string, options: Partial<VideoTimelineClip> = {}): VideoTimelineClip {
  return {
    id,
    start_ms: 0,
    duration_ms: 1000,
    trim_in_ms: 0,
    trim_out_ms: 1000,
    transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
    effects: [],
    keyframes: [],
    ...options,
  };
}

function documentWith(clips: VideoTimelineClip[]): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 1280, height: 720, fps: 30, background: '#000000' },
    duration_ms: 2000,
    tracks: [{
      id: 'track-1',
      type: 'layer',
      name: 'Layer 1',
      locked: false,
      muted: false,
      visible: true,
      clips,
    }],
    markers: [],
    metadata: {},
  };
}

function asset(id: string): VideoAsset {
  return {
    id,
    kind: 'image',
    source_type: 'upload',
    file_name: `${id}.png`,
    file_path: `/tmp/${id}.png`,
    mime_type: 'image/png',
    size_bytes: 128,
    created_at: '2026-08-24T00:00:00Z',
  };
}

describe('canonical preview composition projection', () => {
  it('preserves canonical layer order while binding exact editor identities and assets', () => {
    const high = clip('clip-high', { asset_id: 'asset-high', z_index: 5 });
    const low = clip('clip-low', { z_index: 1, text: { text: 'Low' } });
    const document = documentWith([high, low]);
    const highAsset = asset('asset-high');

    const result = evaluateCanonicalPreviewCompositionFrame(document, [highAsset], 0);

    expect(result.contract_version).toBe(PREVIEW_COMPOSITION_FRAME_V1);
    expect(result.available).toBe(true);
    expect(result.layers?.map((layer) => layer.clip.id)).toEqual(['clip-low', 'clip-high']);
    expect(result.layers?.[0].clip).toBe(low);
    expect(result.layers?.[0].track).toBe(document.tracks[0]);
    expect(result.layers?.[1].clip).toBe(high);
    expect(result.layers?.[1].asset).toBe(highAsset);
    expect(result.layers?.[1].state.clip_id).toBe('clip-high');
    expect(result.frame_state?.frame_index).toBe(0);
  });

  it('keeps adapter ambiguity fail closed instead of inventing preview transition placement', () => {
    const document = documentWith([clip('clip-transition', {
      transitions: [{ id: 'transition-1', type: 'fade', duration_ms: 250 }],
    })]);

    const result = evaluateCanonicalPreviewCompositionFrame(document, [], 0);

    expect(result.available).toBe(false);
    expect(result.layers).toBeUndefined();
    expect(result.error?.code).toBe('V1_TRANSITION_PLACEMENT_AMBIGUOUS');
    expect(result.error?.path).toContain('transitions[0]');
  });

  it('carries canonical transform/source-time state without recomputing it in the projection', () => {
    const document = documentWith([clip('clip-animated', {
      trim_in_ms: 200,
      trim_out_ms: 2200,
      playback_rate: 2,
      transform: { x: 10, y: 20, scale: 1, rotation: 0, opacity: 0.8 },
      keyframes: [
        { id: 'x0', property: 'x', time_ms: 0, value: 10, easing: 'linear' },
        { id: 'x1', property: 'x', time_ms: 1000, value: 110, easing: 'linear' },
      ],
    })]);

    const result = evaluateCanonicalPreviewCompositionFrame(document, [], 15);
    const layer = result.layers?.[0];

    expect(result.available).toBe(true);
    expect(layer?.state.source_time_ms).toBe(1200);
    expect(layer?.state.transform.x).toBe(60);
    expect(layer?.state.transform.opacity).toBe(0.8);
  });
});
