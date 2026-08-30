const HEX_RGB = /^[0-9a-fA-F]{6}$/;

/**
 * Resolve the project canvas background exactly like the FFmpeg renderer's
 * ffmpegColor helper, but into a CSS/Canvas color string. The renderer admits
 * only six-digit RGB (with or without '#') and otherwise falls back to black;
 * preview Canvas consumers must not invent a broader color contract.
 */
export function resolvePreviewCanvasBackgroundColor(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? '';
  if (trimmed.length === 7 && trimmed[0] === '#' && HEX_RGB.test(trimmed.slice(1))) {
    return trimmed;
  }
  if (HEX_RGB.test(trimmed)) return `#${trimmed}`;
  return '#000000';
}
