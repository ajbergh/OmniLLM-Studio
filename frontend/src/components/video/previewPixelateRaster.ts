export const PREVIEW_PIXELATE_RASTER_V1 = 'preview-pixelate-raster-v1' as const;

export interface PreviewPixelateRasterPlan {
  contract_version: typeof PREVIEW_PIXELATE_RASTER_V1;
  width: number;
  height: number;
  block_size: number;
  downsample_width: number;
  downsample_height: number;
}

/**
 * Resolve the browser pixelate raster grid in canonical canvas pixels.
 *
 * The block/downsample dimension policy intentionally mirrors the current
 * FFmpeg region path: blur_radius is rounded to an integer, clamped to at
 * least two pixels for pixelate, and the reduced surface uses integer floor
 * division with a one-pixel minimum. This contract owns only raster-grid
 * geometry; backdrop acquisition/composition remains a separate consumer gate.
 */
export function resolvePreviewPixelateRasterPlan(
  width: number,
  height: number,
  blurRadius: number,
): PreviewPixelateRasterPlan {
  requirePositiveInteger('width', width);
  requirePositiveInteger('height', height);
  if (!Number.isFinite(blurRadius) || blurRadius < 0) {
    throw new Error('preview pixelate blur radius must be finite and non-negative');
  }

  const blockSize = Math.max(2, Math.round(blurRadius));
  return {
    contract_version: PREVIEW_PIXELATE_RASTER_V1,
    width,
    height,
    block_size: blockSize,
    downsample_width: Math.max(1, Math.floor(width / blockSize)),
    downsample_height: Math.max(1, Math.floor(height / blockSize)),
  };
}

/**
 * Pixelate one straight-alpha RGBA byte buffer using a deterministic two-pass
 * nearest-neighbor kernel. The first pass samples the center-mapped source
 * pixel for every reduced pixel; the second pass expands the reduced surface
 * back to the authored region dimensions with nearest-neighbor lookup.
 *
 * This function deliberately does not read Canvas, DOM, media elements, CSS,
 * or renderer state. It is suitable for exact unit fixtures and for a future
 * consumer that can provide the already-composited backdrop raster.
 */
export function pixelatePreviewRgba(
  plan: PreviewPixelateRasterPlan,
  input: Uint8ClampedArray,
  target: Uint8ClampedArray = new Uint8ClampedArray(input.length),
): Uint8ClampedArray {
  validatePlan(plan);
  const expectedLength = plan.width * plan.height * 4;
  if (input.length !== expectedLength || target.length !== expectedLength) {
    throw new Error('preview pixelate raster requires exact width × height RGBA byte buffers');
  }

  const reduced = new Uint8ClampedArray(plan.downsample_width * plan.downsample_height * 4);
  for (let y = 0; y < plan.downsample_height; y += 1) {
    const sourceY = centerMappedIndex(y, plan.downsample_height, plan.height);
    for (let x = 0; x < plan.downsample_width; x += 1) {
      const sourceX = centerMappedIndex(x, plan.downsample_width, plan.width);
      copyPixel(input, ((sourceY * plan.width) + sourceX) * 4, reduced, ((y * plan.downsample_width) + x) * 4);
    }
  }

  for (let y = 0; y < plan.height; y += 1) {
    const reducedY = Math.min(
      plan.downsample_height - 1,
      Math.floor((y * plan.downsample_height) / plan.height),
    );
    for (let x = 0; x < plan.width; x += 1) {
      const reducedX = Math.min(
        plan.downsample_width - 1,
        Math.floor((x * plan.downsample_width) / plan.width),
      );
      copyPixel(
        reduced,
        ((reducedY * plan.downsample_width) + reducedX) * 4,
        target,
        ((y * plan.width) + x) * 4,
      );
    }
  }

  return target;
}

function validatePlan(plan: PreviewPixelateRasterPlan): void {
  if (plan.contract_version !== PREVIEW_PIXELATE_RASTER_V1) {
    throw new Error(`preview pixelate raster requires ${PREVIEW_PIXELATE_RASTER_V1}`);
  }
  requirePositiveInteger('width', plan.width);
  requirePositiveInteger('height', plan.height);
  requirePositiveInteger('block_size', plan.block_size);
  requirePositiveInteger('downsample_width', plan.downsample_width);
  requirePositiveInteger('downsample_height', plan.downsample_height);
  if (plan.block_size < 2) throw new Error('preview pixelate block size must be at least two pixels');
  if (plan.downsample_width !== Math.max(1, Math.floor(plan.width / plan.block_size))
    || plan.downsample_height !== Math.max(1, Math.floor(plan.height / plan.block_size))) {
    throw new Error('preview pixelate downsample dimensions do not match the v1 grid policy');
  }
}

function requirePositiveInteger(name: string, value: number): void {
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`preview pixelate ${name} must be a positive integer`);
  }
}

function centerMappedIndex(index: number, outputSize: number, inputSize: number): number {
  return Math.min(inputSize - 1, Math.floor(((index + 0.5) * inputSize) / outputSize));
}

function copyPixel(
  source: Uint8ClampedArray,
  sourceIndex: number,
  target: Uint8ClampedArray,
  targetIndex: number,
): void {
  target[targetIndex] = source[sourceIndex];
  target[targetIndex + 1] = source[sourceIndex + 1];
  target[targetIndex + 2] = source[sourceIndex + 2];
  target[targetIndex + 3] = source[sourceIndex + 3];
}
