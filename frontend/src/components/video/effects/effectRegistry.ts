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

export type EffectCategory = 'color' | 'blur' | 'stylize' | 'keying';

export const EFFECT_CATEGORIES: Array<{ key: EffectCategory; label: string }> = [
  { key: 'color', label: 'Color' },
  { key: 'blur', label: 'Blur' },
  { key: 'stylize', label: 'Stylize' },
  { key: 'keying', label: 'Keying' },
];

export interface EffectDefinition {
  type: EffectTypeKey;
  label: string;
  category: EffectCategory;
  /** Whether the FFmpeg renderer applies this effect at export today. */
  exportSupported: boolean;
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
