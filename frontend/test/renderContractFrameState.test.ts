import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateVisualFrameState,
  frameRelativeMilliseconds,
  VISUAL_FRAME_STATE_CONTRACT_V1,
  type Matrix4,
} from '../src/video/renderContractFrameState';
import { MEDIA_GEOMETRY_CONTRACT_V1 } from '../src/video/renderContractMediaGeometry';
import { samplePropertyKeyframes } from '../src/video/renderContractProperties';
import type { TimelineV2Document } from '../src/video/renderContractTypes';

interface VisualFrameStateFixture {
  version: number;
  document: TimelineV2Document;
  cases: Array<{
    name: string;
    frame_index: number;
    active_scene_id: string;
    expected_source_time_ms: number;
    expected_transform_x: number;
    expected_view_x: number;
    expected_opacity: number;
    expected_camera_x: number;
    expected_camera_fov: number;
    expected_perspective_distance: number;
    expected_model_matrix: Matrix4;
  }>;
  unresolved_document: TimelineV2Document;
  expected_unresolved: string[];
}

function loadFixture(): VisualFrameStateFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/visual-frame-state-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as VisualFrameStateFixture;
}

describe('canonical visual FrameState', () => {
  const fixture = loadFixture();

  it('uses the versioned shared fixture', () => {
    expect(fixture.version).toBe(1);
  });

  for (const sample of fixture.cases) {
    it(`matches ${sample.name}`, () => {
      const state = evaluateVisualFrameState(fixture.document, sample.frame_index);
      expect(state.contract_version).toBe(VISUAL_FRAME_STATE_CONTRACT_V1);
      expect(state.active_scene_id ?? '').toBe(sample.active_scene_id);
      expect(state.authoritative).toBe(true);
      expect(state.unresolved).toEqual([]);
      expect(state.layers).toHaveLength(1);
      const layer = state.layers[0];
      expect(layer.clip_id).toBe('media');
      expect(layer.source_time_ms).toBeCloseTo(sample.expected_source_time_ms, 9);
      expect(layer.transform.x).toBeCloseTo(sample.expected_transform_x, 9);
      expect(layer.view_transform.x).toBeCloseTo(sample.expected_view_x, 9);
      expect(layer.transform.opacity).toBeCloseTo(sample.expected_opacity, 9);
      expect(state.camera.x).toBeCloseTo(sample.expected_camera_x, 9);
      expect(state.camera.field_of_view).toBeCloseTo(sample.expected_camera_fov, 9);
      expect(state.camera.perspective_distance).toBeCloseTo(sample.expected_perspective_distance, 9);
      layer.model_matrix.forEach((value, index) => expect(value).toBeCloseTo(sample.expected_model_matrix[index], 9));
      expect(layer.content_bounds).toEqual({ x: 0, y: 0, width: 200, height: 100 });
      expect(layer.media_geometry?.contract_version).toBe(MEDIA_GEOMETRY_CONTRACT_V1);
      expect(layer.media_geometry?.painted_bounds).toEqual({ x: 0, y: 0, width: 200, height: 100 });
      expect(layer.transform.crop).toEqual({ top: 0.1, right: 0.2, bottom: 0, left: 0 });
    });
  }

  it('surfaces unresolved paint semantics and missing source provenance instead of claiming authority', () => {
    const state = evaluateVisualFrameState(fixture.unresolved_document, 0);
    expect(state.authoritative).toBe(false);
    expect(state.unresolved).toEqual(fixture.expected_unresolved);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].authoritative).toBe(false);
    expect(state.layers[0].content_bounds).toBeUndefined();
    expect(state.layers[0].media_geometry).toBeUndefined();
  });

  it('keeps fractional frame presentation time for property sampling', () => {
    const timeMs = frameRelativeMilliseconds(1, 120, 5);
    expect(timeMs).toBeCloseTo(3.3333333333333335, 12);
    expect(samplePropertyKeyframes([
      { property: 'x', time_ms: 0, value: 10 },
      { property: 'x', time_ms: 10, value: 20 },
    ], 'x', timeMs)).toBeCloseTo(13.333333333333334, 9);
  });
});
