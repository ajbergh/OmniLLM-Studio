import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateVisualFrameStateForRenderManifest } from '../src/video/renderContractFrameState';
import {
  evaluateSourceProvenance,
  SOURCE_PROVENANCE_CONTRACT_V1,
  type CanonicalSourceProvenance,
} from '../src/video/renderContractSourceProvenance';
import type { Matrix4 } from '../src/video/renderContractFrameState';
import type { RenderManifestV1 } from '../src/video/renderContractTypes';

interface SourceProvenanceFixture {
  version: number;
  manifest: RenderManifestV1;
  expected_source_provenance: CanonicalSourceProvenance;
  expected_model_matrix: Matrix4;
}

function loadFixture(): SourceProvenanceFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/source-provenance-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as SourceProvenanceFixture;
}

function cloneManifest(manifest: RenderManifestV1): RenderManifestV1 {
  return JSON.parse(JSON.stringify(manifest)) as RenderManifestV1;
}

describe('canonical source provenance', () => {
  const fixture = loadFixture();

  it('uses the versioned shared fixture', () => {
    expect(fixture.version).toBe(1);
  });

  it('projects the immutable media probe', () => {
    expect(evaluateSourceProvenance(fixture.manifest)).toEqual([fixture.expected_source_provenance]);
  });

  it('feeds immutable source bounds into FrameState geometry and anchors', () => {
    const state = evaluateVisualFrameStateForRenderManifest(fixture.manifest, 0);
    expect(state.authoritative).toBe(true);
    expect(state.unresolved).toEqual([]);
    expect(state.layers).toHaveLength(1);
    const layer = state.layers[0];
    expect(layer.source_provenance).toEqual(fixture.expected_source_provenance);
    expect(layer.source_provenance?.contract_version).toBe(SOURCE_PROVENANCE_CONTRACT_V1);
    expect(layer.content_bounds).toEqual(fixture.expected_source_provenance.source_bounds);
    expect(layer.media_geometry?.source_bounds).toEqual(fixture.expected_source_provenance.source_bounds);
    expect(layer.model_matrix).toEqual(fixture.expected_model_matrix);
  });

  it('keeps authored content bounds ahead of a source probe', () => {
    const manifest = cloneManifest(fixture.manifest);
    const explicitBounds = { x: 5, y: 6, width: 50, height: 25 };
    manifest.timeline.tracks[0].clips[0].content_bounds = explicitBounds;
    const layer = evaluateVisualFrameStateForRenderManifest(manifest, 0).layers[0];
    expect(layer.content_bounds).toEqual(explicitBounds);
    expect(layer.media_geometry?.source_bounds).toEqual(explicitBounds);
    expect(layer.source_provenance).toEqual(fixture.expected_source_provenance);
  });

  it('leaves a missing manifest probe as explicit FrameState debt', () => {
    const manifest = cloneManifest(fixture.manifest);
    manifest.assets = [];
    const state = evaluateVisualFrameStateForRenderManifest(manifest, 0);
    expect(state.authoritative).toBe(false);
    expect(state.unresolved).toEqual([
      'visual-clip:content_bounds_for_anchor',
      'visual-clip:media_geometry:source_provenance',
    ]);
    expect(state.layers[0]).not.toHaveProperty('content_bounds');
    expect(state.layers[0]).not.toHaveProperty('source_provenance');
    expect(state.layers[0]).not.toHaveProperty('media_geometry');
  });

  it('fails closed when the immutable source is not bound to the active clip', () => {
    const manifest = cloneManifest(fixture.manifest);
    manifest.assets[0].clip_ids = ['other-clip'];
    expect(() => evaluateVisualFrameStateForRenderManifest(manifest, 0)).toThrow('source provenance asset "visual-source" does not bind clip "visual-clip"');
  });

  it('fails closed on partial media dimensions', () => {
    const manifest = cloneManifest(fixture.manifest);
    manifest.assets[0].media = { width: 400 };
    expect(() => evaluateSourceProvenance(manifest)).toThrow('source provenance asset "visual-source" must provide both media width and height');
  });

  it('fails closed on non-positive media dimensions', () => {
    const manifest = cloneManifest(fixture.manifest);
    manifest.assets[0].media = { width: 0, height: 200 };
    expect(() => evaluateSourceProvenance(manifest)).toThrow('source provenance asset "visual-source" media width and height must be positive integers');
  });
});
