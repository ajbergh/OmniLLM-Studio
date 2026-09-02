import {
  TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED,
  TRANSITION_PAIR_PIXEL_BLACK_OPAQUE,
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
} from './renderContractTransitionPairPixels';

const SRGB_DECODE_THRESHOLD = 0.04045;
const SRGB_ENCODE_THRESHOLD = 0.0031308;
const UNIT_SUM_EPSILON = 1e-9;
const LINEAR_TO_SRGB_BUCKET_COUNT = 4096;
const LINEAR_TO_SRGB_REFERENCE_REQUIRED = -1;

const SRGB_BYTE_TO_LINEAR = new Float64Array(256);
for (let value = 0; value < SRGB_BYTE_TO_LINEAR.length; value += 1) {
  const srgb = value / 255;
  SRGB_BYTE_TO_LINEAR[value] = srgb <= SRGB_DECODE_THRESHOLD
    ? srgb / 12.92
    : ((srgb + 0.055) / 1.055) ** 2.4;
}

/**
 * Exact-output acceleration for the inverse transfer function.
 *
 * The kernel ultimately writes into Uint8ClampedArray, so many contiguous
 * linear-light values quantize to the same byte. Cache only buckets whose
 * complete half-open interval provably maps to one identical byte. The 255
 * buckets that straddle an 8-bit quantization boundary stay marked with the
 * sentinel and execute the original transfer-function math at runtime. This
 * removes the millions of exponentiations on ordinary 1080p weighted frames
 * without approximating or changing any byte at a quantization boundary.
 */
const LINEAR_TO_SRGB_BYTE = new Int16Array(LINEAR_TO_SRGB_BUCKET_COUNT);
LINEAR_TO_SRGB_BYTE.fill(LINEAR_TO_SRGB_REFERENCE_REQUIRED);
for (let bucket = 0; bucket < LINEAR_TO_SRGB_BUCKET_COUNT; bucket += 1) {
  const lower = bucket / LINEAR_TO_SRGB_BUCKET_COUNT;
  const upperExclusive = (bucket + 1) / LINEAR_TO_SRGB_BUCKET_COUNT;
  const upperInside = bucket === LINEAR_TO_SRGB_BUCKET_COUNT - 1
    ? 1
    : upperExclusive - Number.EPSILON;
  const lowerByte = toUint8Clamp(encodeLinearSrgbToByteReference(lower));
  const upperByte = toUint8Clamp(encodeLinearSrgbToByteReference(upperInside));
  if (lowerByte === upperByte) LINEAR_TO_SRGB_BYTE[bucket] = lowerByte;
}

/**
 * Compose two straight-alpha sRGB byte buffers using the exact weighted pair
 * semantics declared by transition-pair-pixel-composition-v1.
 *
 * The inputs and returned buffer use Canvas ImageData-compatible RGBA bytes.
 * RGB is decoded to linear sRGB, accumulated in premultiplied form with the
 * canonical weights applied exactly once, converted back to straight alpha,
 * clamped to the unit interval, and encoded to sRGB. Between dip-to-black adds
 * an opaque linear-black alpha contribution without adding RGB energy.
 */
export function composeWeightedTransitionPairRgba(
  composition: CanonicalTransitionPairPixelComposition,
  outgoing: Uint8ClampedArray,
  incoming: Uint8ClampedArray,
  target: Uint8ClampedArray = new Uint8ClampedArray(outgoing.length),
): Uint8ClampedArray {
  const weights = resolveWeightedTransitionPairKernelWeights(composition);
  requireCompatibleRgbaBuffers(outgoing, incoming, target);

  for (let index = 0; index < outgoing.length; index += 4) {
    const outgoingAlpha = outgoing[index + 3] / 255;
    const incomingAlpha = incoming[index + 3] / 255;

    const outputAlpha = clampUnit(
      (weights.outgoing * outgoingAlpha)
      + (weights.incoming * incomingAlpha)
      + weights.black,
    );

    if (outputAlpha === 0) {
      target[index] = 0;
      target[index + 1] = 0;
      target[index + 2] = 0;
      target[index + 3] = 0;
      continue;
    }

    const outgoingPremultipliedWeight = weights.outgoing * outgoingAlpha;
    const incomingPremultipliedWeight = weights.incoming * incomingAlpha;
    const inverseOutputAlpha = 1 / outputAlpha;
    for (let channel = 0; channel < 3; channel += 1) {
      const premultipliedLinear =
        (outgoingPremultipliedWeight * SRGB_BYTE_TO_LINEAR[outgoing[index + channel]])
        + (incomingPremultipliedWeight * SRGB_BYTE_TO_LINEAR[incoming[index + channel]]);
      const straightLinear = clampUnit(premultipliedLinear * inverseOutputAlpha);
      target[index + channel] = encodeLinearSrgbToByte(straightLinear);
    }
    target[index + 3] = outputAlpha * 255;
  }

  return target;
}

export interface WeightedPairKernelWeights {
  outgoing: number;
  incoming: number;
  black: number;
}

