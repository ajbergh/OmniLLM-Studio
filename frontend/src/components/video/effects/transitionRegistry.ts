import type { VideoTimelineTransition } from '../../../types/video';

export type TransitionTypeKey = VideoTimelineTransition['type'];

export interface TransitionDefinition {
  type: TransitionTypeKey;
  label: string;
  /** Whether the FFmpeg renderer applies this transition at export today. */
  exportSupported: boolean;
  exportNote?: string;
  supportsDirection: boolean;
  defaultDurationMs: number;
}

// Export support must track backend/internal/video/renderer.go — fade-style
// transitions render as alpha fades, slide renders as an animated overlay
// position; wipe/zoom are dropped at export.
export const TRANSITION_DEFINITIONS: TransitionDefinition[] = [
  { type: 'fade', label: 'Fade', exportSupported: true, supportsDirection: false, defaultDurationMs: 500 },
  { type: 'crossfade', label: 'Crossfade', exportSupported: true, exportNote: 'Rendered as an alpha fade', supportsDirection: false, defaultDurationMs: 500 },
  { type: 'dip_to_black', label: 'Dip to black', exportSupported: true, exportNote: 'Rendered as an alpha fade', supportsDirection: false, defaultDurationMs: 600 },
  { type: 'slide', label: 'Slide', exportSupported: true, exportNote: 'Slides in from the chosen edge and out the opposite edge', supportsDirection: true, defaultDurationMs: 500 },
  { type: 'wipe', label: 'Wipe', exportSupported: false, supportsDirection: true, defaultDurationMs: 500 },
  { type: 'zoom', label: 'Zoom', exportSupported: false, supportsDirection: false, defaultDurationMs: 500 },
];

export function transitionDefinition(type: string): TransitionDefinition | undefined {
  return TRANSITION_DEFINITIONS.find((definition) => definition.type === type);
}
