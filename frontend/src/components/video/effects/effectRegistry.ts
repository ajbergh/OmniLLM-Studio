/**
 * Effect registry — the single source of truth for the effect browser,
 * inspector rows, preview CSS filters, and export-support badges. Export
 * support must track backend/internal/video/renderer.go.
 */
import type { VideoTimelineEffect } from '../../../types/video';

export type EffectTypeKey = VideoTimelineEffect['type'];

export interface EffectParamMeta {
  key: string;
  label: string;
  min: number;
  max: number;
  step: number;
  defaultValue: number;
}

export type EffectCategory = 'color' | 'blur' | 'stylize' | 'motion' | 'keying';

export const EFFECT_CATEGORIES: Array<{ key: EffectCategory; label: string }> = [
  { key: 'color', label: 'Color' },
  { key: 'blur', label: 'Blur' },
  { key: 'stylize', label: 'Stylize' },
  { key: 'motion', label: 'Motion' },
  { key: 'keying', label: 'Keying' },
];

export interface EffectDefinition {
  type: EffectTypeKey;
  label: string;
  category: EffectCategory;
  /** Whether the FFmpeg renderer applies this effect at export today. */
  exportSupported: boolean;
  /** Backend renderer feature used for runtime support/partial badges. */
  exportFeature?: string;
  params: EffectParamMeta[];
  /** CSS filter fragment for the preview canvas, or null when not previewable. */
  previewFilter: (params: Record<string, unknown>) => string | null;
}

function amountParam(label: string, min: number, max: number, step: number, defaultValue: number): EffectParamMeta {
  return { key: 'amount', label, min, max, step, defaultValue };
}

export function numberParam(params: Record<string, unknown> | undefined, key: string, fallback: number): number {
  const value = Number((params || {})[key]);
  return Number.isFinite(value) ? value : fallback;
}

