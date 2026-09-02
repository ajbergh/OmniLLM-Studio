import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../../video/renderContractMediaGeometry';
import {
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_OWNER_ALPHA,
  TRANSITION_PAINT_PAIR_SLIDE,
  TRANSITION_PAINT_PAIR_WIPE,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import { planPreviewFrameTransitionPairs, shouldConsumePreviewFrameSourceOverPairs } from './previewFrameTransitionPairs';

function state(
  clipId: string,
  transitionPaint?: CanonicalTransitionPaint[],
  overrides: Partial<CanonicalFrameLayerState> = {},
): CanonicalFrameLayerState {
  return {
    clip_id: clipId,
    transition_paint: transitionPaint,
    authoritative: true,
    unresolved: [],
    media_geometry: {
      contract_version: MEDIA_GEOMETRY_CONTRACT_V1,
      fit: 'contain',
      viewport_bounds: { x: 0, y: 0, width: 640, height: 360 },
      source_bounds: { x: 0, y: 0, width: 640, height: 360 },
      visible_source_bounds: { x: 0, y: 0, width: 640, height: 360 },
      painted_bounds: { x: 0, y: 0, width: 640, height: 360 },
      clip_bounds: { x: 0, y: 0, width: 640, height: 360 },
      scale_x: 1,
      scale_y: 1,
    },
    view_transform: {
      x: 0,
      y: 0,
      z: 0,
      scale_x: 1,
      scale_y: 1,
      rotation_x: 0,
      rotation_y: 0,
      rotation_z: 0,
      opacity: 1,
      anchor_x: 0,
      anchor_y: 0,
    },
    ...overrides,
  } as CanonicalFrameLayerState;
}

function layer(
  clipId: string,
  transitionPaint?: CanonicalTransitionPaint[],
  overrides: Partial<CanonicalFrameLayerState> = {},
  mimeType = 'image/png',
) {
  return {
    clip: { id: clipId },
    asset: { mime_type: mimeType },
    canonicalState: state(clipId, transitionPaint, overrides),
  };
}

function paint(overrides: Partial<CanonicalTransitionPaint>): CanonicalTransitionPaint {
  return {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: 'transition-1',
    type: 'slide',
    placement: 'between',
    composition: TRANSITION_PAINT_PAIR_SLIDE,
    owner_clip_id: 'clip-a',
    peer_clip_id: 'clip-b',
    progress: 0.25,
    outgoing_clip_id: 'clip-a',
    incoming_clip_id: 'clip-b',
    ...overrides,
  };
}

describe('planPreviewFrameTransitionPairs', () => {
  it('keeps free-running or partially canonical layers on the legacy slot path', () => {
    const result = planPreviewFrameTransitionPairs(null, [layer('clip-a'), layer('clip-b')]);
    expect(result.mode).toBe('legacy');
    expect(result.slots.map((slot) => slot.kind)).toEqual(['single', 'single']);
    expect(result.weightedRasterDeferredReasons).toEqual([]);
  });

  it('replaces adjacent pair-slide inputs in place and resolves both spatial paints', () => {
    const result = planPreviewFrameTransitionPairs(12, [
      layer('underlay'),
      layer('clip-a', [paint({
        translation_space: TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
        outgoing_offset_x: -0.25,
        outgoing_offset_y: 0,
        incoming_offset_x: 0.75,
        incoming_offset_y: 0,
      })]),
      layer('clip-b'),
      layer('overlay'),
    ]);

    expect(result.mode).toBe('canonical-source-over');
    expect(shouldConsumePreviewFrameSourceOverPairs(result)).toBe(true);
    expect(result.slots.map((slot) => slot.kind)).toEqual(['single', 'pair', 'single']);
    const pair = result.slots[1];
    expect(pair.kind).toBe('pair');
    if (pair.kind !== 'pair') return;
    expect(pair.surface.replacement_layer_index).toBe(1);
    expect(pair.execution).toBe('source-over-dom');
    expect(pair.weightedRasterSource).toBeUndefined();
    expect(pair.layerPaintByClipId.get('clip-a')).toMatchObject({ offsetXFraction: -0.25, offsetYFraction: 0 });
    expect(pair.layerPaintByClipId.get('clip-b')).toMatchObject({ offsetXFraction: 0.75, offsetYFraction: 0 });
  });

  it('maps pair wipe to incoming layer clip without adding opacity weights', () => {
    const result = planPreviewFrameTransitionPairs(20, [
      layer('clip-a', [paint({
        type: 'wipe',
        composition: TRANSITION_PAINT_PAIR_WIPE,
        clip_space: TRANSITION_PAINT_CLIP_LAYER_FRACTION,
        incoming_clip_top: 0,
        incoming_clip_right: 0.4,
        incoming_clip_bottom: 0,
        incoming_clip_left: 0.1,
      })]),
      layer('clip-b'),
    ]);
    const pair = result.slots[0];
    expect(pair.kind).toBe('pair');
    if (pair.kind !== 'pair') return;
    expect(pair.execution).toBe('source-over-dom');
    expect(pair.pixel.outgoing_weight).toBeUndefined();
    expect(pair.pixel.incoming_weight).toBeUndefined();
    expect(pair.layerPaintByClipId.get('clip-b')?.clipPath).toBe('inset(0% 40% 0% 10%)');
  });

  it('classifies weighted crossfade media sources while keeping Canvas execution deferred', () => {
    const result = planPreviewFrameTransitionPairs(30, [
      layer('clip-a', [paint({
        type: 'crossfade',
        composition: TRANSITION_PAINT_CROSSFADE,
        outgoing_weight: 0.6,
        incoming_weight: 0.4,
      })]),
      layer('clip-b', undefined, {}, 'video/mp4'),
    ]);
    const pair = result.slots[0];
    expect(result.mode).toBe('canonical-weighted-deferred');
    expect(shouldConsumePreviewFrameSourceOverPairs(result)).toBe(false);
    expect(result.weightedRasterDeferredReasons).toEqual([]);
    expect(pair.kind).toBe('pair');
    if (pair.kind !== 'pair') return;
    expect(pair.execution).toBe('weighted-canvas-deferred');
    expect(pair.weightedRasterSource).toMatchObject({
      supported: true,
      lower: { clipId: 'clip-a', kind: 'image' },
      upper: { clipId: 'clip-b', kind: 'video' },
    });
    expect(pair.pixel.outgoing_weight).toBe(0.6);
    expect(pair.pixel.incoming_weight).toBe(0.4);
    expect(pair.layerPaintByClipId.get('clip-a')).toEqual({
      offsetXFraction: 0,
      offsetYFraction: 0,
      scaleMultiplier: 1,
    });
  });

  it('keeps a weighted pair adjacent beneath a standalone canonical text overlay', () => {
    const title = {
      clip: { id: 'title', text: {} },
      canonicalState: state('title', undefined, {
        text: { contract_version: 'text-state-v1' } as CanonicalFrameLayerState['text'],
      }),
    };
    const result = planPreviewFrameTransitionPairs(30, [
      layer('clip-a', [paint({
        type: 'crossfade',
        composition: TRANSITION_PAINT_CROSSFADE,
        outgoing_weight: 0.6,
        incoming_weight: 0.4,
      })]),
      layer('clip-b', undefined, {}, 'video/mp4'),
      title,
    ]);

    expect(result.mode).toBe('canonical-weighted-deferred');
    expect(result.deferredReasons).toEqual([]);
    expect(result.weightedRasterDeferredReasons).toEqual([]);
    expect(result.slots.map((slot) => slot.kind)).toEqual(['pair', 'single']);
    const overlay = result.slots[1];
    expect(overlay.kind).toBe('single');
    if (overlay.kind !== 'single') return;
    expect(overlay.layer.clip.id).toBe('title');
  });

  it('records consumer-specific weighted raster blockers separately from pair-surface deferrals', () => {
    const result = planPreviewFrameTransitionPairs(31, [
      layer('clip-a', [paint({
        type: 'crossfade',
        composition: TRANSITION_PAINT_CROSSFADE,
        outgoing_weight: 0.6,
        incoming_weight: 0.4,
      })], {
        text: { contract_version: 'text-state-v1' } as CanonicalFrameLayerState['text'],
      }),
      layer('clip-b'),
    ]);
    const pair = result.slots[0];
    expect(result.deferredReasons).toEqual([]);
    expect(result.weightedRasterDeferredReasons).toEqual([
      'transition-1:clip-a:text-raster-deferred',
    ]);
    expect(pair.kind).toBe('pair');
    if (pair.kind !== 'pair') return;
    expect(pair.weightedRasterSource).toMatchObject({ supported: false });
  });

  it('does not regroup a non-adjacent pair around an unrelated canonical layer', () => {
    const result = planPreviewFrameTransitionPairs(40, [
      layer('clip-a', [paint({
        translation_space: TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
        outgoing_offset_x: -0.5,
        outgoing_offset_y: 0,
        incoming_offset_x: 0.5,
        incoming_offset_y: 0,
      })]),
      layer('unrelated'),
      layer('clip-b'),
    ]);
    expect(result.slots.map((slot) => slot.kind)).toEqual(['single', 'single', 'single']);
    expect(result.deferredReasons).toContain('transition-1:pair-inputs-not-adjacent');
    expect(shouldConsumePreviewFrameSourceOverPairs(result)).toBe(false);
  });

  it('keeps a pair frame on fallback when either pair input owns another active paint', () => {
    const result = planPreviewFrameTransitionPairs(50, [
      layer('clip-a', [
        paint({
          translation_space: TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
          outgoing_offset_x: -0.5,
          outgoing_offset_y: 0,
          incoming_offset_x: 0.5,
          incoming_offset_y: 0,
        }),
        paint({
          transition_id: 'fade-1',
          type: 'fade',
          placement: 'in',
          composition: TRANSITION_PAINT_OWNER_ALPHA,
          peer_clip_id: undefined,
          outgoing_clip_id: undefined,
          incoming_clip_id: undefined,
          owner_opacity: 0.5,
        }),
      ]),
      layer('clip-b'),
    ]);
    expect(result.mode).toBe('canonical-source-over');
    expect(shouldConsumePreviewFrameSourceOverPairs(result)).toBe(false);
  });
});
