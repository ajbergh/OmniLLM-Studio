import type { CSSProperties, ReactNode } from 'react';
import type { VideoTimelineClip } from '../../types/video';
import {
  CURSOR_STATE_CONTRACT_V1,
  CURSOR_STATE_CONTRACT_V2,
  type CanonicalEvaluatedCursorState,
} from '../../video/renderContractCursor';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import {
  CANONICAL_SHAPE_KINDS,
  SHAPE_STATE_CONTRACT_V1,
  type CanonicalEvaluatedShapeState,
} from '../../video/renderContractShape';
import {
  TEXT_LINE_HEIGHT_MULTIPLIER,
  TEXT_LINE_HEIGHT_NORMAL,
  TEXT_STATE_CONTRACT_V1,
  type CanonicalEvaluatedTextState,
} from '../../video/renderContractText';

export type PreviewCanonicalContentMode =
  | 'legacy'
  | 'none'
  | 'canonical-text'
  | 'canonical-shape'
  | 'canonical-omit';
export type PreviewCanonicalCursorMode = 'legacy' | 'none' | 'canonical-cursor' | 'canonical-omit';

export interface PreviewCanonicalPainterPlan {
  mode: 'legacy-authored' | 'canonical-frame';
  content: PreviewCanonicalContentMode;
  cursor: PreviewCanonicalCursorMode;
}

export interface PreviewCanonicalPainterPresence {
  hasMediaBase: boolean;
  hasShape: boolean;
  hasText: boolean;
  hasCursor: boolean;
}

/**
 * Decide which established preview pixels canonical FrameState may replace.
 * Presence of canonical layer state is authoritative for painter omission, just
 * like clip/scene effect consumption. Media remains the base-content painter
 * when the established preview would choose media before shape/text metadata.
 */
export function resolvePreviewCanonicalPainterPlan(
  canonicalState: Pick<CanonicalFrameLayerState, 'text' | 'shape' | 'cursor'> | undefined,
  presence: PreviewCanonicalPainterPresence,
): PreviewCanonicalPainterPlan {
  if (!canonicalState) {
    return { mode: 'legacy-authored', content: 'legacy', cursor: 'legacy' };
  }

  if (canonicalState.text && !presence.hasText) {
    throw new Error('canonical preview text state is not bound to authored text');
  }
  if (canonicalState.shape && !presence.hasShape) {
    throw new Error('canonical preview shape state is not bound to authored shape');
  }
  if (canonicalState.cursor && !presence.hasCursor) {
    throw new Error('canonical preview cursor state is not bound to authored cursor metadata');
  }

  let content: PreviewCanonicalContentMode = 'none';
  if (presence.hasMediaBase) {
    content = 'legacy';
  } else if (presence.hasShape) {
    content = canonicalState.shape ? 'canonical-shape' : 'canonical-omit';
  } else if (presence.hasText) {
    content = canonicalState.text ? 'canonical-text' : 'canonical-omit';
  }

  let cursor: PreviewCanonicalCursorMode = 'none';
  if (presence.hasCursor) {
    cursor = canonicalState.cursor ? 'canonical-cursor' : 'canonical-omit';
  }

  return { mode: 'canonical-frame', content, cursor };
}

export interface CanonicalPreviewTextPaint {
  style: CSSProperties;
  /** Explicit box dimensions are canonical; glyph measurement still is not. */
  boxMode: 'canonical-explicit-box' | 'intrinsic-size';
  /** Phase 3–5 debt until one shared packaged-font measurement path owns glyph metrics. */
  metricsMode: 'browser-intrinsic-deferred';
}

