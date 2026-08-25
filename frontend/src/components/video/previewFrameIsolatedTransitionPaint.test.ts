import { describe, expect, it } from 'vitest';
import {
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_DIP_BLACK,
  TRANSITION_PAINT_PAIR_SLIDE,
  TRANSITION_PAINT_PAIR_WIPE,
  TRANSITION_PAINT_PAIR_ZOOM,
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import { resolvePreviewFrameIsolatedTransitionPaint } from './previewFrameIsolatedTransitionPaint';

function paint(overrides: Partial<CanonicalTransitionPaint>): CanonicalTransitionPaint {
  return {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: 'transition-1',
    type: 'crossfade',
    placement: 'between',
    composition: TRANSITION_PAINT_CROSSFADE,
    owner_clip_id: 'out',
    peer_clip_id: 'in',
    outgoing_clip_id: 'out',
    incoming_clip_id: 'in',
    outgoing_weight: 0.6,
    incoming_weight: 0.4,
    progress: 0.4,
    ...overrides,
  };
}

function layer(clipId: string, transitionPaint?: CanonicalTransitionPaint[]) {
  return {
    clipId,
    canonicalState: { clip_id: clipId, transition_paint: transitionPaint },
  };
}

describe('resolvePreviewFrameIsolatedTransitionPaint', () => {
  it('builds an additive isolated crossfade for adjacent canonical pair members', () => {
    const resolved = resolvePreviewFrameIsolatedTransitionPaint([
      layer('under'),
      layer('out', [paint({})]),
      layer('in'),
      layer('over'),
    ]);
    expect(resolved).toMatchObject({
      mode: 'canonical-isolated',
      insertionIndex: 2,
      blackWeight: 0,
      layers: [
        { clipId: 'out', opacityMultiplier: 0.6, additive: true },
        { clipId: 'in', opacityMultiplier: 0.4, additive: true },
      ],
    });
  });

  it('defers pair paint when collapsing the pair would reorder an unrelated layer', () => {
    expect(resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({})]),
      layer('middle'),
      layer('in'),
    ])).toMatchObject({
      mode: 'canonical-deferred',
      deferredReason: 'pair-members-not-adjacent',
    });
  });

  it('maps pair slide canvas-fraction offsets without blend weights', () => {
    const resolved = resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({
        type: 'slide',
        composition: TRANSITION_PAINT_PAIR_SLIDE,
        translation_space: TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
        outgoing_offset_x: -0.25,
        outgoing_offset_y: 0,
        incoming_offset_x: 0.75,
        incoming_offset_y: 0,
        outgoing_weight: undefined,
        incoming_weight: undefined,
      })]),
      layer('in'),
    ]);
    expect(resolved.layers).toEqual([
      expect.objectContaining({ clipId: 'out', offsetXFraction: -0.25, additive: false }),
      expect.objectContaining({ clipId: 'in', offsetXFraction: 0.75, additive: false }),
    ]);
  });

  it('maps pair wipe to the incoming isolated layer surface', () => {
    const resolved = resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({
        type: 'wipe',
        composition: TRANSITION_PAINT_PAIR_WIPE,
        clip_space: TRANSITION_PAINT_CLIP_LAYER_FRACTION,
        incoming_clip_top: 0,
        incoming_clip_right: 0.5,
        incoming_clip_bottom: 0,
        incoming_clip_left: 0,
        outgoing_weight: undefined,
        incoming_weight: undefined,
      })]),
      layer('in'),
    ]);
    expect(resolved.layers[1]).toMatchObject({ clipId: 'in', clipPath: 'inset(0% 50% 0% 0%)' });
  });

  it('preserves pair zoom weights and layer-scale multipliers', () => {
    const resolved = resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({
        type: 'zoom',
        composition: TRANSITION_PAINT_PAIR_ZOOM,
        scale_space: TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
        outgoing_weight: 0.3,
        incoming_weight: 0.7,
        outgoing_scale: 0.9,
        incoming_scale: 0.95,
      })]),
      layer('in'),
    ]);
    expect(resolved.layers).toEqual([
      expect.objectContaining({ clipId: 'out', opacityMultiplier: 0.3, scaleMultiplier: 0.9, additive: true }),
      expect.objectContaining({ clipId: 'in', opacityMultiplier: 0.7, scaleMultiplier: 0.95, additive: true }),
    ]);
  });

  it('keeps canonical black contribution explicit for between dip-to-black', () => {
    const resolved = resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({
        type: 'dip_to_black',
        composition: TRANSITION_PAINT_DIP_BLACK,
        outgoing_weight: 0.25,
        incoming_weight: 0,
        black_weight: 0.75,
      })]),
      layer('in'),
    ]);
    expect(resolved).toMatchObject({
      mode: 'canonical-isolated',
      blackWeight: 0.75,
      layers: [
        { clipId: 'out', opacityMultiplier: 0.25, additive: true },
        { clipId: 'in', opacityMultiplier: 0, additive: true },
      ],
    });
  });

  it('resolves one-sided dip-to-black at the owner stacking position', () => {
    const oneSided = paint({
      type: 'dip_to_black',
      placement: 'out',
      composition: TRANSITION_PAINT_DIP_BLACK,
      peer_clip_id: undefined,
      outgoing_clip_id: 'out',
      incoming_clip_id: undefined,
      outgoing_weight: 0.2,
      incoming_weight: undefined,
      black_weight: 0.8,
    });
    expect(resolvePreviewFrameIsolatedTransitionPaint([
      layer('under'),
      layer('out', [oneSided]),
      layer('over'),
    ])).toMatchObject({
      mode: 'canonical-isolated',
      insertionIndex: 1,
      blackWeight: 0.8,
      layers: [{ clipId: 'out', opacityMultiplier: 0.2, additive: true }],
    });
  });

  it('fails closed when arithmetic blend contributions do not sum to one', () => {
    expect(() => resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({ outgoing_weight: 0.7, incoming_weight: 0.5 })]),
      layer('in'),
    ])).toThrow(/weights must sum to 1/);
  });

  it('defers more than one isolated transition in the same frame', () => {
    const second = paint({ transition_id: 'transition-2', owner_clip_id: 'other-out', peer_clip_id: 'other-in', outgoing_clip_id: 'other-out', incoming_clip_id: 'other-in' });
    expect(resolvePreviewFrameIsolatedTransitionPaint([
      layer('out', [paint({})]),
      layer('in'),
      layer('other-out', [second]),
      layer('other-in'),
    ])).toMatchObject({ mode: 'canonical-deferred', deferredReason: 'multiple-isolated-transitions' });
  });
});
