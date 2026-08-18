import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  renderManifestAssetKinds, renderManifestTypeProjection, timelineV2Easings, timelineV2EffectTypes,
  timelineV2MediaFits, timelineV2ShapeKinds, timelineV2TextAlignments, timelineV2TrackTypes,
  timelineV2TransitionDirections, timelineV2TransitionPlacements, timelineV2TransitionTypes,
  timelineV2TypeProjection, timelineV2VerticalAlignments,
} from '../src/video/renderContractTypes';
import { renderManifestRequiredProjection, timelineV2RequiredProjection } from '../src/video/renderContractProjection';

interface SchemaNode { type?: string; const?: string | number; enum?: Array<string | number>; properties?: Record<string, SchemaNode>; required?: string[]; oneOf?: SchemaNode[]; }
interface SchemaDocument extends SchemaNode { $defs: Record<string, SchemaNode>; }

function loadSchema(name: string): SchemaDocument {
  const schemaURL = new URL(`../../video-renderer/contracts/${name}`, import.meta.url);
  return JSON.parse(readFileSync(schemaURL, 'utf8')) as SchemaDocument;
}
function sorted(values: readonly string[] | undefined): string[] { return [...(values ?? [])].sort(); }
function objectKeys(node: SchemaNode): string[] { return sorted(Object.keys(node.properties ?? {})); }
function unionKeys(node: SchemaNode): string[] {
  const keys = new Set<string>();
  for (const variant of node.oneOf ?? []) for (const key of Object.keys(variant.properties ?? {})) keys.add(key);
  return sorted([...keys]);
}
function expectProjection(projection: Record<string, readonly string[]>, requiredProjection: Record<string, readonly string[]>, schema: SchemaDocument, rootName: string, skipRequired: ReadonlySet<string> = new Set()): void {
  for (const [name, fields] of Object.entries(projection)) {
    const node = name === rootName ? schema : schema.$defs[name];
    expect(node, `missing schema node ${name}`).toBeDefined();
    expect(sorted(fields), `${name} property drift`).toEqual(node.oneOf ? unionKeys(node) : objectKeys(node));
    if (!skipRequired.has(name)) expect(sorted(requiredProjection[name]), `${name} required-field drift`).toEqual(sorted(node.required));
  }
}
function enumValues(schema: SchemaDocument, definition: string, property: string): Array<string | number> {
  return schema.$defs[definition]?.properties?.[property]?.enum ?? [];
}

describe('canonical render contract type projections', () => {
  const timeline = loadSchema('timeline-v2.schema.json');
  const manifest = loadSchema('render-manifest-v1.schema.json');

  it('keeps Timeline v2 TypeScript keys and optionality aligned with JSON Schema', () => {
    expectProjection(timelineV2TypeProjection, timelineV2RequiredProjection, timeline, 'timeline', new Set(['motionCurve']));
  });
  it('keeps Render Manifest v1 TypeScript keys and optionality aligned with JSON Schema', () => {
    expectProjection(renderManifestTypeProjection, renderManifestRequiredProjection, manifest, 'manifest');
  });
  it('keeps Timeline v2 enum projections aligned with JSON Schema', () => {
    expect([...timelineV2TrackTypes]).toEqual(enumValues(timeline, 'track', 'type'));
    expect([...timelineV2MediaFits]).toEqual(enumValues(timeline, 'clip', 'media_fit'));
    expect([...timelineV2TextAlignments]).toEqual(enumValues(timeline, 'text', 'text_align'));
    expect([...timelineV2VerticalAlignments]).toEqual(enumValues(timeline, 'text', 'vertical_align'));
    expect([...timelineV2ShapeKinds]).toEqual(enumValues(timeline, 'shape', 'kind'));
    expect([...timelineV2EffectTypes]).toEqual(enumValues(timeline, 'effect', 'type'));
    expect([...timelineV2TransitionTypes]).toEqual(enumValues(timeline, 'transition', 'type'));
    expect([...timelineV2TransitionDirections]).toEqual(enumValues(timeline, 'transition', 'direction'));
    expect([...timelineV2TransitionPlacements]).toEqual(enumValues(timeline, 'transition', 'placement'));
    expect([...timelineV2Easings]).toEqual(enumValues(timeline, 'keyframe', 'easing'));
  });
  it('keeps motion-curve union semantics aligned with JSON Schema', () => {
    const variants = timeline.$defs.motionCurve.oneOf ?? [];
    expect(variants).toHaveLength(3);
    expect(variants[0].properties?.type?.enum).toEqual([...timelineV2Easings]);
    expect(variants[0].required).toEqual(['type']);
    expect(variants[1].properties?.type?.const).toBe('bezier');
    expect(sorted(variants[1].required)).toEqual(sorted(['type', 'x1', 'y1', 'x2', 'y2']));
    expect(variants[2].properties?.type?.const).toBe('spring');
    expect(sorted(variants[2].required)).toEqual(sorted(['type', 'stiffness', 'damping', 'mass']));
  });
  it('keeps manifest enum and fixed contract values aligned with JSON Schema', () => {
    expect([...renderManifestAssetKinds]).toEqual(enumValues(manifest, 'asset', 'kind'));
    expect(manifest.properties?.version?.const).toBe(1);
    expect(manifest.properties?.contract_version?.const).toBe('timeline-v2');
    expect(manifest.$defs.settings.properties?.audio_sample_rate?.const).toBe(48000);
    expect(manifest.$defs.settings.properties?.audio_channels?.const).toBe(2);
    expect(manifest.$defs.settings.properties?.working_color_space?.const).toBe('srgb');
  });
});
