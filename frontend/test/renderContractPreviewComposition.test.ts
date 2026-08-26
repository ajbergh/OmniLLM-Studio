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

function asset(id: string, dimensions: { width?: number; height?: number } = {}): VideoAsset {
  return {
    id,
    kind: 'image',
    source_type: 'upload',
    file_name: `${id}.png`,
    file_path: `/tmp/${id}.png`,
    mime_type: 'image/png',
    size_bytes: 128,
    ...dimensions,
    created_at: '2026-08-24T00:00:00Z',
  };
}

function fontAsset(id: string, resourceID: string, fileName = `${id}.woff2`): VideoAsset {
  return {
    id,
    kind: 'font',
    source_type: 'upload',
    file_name: fileName,
    file_path: `/tmp/${fileName}`,
    mime_type: 'font/woff2',
    size_bytes: 256,
    metadata_json: JSON.stringify({ font_resource_id: resourceID }),
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

  it('projects persisted asset dimensions into canonical contain geometry without fabricating source provenance', () => {
    const document = documentWith([clip('clip-portrait', { asset_id: 'asset-portrait' })]);
    const portrait = asset('asset-portrait', { width: 1000, height: 2000 });

    const result = evaluateCanonicalPreviewCompositionFrame(document, [portrait], 0);
    const state = result.layers?.[0].state;

    expect(result.available).toBe(true);
    expect(state?.content_bounds).toEqual({ x: 0, y: 0, width: 1000, height: 2000 });
    expect(state?.source_provenance).toBeUndefined();
    expect(state?.media_geometry).toEqual({
      contract_version: 'media-geometry-v1',
      fit: 'contain',
      viewport_bounds: { x: 0, y: 0, width: 1280, height: 720 },
      source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
      visible_source_bounds: { x: 0, y: 0, width: 1000, height: 2000 },
      painted_bounds: { x: 460, y: 0, width: 360, height: 720 },
      clip_bounds: { x: 0, y: 0, width: 1280, height: 720 },
      scale_x: 0.36,
      scale_y: 0.36,
    });
    expect(state?.unresolved).toEqual([]);
    expect(state?.authoritative).toBe(true);
    expect(result.frame_state?.authoritative).toBe(true);
  });

  it('keeps media geometry unresolved when persisted probe dimensions are incomplete or invalid', () => {
    const document = documentWith([clip('clip-missing-bounds', { asset_id: 'asset-missing-bounds' })]);
    const incomplete = asset('asset-missing-bounds', { width: 1920 });

    const result = evaluateCanonicalPreviewCompositionFrame(document, [incomplete], 0);
    const state = result.layers?.[0].state;

    expect(result.available).toBe(true);
    expect(state?.content_bounds).toBeUndefined();
    expect(state?.media_geometry).toBeUndefined();
    expect(state?.unresolved).toContain('media_geometry:content_bounds');
    expect(state?.authoritative).toBe(false);
    expect(result.frame_state?.authoritative).toBe(false);
  });

  it('binds an authored font_resource_id to the exact current font asset without claiming packaged provenance', () => {
    const text = clip('clip-font', {
      text: { text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' },
    });
    const document = documentWith([text]);
    const font = fontAsset('font-1', 'inter-400-normal', 'Inter-Regular.woff2');

    const result = evaluateCanonicalPreviewCompositionFrame(document, [font], 0);
    const layer = result.layers?.[0];

    expect(result.available).toBe(true);
    expect(layer?.font_asset).toBe(font);
    expect(layer?.state.text?.font_resource_id).toBe('inter-400-normal');
    expect(layer?.state.text?.font_face_source).toBe('family-name-only');
  });

  it('fails closed when an authored font_resource_id has no current project font asset', () => {
    const document = documentWith([clip('clip-font-missing', {
      text: { text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' },
    })]);

    const result = evaluateCanonicalPreviewCompositionFrame(document, [], 0);

    expect(result.available).toBe(false);
    expect(result.error?.code).toBe('EDITOR_FONT_RESOURCE_MISSING');
    expect(result.error?.path).toBe('tracks[0].clips[0].text.font_resource_id');
    expect(result.error?.message).toContain('inter-400-normal');
  });

  it('fails closed when two project font assets declare the same font_resource_id', () => {
    const document = documentWith([clip('clip-font-duplicate', {
      text: { text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' },
    })]);
    const first = fontAsset('font-1', 'inter-400-normal', 'Inter-Regular.woff2');
    const second = fontAsset('font-2', 'inter-400-normal', 'Inter-Regular-Copy.woff2');

    const result = evaluateCanonicalPreviewCompositionFrame(document, [first, second], 0);

    expect(result.available).toBe(false);
    expect(result.error?.code).toBe('EDITOR_FONT_RESOURCE_DUPLICATE');
    expect(result.error?.path).toBe('assets');
    expect(result.error?.message).toContain('Inter-Regular.woff2');
    expect(result.error?.message).toContain('Inter-Regular-Copy.woff2');
  });
});
