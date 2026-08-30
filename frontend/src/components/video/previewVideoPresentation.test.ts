import { describe, expect, it } from 'vitest';
import {
  previewVideoPresentationMediaTimeMatches,
  previewVideoPresentationToken,
} from './previewVideoPresentation';

describe('previewVideoPresentationToken', () => {
  it('binds clip, canonical frame, and canonical source time deterministically', () => {
    expect(previewVideoPresentationToken('clip alpha/video', 15, 500)).toBe(
      'preview-video-presentation-v1:clip%20alpha%2Fvideo:15:500.000000',
    );
  });

  it('rejects invalid frame identities instead of creating ambiguous tokens', () => {
    expect(() => previewVideoPresentationToken('', 0, 0)).toThrow('clip id');
    expect(() => previewVideoPresentationToken('clip', -1, 0)).toThrow('frame index');
    expect(() => previewVideoPresentationToken('clip', 0.5, 0)).toThrow('frame index');
    expect(() => previewVideoPresentationToken('clip', 0, Number.NaN)).toThrow('source time');
  });
});

describe('previewVideoPresentationMediaTimeMatches', () => {
  it('accepts only the presented source timestamp inside the deterministic tolerance', () => {
    expect(previewVideoPresentationMediaTimeMatches(500, 0.5, 0.0005)).toBe(true);
    expect(previewVideoPresentationMediaTimeMatches(500, 0.5004, 0.0005)).toBe(true);
    expect(previewVideoPresentationMediaTimeMatches(500, 0.501, 0.0005)).toBe(false);
  });

  it('fails closed for invalid timestamps or tolerance', () => {
    expect(previewVideoPresentationMediaTimeMatches(-1, 0, 0.0005)).toBe(false);
    expect(previewVideoPresentationMediaTimeMatches(0, Number.NaN, 0.0005)).toBe(false);
    expect(previewVideoPresentationMediaTimeMatches(0, 0, -1)).toBe(false);
  });
});