/** Convert already-evaluated text-state-v1 into the existing DOM/CSS painter inputs. */
export function resolveCanonicalPreviewTextPaint(
  text: CanonicalEvaluatedTextState,
  stageScale: number,
  fontFamilyOverride?: string,
): CanonicalPreviewTextPaint {
  validateStageScale(stageScale);
  validateCanonicalTextState(text);

  const style: CSSProperties = {
    fontSize: text.font_size * stageScale,
    fontFamily: fontFamilyOverride || text.font_family || undefined,
    fontWeight: text.font_weight as CSSProperties['fontWeight'],
    color: text.color,
    background: text.background,
    borderRadius: text.border_radius * stageScale,
    textAlign: text.text_align,
    lineHeight: text.line_height_mode === TEXT_LINE_HEIGHT_MULTIPLIER ? text.line_height : 'normal',
    letterSpacing: text.letter_spacing * stageScale,
    WebkitTextStroke: text.stroke && text.stroke_width > 0
      ? `${text.stroke_width * stageScale}px ${text.stroke}`
      : undefined,
    textShadow: text.shadow
      ? `${text.shadow.offset_x * stageScale}px ${text.shadow.offset_y * stageScale}px ${text.shadow.blur_radius * stageScale}px ${text.shadow.color}`
      : undefined,
    padding: `${text.padding.top * stageScale}px ${text.padding.right * stageScale}px ${text.padding.bottom * stageScale}px ${text.padding.left * stageScale}px`,
    width: text.box_width !== undefined ? text.box_width * stageScale : undefined,
    height: text.box_height !== undefined ? text.box_height * stageScale : undefined,
    boxSizing: 'border-box',
    whiteSpace: 'pre-wrap',
  };

  if (text.box_height !== undefined) {
    style.display = 'flex';
    style.alignItems = verticalAlignment(text.vertical_align);
    style.justifyContent = horizontalAlignment(text.text_align);
  }

  return {
    style,
    boxMode: text.box_width !== undefined && text.box_height !== undefined
      ? 'canonical-explicit-box'
      : 'intrinsic-size',
    metricsMode: 'browser-intrinsic-deferred',
  };
}

export interface CanonicalPreviewShapeGeometry {
  width: number;
  height: number;
  strokeWidth: number;
  cornerRadius: number;
  blurRadius: number;
}

export function resolveCanonicalPreviewShapeGeometry(
  shape: CanonicalEvaluatedShapeState,
  stageScale: number,
): CanonicalPreviewShapeGeometry {
  validateStageScale(stageScale);
  validateCanonicalShapeState(shape);
  return {
    width: Math.max(2, shape.width * stageScale),
    height: Math.max(2, shape.height * stageScale),
    strokeWidth: Math.max(1, shape.stroke_width * stageScale),
    cornerRadius: shape.corner_radius * stageScale,
    blurRadius: Math.max(1, shape.blur_radius * stageScale),
  };
}

export interface CanonicalPreviewCursorGeometry {
  left: number;
  top: number;
  size: number;
}

export function resolveCanonicalPreviewCursorGeometry(
  cursor: CanonicalEvaluatedCursorState,
  stageScale: number,
): CanonicalPreviewCursorGeometry {
  validateStageScale(stageScale);
  validateCanonicalCursorState(cursor);
  return {
    left: cursor.x * stageScale,
    top: cursor.y * stageScale,
    size: 16 * cursor.scale * stageScale * 4,
  };
}

export function CanonicalPreviewText({
  text,
  stageScale,
  embedded = false,
  fontFamilyOverride,
}: {
  text: CanonicalEvaluatedTextState;
  stageScale: number;
  embedded?: boolean;
  fontFamilyOverride?: string;
}) {
  const paint = resolveCanonicalPreviewTextPaint(text, stageScale, fontFamilyOverride);
  return (
    <div
      data-preview-canonical-content={embedded ? undefined : 'text'}
      data-preview-text-state-mode="canonical-frame"
      data-preview-text-box-mode={paint.boxMode}
      data-preview-text-metrics-mode={paint.metricsMode}
      data-preview-text-font-face-source={text.font_face_source}
      data-preview-text-font-face-runtime={fontFamilyOverride ? 'editor-resource-loaded' : text.font_resource_id ? 'editor-resource-unloaded' : undefined}
      style={paint.style}
    >
      {text.text}
    </div>
  );
}

