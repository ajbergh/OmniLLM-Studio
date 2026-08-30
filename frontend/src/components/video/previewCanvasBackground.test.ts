import { describe, expect, it } from 'vitest';
import { resolvePreviewCanvasBackgroundColor } from './previewCanvasBackground';

describe('resolvePreviewCanvasBackgroundColor', () => {
  it('preserves renderer-supported six-digit hash RGB', () => {
    expect(resolvePreviewCanvasBackgroundColor('#19324A')).toBe('#19324A');
    expect(resolvePreviewCanvasBackgroundColor('  #a0b1c2  ')).toBe('#a0b1c2');
  });

  it('accepts renderer-supported bare six-digit RGB', () => {
    expect(resolvePreviewCanvasBackgroundColor('19324A')).toBe('#19324A');
  });

  it('falls back to black for unsupported or missing values', () => {
    expect(resolvePreviewCanvasBackgroundColor('#1234')).toBe('#000000');
    expect(resolvePreviewCanvasBackgroundColor('#11223344')).toBe('#000000');
    expect(resolvePreviewCanvasBackgroundColor('red')).toBe('#000000');
    expect(resolvePreviewCanvasBackgroundColor(undefined)).toBe('#000000');
  });
});