// Export support must track backend/internal/video/renderer.go — the renderer
// maps brightness/contrast/saturation/blur/grayscale/sharpen/vignette/
// chroma_key and skips shadow/background_blur.
export const EFFECT_DEFINITIONS: EffectDefinition[] = [
  {
    type: 'brightness',
    label: 'Brightness',
    category: 'color',
    exportSupported: true,
    params: [amountParam('Amount', 0, 2, 0.05, 1.1)],
    previewFilter: (params) => `brightness(${numberParam(params, 'amount', 1)})`,
  },
  {
    type: 'contrast',
    label: 'Contrast',
    category: 'color',
    exportSupported: true,
    params: [amountParam('Amount', 0, 3, 0.05, 1.2)],
    previewFilter: (params) => `contrast(${numberParam(params, 'amount', 1)})`,
  },
  {
    type: 'saturation',
    label: 'Saturation',
    category: 'color',
    exportSupported: true,
    params: [amountParam('Amount', 0, 3, 0.05, 1.3)],
    previewFilter: (params) => `saturate(${numberParam(params, 'amount', 1)})`,
  },
  {
    type: 'blur',
    label: 'Blur',
    category: 'blur',
    exportSupported: true,
    params: [amountParam('Radius', 0, 30, 1, 6)],
    previewFilter: (params) => `blur(${numberParam(params, 'amount', 0)}px)`,
  },
  {
    type: 'grayscale',
    label: 'Grayscale',
    category: 'color',
    exportSupported: true,
    params: [],
    previewFilter: () => 'grayscale(1)',
  },
  {
    type: 'sharpen',
    label: 'Sharpen',
    category: 'stylize',
    exportSupported: true,
    params: [amountParam('Amount', 0, 3, 0.1, 1)],
    previewFilter: () => null,
  },
  {
    type: 'vignette',
    label: 'Vignette',
    category: 'stylize',
    exportSupported: true,
    params: [amountParam('Strength', 0, 1, 0.05, 0.4)],
    previewFilter: () => null,
  },
  {
    type: 'shadow',
    label: 'Drop shadow',
    category: 'stylize',
    exportSupported: false,
    params: [],
    previewFilter: () => 'drop-shadow(2px 4px 6px rgba(0,0,0,0.6))',
  },
  {
    type: 'background_blur',
    label: 'Background blur',
    category: 'blur',
    exportSupported: false,
    params: [amountParam('Radius', 0, 30, 1, 10)],
    previewFilter: () => null,
  },
  {
    // Keys out green (or params.color) at export via FFmpeg chromakey; CSS
    // cannot preview it, so the canvas shows the unkeyed frame.
    type: 'chroma_key',
    label: 'Chroma key (export only)',
    category: 'keying',
    exportSupported: true,
    params: [
      { key: 'similarity', label: 'Similarity', min: 0.01, max: 1, step: 0.01, defaultValue: 0.3 },
      { key: 'blend', label: 'Blend', min: 0, max: 0.5, step: 0.01, defaultValue: 0.05 },
    ],
    previewFilter: () => null,
  },
  { type: 'film_grain', label: 'Film grain', category: 'stylize', exportSupported: true, exportFeature: 'film_grain', params: [amountParam('Amount', 0, 40, 1, 8)], previewFilter: (params) => `contrast(${1 + numberParam(params, 'amount', 8) / 200})` },
  { type: 'bloom', label: 'Bloom', category: 'stylize', exportSupported: true, exportFeature: 'bloom', params: [amountParam('Amount', 0, 1, 0.05, 0.25)], previewFilter: (params) => `brightness(${1 + numberParam(params, 'amount', 0.25) * 0.3}) saturate(${1 + numberParam(params, 'amount', 0.25) * 0.2})` },
  { type: 'color_grade', label: 'Color grade', category: 'color', exportSupported: true, exportFeature: 'color_grade', params: [amountParam('Intensity', 0.5, 2, 0.05, 1.08)], previewFilter: (params) => `contrast(${numberParam(params, 'amount', 1.08)}) saturate(${numberParam(params, 'amount', 1.08)})` },
  { type: 'edge_fade', label: 'Edge fade', category: 'stylize', exportSupported: true, exportFeature: 'edge_fade', params: [amountParam('Strength', 0, 1, 0.05, 0.35)], previewFilter: () => null },
  { type: 'rgb_split', label: 'RGB split', category: 'stylize', exportSupported: true, exportFeature: 'rgb_split', params: [amountParam('Offset', 0, 20, 1, 3)], previewFilter: () => null },
  { type: 'ghost_trail', label: 'Ghost trail', category: 'motion', exportSupported: true, exportFeature: 'ghost_trail', params: [amountParam('Frames', 2, 5, 1, 3)], previewFilter: () => null },
  { type: 'motion_blur', label: 'Motion blur', category: 'motion', exportSupported: true, exportFeature: 'motion_blur', params: [amountParam('Amount', 0, 1, 0.05, 0.5)], previewFilter: (params) => `blur(${numberParam(params, 'amount', 0.5)}px)` },
  { type: 'depth_of_field', label: 'Depth of field', category: 'blur', exportSupported: true, exportFeature: 'depth_of_field', params: [amountParam('Blur', 0, 12, 0.5, 2)], previewFilter: (params) => `blur(${numberParam(params, 'amount', 2)}px)` },
  { type: 'rack_focus', label: 'Rack focus', category: 'blur', exportSupported: true, exportFeature: 'rack_focus', params: [amountParam('Blur', 0, 12, 0.5, 2)], previewFilter: (params) => `blur(${numberParam(params, 'amount', 2)}px)` },
];

export function effectDefinition(type: string): EffectDefinition | undefined {
  return EFFECT_DEFINITIONS.find((definition) => definition.type === type);
}

export function defaultEffectParams(definition: EffectDefinition): Record<string, unknown> {
  return Object.fromEntries(definition.params.map((param) => [param.key, param.defaultValue]));
}

/** Composes the CSS `filter` value for a clip's enabled, previewable effects. */
export function composePreviewFilter(effects: VideoTimelineEffect[] | undefined): string | undefined {
  const parts = (effects || [])
    .filter((effect) => effect.enabled)
    .map((effect) => effectDefinition(effect.type)?.previewFilter(effect.params) ?? null)
    .filter((part): part is string => Boolean(part));
  return parts.length > 0 ? parts.join(' ') : undefined;
}