/** Validate the v1 weighted-pair contract and return its normalized weights. */
export function resolveWeightedTransitionPairKernelWeights(
  composition: CanonicalTransitionPairPixelComposition,
): WeightedPairKernelWeights {
  if (composition.contract_version !== TRANSITION_PAIR_PIXEL_COMPOSITION_V1
    || composition.input_color_encoding !== TRANSITION_PAIR_PIXEL_INPUT_COLOR_SRGB
    || composition.working_color_space !== TRANSITION_PAIR_PIXEL_WORKING_COLOR_LINEAR_SRGB
    || composition.output_color_encoding !== TRANSITION_PAIR_PIXEL_OUTPUT_COLOR_SRGB
    || composition.transfer_function !== TRANSITION_PAIR_PIXEL_TRANSFER_SRGB_IEC_61966_2_1
    || composition.input_alpha !== TRANSITION_PAIR_PIXEL_INPUT_ALPHA_STRAIGHT
    || composition.accumulator_alpha !== TRANSITION_PAIR_PIXEL_ACCUMULATOR_PREMULTIPLIED
    || composition.output_alpha !== TRANSITION_PAIR_PIXEL_OUTPUT_ALPHA_STRAIGHT
    || composition.clamp_policy !== TRANSITION_PAIR_PIXEL_CLAMP_UNIT_BEFORE_OUTPUT_TRANSFER) {
    throw new Error('weighted transition pair pixel kernel requires the exact v1 color/alpha contract');
  }
  if (composition.blend_operator !== TRANSITION_PAIR_PIXEL_BLEND_WEIGHTED_SUM) {
    throw new Error('weighted transition pair pixel kernel requires weighted-sum composition');
  }

  const outgoing = requireUnitWeight(composition.transition_id, 'outgoing', composition.outgoing_weight);
  const incoming = requireUnitWeight(composition.transition_id, 'incoming', composition.incoming_weight);

  switch (composition.composition) {
    case 'pair-crossfade':
    case 'pair-zoom': {
      if (composition.black_weight !== undefined || composition.black_source !== undefined) {
        throw new Error(`transition ${JSON.stringify(composition.transition_id)} weighted pair must not carry black contribution`);
      }
      if (!unitSum(outgoing + incoming)) {
        throw new Error(`transition ${JSON.stringify(composition.transition_id)} pair weights must sum to 1`);
      }
      return { outgoing, incoming, black: 0 };
    }

    case 'dip-to-black': {
      const black = requireUnitWeight(composition.transition_id, 'black', composition.black_weight);
      if (composition.black_source !== TRANSITION_PAIR_PIXEL_BLACK_OPAQUE) {
        throw new Error(`transition ${JSON.stringify(composition.transition_id)} dip-to-black requires opaque linear black`);
      }
      if (!unitSum(outgoing + incoming + black)) {
        throw new Error(`transition ${JSON.stringify(composition.transition_id)} dip-to-black weights must sum to 1`);
      }
      return { outgoing, incoming, black };
    }

    default:
      throw new Error(`transition ${JSON.stringify(composition.transition_id)} composition ${JSON.stringify(composition.composition)} is not a weighted pair kernel family`);
  }
}

function requireCompatibleRgbaBuffers(
  outgoing: Uint8ClampedArray,
  incoming: Uint8ClampedArray,
  target: Uint8ClampedArray,
): void {
  if (outgoing.length === 0 || outgoing.length % 4 !== 0) {
    throw new Error('weighted transition pair pixel kernel requires non-empty RGBA byte buffers');
  }
  if (incoming.length !== outgoing.length || target.length !== outgoing.length) {
    throw new Error('weighted transition pair pixel kernel requires equal-sized outgoing, incoming, and target buffers');
  }
}

function requireUnitWeight(transitionId: string, label: string, value: number | undefined): number {
  if (value === undefined || !Number.isFinite(value) || value < 0 || value > 1) {
    throw new Error(`transition ${JSON.stringify(transitionId)} ${label} weight must be finite and within [0,1]`);
  }
  return value;
}

function encodeLinearSrgbToByte(linear: number): number {
  if (linear <= 0) return 0;
  if (linear >= 1) return 255;
  const bucket = Math.min(
    LINEAR_TO_SRGB_BUCKET_COUNT - 1,
    Math.floor(linear * LINEAR_TO_SRGB_BUCKET_COUNT),
  );
  const cached = LINEAR_TO_SRGB_BYTE[bucket];
  return cached === LINEAR_TO_SRGB_REFERENCE_REQUIRED
    ? encodeLinearSrgbToByteReference(linear)
    : cached;
}

function encodeLinearSrgbToByteReference(linear: number): number {
  const srgb = linear <= SRGB_ENCODE_THRESHOLD
    ? 12.92 * linear
    : (1.055 * (linear ** (1 / 2.4))) - 0.055;
  return clampUnit(srgb) * 255;
}

/** ECMAScript ToUint8Clamp, used only while proving cache buckets at startup. */
function toUint8Clamp(value: number): number {
  if (value <= 0 || Number.isNaN(value)) return 0;
  if (value >= 255) return 255;
  const floor = Math.floor(value);
  const fraction = value - floor;
  if (fraction < 0.5) return floor;
  if (fraction > 0.5) return floor + 1;
  return floor % 2 === 0 ? floor : floor + 1;
}

function clampUnit(value: number): number {
  if (value <= 0) return 0;
  if (value >= 1) return 1;
  return value;
}

function unitSum(value: number): boolean {
  return Math.abs(value - 1) <= UNIT_SUM_EPSILON;
}
