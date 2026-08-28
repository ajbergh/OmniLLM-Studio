import { describe, expect, it } from 'vitest';
import {
  buildPreviewTextLayoutSnapshot,
  previewTextLayoutSnapshotStable,
  type PreviewTextLayoutMeasurement,
} from './previewTextLayoutSnapshot';

function measurement(overrides: Partial<PreviewTextLayoutMeasurement> = {}): PreviewTextLayoutMeasurement {
  return {
    text: 'hello world',
    boxMode: 'intrinsic-size',
    fontFaceSource: 'family-name-only',
    fontFaceRuntime: 'editor-resource-loaded',
    fontFamily: 'OmniLLMPreview_inter_asset_600',
    fontWeight: '600',
    fontSizePx: 24,
    lineHeightPx: 30,
    letterSpacingPx: 1,
    textAlign: 'center',
    whiteSpace: 'pre-wrap',
    paddingTopPx: 4,
    paddingRightPx: 8,
    paddingBottomPx: 4,
    paddingLeftPx: 8,
    borderBoxWidthPx: 120,
    borderBoxHeightPx: 40,
    authoredWidth: false,
    authoredHeight: false,
    lineFragmentCount: 1,
    ...overrides,
  };
}

describe('preview text layout snapshot', () => {
  it('normalizes Chromium measurements into canonical canvas pixels', () => {
    const snapshot = buildPreviewTextLayoutSnapshot(measurement(), 0.5);
    expect(snapshot).toMatchObject({
      contract_version: 'preview-text-layout-snapshot-v1',
      box_mode: 'intrinsic-size',
      border_box_width: 240,
      border_box_height: 80,
      font_size: 48,
      line_height: 60,
      letter_spacing: 2,
      padding: { top: 8, right: 16, bottom: 8, left: 16 },
      authored_width: false,
      authored_height: false,
      line_fragment_count: 1,
      hard_line_count: 1,
      wraps_soft_lines: false,
    });
    expect(snapshot.input_fingerprint).toMatch(/^[0-9a-f]{8}$/);
  });

  it('preserves Chromium normal line-height without inventing a numeric multiplier', () => {
    const snapshot = buildPreviewTextLayoutSnapshot(measurement({ lineHeightPx: undefined }), 1);
    expect(snapshot.line_height).toBe('normal');
  });

  it('diagnoses soft wrapping separately from authored hard lines', () => {
    const snapshot = buildPreviewTextLayoutSnapshot(measurement({
      text: 'first line\nsecond line wraps',
      lineFragmentCount: 3,
      authoredWidth: true,
      boxMode: 'canonical-explicit-box',
    }), 1);
    expect(snapshot.hard_line_count).toBe(2);
    expect(snapshot.line_fragment_count).toBe(3);
    expect(snapshot.wraps_soft_lines).toBe(true);
    expect(snapshot.authored_width).toBe(true);
  });

  it('keeps the input fingerprint invariant across proportional stage scaling', () => {
    const full = buildPreviewTextLayoutSnapshot(measurement(), 1);
    const half = buildPreviewTextLayoutSnapshot(measurement({
      fontSizePx: 12,
      lineHeightPx: 15,
      letterSpacingPx: 0.5,
      paddingTopPx: 2,
      paddingRightPx: 4,
      paddingBottomPx: 2,
      paddingLeftPx: 4,
      borderBoxWidthPx: 60,
      borderBoxHeightPx: 20,
    }), 0.5);
    expect(half.input_fingerprint).toBe(full.input_fingerprint);
    expect(half.border_box_width).toBe(full.border_box_width);
    expect(half.border_box_height).toBe(full.border_box_height);
  });

  it('requires a stable second Chromium pass after intrinsic dimensions are frozen', () => {
    const before = buildPreviewTextLayoutSnapshot(measurement(), 1);
    expect(previewTextLayoutSnapshotStable(before, before)).toBe(true);
    expect(previewTextLayoutSnapshotStable(before, {
      border_box_width: before.border_box_width + 0.02,
      border_box_height: before.border_box_height,
      line_fragment_count: before.line_fragment_count,
    })).toBe(false);
    expect(previewTextLayoutSnapshotStable(before, {
      border_box_width: before.border_box_width,
      border_box_height: before.border_box_height,
      line_fragment_count: before.line_fragment_count + 1,
    })).toBe(false);
  });

  it('fails closed on invalid scale or negative browser measurements', () => {
    expect(() => buildPreviewTextLayoutSnapshot(measurement(), 0)).toThrow(/stage scale/);
    expect(() => buildPreviewTextLayoutSnapshot(measurement({ borderBoxWidthPx: -1 }), 1)).toThrow(/finite and non-negative/);
  });
});
