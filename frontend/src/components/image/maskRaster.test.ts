import { describe, expect, it } from 'vitest';
import { featherRadiusPixels, featheredCoreWidth } from './maskRaster';

describe('mask raster feathering', () => {
  it('keeps a hard brush when feather is zero', () => {
    expect(featherRadiusPixels(40, 0)).toBe(0);
    expect(featheredCoreWidth(40, 0)).toBe(40);
  });

  it('converts feather percentage into a symmetric edge radius', () => {
    expect(featherRadiusPixels(40, 50)).toBe(10);
    expect(featheredCoreWidth(40, 50)).toBe(20);
  });

  it('clamps out-of-range feather values and preserves a drawable core', () => {
    expect(featherRadiusPixels(20, 150)).toBe(10);
    expect(featherRadiusPixels(20, -10)).toBe(0);
    expect(featheredCoreWidth(20, 100)).toBe(1);
  });
});
