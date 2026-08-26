import { describe, expect, it } from 'vitest';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import type { CanonicalEvaluatedCursorState } from '../../video/renderContractCursor';
import type { CanonicalEvaluatedShapeState } from '../../video/renderContractShape';
import type { CanonicalEvaluatedTextState } from '../../video/renderContractText';
import {
  resolveCanonicalPreviewCursorGeometry,
  resolveCanonicalPreviewShapeGeometry,
  resolveCanonicalPreviewTextPaint,
  resolvePreviewCanonicalPainterPlan,
} from './PreviewCanonicalPainters';

function canonicalState(
  values: Partial<Pick<CanonicalFrameLayerState, 'text' | 'shape' | 'cursor'>>,
): Pick<CanonicalFrameLayerState, 'text' | 'shape' | 'cursor'> {
  return values;
}

function textState(overrides: Partial<CanonicalEvaluatedTextState> = {}): CanonicalEvaluatedTextState {
  return {
    contract_version: 'text-state-v1',
    text: 'Title',
    font_family: 'Inter',
    font_family_source: 'authored',
    font_face_source: 'family-name-only',
    font_size: 48,
    font_weight: '600',
    color: '#abcdef',
    background: 'rgba(1,2,3,0.5)',
    stroke: '#123456',
    stroke_width: 3,
    shadow: { offset_x: 2, offset_y: 3, blur_radius: 4, color: 'rgba(0,0,0,0.7)' },
    text_align: 'left',
    vertical_align: 'top',
    line_height_mode: 'multiplier',
    line_height: 1.4,
    letter_spacing: 2.5,
    border_radius: 12,
    box_width: 400,
    box_height: 120,
    padding: { top: 4, right: 5, bottom: 6, left: 7 },
    ...overrides,
  };
}

function shapeState(overrides: Partial<CanonicalEvaluatedShapeState> = {}): CanonicalEvaluatedShapeState {
  return {
    contract_version: 'shape-state-v1',
    kind: 'rounded_rectangle',
    width: 400,
    height: 120,
    fill: '#112233',
    stroke: '#abcdef',
    stroke_width: 6,
    blur_radius: 12,
    corner_radius: 18,
    ...overrides,
  };
}

function cursorState(overrides: Partial<CanonicalEvaluatedCursorState> = {}): CanonicalEvaluatedCursorState {
  return {
    contract_version: 'cursor-state-v1',
    visible: true,
    scale: 1.5,
    highlight: true,
    click_rings: true,
    x: 120,
    y: 80,
    click: true,
    ...overrides,
  };
}

describe('canonical deterministic preview painter planning', () => {
  it('keeps the authored painter path when canonical FrameState is unavailable', () => {
    expect(resolvePreviewCanonicalPainterPlan(undefined, {
      hasMediaBase: false,
      hasShape: false,
      hasText: true,
      hasCursor: true,
    })).toEqual({ mode: 'legacy-authored', content: 'legacy', cursor: 'legacy' });
  });

  it('treats canonical omission as authoritative for authored text and cursor paint', () => {
    expect(resolvePreviewCanonicalPainterPlan(canonicalState({}), {
      hasMediaBase: false,
      hasShape: false,
      hasText: true,
      hasCursor: true,
    })).toEqual({ mode: 'canonical-frame', content: 'canonical-omit', cursor: 'canonical-omit' });
  });

  it('preserves established shape-before-text precedence while consuming both canonical states', () => {
    const text = textState();
    const shape = shapeState();
    expect(resolvePreviewCanonicalPainterPlan(canonicalState({ text, shape }), {
      hasMediaBase: false,
      hasShape: true,
      hasText: true,
      hasCursor: false,
    })).toEqual({ mode: 'canonical-frame', content: 'canonical-shape', cursor: 'none' });
  });

  it('leaves media as the base painter but independently canonicalizes cursor paint', () => {
    const cursor = cursorState();
    expect(resolvePreviewCanonicalPainterPlan(canonicalState({ text: textState(), cursor }), {
      hasMediaBase: true,
      hasShape: false,
      hasText: true,
      hasCursor: true,
    })).toEqual({ mode: 'canonical-frame', content: 'legacy', cursor: 'canonical-cursor' });
  });

  it('fails closed when canonical painter state is bound to no authored source metadata', () => {
    expect(() => resolvePreviewCanonicalPainterPlan(canonicalState({ text: textState() }), {
      hasMediaBase: false,
      hasShape: false,
      hasText: false,
      hasCursor: false,
    })).toThrow(/not bound to authored text/);
  });
});