export function CanonicalPreviewShape({
  shape,
  text,
  stageScale,
  textFontFamilyOverride,
}: {
  shape: CanonicalEvaluatedShapeState;
  text?: CanonicalEvaluatedTextState;
  stageScale: number;
  textFontFamilyOverride?: string;
}) {
  const geometry = resolveCanonicalPreviewShapeGeometry(shape, stageScale);
  const { width, height, strokeWidth, cornerRadius, blurRadius } = geometry;
  const strokeBorder = shape.stroke ? `${strokeWidth}px solid ${shape.stroke}` : undefined;
  const textNode: ReactNode = text?.text
    ? <CanonicalPreviewText text={text} stageScale={stageScale} embedded fontFamilyOverride={textFontFamilyOverride} />
    : null;

  const svgShape = (children: ReactNode) => (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible">
      {children}
    </svg>
  );

  let content: ReactNode;
  switch (shape.kind) {
    case 'highlight':
      content = <div className="absolute inset-0" style={{ background: shape.fill }} />;
      break;
    case 'rectangle':
      content = <div className="absolute inset-0" style={{ background: shape.fill, border: strokeBorder }} />;
      break;
    case 'rounded_rectangle':
      content = <div className="absolute inset-0" style={{ background: shape.fill, border: strokeBorder, borderRadius: cornerRadius }} />;
      break;
    case 'ellipse':
      content = <div className="absolute inset-0" style={{ background: shape.fill, border: strokeBorder, borderRadius: '50%' }} />;
      break;
    case 'blur':
      content = <div className="absolute inset-0" style={{ backdropFilter: `blur(${blurRadius}px)` }} />;
      break;
    case 'pixelate':
      content = (
        <div
          className="absolute inset-0"
          style={{
            backdropFilter: `blur(${blurRadius}px)`,
            backgroundImage: `repeating-linear-gradient(0deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, blurRadius)}px), repeating-linear-gradient(90deg, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.06) 1px, transparent 1px, transparent ${Math.max(4, blurRadius)}px)`,
          }}
        />
      );
      break;
    case 'spotlight':
      content = <div className="absolute inset-0 rounded-full" style={{ boxShadow: `0 0 0 100000px ${shape.fill}` }} />;
      break;
    case 'arrow': {
      const y = height / 2;
      const head = Math.min(width * 0.3, Math.max(strokeWidth * 2.5, 12));
      content = svgShape(
        <>
          <line x1={0} y1={y} x2={width - head} y2={y} stroke={shape.stroke} strokeWidth={strokeWidth} strokeLinecap="round" />
          <polygon points={`${width},${y} ${width - head},${y - head / 2} ${width - head},${y + head / 2}`} fill={shape.stroke} />
        </>,
      );
      break;
    }
    case 'line':
      content = svgShape(
        <line x1={0} y1={height / 2} x2={width} y2={height / 2} stroke={shape.stroke} strokeWidth={strokeWidth} strokeLinecap="round" />,
      );
      break;
    case 'checkmark':
      content = svgShape(
        <polyline
          points={`${width * 0.15},${height * 0.55} ${width * 0.4},${height * 0.8} ${width * 0.85},${height * 0.2}`}
          fill="none"
          stroke={shape.stroke}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
        />,
      );
      break;
    case 'x_mark':
      content = svgShape(
        <>
          <line x1={width * 0.18} y1={height * 0.18} x2={width * 0.82} y2={height * 0.82} stroke={shape.stroke} strokeWidth={strokeWidth} strokeLinecap="round" />
          <line x1={width * 0.82} y1={height * 0.18} x2={width * 0.18} y2={height * 0.82} stroke={shape.stroke} strokeWidth={strokeWidth} strokeLinecap="round" />
        </>,
      );
      break;
    case 'step_marker':
      content = <div className="absolute inset-0 flex items-center justify-center rounded-full" style={{ background: shape.fill }}>{textNode}</div>;
      break;
    case 'speech_bubble': {
      const tail = Math.min(height * 0.25, 24 * stageScale);
      content = (
        <>
          <div
            className="absolute flex items-center justify-center px-2"
            style={{ inset: 0, bottom: tail, background: shape.fill, borderRadius: cornerRadius, border: strokeBorder }}
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
              borderTop: `${tail}px solid ${shape.fill}`,
            }}
          />
        </>
      );
      break;
    }
    case 'label':
      content = (
        <div
          className="absolute inset-0 flex items-center justify-center"
          style={{ background: shape.fill, borderRadius: cornerRadius, border: strokeBorder, padding: `0 ${10 * stageScale}px` }}
        >
          {textNode}
        </div>
      );
      break;
    default:
      throw new Error(`unsupported canonical preview shape kind ${JSON.stringify(shape.kind)}`);
  }

  return (
    <div
      data-preview-canonical-content="shape"
      data-preview-shape-state-mode="canonical-frame"
      data-preview-shape-painter-deferred={shape.kind === 'pixelate' ? 'pixelate-css-approximation' : undefined}
      className="relative flex items-center justify-center"
      style={{ width, height }}
    >
      {content}
    </div>
  );
}

