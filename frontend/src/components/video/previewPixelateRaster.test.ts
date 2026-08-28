import { describe, expect, it } from 'vitest';
import {
  PREVIEW_PIXELATE_RASTER_V1,
  pixelatePreviewRgba,
  resolvePreviewPixelateRasterPlan,
} from './previewPixelateRaster';

function grayscaleRgba(values: number[]): Uint8ClampedArray {
  const bytes = new Uint8ClampedArray(values.length * 4);
  values.forEach((value, index) => {
    const offset = index * 4;
    bytes[offset] = value;
    bytes[offset + 1] = value;
    bytes[offset + 2] = value;
    bytes[offset + 3] = 255;
  });
  return bytes;
}

function redValues(bytes: Uint8ClampedArray): number[] {
  const result: number[] = [];
  for (let index = 0; index < bytes.length; index += 4) result.push(bytes[index]);
  return result;
}

describe('preview-pixelate-raster-v1', () => {
  it('mirrors the renderer block/downsample dimension policy in canonical pixels', () => {
    expect(resolvePreviewPixelateRasterPlan(320, 181, 11.6)).toEqual({
      contract_version: PREVIEW_PIXELATE_RASTER_V1,
      width: 320,
      height: 181,
      block_size: 12,
      downsample_width: 26,
      downsample_height: 15,
    });
    expect(resolvePreviewPixelateRasterPlan(3, 2, 0)).toMatchObject({
      block_size: 2,
      downsample_width: 1,
      downsample_height: 1,
    });
  });

  it('uses the FFmpeg nearest-neighbor sample map for aligned dimensions', () => {
    const plan = resolvePreviewPixelateRasterPlan(4, 4, 2);
    const input = grayscaleRgba([
      0, 1, 2, 3,
      4, 5, 6, 7,
      8, 9, 10, 11,
      12, 13, 14, 15,
    ]);
    expect(redValues(pixelatePreviewRgba(plan, input))).toEqual([
      5, 5, 7, 7,
      5, 5, 7, 7,
      13, 13, 15, 15,
      13, 13, 15, 15,
    ]);
  });

  it('keeps edge distribution deterministic when dimensions are not block-aligned', () => {
    const plan = resolvePreviewPixelateRasterPlan(5, 3, 2);
    expect(plan).toMatchObject({ downsample_width: 2, downsample_height: 1 });
    const input = grayscaleRgba([
      0, 1, 2, 3, 4,
      5, 6, 7, 8, 9,
      10, 11, 12, 13, 14,
    ]);
    expect(redValues(pixelatePreviewRgba(plan, input))).toEqual([
      6, 6, 6, 8, 8,
      6, 6, 6, 8, 8,
      6, 6, 6, 8, 8,
    ]);
  });

  it('preserves libswscale fixed-point tie behavior during non-divisible expansion', () => {
    const plan = resolvePreviewPixelateRasterPlan(7, 2, 2);
    expect(plan).toMatchObject({ downsample_width: 3, downsample_height: 1 });
    const input = grayscaleRgba([
      0, 1, 2, 3, 4, 5, 6,
      7, 8, 9, 10, 11, 12, 13,
    ]);
    expect(redValues(pixelatePreviewRgba(plan, input))).toEqual([
      8, 8, 10, 10, 10, 12, 12,
      8, 8, 10, 10, 10, 12, 12,
    ]);
  });

  it('preserves straight-alpha RGBA bytes from the selected reduced sample', () => {
    const plan = resolvePreviewPixelateRasterPlan(2, 2, 2);
    const input = new Uint8ClampedArray([
      10, 20, 30, 40, 50, 60, 70, 80,
      90, 100, 110, 120, 130, 140, 150, 160,
    ]);
    expect([...pixelatePreviewRgba(plan, input)]).toEqual([
      130, 140, 150, 160, 130, 140, 150, 160,
      130, 140, 150, 160, 130, 140, 150, 160,
    ]);
  });

  it('fails closed on malformed plans and mismatched buffers', () => {
    const plan = resolvePreviewPixelateRasterPlan(4, 4, 2);
    expect(() => pixelatePreviewRgba({
      ...plan,
      contract_version: 'preview-pixelate-raster-v0' as never,
    }, new Uint8ClampedArray(64))).toThrow(/preview-pixelate-raster-v1/);
    expect(() => pixelatePreviewRgba({
      ...plan,
      downsample_width: 3,
    }, new Uint8ClampedArray(64))).toThrow(/grid policy/);
    expect(() => pixelatePreviewRgba(plan, new Uint8ClampedArray(60))).toThrow(/width × height RGBA/);
    expect(() => resolvePreviewPixelateRasterPlan(4.5, 4, 2)).toThrow(/positive integer/);
    expect(() => resolvePreviewPixelateRasterPlan(4, 4, Number.NaN)).toThrow(/finite and non-negative/);
  });
});