describe('canonical text painter inputs', () => {
  it('maps evaluated text-state-v1 fields without re-defaulting authored values', () => {
    const paint = resolveCanonicalPreviewTextPaint(textState(), 0.5);
    expect(paint.boxMode).toBe('canonical-explicit-box');
    expect(paint.metricsMode).toBe('browser-intrinsic-deferred');
    expect(paint.style).toMatchObject({
      fontSize: 24,
      fontFamily: 'Inter',
      fontWeight: '600',
      color: '#abcdef',
      background: 'rgba(1,2,3,0.5)',
      borderRadius: 6,
      textAlign: 'left',
      lineHeight: 1.4,
      letterSpacing: 1.25,
      WebkitTextStroke: '1.5px #123456',
      textShadow: '1px 1.5px 2px rgba(0,0,0,0.7)',
      padding: '2px 2.5px 3px 3.5px',
      width: 200,
      height: 60,
      boxSizing: 'border-box',
      display: 'flex',
      alignItems: 'flex-start',
      justifyContent: 'flex-start',
    });
  });

  it('keeps intrinsic glyph metrics explicit debt when no canonical box is authored', () => {
    const paint = resolveCanonicalPreviewTextPaint(textState({
      box_width: undefined,
      box_height: undefined,
      line_height_mode: 'normal',
      line_height: undefined,
      vertical_align: 'middle',
      text_align: 'center',
    }), 1);
    expect(paint.boxMode).toBe('intrinsic-size');
    expect(paint.metricsMode).toBe('browser-intrinsic-deferred');
    expect(paint.style.lineHeight).toBe('normal');
    expect(paint.style.width).toBeUndefined();
    expect(paint.style.height).toBeUndefined();
  });

  it('rejects malformed contract/version combinations rather than falling back', () => {
    expect(() => resolveCanonicalPreviewTextPaint({
      ...textState(),
      contract_version: 'text-state-v0' as never,
    }, 1)).toThrow(/text-state-v1/);
    expect(() => resolveCanonicalPreviewTextPaint(textState({
      line_height_mode: 'multiplier',
      line_height: undefined,
    }), 1)).toThrow(/requires a value/);
  });
});

describe('canonical shape and cursor painter inputs', () => {
  it('scales evaluated shape dimensions and style parameters exactly once', () => {
    expect(resolveCanonicalPreviewShapeGeometry(shapeState(), 0.5)).toEqual({
      width: 200,
      height: 60,
      strokeWidth: 3,
      cornerRadius: 9,
      blurRadius: 6,
    });
  });

  it('uses exact sampled cursor position, scale, and click state geometry', () => {
    expect(resolveCanonicalPreviewCursorGeometry(cursorState(), 0.5)).toEqual({
      left: 60,
      top: 40,
      size: 48,
    });
  });

  it('fails closed on painter contract drift or invalid stage scale', () => {
    expect(() => resolveCanonicalPreviewShapeGeometry({
      ...shapeState(),
      contract_version: 'shape-state-v0' as never,
    }, 1)).toThrow(/shape-state-v1/);
    expect(() => resolveCanonicalPreviewCursorGeometry({
      ...cursorState(),
      contract_version: 'cursor-state-v0' as never,
    }, 1)).toThrow(/cursor-state-v1/);
    expect(() => resolveCanonicalPreviewCursorGeometry(cursorState(), Number.NaN)).toThrow(/stage scale/);
  });
});
