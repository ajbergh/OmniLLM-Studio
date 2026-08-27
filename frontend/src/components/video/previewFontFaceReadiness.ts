import { getAuthToken, videoApi } from '../../api';
import type { VideoAsset } from '../../types/video';
import { editorFontResourceIDFromAsset } from '../../video/renderContractEditorFontResources';
import type { CanonicalEvaluatedTextState } from '../../video/renderContractText';
import { installPreviewTextLayoutReadinessGate } from './previewTextLayoutSnapshot';

export const PREVIEW_FONT_FACE_READINESS_V1 = 'preview-font-face-readiness-v1' as const;

export interface PreviewFontFaceBinding {
  contract_version: typeof PREVIEW_FONT_FACE_READINESS_V1;
  key: string;
  familyAlias: string;
  fontResourceId: string;
  fontWeight: string;
  asset: VideoAsset;
}

export interface PreviewFontFaceLike {
  load(): Promise<PreviewFontFaceLike>;
}

export interface PreviewFontFaceRuntime {
  fetchAssetBytes(asset: VideoAsset): Promise<ArrayBuffer>;
  createFace(familyAlias: string, bytes: ArrayBuffer, fontWeight: string): PreviewFontFaceLike;
  addFace(face: PreviewFontFaceLike): void;
}

export interface PreviewLoadedFontFace {
  binding: PreviewFontFaceBinding;
  face: PreviewFontFaceLike;
}

export interface PreviewFontFaceLoader {
  ensure(binding: PreviewFontFaceBinding): Promise<PreviewLoadedFontFace>;
  clear(): void;
}

/**
 * Resolve the exact mutable editor asset that must back a resource-bound text
 * painter. This proves only current browser input identity. It never upgrades
 * text-state-v1 to immutable packaged-resource provenance.
 */
export function resolvePreviewFontFaceBinding(
  text: CanonicalEvaluatedTextState,
  fontAsset: VideoAsset | undefined,
): PreviewFontFaceBinding | undefined {
  const fontResourceId = text.font_resource_id?.trim() ?? '';
  if (!fontResourceId) return undefined;
  if (!fontAsset) {
    throw new Error(`preview font resource ${JSON.stringify(fontResourceId)} has no bound editor asset`);
  }
  if (fontAsset.kind !== 'font') {
    throw new Error(`preview font resource ${JSON.stringify(fontResourceId)} is bound to non-font asset ${JSON.stringify(fontAsset.id)}`);
  }
  const assetResourceId = editorFontResourceIDFromAsset(fontAsset);
  if (assetResourceId !== fontResourceId) {
    throw new Error(
      `preview font resource ${JSON.stringify(fontResourceId)} is bound to asset ${JSON.stringify(fontAsset.id)} declaring ${JSON.stringify(assetResourceId ?? '')}`,
    );
  }
  const fontWeight = canonicalPreviewFontWeight(text.font_weight);
  return {
    contract_version: PREVIEW_FONT_FACE_READINESS_V1,
    key: JSON.stringify([fontResourceId, fontAsset.id, fontWeight]),
    familyAlias: previewFontFaceFamilyAlias(fontResourceId, fontAsset.id, fontWeight),
    fontResourceId,
    fontWeight,
    asset: fontAsset,
  };
}

/**
 * Browser-only family alias. Resource/asset/weight identity is encoded into the
 * name so an editor-loaded face cannot collide with a system or web font and a
 * single resource used at different authored weights never shares CSS matching
 * state accidentally.
 */
export function previewFontFaceFamilyAlias(
  fontResourceId: string,
  assetId: string,
  fontWeight: string,
): string {
  const resource = safeAliasToken(fontResourceId);
  const asset = safeAliasToken(assetId);
  const weight = safeAliasToken(canonicalPreviewFontWeight(fontWeight));
  if (!resource || !asset) throw new Error('preview font face alias requires resource and asset identity');
  return `OmniLLMPreview_${resource}_${asset}_${weight}`;
}

/**
 * Create a deduplicating loader. Failed entries are evicted so a later frame or
 * explicit retry may re-fetch after a transient browser/network failure.
 */
export function createPreviewFontFaceLoader(runtime: PreviewFontFaceRuntime): PreviewFontFaceLoader {
  const pending = new Map<string, Promise<PreviewLoadedFontFace>>();
  return {
    ensure(binding) {
      const existing = pending.get(binding.key);
      if (existing) return existing;
      const load = (async (): Promise<PreviewLoadedFontFace> => {
        const bytes = await runtime.fetchAssetBytes(binding.asset);
        if (!(bytes instanceof ArrayBuffer) || bytes.byteLength === 0) {
          throw new Error(`preview font resource ${JSON.stringify(binding.fontResourceId)} returned empty font bytes`);
        }
        const face = runtime.createFace(binding.familyAlias, bytes, binding.fontWeight);
        const loaded = await face.load();
        runtime.addFace(loaded);
        return { binding, face: loaded };
      })();
      pending.set(binding.key, load);
      void load.catch(() => {
        if (pending.get(binding.key) === load) pending.delete(binding.key);
      });
      return load;
    },
    clear() {
      pending.clear();
    },
  };
}

function browserFontFaceRuntime(): PreviewFontFaceRuntime {
  return {
    async fetchAssetBytes(asset) {
      const headers: Record<string, string> = {};
      const token = getAuthToken();
      if (token) headers.Authorization = `Bearer ${token}`;
      const response = await fetch(videoApi.downloadUrl(asset.id), { headers });
      if (!response.ok) {
        throw new Error(`font asset ${JSON.stringify(asset.id)} download failed with HTTP ${response.status}`);
      }
      return response.arrayBuffer();
    },
    createFace(familyAlias, bytes, fontWeight) {
      if (typeof FontFace === 'undefined') {
        throw new Error('browser FontFace API is unavailable');
      }
      return new FontFace(familyAlias, bytes, { weight: fontWeight }) as PreviewFontFaceLike;
    },
    addFace(face) {
      if (typeof document === 'undefined' || !document.fonts) {
        throw new Error('browser FontFaceSet is unavailable');
      }
      document.fonts.add(face as FontFace);
    },
  };
}

export const browserPreviewFontFaceLoader = createPreviewFontFaceLoader(browserFontFaceRuntime());

function canonicalPreviewFontWeight(value: string): string {
  const weight = value.trim().toLowerCase();
  if (weight === 'normal' || weight === 'bold') return weight;
  if (!/^\d{1,4}$/.test(weight)) {
    throw new Error(`preview resource-backed font weight ${JSON.stringify(value)} is not a supported CSS weight`);
  }
  const numeric = Number(weight);
  if (!Number.isInteger(numeric) || numeric < 1 || numeric > 1000) {
    throw new Error(`preview resource-backed font weight ${JSON.stringify(value)} must be between 1 and 1000`);
  }
  return String(numeric);
}

function safeAliasToken(value: string): string {
  return value.trim().replace(/[^A-Za-z0-9._-]/g, (character) => `_${character.codePointAt(0)?.toString(16) ?? '0'}_`);
}

// Module initialization happens before the React readiness listeners register,
// so this capture-phase gate can wait for font readiness first and then resume
// into the existing font/weighted-Canvas gates without bypassing either one.
installPreviewTextLayoutReadinessGate();
