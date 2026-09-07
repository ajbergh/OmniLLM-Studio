import type { CSSProperties, ReactNode } from 'react';
import type { VideoTimelineClip, VideoTimelineShape } from '../../types/video';
import { evaluateShapeState } from '../../video/renderContractShape';
import { CanonicalPreviewShape } from './PreviewCanonicalPainters';

const ROUNDED_RECTANGLE_PLAYBACK_STYLE = `
[data-preview-rounded-rectangle-playback] > [data-preview-shape-playback-consumer="canonical-inline"] {
  display: none;
}
[data-testid="video-preview-program"][data-preview-visual-frame-mode="canonical-playback"]
  [data-preview-rounded-rectangle-playback] > [data-preview-shape-playback-consumer="legacy-time-fallback"] {
  display: none;
}
[data-testid="video-preview-program"][data-preview-visual-frame-mode="canonical-playback"]
  [data-preview-rounded-rectangle-playback] > [data-preview-shape-playback-consumer="canonical-inline"] {
  display: flex;
}
`;

/**
 * Preview rendering for annotation/shape clips. Sizing is in canvas pixels
 * scaled to the stage; position/rotation/opacity come from the clip wrapper.
 * Export support varies per kind — see annotationRegistry / renderer notes.
 *
 * The export-proven static rounded-rectangle subset carries both the established
 * compatibility painter and CanonicalPreviewShape. The stage's all-frame normal
 * playback decision is the sole switch: canonical playback shows the canonical
 * surface, while every fallback/deterministic/editor mode retains the established
 * surface (the deterministic wrapper separately portals its canonical painter).
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

  let legacyContent: ReactNode;
  switch (shape.kind) {
    case 'highlight':
      legacyContent = box({ background: shape.fill || '#facc15' });
      break;
    case 'rectangle':
      legacyContent = box({ border: `${strokeWidth}px solid ${stroke}` });
      break;
    case 'rounded_rectangle':
      legacyContent = box({ border: `${strokeWidth}px solid ${stroke}`, borderRadius: cornerRadius || 12 * stageScale, background: shape.fill || 'transparent' });
      break;
    case 'ellipse':
      legacyContent = box({ border: `${strokeWidth}px solid ${stroke}`, borderRadius: '50%', background: shape.fill || 'transparent' });
      break;
    case 'blur':
      // Blur regions blur whatever composites beneath them, like export.
      legacyContent = box({ backdropFilter: `blur(${Math.max(1, (shape.blur_radius || 12) * stageScale)}px)` });
      break;
    case 'pixelate':
      // CSS cannot pixelate the backdrop — approximate with a blur plus a
      // mosaic grid; the export performs a true pixelation.
      legacyContent = box({
        backdropFilter: `blur(${Math.max(1, (shape.blur_radius || 12) * stageScale)}px)`,
        backgroundImage: `repeating-linear-gradient(0deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, (shape.blur_radius || 12) * stageScale)}px), repeating-linear-gradient(90deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, (shape.blur_radius || 12) * stageScale)}px)`,
      });
      break;
    case 'spotlight':
      // The giant box-shadow dims everything outside the ellipse; the stage's
      // overflow-hidden clips it to the canvas.
      legacyContent = box({ borderRadius: '50%', boxShadow: `0 0 0 100000px ${shape.fill || 'rgba(0,0,0,0.6)'}` });
      break;
    case 'arrow': {
      const y = height / 2;
      const head = Math.min(width * 0.3, Math.max(strokeWidth * 2.5, 12));
      legacyContent = svgShape(
        <>
          <line x1={0} y1={y} x2={width - head} y2={y} stroke={stroke} strokeWidth={strokeWidth} strokeLinecap="round" />
          <polygon points={`${width},${y} ${width - head},${y - head / 2} ${width - head},${y + head / 2}`} fill={stroke} />
        </>,
      );
      break;
    }
    case 'line':
      legacyContent = svgShape(
        <line x1={0} y1={height / 2} x2={width} y2={height / 2} stroke={stroke} strokeWidth={strokeWidth} strokeLinecap="round" />,
      );
      break;
    case 'checkmark':
      legacyContent = svgShape(
        <polyline
          points={`${width * 0.15},${height * 0.55} ${width * 0.4},${height * 0.8} ${width * 0.85},${height * 0.2}`}
          fill="none"
          stroke={shape.stroke || '#22c55e'}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
        />,
      );
      break;
    case 'x_mark':
      legacyContent = svgShape(
        <>
          <line x1={width * 0.18} y1={height * 0.18} x2={width * 0.82} y2={height * 0.82} stroke={shape.stroke || '#ef4444'} strokeWidth={strokeWidth} strokeLinecap="round" />
          <line x1={width * 0.82} y1={height * 0.18} x2={width * 0.18} y2={height * 0.82} stroke={shape.stroke || '#ef4444'} strokeWidth={strokeWidth} strokeLinecap="round" />
        </>,
      );
      break;
    case 'step_marker':
      legacyContent = box({ background: shape.fill || '#2563eb', borderRadius: '50%' });
      break;
    case 'speech_bubble': {
      const radius = cornerRadius || 18 * stageScale;
      const tail = Math.min(height * 0.25, 24 * stageScale);
      legacyContent = (
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
      break;
    }
    case 'label':
      legacyContent = box({ background: shape.fill || '#1e293b', borderRadius: cornerRadius || 10 * stageScale, border: shape.stroke ? `${strokeWidth}px solid ${shape.stroke}` : undefined, padding: `0 ${10 * stageScale}px` });
      break;
    default:
      legacyContent = box({ border: `${strokeWidth}px solid ${stroke}` });
      break;
  }

  if (shape.kind !== 'rounded_rectangle') {
    return (
      <div
        data-preview-shape-playback-clip-id={clip.id}
        data-preview-shape-playback-consumer="legacy-time-fallback"
        data-preview-shape-state-mode="legacy-time"
      >
        {legacyContent}
      </div>
    );
  }

  const canonicalShape = evaluateShapeState(shape);
  return (
    <div data-preview-rounded-rectangle-playback data-preview-shape-playback-clip-id={clip.id} className="contents">
      <style>{ROUNDED_RECTANGLE_PLAYBACK_STYLE}</style>
      <div
        data-preview-shape-playback-consumer="legacy-time-fallback"
        data-preview-shape-state-mode="legacy-time"
      >
        {legacyContent}
      </div>
      {canonicalShape && (
        <div
          data-preview-shape-playback-consumer="canonical-inline"
          data-preview-shape-state-mode="canonical-frame"
          data-preview-shape-kind={canonicalShape.kind}
          data-preview-shape-width={canonicalShape.width}
          data-preview-shape-height={canonicalShape.height}
          data-preview-shape-corner-radius={canonicalShape.corner_radius}
          data-preview-shape-stroke-width={canonicalShape.stroke_width}
        >
          <CanonicalPreviewShape shape={canonicalShape} stageScale={stageScale} />
        </div>
      )}
    </div>
  );
}
