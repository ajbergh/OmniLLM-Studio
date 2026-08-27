import { describe, expect, it, vi } from 'vitest';
import type { VideoAsset } from '../../types/video';
import type { CanonicalEvaluatedTextState } from '../../video/renderContractText';
import {
  createPreviewFontFaceLoader,
  previewFontFaceFamilyAlias,
  resolvePreviewFontFaceBinding,
  type PreviewFontFaceLike,
} from './previewFontFaceReadiness';

function fontAsset(overrides: Partial<VideoAsset> = {}): VideoAsset {
  return {
    id: 'asset-font-1',
    project_id: 'project-1',
    source_type: 'upload',
    kind: 'font',
    file_name: 'Inter-Regular.woff2',
    file_path: 'video/assets/font.woff2',
    mime_type: 'font/woff2',
    size_bytes: 128,
    metadata_json: JSON.stringify({ font_resource_id: 'inter-regular' }),
    created_at: new Date(0).toISOString(),
    ...overrides,
  };
}

function textState(overrides: Partial<CanonicalEvaluatedTextState> = {}): CanonicalEvaluatedTextState {
  return {
    contract_version: 'text-state-v1',
    text: 'Title',
    font_family: 'Inter',
    font_family_source: 'authored',
    font_face_source: 'family-name-only',
    font_size: 48,
    font_weight: '600',
    color: '#ffffff',
    stroke_width: 0,
    text_align: 'center',
    vertical_align: 'middle',
    line_height_mode: 'normal',
    letter_spacing: 0,
    border_radius: 0,
    padding: { top: 0, right: 0, bottom: 0, left: 0 },
    ...overrides,
  };
}

describe('preview font face binding', () => {
  it('does not require a browser face when canonical text has no resource binding', () => {
    expect(resolvePreviewFontFaceBinding(textState(), undefined)).toBeUndefined();
  });

  it('binds exact editor asset identity and authored weight to a collision-safe family alias', () => {
    const asset = fontAsset();
    const binding = resolvePreviewFontFaceBinding(textState({ font_resource_id: 'inter-regular' }), asset);
    expect(binding).toMatchObject({
      contract_version: 'preview-font-face-readiness-v1',
      fontResourceId: 'inter-regular',
      fontWeight: '600',
      asset,
    });
    expect(binding?.familyAlias).toBe(previewFontFaceFamilyAlias('inter-regular', asset.id, '600'));
    expect(binding?.familyAlias).toMatch(/^OmniLLMPreview_/);
    expect(binding?.familyAlias).not.toContain('Inter');
  });

  it('fails closed on missing, non-font, or mismatched editor resource bindings', () => {
    const text = textState({ font_resource_id: 'inter-regular' });
    expect(() => resolvePreviewFontFaceBinding(text, undefined)).toThrow(/no bound editor asset/);
    expect(() => resolvePreviewFontFaceBinding(text, fontAsset({ kind: 'image' }))).toThrow(/non-font asset/);
    expect(() => resolvePreviewFontFaceBinding(text, fontAsset({
      metadata_json: JSON.stringify({ font_resource_id: 'other-face' }),
    }))).toThrow(/declaring "other-face"/);
  });

  it('fails closed instead of inventing unsupported CSS weight semantics', () => {
    const asset = fontAsset();
    expect(() => resolvePreviewFontFaceBinding(
      textState({ font_resource_id: 'inter-regular', font_weight: 'semi-bold' }),
      asset,
    )).toThrow(/supported CSS weight/);
    expect(() => previewFontFaceFamilyAlias('inter-regular', asset.id, '1001')).toThrow(/between 1 and 1000/);
  });
});

describe('preview font face loader', () => {
  it('deduplicates concurrent loads and registers the loaded exact-byte face once', async () => {
    const asset = fontAsset();
    const binding = resolvePreviewFontFaceBinding(textState({ font_resource_id: 'inter-regular' }), asset);
    expect(binding).toBeDefined();
    const loadedFace: PreviewFontFaceLike = { load: vi.fn() as never };
    loadedFace.load = vi.fn(async () => loadedFace);
    const fetchAssetBytes = vi.fn(async () => new ArrayBuffer(16));
    const createFace = vi.fn(() => loadedFace);
    const addFace = vi.fn();
    const loader = createPreviewFontFaceLoader({ fetchAssetBytes, createFace, addFace });

    const first = loader.ensure(binding!);
    const second = loader.ensure(binding!);
    expect(second).toBe(first);
    await expect(first).resolves.toMatchObject({ binding });
    expect(fetchAssetBytes).toHaveBeenCalledTimes(1);
    expect(createFace).toHaveBeenCalledWith(binding!.familyAlias, expect.any(ArrayBuffer), '600');
    expect(loadedFace.load).toHaveBeenCalledTimes(1);
    expect(addFace).toHaveBeenCalledWith(loadedFace);
  });

  it('rejects empty bytes and evicts failed loads so a later retry can succeed', async () => {
    const binding = resolvePreviewFontFaceBinding(
      textState({ font_resource_id: 'inter-regular' }),
      fontAsset(),
    );
    expect(binding).toBeDefined();
    const loadedFace: PreviewFontFaceLike = { load: async () => loadedFace };
    const fetchAssetBytes = vi.fn()
      .mockResolvedValueOnce(new ArrayBuffer(0))
      .mockResolvedValueOnce(new ArrayBuffer(8));
    const createFace = vi.fn(() => loadedFace);
    const addFace = vi.fn();
    const loader = createPreviewFontFaceLoader({ fetchAssetBytes, createFace, addFace });

    await expect(loader.ensure(binding!)).rejects.toThrow(/empty font bytes/);
    await expect(loader.ensure(binding!)).resolves.toMatchObject({ binding });
    expect(fetchAssetBytes).toHaveBeenCalledTimes(2);
    expect(createFace).toHaveBeenCalledTimes(1);
    expect(addFace).toHaveBeenCalledTimes(1);
  });
});
