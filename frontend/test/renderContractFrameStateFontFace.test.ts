import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateVisualFrameState, evaluateVisualFrameStateForRenderManifest } from '../src/video/renderContractFrameState';
import type { RenderManifestV1, TimelineV2Document, TimelineV2Text } from '../src/video/renderContractTypes';

interface FontResourceProvenanceFixture {
  version: number;
  manifest: RenderManifestV1;
}

function loadFontResourceFixture(): FontResourceProvenanceFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/font-resource-provenance-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as FontResourceProvenanceFixture;
}

function textFrameStateDocument(text: TimelineV2Text): TimelineV2Document {
  return {
    version: 2,
    canvas: { width: 200, height: 100, fps: 30, background: '#000000' },
    duration_ms: 1000,
    metadata: {},
    markers: [],
    scenes: [],
    tracks: [{
      id: 'track-1', type: 'layer', name: 'Track 1', locked: false, muted: false, visible: true,
      clips: [{
        id: 'text-clip', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
        text,
        effects: [], keyframes: [],
      }],
    }],
  };
}

describe('canonical font-face binding in FrameState', () => {
  it('resolves an authored font_resource_id to a packaged face', () => {
    const fixture = loadFontResourceFixture();
    fixture.manifest.timeline = textFrameStateDocument({ text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' });
    const state = evaluateVisualFrameStateForRenderManifest(fixture.manifest, 0);
    expect(state.layers).toHaveLength(1);
    expect(state.layers[0].text?.font_resource_id).toBe('inter-400-normal');
    expect(state.layers[0].text?.font_face_source).toBe('packaged-resource');
  });

  it('fails closed when the manifest does not package the named resource', () => {
    const fixture = loadFontResourceFixture();
    fixture.manifest.timeline = textFrameStateDocument({ text: 'Title', font_family: 'Inter', font_resource_id: 'missing-resource' });
    expect(() => evaluateVisualFrameStateForRenderManifest(fixture.manifest, 0))
      .toThrow(/names font resource "missing-resource" that the manifest does not package/);
  });

  it('fails closed when the authored family conflicts with the packaged family', () => {
    const fixture = loadFontResourceFixture();
    fixture.manifest.timeline = textFrameStateDocument({ text: 'Title', font_family: 'Roboto', font_resource_id: 'inter-400-normal' });
    expect(() => evaluateVisualFrameStateForRenderManifest(fixture.manifest, 0))
      .toThrow(/with family "Inter" but authors family "Roboto"/);
  });

  it('fails closed on timeline-only evaluation with an unverifiable font_resource_id', () => {
    expect(() => evaluateVisualFrameState(textFrameStateDocument({ text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' }), 0))
      .toThrow(/names font resource "inter-400-normal" that the manifest does not package/);
  });
});
