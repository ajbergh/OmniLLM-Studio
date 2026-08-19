import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { evaluateMediaGeometry, MEDIA_GEOMETRY_CONTRACT_V1 } from '../src/video/renderContractMediaGeometry';
import type { TimelineV2Canvas, TimelineV2Clip, TimelineV2ContentBounds, TimelineV2Crop } from '../src/video/renderContractTypes';

interface FixtureCase {
  name: string;
  fit: 'contain' | 'cover' | 'fill' | 'none';
  content_bounds: TimelineV2ContentBounds;
  mask_source_crop?: TimelineV2Crop;
  transform_crop?: TimelineV2Crop;
  expected_scale_x: number;
  expected_scale_y: number;
  expected_visible_source_bounds: TimelineV2ContentBounds;
  expected_painted_bounds: TimelineV2ContentBounds;
  expected_clip_bounds: TimelineV2ContentBounds;
}
interface Fixture { version: number; canvas: TimelineV2Canvas; cases: FixtureCase[]; }

const fixturePath = fileURLToPath(new URL('../../video-renderer/test/fixtures/media-geometry-v1.json', import.meta.url));
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as Fixture;

function clipFor(sample: FixtureCase): TimelineV2Clip {
  return {
    id: 'media', asset_id: 'asset', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
    media_fit: sample.fit,
    content_bounds: sample.content_bounds,
    ...(sample.mask_source_crop ? { mask_source_crop: sample.mask_source_crop } : {}),
    transform: sample.transform_crop ? { crop: sample.transform_crop } : {},
    effects: [], keyframes: [],
  };
}

describe('canonical media geometry', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const geometry = evaluateMediaGeometry(fixture.canvas, clipFor(sample));
      expect(geometry.contract_version).toBe(MEDIA_GEOMETRY_CONTRACT_V1);
      expect(geometry.fit).toBe(sample.fit);
      expect(geometry.scale_x).toBeCloseTo(sample.expected_scale_x, 12);
      expect(geometry.scale_y).toBeCloseTo(sample.expected_scale_y, 12);
      expect(geometry.visible_source_bounds).toEqual(sample.expected_visible_source_bounds);
      expect(geometry.painted_bounds).toEqual(sample.expected_painted_bounds);
      expect(geometry.clip_bounds).toEqual(sample.expected_clip_bounds);
    }
  });

  it('fails closed when source bounds are absent', () => {
    const clip = clipFor(fixture.cases[0]);
    delete clip.content_bounds;
    expect(() => evaluateMediaGeometry(fixture.canvas, clip)).toThrow(/explicit content_bounds/);
  });
});
