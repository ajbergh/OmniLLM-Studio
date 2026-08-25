import { describe, expect, it } from 'vitest';
import { composeWeightedTransitionPairRgba } from '../src/video/renderContractTransitionPairPixelKernel';
import {
  TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED,
  TRANSITION_PAIR_PIXEL_BLACK_OPAQUE,
  TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK,
  TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM,
  TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER,
  TRANSITION_PAIR_PIXEL_COMPOSITION_V1,
  TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT,
  TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB,
  TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT,
  TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB,
  TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1,
  TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB,
  type CanonicalTransitionPairPixelComposition,
} from '../src/video/renderContractTransitionPairPixels';

describe('weighted transition pair pixel kernel', () => {
  it('blends opaque crossfade pixels in linear sRGB instead of encoded sRGB', () => {
    const result = composeWeightedTransitionPairRgba(
      weightedComposition('pair-crossfade', 0.5, 0.5),
      rgba(255, 0, 0, 255),
      rgba(0, 255, 0, 255),
    );

    expect([...result]).toEqual([188, 188, 0, 255]);
  });

  it('uses premultiplied accumulation and recovers straight output alpha', () => {
    const result = composeWeightedTransitionPairRgba(
      weightedComposition('pair-crossfade', 0.5, 0.5),
      rgba(255, 0, 0, 128),
      rgba(0, 0, 255, 255),
    );

    expect([...result]).toEqual([156, 0, 213, 192]);
  });

  it('adds dip-to-black as opaque linear black without RGB energy', () => {
    const result = composeWeightedTransitionPairRgba(
      weightedComposition('dip-to-black', 0.25, 0.25, 0.5),
      rgba(255, 255, 255, 255),
      rgba(255, 255, 255, 255),
    );

    expect([...result]).toEqual([188, 188, 188, 255]);
  });

  it('returns transparent black when all weighted source alpha is zero', () => {
    const result = composeWeightedTransitionPairRgba(
      weightedComposition('pair-zoom', 0.75, 0.25),
      rgba(255, 0, 0, 0),
      rgba(0, 255, 0, 0),
    );

    expect([...result]).toEqual([0, 0, 0, 0]);
  });

  it('writes every RGBA pixel into a caller-provided Canvas-compatible target', () => {
    const target = new Uint8ClampedArray(8);
    const result = composeWeightedTransitionPairRgba(
      weightedComposition('pair-crossfade', 0.5, 0.5),
      new Uint8ClampedArray([255, 0, 0, 255, 0, 0, 0, 0]),
      new Uint8ClampedArray([0, 255, 0, 255, 0, 0, 255, 255]),
      target,
    );

    expect(result).toBe(target);
    expect([...result]).toEqual([188, 188, 0, 255, 0, 0, 255, 128]);
  });

  it('rejects source-over pair contracts instead of approximating them as weighted pixels', () => {
    const composition = weightedComposition('pair-crossfade', 0.5, 0.5);
    composition.blend_operator = TRANSITION_PAIR_PIXEL_BLEND_SOURCE_OVER_STACK;

    expect(() => composeWeightedTransitionPairRgba(
      composition,
      rgba(255, 0, 0, 255),
      rgba(0, 0, 255, 255),
    )).toThrow(/requires weighted-sum composition/);
  });

  it('rejects color or alpha contract drift', () => {
    const composition = weightedComposition('pair-crossfade', 0.5, 0.5);
    (composition as unknown as { working_color_space: string }).working_color_space = 'srgb';

    expect(() => composeWeightedTransitionPairRgba(
      composition,
      rgba(255, 0, 0, 255),
      rgba(0, 0, 255, 255),
    )).toThrow(/requires the exact v1 color\/alpha contract/);
  });

  it('rejects malformed dip-to-black metadata and non-unit weights', () => {
    const missingBlackSource = weightedComposition('dip-to-black', 0.25, 0.25, 0.5);
    delete missingBlackSource.black_source;
    expect(() => composeWeightedTransitionPairRgba(
      missingBlackSource,
      rgba(255, 255, 255, 255),
      rgba(255, 255, 255, 255),
    )).toThrow(/requires opaque linear black/);

    const badSum = weightedComposition('pair-zoom', 0.8, 0.3);
    expect(() => composeWeightedTransitionPairRgba(
      badSum,
      rgba(255, 255, 255, 255),
      rgba(255, 255, 255, 255),
    )).toThrow(/pair weights must sum to 1/);
  });

  it('rejects empty, non-RGBA, and unequal byte buffers', () => {
    const composition = weightedComposition('pair-crossfade', 0.5, 0.5);

    expect(() => composeWeightedTransitionPairRgba(
      composition,
      new Uint8ClampedArray(),
      new Uint8ClampedArray(),
    )).toThrow(/non-empty RGBA byte buffers/);

    expect(() => composeWeightedTransitionPairRgba(
      composition,
      new Uint8ClampedArray(5),
      new Uint8ClampedArray(5),
    )).toThrow(/non-empty RGBA byte buffers/);

    expect(() => composeWeightedTransitionPairRgba(
      composition,
      new Uint8ClampedArray(4),
      new Uint8ClampedArray(8),
    )).toThrow(/equal-sized outgoing, incoming, and target buffers/);
  });
});

function rgba(red: number, green: number, blue: number, alpha: number): Uint8ClampedArray {
  return new Uint8ClampedArray([red, green, blue, alpha]);
}

function weightedComposition(
  composition: 'pair-crossfade' | 'pair-zoom' | 'dip-to-black',
  outgoingWeight: number,
  incomingWeight: number,
  blackWeight?: number,
): CanonicalTransitionPairPixelComposition {
  return {
    contract_version: TRANSITION_PAIR_PIXEL_COMPOSITION_V1,
    transition_id: 'transition',
    composition,
    input_color_encoding: TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB,
    working_color_space: TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB,
    output_color_encoding: TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB,
    transfer_function: TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1,
    input_alpha: TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT,
    accumulator_alpha: TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED,
    output_alpha: TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT,
    clamp_policy: TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER,
    blend_operator: TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM,
    lower_clip_id: 'outgoing',
    upper_clip_id: 'incoming',
    outgoing_clip_id: 'outgoing',
    incoming_clip_id: 'incoming',
    outgoing_weight: outgoingWeight,
    incoming_weight: incomingWeight,
    ...(blackWeight === undefined ? {} : {
      black_weight: blackWeight,
      black_source: TRANSITION_PAIR_PIXEL_BLACK_OPAQUE,
    }),
  };
}
