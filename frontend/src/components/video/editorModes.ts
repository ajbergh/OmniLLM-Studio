// Editor modes hide feature groups in the UI without changing persisted
// timeline semantics — a timeline edited in "simple trim" opens identically in
// the full editor.

export type EditorModeKey = 'full' | 'simple_trim' | 'captions_only' | 'social_clip';

export interface EditorModeFeatures {
  /** AI assistant section in the inspector. */
  assistant: boolean;
  /** Transform sliders + layer-order controls. */
  transformControls: boolean;
  /** Effect/transition/keyframe pickers and rows. */
  effectControls: boolean;
  captionsPanel: boolean;
  templates: boolean;
  /** Canvas size/FPS/background section when no clip is selected. */
  canvasControls: boolean;
  addTrack: boolean;
  addTextClip: boolean;
}

export interface EditorModeDefinition {
  key: EditorModeKey;
  label: string;
  description: string;
  features: EditorModeFeatures;
}

export const EDITOR_MODES: EditorModeDefinition[] = [
  {
    key: 'full',
    label: 'Full editor',
    description: 'Every editing feature.',
    features: {
      assistant: true,
      transformControls: true,
      effectControls: true,
      captionsPanel: true,
      templates: true,
      canvasControls: true,
      addTrack: true,
      addTextClip: true,
    },
  },
  {
    key: 'simple_trim',
    label: 'Simple trim',
    description: 'Arrange, trim, and export — styling tools hidden.',
    features: {
      assistant: false,
      transformControls: false,
      effectControls: false,
      captionsPanel: false,
      templates: false,
      canvasControls: false,
      addTrack: false,
      addTextClip: false,
    },
  },
  {
    key: 'captions_only',
    label: 'Captions only',
    description: 'Focused caption editing over the existing timeline.',
    features: {
      assistant: false,
      transformControls: false,
      effectControls: false,
      captionsPanel: true,
      templates: false,
      canvasControls: false,
      addTrack: false,
      addTextClip: false,
    },
  },
  {
    key: 'social_clip',
    label: 'Social clip',
    description: 'Quick social cuts: assistant, captions, templates, and titles.',
    features: {
      assistant: true,
      transformControls: true,
      effectControls: false,
      captionsPanel: true,
      templates: true,
      canvasControls: true,
      addTrack: false,
      addTextClip: true,
    },
  },
];

export function editorModeFeatures(key: string): EditorModeFeatures {
  return (EDITOR_MODES.find((mode) => mode.key === key) || EDITOR_MODES[0]).features;
}
