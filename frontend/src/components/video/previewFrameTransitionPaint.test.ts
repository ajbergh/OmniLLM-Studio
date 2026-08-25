import { describe, expect, it } from 'vitest';
import {
  TRANSITION_PAINT_CONTRACT_V1,
  TRANSITION_PAINT_OWNER_ALPHA,
  TRANSITION_PAINT_OWNER_TRANSLATE,
  TRANSITION_PAINT_OWNER_WIPE,
  TRANSITION_PAINT_OWNER_ZOOM,
  TRANSITION_PAINT_CROSSFADE,
  TRANSITION_PAINT_CLIP_LAYER_FRACTION,
  TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
  TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
  type CanonicalTransitionPaint,
} from '../../video/renderContractTransitionPaint';
import { resolvePreviewFrameOwnerTransitionPaint } from './previewFrameTransitionPaint';

function paint(overrides: Partial<CanonicalTransitionPaint>): CanonicalTransitionPaint {
  return {
    contract_version: TRANSITION_PAINT_CONTRACT_V1,
    transition_id: 'transition-1',
    type: 'fade',
    placement: 'in',
    composition: TRANSITION_PAINT_OWNER_ALPHA,
    owner_clip_id: 'clip-a',
    progress: 0.5,
    ...overrides,
  };
}

function state(transitionPaint?: CanonicalTransitionPaint[]) {
  return { clip_id: 'clip-a', transition_paint: transitionPaint };
}

describe('resolvePreviewFrameOwnerTransitionPaint', () => {
  it('treats canonical omission as authoritative identity paint', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state(), false)).toEqual({
      mode: 'canonical-frame',
      opacityMultiplier: 1,
      offsetXFraction: 0,
      offsetYFraction: 0,
      scaleMultiplier: 1,
    });
  });

  it('consumes owner opacity without changing other presentation terms', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({ owner_opacity: 0.25 }),
    ]), false)).toEqual({
      mode: 'canonical-frame',
      opacityMultiplier: 0.25,
      offsetXFraction: 0,
      offsetYFraction: 0,
      scaleMultiplier: 1,
    });
  });

  it('consumes canvas-fraction owner translation', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({
        composition: TRANSITION_PAINT_OWNER_TRANSLATE,
        type: 'slide',
        owner_offset_x: -0.75,
        owner_offset_y: 0.125,
        translation_space: TRANSITION_PAINT_TRANSLATION_CANVAS_FRACTION,
      }),
    ]), false)).toMatchObject({
      mode: 'canonical-frame',
      offsetXFraction: -0.75,
      offsetYFraction: 0.125,
    });
  });

  it('maps owner wipe fractions to a layer-surface inset', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({
        composition: TRANSITION_PAINT_OWNER_WIPE,
        type: 'wipe',
        clip_space: TRANSITION_PAINT_CLIP_LAYER_FRACTION,
        owner_clip_top: 0,
        owner_clip_right: 0.4,
        owner_clip_bottom: 0,
        owner_clip_left: 0.1,
      }),
    ]), false)).toMatchObject({
      mode: 'canonical-frame',
      clipPath: 'inset(0% 40% 0% 10%)',
    });
  });

  it('consumes owner zoom scale and opacity together', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({
        composition: TRANSITION_PAINT_OWNER_ZOOM,
        type: 'zoom',
        scale_space: TRANSITION_PAINT_SCALE_LAYER_MULTIPLIER,
        owner_opacity: 0.6,
        owner_scale: 0.8,
      }),
    ]), false)).toMatchObject({
      mode: 'canonical-frame',
      opacityMultiplier: 0.6,
      scaleMultiplier: 0.8,
    });
  });

  it('defers pair composition instead of approximating a two-input blend', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({
        composition: TRANSITION_PAINT_CROSSFADE,
        type: 'crossfade',
        placement: 'between',
        peer_clip_id: 'clip-b',
        outgoing_clip_id: 'clip-a',
        incoming_clip_id: 'clip-b',
        outgoing_weight: 0.5,
        incoming_weight: 0.5,
      }),
    ]), false)).toMatchObject({
      mode: 'canonical-deferred',
      deferredComposition: TRANSITION_PAINT_CROSSFADE,
      opacityMultiplier: 1,
    });
  });

  it('keeps live manipulation on the legacy interaction path', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({ owner_opacity: 0.2 }),
    ]), true).mode).toBe('legacy-none');
  });

  it('fails closed when canonical paint is bound to the wrong owner', () => {
    expect(() => resolvePreviewFrameOwnerTransitionPaint(state([
      paint({ owner_clip_id: 'clip-b', owner_opacity: 0.5 }),
    ]), false)).toThrow(/does not match layer/);
  });

  it('defers multiple simultaneous transition paints until ordering semantics are explicit', () => {
    expect(resolvePreviewFrameOwnerTransitionPaint(state([
      paint({ transition_id: 'transition-a', owner_opacity: 0.8 }),
      paint({ transition_id: 'transition-b', owner_opacity: 0.6 }),
    ]), false)).toMatchObject({
      mode: 'canonical-deferred',
      deferredComposition: 'multiple-active-transitions',
    });
  });
});
