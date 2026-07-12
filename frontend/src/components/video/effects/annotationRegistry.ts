import type { VideoTimelineShape, VideoTimelineShapeKind, VideoTimelineText } from '../../../types/video';

export type AnnotationExportSupport = 'full' | 'partial' | 'preview';

export interface AnnotationDefinition {
  kind: VideoTimelineShapeKind;
  label: string;
  category: 'callout' | 'redaction' | 'mark' | 'focus';
  exportSupport: AnnotationExportSupport;
  exportNote?: string;
}

/**
 * Catalogue of annotation/shape kinds. Export support mirrors the backend
 * renderer (renderer_capabilities.go) — keep both in sync.
 */
export const ANNOTATION_DEFINITIONS: AnnotationDefinition[] = [
  { kind: 'rectangle', label: 'Rectangle outline', category: 'callout', exportSupport: 'full' },
  { kind: 'highlight', label: 'Highlight box', category: 'callout', exportSupport: 'full' },
  { kind: 'rounded_rectangle', label: 'Rounded rectangle', category: 'callout', exportSupport: 'partial', exportNote: 'Exports with square corners' },
  { kind: 'label', label: 'Label callout', category: 'callout', exportSupport: 'partial', exportNote: 'Exports as a square-corner filled box with text' },
  { kind: 'speech_bubble', label: 'Speech bubble', category: 'callout', exportSupport: 'preview' },
  { kind: 'blur', label: 'Blur region', category: 'redaction', exportSupport: 'full' },
  { kind: 'pixelate', label: 'Pixelate region', category: 'redaction', exportSupport: 'full', exportNote: 'Preview approximates the mosaic with a blur' },
  { kind: 'spotlight', label: 'Spotlight', category: 'focus', exportSupport: 'preview' },
  { kind: 'ellipse', label: 'Ellipse', category: 'mark', exportSupport: 'preview' },
  { kind: 'arrow', label: 'Arrow', category: 'mark', exportSupport: 'preview' },
  { kind: 'line', label: 'Line', category: 'mark', exportSupport: 'preview' },
  { kind: 'checkmark', label: 'Checkmark', category: 'mark', exportSupport: 'preview' },
  { kind: 'x_mark', label: 'X mark', category: 'mark', exportSupport: 'preview' },
  { kind: 'step_marker', label: 'Numbered step', category: 'mark', exportSupport: 'preview' },
];

export function annotationDefinition(kind: VideoTimelineShapeKind): AnnotationDefinition | undefined {
  return ANNOTATION_DEFINITIONS.find((definition) => definition.kind === kind);
}

export interface AnnotationDefaults {
  shape: VideoTimelineShape;
  opacity: number;
  text?: Partial<VideoTimelineText> & { text: string };
}

/** Sensible creation defaults per annotation kind, sized relative to the canvas. */
export function annotationDefaults(kind: VideoTimelineShapeKind, canvas: { width: number; height: number }): AnnotationDefaults {
  const size = { width: Math.round(canvas.width / 3), height: Math.round(canvas.height / 4) };
  const square = Math.round(Math.min(canvas.width, canvas.height) / 8);
  switch (kind) {
    case 'highlight':
      return { shape: { kind, ...size, fill: '#facc15' }, opacity: 0.4 };
    case 'blur':
      return { shape: { kind, ...size, blur_radius: 12 }, opacity: 1 };
    case 'pixelate':
      return { shape: { kind, ...size, blur_radius: 12 }, opacity: 1 };
    case 'rounded_rectangle':
      return { shape: { kind, ...size, stroke: '#f59e0b', stroke_width: 6, corner_radius: 24 }, opacity: 1 };
    case 'ellipse':
      return { shape: { kind, ...size, stroke: '#f59e0b', stroke_width: 6 }, opacity: 1 };
    case 'arrow':
      return { shape: { kind, width: size.width, height: Math.round(size.height / 2), stroke: '#ef4444', stroke_width: 10 }, opacity: 1 };
    case 'line':
      return { shape: { kind, width: size.width, height: Math.round(size.height / 3), stroke: '#ef4444', stroke_width: 6 }, opacity: 1 };
    case 'speech_bubble':
      return {
        shape: { kind, ...size, fill: '#ffffff', corner_radius: 18 },
        opacity: 1,
        text: { text: 'Say something…', color: '#111827', font_size: Math.round(canvas.height / 28), shadow: false },
      };
    case 'spotlight':
      return { shape: { kind, width: Math.round(canvas.width / 2.5), height: Math.round(canvas.height / 2.5) }, opacity: 1 };
    case 'checkmark':
      return { shape: { kind, width: square, height: square, stroke: '#22c55e', stroke_width: 12 }, opacity: 1 };
    case 'x_mark':
      return { shape: { kind, width: square, height: square, stroke: '#ef4444', stroke_width: 12 }, opacity: 1 };
    case 'step_marker':
      return {
        shape: { kind, width: square, height: square, fill: '#2563eb' },
        opacity: 1,
        text: { text: '1', color: '#ffffff', font_size: Math.round(square / 2), shadow: false },
      };
    case 'label':
      return {
        shape: { kind, width: size.width, height: Math.round(size.height / 2), fill: '#1e293b', corner_radius: 10 },
        opacity: 0.95,
        text: { text: 'Label', color: '#f8fafc', font_size: Math.round(canvas.height / 26), shadow: false },
      };
    case 'rectangle':
    default:
      return { shape: { kind: 'rectangle', ...size, stroke: '#f59e0b', stroke_width: 6 }, opacity: 1 };
  }
}

export interface AnnotationPreset {
  key: string;
  label: string;
  /** Applies on top of an existing annotation clip's shape/text/opacity. */
  shape: Partial<VideoTimelineShape>;
  text?: Partial<VideoTimelineText>;
  opacity?: number;
}

export const ANNOTATION_PRESETS: AnnotationPreset[] = [
  { key: 'yellow-highlight', label: 'Yellow highlight', shape: { kind: 'highlight', fill: '#facc15' }, opacity: 0.4 },
  { key: 'red-outline', label: 'Red outline box', shape: { kind: 'rectangle', stroke: '#ef4444', stroke_width: 6 }, opacity: 1 },
  {
    key: 'blue-callout',
    label: 'Blue tutorial callout',
    shape: { kind: 'label', fill: '#1d4ed8', corner_radius: 12 },
    text: { color: '#ffffff', font_weight: '600' },
    opacity: 0.95,
  },
  {
    key: 'dark-lower-third',
    label: 'Dark lower-third',
    shape: { kind: 'label', fill: '#0f172a', corner_radius: 6 },
    text: { color: '#f8fafc', text_align: 'left' },
    opacity: 0.9,
  },
  {
    key: 'keyboard-shortcut',
    label: 'Keyboard shortcut',
    shape: { kind: 'label', fill: '#111827', stroke: '#6b7280', stroke_width: 2, corner_radius: 8 },
    text: { color: '#e5e7eb', font_family: 'monospace', font_weight: '700' },
    opacity: 1,
  },
  {
    key: 'numbered-step',
    label: 'Numbered step',
    shape: { kind: 'step_marker', fill: '#2563eb' },
    text: { color: '#ffffff', font_weight: '700' },
    opacity: 1,
  },
];