export function CanonicalPreviewCursor({
  cursor,
  stageScale,
}: {
  cursor: CanonicalEvaluatedCursorState;
  stageScale: number;
}) {
  const geometry = resolveCanonicalPreviewCursorGeometry(cursor, stageScale);
  const { size, left, top } = geometry;
  return (
    <div
      data-preview-canonical-cursor="true"
      data-preview-cursor-state-mode="canonical-frame"
      className="pointer-events-none absolute"
      style={{ left, top }}
      aria-hidden="true"
    >
      {cursor.highlight && (
        <div
          className="absolute rounded-full"
          style={{ width: size * 2.2, height: size * 2.2, left: -size * 1.1, top: -size * 1.1, background: 'rgba(255, 223, 32, 0.3)' }}
        />
      )}
      {cursor.click && cursor.click_rings && (
        <div
          className="absolute rounded-full"
          style={{ width: size * 2.6, height: size * 2.6, left: -size * 1.3, top: -size * 1.3, border: '2px solid rgba(0, 188, 255, 0.8)' }}
        />
      )}
      <svg width={size} height={size} viewBox="0 0 16 16">
        <path d="M2 1 L2 12 L5.5 9.5 L7.5 14 L9.5 13 L7.5 8.8 L12 8.5 Z" fill="#ffffff" stroke="#111827" strokeWidth="1" />
      </svg>
    </div>
  );
}

/** True only for annotation kinds whose canonical painter actually emits text. */
export function canonicalPreviewShapeUsesText(shape: CanonicalEvaluatedShapeState | undefined): boolean {
  return shape?.kind === 'step_marker' || shape?.kind === 'speech_bubble' || shape?.kind === 'label';
}

function validateCanonicalTextState(text: CanonicalEvaluatedTextState): void {
  if (text.contract_version !== TEXT_STATE_CONTRACT_V1) {
    throw new Error(`canonical preview text requires ${TEXT_STATE_CONTRACT_V1}`);
  }
  if (text.line_height_mode === TEXT_LINE_HEIGHT_MULTIPLIER && text.line_height === undefined) {
    throw new Error('canonical preview text multiplier line height requires a value');
  }
  if (text.line_height_mode === TEXT_LINE_HEIGHT_NORMAL && text.line_height !== undefined) {
    throw new Error('canonical preview normal line height must not carry a multiplier');
  }
}

function validateCanonicalShapeState(shape: CanonicalEvaluatedShapeState): void {
  if (shape.contract_version !== SHAPE_STATE_CONTRACT_V1) {
    throw new Error(`canonical preview shape requires ${SHAPE_STATE_CONTRACT_V1}`);
  }
  if (!CANONICAL_SHAPE_KINDS.includes(shape.kind)) {
    throw new Error(`unsupported canonical preview shape kind ${JSON.stringify(shape.kind)}`);
  }
}

function validateCanonicalCursorState(cursor: CanonicalEvaluatedCursorState): void {
  const supportedContract = cursor.contract_version === CURSOR_STATE_CONTRACT_V1
    || cursor.contract_version === CURSOR_STATE_CONTRACT_V2;
  if (!supportedContract || cursor.visible !== true) {
    throw new Error(`canonical preview cursor requires visible ${CURSOR_STATE_CONTRACT_V1} or ${CURSOR_STATE_CONTRACT_V2}`);
  }
}

function validateStageScale(stageScale: number): void {
  if (!Number.isFinite(stageScale) || stageScale < 0) {
    throw new Error('canonical preview painter stage scale must be finite and non-negative');
  }
}

function verticalAlignment(value: CanonicalEvaluatedTextState['vertical_align']): CSSProperties['alignItems'] {
  if (value === 'top') return 'flex-start';
  if (value === 'bottom') return 'flex-end';
  return 'center';
}

function horizontalAlignment(value: CanonicalEvaluatedTextState['text_align']): CSSProperties['justifyContent'] {
  if (value === 'left') return 'flex-start';
  if (value === 'right') return 'flex-end';
  return 'center';
}

/** Existing preview content precedence used by the outer deterministic wrapper. */
export function previewClipHasMediaBase(clip: VideoTimelineClip, mimeType: string | undefined): boolean {
  return Boolean(clip.asset_id && mimeType && (mimeType.startsWith('video/') || mimeType.startsWith('image/')));
}
