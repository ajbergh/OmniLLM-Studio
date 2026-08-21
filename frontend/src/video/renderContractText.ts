import type { TimelineV2Text } from './renderContractTypes';

export const TEXT_STATE_CONTRACT_V1 = 'text-state-v1' as const;
export const TEXT_FONT_FAMILY_SOURCE_AUTHORED = 'authored' as const;
export const TEXT_FONT_FAMILY_SOURCE_COMPOSITION_DEFAULT = 'composition-default' as const;
export const TEXT_LINE_HEIGHT_NORMAL = 'normal' as const;
export const TEXT_LINE_HEIGHT_MULTIPLIER = 'multiplier' as const;

// Font-face provenance for evaluated text. A packaged resource is the only
// deterministic face; a family name alone stays renderer-dependent until a
// packaged face binds it.
export const TEXT_FONT_FACE_SOURCE_PACKAGED_RESOURCE = 'packaged-resource' as const;
export const TEXT_FONT_FACE_SOURCE_FAMILY_NAME_ONLY = 'family-name-only' as const;
export const TEXT_FONT_FACE_SOURCE_COMPOSITION_DEFAULT = 'composition-default' as const;

export interface CanonicalEvaluatedTextPadding {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export interface CanonicalEvaluatedTextShadow {
  offset_x: number;
  offset_y: number;
  blur_radius: number;
  color: string;
}

export interface CanonicalEvaluatedTextState {
  contract_version: typeof TEXT_STATE_CONTRACT_V1;
  text: string;
  font_family: string;
  font_family_source: typeof TEXT_FONT_FAMILY_SOURCE_AUTHORED | typeof TEXT_FONT_FAMILY_SOURCE_COMPOSITION_DEFAULT;
  font_resource_id?: string;
  font_face_source:
    | typeof TEXT_FONT_FACE_SOURCE_PACKAGED_RESOURCE
    | typeof TEXT_FONT_FACE_SOURCE_FAMILY_NAME_ONLY
    | typeof TEXT_FONT_FACE_SOURCE_COMPOSITION_DEFAULT;
  font_size: number;
  font_weight: string;
  color: string;
  background?: string;
  stroke?: string;
  stroke_width: number;
  shadow?: CanonicalEvaluatedTextShadow;
  text_align: 'left' | 'center' | 'right';
  vertical_align: 'top' | 'middle' | 'bottom';
  line_height_mode: typeof TEXT_LINE_HEIGHT_NORMAL | typeof TEXT_LINE_HEIGHT_MULTIPLIER;
  line_height?: number;
  letter_spacing: number;
  border_radius: number;
  box_width?: number;
  box_height?: number;
  padding: CanonicalEvaluatedTextPadding;
}

/**
 * Normalize Timeline v2 text into renderer-independent static text state.
 * Timeline v2 currently defines no text-style keyframe properties, so this
 * evaluator intentionally does not manufacture text animation semantics.
 */
export function evaluateTextState(text: TimelineV2Text | undefined, canvasHeight: number): CanonicalEvaluatedTextState | undefined {
  if (!text) return undefined;
  const fontFamily = text.font_family?.trim() ?? '';
  const fontResourceID = text.font_resource_id?.trim() ?? '';
  if (fontResourceID && text.font_resource_id !== fontResourceID) {
    throw new Error(`canonical text font_resource_id ${JSON.stringify(text.font_resource_id)} must not have surrounding whitespace`);
  }
  const background = text.background?.trim() ?? '';
  const stroke = text.stroke?.trim() ?? '';
  const rawTextAlign = text.text_align?.trim().toLowerCase() ?? '';
  const rawVerticalAlign = text.vertical_align?.trim().toLowerCase() ?? '';
  const textAlign = rawTextAlign || 'center';
  const verticalAlign = rawVerticalAlign || 'middle';
  if (textAlign !== 'left' && textAlign !== 'center' && textAlign !== 'right') {
    throw new Error(`unsupported canonical text_align ${JSON.stringify(textAlign)}`);
  }
  if (verticalAlign !== 'top' && verticalAlign !== 'middle' && verticalAlign !== 'bottom') {
    throw new Error(`unsupported canonical vertical_align ${JSON.stringify(verticalAlign)}`);
  }

  let fontSize = Math.round(canvasHeight / 18);
  if (text.font_size !== undefined) {
    if (!Number.isFinite(text.font_size) || text.font_size < 0) throw new Error('canonical text font_size must be non-negative');
    if (text.font_size > 0) fontSize = text.font_size;
  }
  const state: CanonicalEvaluatedTextState = {
    contract_version: TEXT_STATE_CONTRACT_V1,
    text: text.text,
    font_family: fontFamily,
    font_family_source: fontFamily ? TEXT_FONT_FAMILY_SOURCE_AUTHORED : TEXT_FONT_FAMILY_SOURCE_COMPOSITION_DEFAULT,
    // Font-face provenance is resolved by the caller when manifest font
    // resources are available; without them a family name alone stays
    // renderer-dependent and never claims a packaged face.
    font_face_source: fontFamily ? TEXT_FONT_FACE_SOURCE_FAMILY_NAME_ONLY : TEXT_FONT_FACE_SOURCE_COMPOSITION_DEFAULT,
    ...(fontResourceID ? { font_resource_id: fontResourceID } : {}),
    font_size: fontSize,
    font_weight: text.font_weight?.trim() || '700',
    color: text.color?.trim() || '#ffffff',
    ...(background ? { background } : {}),
    ...(stroke ? { stroke } : {}),
    stroke_width: 0,
    text_align: textAlign,
    vertical_align: verticalAlign,
    line_height_mode: TEXT_LINE_HEIGHT_NORMAL,
    letter_spacing: 0,
    border_radius: 0,
    padding: background ? { top: 8, right: 18, bottom: 8, left: 18 } : { top: 0, right: 0, bottom: 0, left: 0 },
  };

  if (text.line_height !== undefined) {
    requireFiniteNonNegative('line_height', text.line_height);
    if (text.line_height > 0) {
      state.line_height_mode = TEXT_LINE_HEIGHT_MULTIPLIER;
      state.line_height = text.line_height;
    }
  }
  if (text.letter_spacing !== undefined) {
    requireFinite('letter_spacing', text.letter_spacing);
    state.letter_spacing = text.letter_spacing;
  }
  if (text.border_radius !== undefined) {
    requireFiniteNonNegative('border_radius', text.border_radius);
    state.border_radius = text.border_radius;
  }
  if (text.box_width !== undefined) {
    requireFinitePositive('box_width', text.box_width);
    state.box_width = text.box_width;
  }
  if (text.box_height !== undefined) {
    requireFinitePositive('box_height', text.box_height);
    state.box_height = text.box_height;
  }

  if (stroke) {
    state.stroke_width = 2;
    if (text.stroke_width !== undefined) {
      requireFiniteNonNegative('stroke_width', text.stroke_width);
      if (text.stroke_width > 0) state.stroke_width = text.stroke_width;
    }
  } else if (text.stroke_width !== undefined) {
    requireFiniteNonNegative('stroke_width', text.stroke_width);
  }
  if (text.shadow) {
    state.shadow = { offset_x: 2, offset_y: 2, blur_radius: 4, color: 'rgba(0,0,0,0.7)' };
  }

  const padding: Array<[keyof CanonicalEvaluatedTextPadding, string, number | undefined]> = [
    ['top', 'padding_top', text.padding_top],
    ['right', 'padding_right', text.padding_right],
    ['bottom', 'padding_bottom', text.padding_bottom],
    ['left', 'padding_left', text.padding_left],
  ];
  for (const [side, name, value] of padding) {
    if (value === undefined) continue;
    requireFiniteNonNegative(name, value);
    state.padding[side] = value;
  }
  return state;
}

function requireFinite(name: string, value: number): void {
  if (!Number.isFinite(value)) throw new Error(`canonical text ${name} must be finite`);
}
function requireFiniteNonNegative(name: string, value: number): void {
  requireFinite(name, value);
  if (value < 0) throw new Error(`canonical text ${name} must be non-negative`);
}
function requireFinitePositive(name: string, value: number): void {
  requireFinite(name, value);
  if (value <= 0) throw new Error(`canonical text ${name} must be positive`);
}
