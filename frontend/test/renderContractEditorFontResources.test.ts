import { describe, expect, it } from 'vitest';
import type { VideoAsset } from '../src/types/video';
import {
  EDITOR_FONT_RESOURCE_DUPLICATE,
  EDITOR_FONT_RESOURCE_MISSING,
  editorFontResourceIDFromAsset,
  indexEditorFontResources,
} from '../src/video/renderContractEditorFontResources';

function fontAsset(id: string, resourceID: string, fileName = `${id}.woff2`): VideoAsset {
  return {
    id,
    kind: 'font',
    source_type: 'upload',
    file_name: fileName,
    file_path: `fonts/${fileName}`,
    mime_type: 'font/woff2',
    size_bytes: 128,
    metadata_json: JSON.stringify({ font_resource_id: resourceID }),
    created_at: '2026-08-26T00:00:00Z',
  };
}

describe('editor font resource binding', () => {
  it('binds one current font asset to one required canonical resource id', () => {
    const asset = fontAsset('font-1', 'inter-400-normal');
    const index = indexEditorFontResources([asset], ['inter-400-normal']);

    expect(index.issue).toBeUndefined();
    expect(index.required_ids).toEqual(['inter-400-normal']);
    expect(index.resources_by_id.get('inter-400-normal')).toBe(asset);
  });

  it('fails closed when a required resource id is missing', () => {
    const index = indexEditorFontResources([], ['inter-400-normal']);

    expect(index.issue?.code).toBe(EDITOR_FONT_RESOURCE_MISSING);
    expect(index.issue?.font_resource_id).toBe('inter-400-normal');
  });

  it('fails closed when current project assets duplicate a resource id', () => {
    const first = fontAsset('font-1', 'inter-400-normal', 'Inter-Regular.woff2');
    const second = fontAsset('font-2', 'inter-400-normal', 'Inter-Regular-Copy.woff2');
    const index = indexEditorFontResources([first, second], ['inter-400-normal']);

    expect(index.issue?.code).toBe(EDITOR_FONT_RESOURCE_DUPLICATE);
    expect(index.issue?.message).toContain('Inter-Regular.woff2');
    expect(index.issue?.message).toContain('Inter-Regular-Copy.woff2');
  });

  it('ignores stale or imported metadata that violates the canonical resource-id grammar', () => {
    const malformed = fontAsset('font-1', 'Inter 400');

    expect(editorFontResourceIDFromAsset(malformed)).toBeUndefined();
    const index = indexEditorFontResources([malformed], ['inter-400-normal']);
    expect(index.issue?.code).toBe(EDITOR_FONT_RESOURCE_MISSING);
  });

  it('ignores invalid JSON metadata instead of manufacturing a resource identity', () => {
    const malformed = fontAsset('font-1', 'inter-400-normal');
    malformed.metadata_json = '{not-json';

    expect(editorFontResourceIDFromAsset(malformed)).toBeUndefined();
  });

  it('does not inspect unrelated font metadata when the timeline needs no font resources', () => {
    const malformed = fontAsset('font-1', 'Inter 400');
    const index = indexEditorFontResources([malformed], []);

    expect(index.issue).toBeUndefined();
    expect(index.resources_by_id.size).toBe(0);
  });
});
