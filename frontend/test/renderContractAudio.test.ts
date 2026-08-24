import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { evaluateAudioGraphV1, type AudioGraphV1 } from '../src/video/renderContractAudio';
import type { RenderManifestV1 } from '../src/video/renderContractTypes';

interface AudioGraphFixture {
  manifest: RenderManifestV1;
  expected: AudioGraphV1;
}

function loadFixture(): AudioGraphFixture {
  return JSON.parse(
    readFileSync(new URL('../../backend/internal/video/rendercontract/testdata/audio_graph_v1.json', import.meta.url), 'utf8'),
  ) as AudioGraphFixture;
}

describe('canonical AudioGraph v1', () => {
  it('matches the shared Go/TypeScript fixture exactly', () => {
    const fixture = loadFixture();
    expect(evaluateAudioGraphV1(fixture.manifest)).toEqual(fixture.expected);
  });

  it('fails closed when a source channel layout has no v1 mapping', () => {
    const fixture = loadFixture();
    const manifest = structuredClone(fixture.manifest);
    if (!manifest.assets[0].media) throw new Error('fixture asset has no media probe');
    manifest.assets[0].media.channels = 6;
    expect(() => evaluateAudioGraphV1(manifest)).toThrow(/no canonical v1 channel mapping/);
  });

  it('fails closed on unknown program-processing fields', () => {
    const fixture = loadFixture();
    const manifest = structuredClone(fixture.manifest);
    const processing = manifest.timeline.metadata.render_audio_processing as Record<string, unknown>;
    processing.custom_curve = [0, 1];
    expect(() => evaluateAudioGraphV1(manifest)).toThrow(/unsupported field/);
  });

  it('records mute and solo suppression without dropping source identity', () => {
    const graph = evaluateAudioGraphV1(loadFixture().manifest);
    expect(Object.fromEntries(graph.sources.map((source) => [source.clip_id, source.suppression_reason]))).toMatchObject({
      'clip-muted': 'clip-muted',
      'clip-b': 'solo-suppressed',
      'clip-c': 'track-muted',
    });
  });
});
