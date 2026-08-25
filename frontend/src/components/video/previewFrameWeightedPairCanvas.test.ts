import { describe, expect, it, vi } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../../video/renderContractMediaGeometry';
import {
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_PAIR_ZOOM,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import { planPreviewFrameTransitionPairs } from './previewFrameTransitionPairs';
import {
  paintPreviewWeightedPairCanvasLayer,
  resolvePreviewWeightedPairCanvasLayerPlan,
  shouldConsumePreviewFrameWeightedPairs,
  weightedPairCanvasClipIds,
} from './previewFrameWeightedPairCanvas';

function state(clipId: string, transitionPaint?: CanonicalTransitionPaint[], overrides: Partial<CanonicalFrameLayerState> = {}): CanonicalFrameLayerState {
  return {
    clip_id: clipId,
    authoritative: true,
    unresolved: [],
    transition_paint: transitionPaint,
    media_geometry: {
      contract_version: MEDIA_GEOMETRY_CONTRACT_V1,
      fit: 'contain',
      viewport_bounds: { x: 0, y: 0, width: 640, height: 360 },
      source_bounds: { x: 0, y: 0, width: 1000, height: 500 },
      visible_source_bounds: { x: 100, y: 50, width: 800, height: 400 },
      painted_bounds: { x: 0, y: 20, width: 640, height: 320 },
      clip_bounds: { x: 10, y: 30, width: 620, height: 300 },
      scale_x: 0.8,
      scale_y: 0.8,
    },
    transform: {
      x: 12,
      y: -7,
      z: 0,
      scale_x: 1.25,
      scale_y: 0.75,
      rotation_x: 0,
      rotation_y: 0,
      rotation_z: 30,
      opacity: 0.6,
      anchor_x: 20,
      anchor_y: -10,
    },
    view_transform: {
      x: 8,
      y: -5,
      z: 0,
      scale_x: 1.25,
      scale_y: 0.75,
      rotation_x: 0,
      rotation_y: 0,
      rotation_z: 30,
      opacity: 0.6,
      anchor_x: 20,
      anchor_y: -10,
    },
    ...overrides,
  } as CanonicalFrameLayerState;
}

function paint(overrides: Partial<CanonicalTransitionPaint> = {}): CanonicalTransitionPaint {
  return {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: 'transition-1',
    type: 'crossfade',
    placement: 'between',
    composition: TRANSITION_PAINT_CROSSFADE,
    owner_clip_id: 'clip-a',
    peer_clip_id: 'clip-b',
    progress: 0.4,
    outgoing_clip_id: 'clip-a',
    incoming_clip_id: 'clip-b',
    outgoing_weight: 0.6,
    incoming_weight: 0.4,
    ...overrides,
  };
}

function layer(clipId: string, transitionPaint?: CanonicalTransitionPaint[], overrides: Partial<CanonicalFrameLayerState> = {}, mimeType = 'image/png') {
  return {
    clip: { id: clipId },
    asset: { mime_type: mimeType },
    canonicalState: state(clipId, transitionPaint, overrides),
  };
}

function weightedPlan(p: CanonicalTransitionPaint = paint()) {
  return planPreviewFrameTransitionPairs(12, [
    layer('clip-a', [p]),
    layer('clip-b', undefined, {}, 'video/mp4'),
  ]);
}

describe('weighted pair Canvas admission', () => {
  it('admits only a clean all-weighted raster-capable plan', () => {
    const plan = weightedPlan();
    expect(shouldConsumePreviewFrameWeightedPairs(plan)).toBe(true);
    expect(weightedPairCanvasClipIds(plan)).toEqual(['clip-a', 'clip-b']);

    const blocked = planPreviewFrameTransitionPairs(12, [
      layer('clip-a', [paint()], {
        effects: [{ contract_version: 'effect-state-v1' } as CanonicalFrameLayerState['effects'][number]],
      }),
      layer('clip-b'),
    ]);
    expect(shouldConsumePreviewFrameWeightedPairs(blocked)).toBe(false);
  });

  it('fails closed when the pair owner has another active paint', () => {
    const extra = paint({
      transition_id: 'transition-extra',
      outgoing_weight: 0.5,
      incoming_weight: 0.5,
    });
    const plan = planPreviewFrameTransitionPairs(12, [
      layer('clip-a', [paint(), extra]),
      layer('clip-b'),
    ]);
    expect(shouldConsumePreviewFrameWeightedPairs(plan)).toBe(false);
  });
});

describe('resolvePreviewWeightedPairCanvasLayerPlan', () => {
  it('maps canonical visible source crop into intrinsic pixels and preserves canonical 2D paint', () => {
    const plan = weightedPlan();
    const slot = plan.slots[0];
    expect(slot.kind).toBe('pair');
    if (slot.kind !== 'pair') return;
    const resolved = resolvePreviewWeightedPairCanvasLayerPlan(slot, slot.lower, 2000, 1000);
    expect(resolved.source).toEqual({ x: 200, y: 100, width: 1600, height: 800 });
    expect(resolved.destination).toEqual({ x: 0, y: 20, width: 640, height: 320 });
    expect(resolved.clip).toEqual({ x: 10, y: 30, width: 620, height: 300 });
    expect(resolved.originX).toBe(340);
    expect(resolved.originY).toBe(170);
    expect(resolved.translateX).toBe(8);
    expect(resolved.translateY).toBe(-5);
    expect(resolved.rotationRadians).toBeCloseTo(Math.PI / 6);
    expect(resolved.scaleX).toBe(1.25);
    expect(resolved.scaleY).toBe(0.75);
    expect(resolved.opacity).toBe(0.6);
  });

  it('applies pair-zoom spatial scale exactly once but never pair opacity weights', () => {
    const zoom = paint({
      type: 'zoom',
      composition: TRANSITION_PAINT_PAIR_ZOOM,
      scale_space: TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
      outgoing_scale: 0.9,
      incoming_scale: 0.85,
    });
    const plan = weightedPlan(zoom);
    const slot = plan.slots[0];
    if (slot.kind !== 'pair') throw new Error('expected pair slot');
    const outgoing = resolvePreviewWeightedPairCanvasLayerPlan(slot, slot.lower, 1000, 500);
    const incoming = resolvePreviewWeightedPairCanvasLayerPlan(slot, slot.upper, 1000, 500);
    expect(outgoing.scaleX).toBeCloseTo(1.25 * 0.9);
    expect(outgoing.scaleY).toBeCloseTo(0.75 * 0.9);
    expect(incoming.scaleX).toBeCloseTo(1.25 * 0.85);
    expect(incoming.opacity).toBe(0.6);
  });

  it('rejects invalid decoded dimensions', () => {
    const plan = weightedPlan();
    const slot = plan.slots[0];
    if (slot.kind !== 'pair') throw new Error('expected pair slot');
    expect(() => resolvePreviewWeightedPairCanvasLayerPlan(slot, slot.lower, 0, 1000))
      .toThrow('positive intrinsic source dimensions');
  });
});

describe('paintPreviewWeightedPairCanvasLayer', () => {
  it('isolates opacity/transform/clip before drawing the source', () => {
    const calls: Array<[string, ...number[]]> = [];
    const context = {
      globalAlpha: 1,
      save: vi.fn(() => calls.push(['save'])),
      restore: vi.fn(() => calls.push(['restore'])),
      translate: vi.fn((x: number, y: number) => calls.push(['translate', x, y])),
      rotate: vi.fn((value: number) => calls.push(['rotate', value])),
      scale: vi.fn((x: number, y: number) => calls.push(['scale', x, y])),
      beginPath: vi.fn(() => calls.push(['beginPath'])),
      rect: vi.fn((x: number, y: number, width: number, height: number) => calls.push(['rect', x, y, width, height])),
      clip: vi.fn(() => calls.push(['clip'])),
      drawImage: vi.fn((...args: unknown[]) => calls.push(['drawImage', ...(args.slice(1) as number[])])),
    } as unknown as CanvasRenderingContext2D;
    paintPreviewWeightedPairCanvasLayer(context, {} as CanvasImageSource, {
      source: { x: 1, y: 2, width: 3, height: 4 },
      destination: { x: 5, y: 6, width: 7, height: 8 },
      clip: { x: 9, y: 10, width: 11, height: 12 },
      originX: 20,
      originY: 30,
      translateX: 4,
      translateY: -2,
      rotationRadians: 0.5,
      scaleX: 1.2,
      scaleY: 0.8,
      opacity: 0.4,
    });
    expect(context.globalAlpha).toBe(0.4);
    expect(calls).toEqual([
      ['save'],
      ['translate', 4, -2],
      ['translate', 20, 30],
      ['rotate', 0.5],
      ['scale', 1.2, 0.8],
      ['translate', -20, -30],
      ['beginPath'],
      ['rect', 9, 10, 11, 12],
      ['clip'],
      ['drawImage', 1, 2, 3, 4, 5, 6, 7, 8],
      ['restore'],
    ]);
  });
});
