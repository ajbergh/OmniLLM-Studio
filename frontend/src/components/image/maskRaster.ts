import type { MaskStroke } from '../../stores/imageEditor';

export type ImageMaskingMode = 'none' | 'semantic' | 'pixel';

export function featherRadiusPixels(brushSize: number, feather: number): number {
  const size = Math.max(1, brushSize);
  const pct = Math.max(0, Math.min(100, feather));
  return (size * pct) / 200;
}

export function featheredCoreWidth(brushSize: number, feather: number): number {
  const radius = featherRadiusPixels(brushSize, feather);
  return Math.max(1, brushSize - radius * 2);
}

interface DrawMaskStrokeOptions {
  mode: 'preview' | 'pixel' | 'semantic';
  opacity?: number;
}

/**
 * Draw a mask stroke with the same feather semantics for preview and export.
 * Pixel export uses transparency for the selected region. Semantic export uses
 * a visible white-on-black guide image because semantic-capable models consume
 * the selection as visual guidance rather than an exact alpha mask.
 */
export function drawMaskStroke(
  ctx: CanvasRenderingContext2D,
  stroke: MaskStroke,
  options: DrawMaskStrokeOptions,
): void {
  if (stroke.points.length === 0) return;

  const featherRadius = featherRadiusPixels(stroke.brushSize, stroke.feather);
  const coreWidth = featheredCoreWidth(stroke.brushSize, stroke.feather);

  ctx.save();
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.lineWidth = coreWidth;
  ctx.filter = featherRadius > 0 ? `blur(${featherRadius}px)` : 'none';

  if (options.mode === 'preview') {
    if (stroke.tool === 'eraser') {
      ctx.globalCompositeOperation = 'destination-out';
      ctx.globalAlpha = 1;
    } else {
      ctx.globalCompositeOperation = 'source-over';
      ctx.globalAlpha = options.opacity ?? 0.5;
      ctx.strokeStyle = 'rgba(239, 68, 68, 0.85)';
      ctx.fillStyle = 'rgba(239, 68, 68, 0.85)';
    }
  } else if (options.mode === 'pixel') {
    if (stroke.tool === 'eraser') {
      ctx.globalCompositeOperation = 'source-over';
      ctx.strokeStyle = '#000000';
      ctx.fillStyle = '#000000';
    } else {
      ctx.globalCompositeOperation = 'destination-out';
    }
  } else {
    ctx.globalCompositeOperation = 'source-over';
    const value = stroke.tool === 'eraser' ? '#000000' : '#ffffff';
    ctx.strokeStyle = value;
    ctx.fillStyle = value;
  }

  if (stroke.points.length === 1) {
    ctx.beginPath();
    ctx.arc(stroke.points[0].x, stroke.points[0].y, coreWidth / 2, 0, Math.PI * 2);
    ctx.fill();
  } else {
    ctx.beginPath();
    ctx.moveTo(stroke.points[0].x, stroke.points[0].y);
    for (let i = 1; i < stroke.points.length; i += 1) {
      ctx.lineTo(stroke.points[i].x, stroke.points[i].y);
    }
    ctx.stroke();
  }

  ctx.restore();
}
