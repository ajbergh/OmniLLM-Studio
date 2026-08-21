import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  evaluateFontResourceProvenance,
  FONT_RESOURCE_PROVENANCE_CONTRACT_V1,
  type CanonicalFontResourceProvenance,
} from '../src/video/renderContractFontResourceProvenance';
import type { RenderManifestV1 } from '../src/video/renderContractTypes';

interface FontResourceProvenanceFixture {
  version: number;
  manifest: RenderManifestV1;
  expected_font_resource_provenance: CanonicalFontResourceProvenance[];
}

function loadFixture(): FontResourceProvenanceFixture {
  const fixtureURL = new URL('../../video-renderer/test/fixtures/font-resource-provenance-v1.json', import.meta.url);
  return JSON.parse(readFileSync(fixtureURL, 'utf8')) as FontResourceProvenanceFixture;
}

function cloneManifest(manifest: RenderManifestV1): RenderManifestV1 {
  return JSON.parse(JSON.stringify(manifest)) as RenderManifestV1;
}

describe('canonical font resource provenance', () => {
  const fixture = loadFixture();

  it('uses the versioned shared fixture', () => {
    expect(fixture.version).toBe(1);
  });

  it('projects packaged static font faces in stable resource-id order', () => {
    expect(evaluateFontResourceProvenance(fixture.manifest)).toEqual(fixture.expected_font_resource_provenance);
    expect(evaluateFontResourceProvenance(fixture.manifest)[0].contract_version).toBe(FONT_RESOURCE_PROVENANCE_CONTRACT_V1);
  });

  it('allows a manifest with no packaged fonts without inventing a system fallback', () => {
    const manifest = cloneManifest(fixture.manifest);
    delete manifest.font_resources;
    expect(evaluateFontResourceProvenance(manifest)).toEqual([]);
  });

  it.each([
    ['noncanonical id', (manifest: RenderManifestV1) => { manifest.font_resources![0].font_resource_id = 'Inter 700'; }, 'font resource provenance font resource id "Inter 700" must use lowercase ASCII letters, digits, dots, underscores, or hyphens'],
    ['duplicate id', (manifest: RenderManifestV1) => { manifest.font_resources![1].font_resource_id = manifest.font_resources![0].font_resource_id; }, 'font resource provenance has duplicate font resource "inter-700-italic"'],
    ['surrounding family whitespace', (manifest: RenderManifestV1) => { manifest.font_resources![0].font_family = ' Inter'; }, 'font resource provenance font resource "inter-700-italic" font family " Inter" must not have surrounding whitespace'],
    ['unsupported style', (manifest: RenderManifestV1) => { (manifest.font_resources![0] as { font_style: string }).font_style = 'oblique'; }, 'font resource provenance font resource "inter-700-italic" has unsupported font style "oblique"'],
    ['variable-font face class', (manifest: RenderManifestV1) => { (manifest.font_resources![0] as { face_class: string }).face_class = 'variable'; }, 'font resource provenance font resource "inter-700-italic" has unsupported face_class "variable"; only static faces have canonical semantics'],
    ['missing face class', (manifest: RenderManifestV1) => { (manifest.font_resources![0] as { face_class: string }).face_class = ''; }, 'font resource provenance font resource "inter-700-italic" has unsupported face_class ""; only static faces have canonical semantics'],
    ['unsafe staged path', (manifest: RenderManifestV1) => { manifest.font_resources![0].staged_path = '../system-font.woff2'; }, 'font resource provenance font resource "inter-700-italic" staged path must be a clean relative POSIX path'],
    ['invalid hash', (manifest: RenderManifestV1) => { manifest.font_resources![0].file_sha256 = manifest.font_resources![0].file_sha256.toUpperCase(); }, 'font resource provenance font resource "inter-700-italic" has an invalid file_sha256'],
    ['empty bytes', (manifest: RenderManifestV1) => { manifest.font_resources![0].size_bytes = 0; }, 'font resource provenance font resource "inter-700-italic" size_bytes must be positive'],
  ])('fails closed on %s', (_name, mutate, error) => {
    const manifest = cloneManifest(fixture.manifest);
    mutate(manifest);
    expect(() => evaluateFontResourceProvenance(manifest)).toThrow(error);
  });
});
