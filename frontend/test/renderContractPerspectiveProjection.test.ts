import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  evaluatePerspectiveProjection,
  PERSPECTIVE_PROJECTION_CONTRACT_V1,
} from '../src/video/renderContractPerspectiveProjection';
import type { Matrix4 } from '../src/video/renderContractFrameState';

interface FixtureCase {
  name: string;
  camera_distance: number;
  clip_perspective?: number;
  view_z: number;
  expected_source: 'camera' | 'clip';
  expected_distance: number;
  expected_origin_w: number;
  expected_matrix: Matrix4;
}
interface Fixture { version: number; cases: FixtureCase[]; }

const fixturePath = fileURLToPath(new URL('../../video-renderer/test/fixtures/perspective-projection-v1.json', import.meta.url));
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as Fixture;

describe('canonical perspective projection', () => {
  it('matches the shared Go/TypeScript fixture', () => {
    expect(fixture.version).toBe(1);
    for (const sample of fixture.cases) {
      const projection = evaluatePerspectiveProjection(
        { perspective_distance: sample.camera_distance },
        { z: sample.view_z, ...(sample.clip_perspective !== undefined ? { perspective: sample.clip_perspective } : {}) },
      );
      expect(projection.contract_version).toBe(PERSPECTIVE_PROJECTION_CONTRACT_V1);
      expect(projection.source).toBe(sample.expected_source);
      expect(projection.distance).toBeCloseTo(sample.expected_distance, 12);
      expect(projection.origin_w).toBeCloseTo(sample.expected_origin_w, 12);
      projection.matrix.forEach((value, index) => {
        expect(value).toBeCloseTo(sample.expected_matrix[index], 12);
      });
    }
  });

  it('fails closed for a negative clip perspective', () => {
    expect(() => evaluatePerspectiveProjection(
      { perspective_distance: 1000 },
      { z: 0, perspective: -100 },
    )).toThrow(/finite and positive/);
  });
});
