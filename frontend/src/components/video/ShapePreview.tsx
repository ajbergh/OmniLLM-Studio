import type { CSSProperties, ReactNode } from 'react';
import type { VideoTimelineClip, VideoTimelineShape } from '../../types/video';

/**
 * Preview rendering for annotation/shape clips. Sizing is in canvas pixels
 * scaled to the stage; position/rotation/opacity come from the clip wrapper.
 * Export support varies per kind — see annotationRegistry / renderer notes.
 */
export function ShapePreview({
  shape,
  clip,
  stageScale,
  canvasHeight,
  liveWidth,
  liveHeight,
}: {
  shape: VideoTimelineShape;
  clip: VideoTimelineClip;
  stageScale: number;
  canvasHeight: number;
  liveWidth?: number;
  liveHeight?: number;
}) {
  const width = Math.max(2, (liveWidth ?? shape.width ?? 320) * stageScale);
  const height = Math.max(2, (liveHeight ?? shape.height ?? 180) * stageScale);
  const stroke = shape.stroke || '#f59e0b';
  const strokeWidth = Math.max(1, (shape.stroke_width || 6) * stageScale);
  const cornerRadius = (shape.corner_radius || 0) * stageScale;

  const textNode: ReactNode = clip.text?.text ? (
    <span
      className="pointer-events-none whitespace-pre-wrap text-center"
      style={{
        fontSize: (clip.text.font_size || Math.round(canvasHeight / 24)) * stageScale,
        fontWeight: (clip.text.font_weight as CSSProperties['fontWeight']) || 700,
        fontFamily: clip.text.font_family || undefined,
        color: clip.text.color || '#ffffff',
        textShadow: clip.text.shadow ? '2px 2px 4px rgba(0,0,0,0.7)' : undefined,
        textAlign: (clip.text.text_align as CSSProperties['textAlign']) || 'center',
      }}
    >
      {clip.text.text}
    </span>
  ) : null;

  const box = (style: CSSProperties, children?: ReactNode) => (
    <div style={{ width, height, ...style }} className="flex items-center justify-center">
      {children ?? textNode}
    </div>
  );

  const svgShape = (children: ReactNode) => (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible">
      {children}
    </svg>
  );

  switch (shape.kind) {
    case 'highlight':
      return box({ background: shape.fill || '#facc15' });
    case 'rectangle':
      return box({ border: `${strokeWidth}px solid ${stroke}` });
    case 'rounded_rectangle':
      return box({ border: `${strokeWidth}px solid ${stroke}`, borderRadius: cornerRadius || 12 * stageScale, background: shape.fill || 'transparent' });
    case 'ellipse':
      return box({ border: `${strokeWidth}px solid ${stroke}`, borderRadius: '50%', background: shape.fill || 'transparent' });
    case 'blur':
      // Blur regions blur whatever composites beneath them, like export.
      return box({ backdropFilter: `blur(${Math.max(1, (shape.blur_radius || 12) * stageScale)}px)` });
    case 'pixelate':
      // CSS cannot pixelate the backdrop — approximate with a blur plus a
      // mosaic grid; the export performs a true pixelation.
      return box({
        backdropFilter: `blur(${Math.max(1, (shape.blur_radius || 12) * stageScale)}px)`,
        backgroundImage: `repeating-linear-gradient(0deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, (shape.blur_radius || 12) * stageScale)}px), repeating-linear-gradient(90deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, (shape.blur_radius || 12) * stageScale)}px)`,
      });
    case 'spotlight':
      // The giant box-shadow dims everything outside the ellipse; the stage's
      // overflow-hidden clips it to the canvas.
      return box({ borderRadius: '50%', boxShadow: `0 0 0 100000px ${shape.fill || 'rgba(0,0,0,0.6)'}` });
    case 'arrow': {
      const y = height / 2;
      const head = Math.min(width * 0.3, Math.max(strokeWidth * 2.5, 12));
      return svgShape(
        <>
          <line x1={0} y1={y} x2={width - head} y2={y} stroke={stroke} strokeWidth={strokeWidth} strokeLinecap="round" />
          <polygon points={`${width},${y} ${width - head},${y - head / 2} ${width - head},${y + head / 2}`} fill={stroke} />
        </>,
      );
    }
    case 'line':
      return svgShape(
        <line x1={0} y1={height / 2} x2={width} y2={height / 2} stroke={stroke} strokeWidth={strokeWidth} strokeLinecap="round" />,
      );
    case 'checkmark':
      return svgShape(
        <polyline
          points={`${width * 0.15},${height * 0.55} ${width * 0.4},${height * 0.8} ${width * 0.85},${height * 0.2}`}
          fill="none"
          stroke={shape.stroke || '#22c55e'}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
        />,
      );
    case 'x_mark':
      return svgShape(
        <>
          <line x1={width * 0.18} y1={height * 0.18} x2={width * 0.82} y2={height * 0.82} stroke={shape.stroke || '#ef4444'} strokeWidth={strokeWidth} strokeLinecap="round" />
          <line x1={width * 0.82} y1={height * 0.18} x2={width * 0.18} y2={height * 0.82} stroke={shape.stroke || '#ef4444'} strokeWidth={strokeWidth} strokeLinecap="round" />
        </>,
      );
    case 'step_marker':
      return box({ background: shape.fill || '#2563eb', borderRadius: '50%' });
    case 'speech_bubble': {
      const radius = cornerRadius || 18 * stageScale;
      const tail = Math.min(height * 0.25, 24 * stageScale);
      return (
        <div style={{ width, height }} className="relative">
          <div
            className="absolute flex items-center justify-center px-2"
            style={{ inset: 0, bottom: tail, background: shape.fill || '#ffffff', borderRadius: radius, border: shape.stroke ? `${strokeWidth}px solid ${shape.stroke}` : undefined }}
          >
            {textNode}
          </div>
          <div
            className="absolute"
            style={{
              left: width * 0.22,
              bottom: 0,
              width: 0,
              height: 0,
              borderLeft: `${tail}px solid transparent`,
              borderRight: `${tail * 0.4}px solid transparent`,
              borderTop: `${tail}px solid ${shape.fill || '#ffffff'}`,
            }}
          />
        </div>
      );
    }
    case 'label':
      return box({ background: shape.fill || '#1e293b', borderRadius: cornerRadius || 10 * stageScale, border: shape.stroke ? `${strokeWidth}px solid ${shape.stroke}` : undefined, padding: `0 ${10 * stageScale}px` });
    default:
      return box({ border: `${strokeWidth}px solid ${stroke}` });
  }
}
