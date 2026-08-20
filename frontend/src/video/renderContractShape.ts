import type { TimelineV2Shape } from './renderContractTypes';

export const SHAPE_STATE_CONTRACT_V1 = 'shape-state-v1' as const;

export const CANONICAL_SHAPE_KINDS = [
  'rectangle',
  'highlight',
  'blur',
  'rounded_rectangle',
  'ellipse',
  'arrow',
  'line',
  'speech_bubble',
  'spotlight',
  'pixelate',
  'checkmark',
  'x_mark',
  'step_marker',
  'label',
] as const;

export type CanonicalShapeKind = (typeof CANONICAL_SHAPE_KINDS)[number];

export interface CanonicalEvaluatedShapeState {
  contract_version: typeof SHAPE_STATE_CONTRACT_V1;
  kind: CanonicalShapeKind;
  width: number;
  height: number;
  fill: string;
  stroke: string;
  stroke_width: number;
  blur_radius: number;
  corner_radius: number;
}

/**
 * Normalize Timeline v2 shape metadata into renderer-independent annotation
 * state. An empty stroke means the kind has no default stroke. Legacy FFmpeg
 * approximations are intentionally not semantic authority.
 */
export function evaluateShapeState(shape: TimelineV2Shape | undefined): CanonicalEvaluatedShapeState | undefined {
  if (!shape) return undefined;
  const kind = shape.kind.trim().toLowerCase();
  if (!CANONICAL_SHAPE_KINDS.includes(kind as CanonicalShapeKind)) {
    throw new Error(`unsupported canonical shape kind ${JSON.stringify(kind)}`);
  }

  const canonicalKind = kind as CanonicalShapeKind;
  const width = shape.width ?? 320;
  const height = shape.height ?? 180;
  if (!Number.isFinite(width) || width <= 0) throw new Error('canonical shape width must be positive');
  if (!Number.isFinite(height) || height <= 0) throw new Error('canonical shape height must be positive');

  let strokeWidth = 6;
  if (shape.stroke_width !== undefined) {
    requireFiniteNonNegative('stroke_width', shape.stroke_width);
    if (shape.stroke_width > 0) strokeWidth = Math.max(1, shape.stroke_width);
  }
  let blurRadius = 12;
  if (shape.blur_radius !== undefined) {
    requireFiniteNonNegative('blur_radius', shape.blur_radius);
    if (shape.blur_radius > 0) blurRadius = Math.max(1, shape.blur_radius);
  }
  let cornerRadius = defaultCornerRadius(canonicalKind);
  if (shape.corner_radius !== undefined) {
    requireFiniteNonNegative('corner_radius', shape.corner_radius);
    if (shape.corner_radius > 0) cornerRadius = shape.corner_radius;
  }

  return {
    contract_version: SHAPE_STATE_CONTRACT_V1,
    kind: canonicalKind,
    width,
    height,
    fill: shape.fill?.trim() || defaultFill(canonicalKind),
    stroke: shape.stroke?.trim() || defaultStroke(canonicalKind),
    stroke_width: strokeWidth,
    blur_radius: blurRadius,
    corner_radius: cornerRadius,
  };
}

function defaultFill(kind: CanonicalShapeKind): string {
  switch (kind) {
    case 'highlight': return '#facc15';
    case 'spotlight': return 'rgba(0,0,0,0.6)';
    case 'step_marker': return '#2563eb';
    case 'speech_bubble': return '#ffffff';
    case 'label': return '#1e293b';
    default: return 'transparent';
  }
}

function defaultStroke(kind: CanonicalShapeKind): string {
  switch (kind) {
    case 'checkmark': return '#22c55e';
    case 'x_mark': return '#ef4444';
    case 'speech_bubble':
    case 'label': return '';
    default: return '#f59e0b';
  }
}

function defaultCornerRadius(kind: CanonicalShapeKind): number {
  switch (kind) {
    case 'rounded_rectangle': return 12;
    case 'speech_bubble': return 18;
    case 'label': return 10;
    default: return 0;
  }
}

function requireFiniteNonNegative(name: string, value: number): void {
  if (!Number.isFinite(value)) throw new Error(`canonical shape ${name} must be finite`);
  if (value < 0) throw new Error(`canonical shape ${name} must be non-negative`);
}
