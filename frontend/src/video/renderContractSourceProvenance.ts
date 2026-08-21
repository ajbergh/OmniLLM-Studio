import type { RenderManifestV1, TimelineV2ContentBounds } from './renderContractTypes';

export const SOURCE_PROVENANCE_CONTRACT_V1 = 'source-provenance-v1' as const;

/**
 * Immutable source identity and decoded source box projected from a Render
 * Manifest asset. This contract never reads files or infers dimensions from a
 * destination canvas.
 */
export interface CanonicalSourceProvenance {
  contract_version: typeof SOURCE_PROVENANCE_CONTRACT_V1;
  asset_id: string;
  clip_ids: string[];
  file_sha256: string;
  source_bounds: TimelineV2ContentBounds;
}

/**
 * Project valid visual source dimensions from immutable Render Manifest v1
 * media probes. Assets without visual dimensions produce no source
 * provenance; partial or invalid dimensions fail closed.
 */
export function evaluateSourceProvenance(manifest: RenderManifestV1): CanonicalSourceProvenance[] {
  const seenAssetIDs = new Set<string>();
  const result: CanonicalSourceProvenance[] = [];
  for (const asset of manifest.assets) {
    const assetID = asset.asset_id;
    if (!assetID.trim()) throw new Error('source provenance asset id is required');
    if (assetID !== assetID.trim()) {
      throw new Error(`source provenance asset id ${JSON.stringify(assetID)} must not have surrounding whitespace`);
    }
    if (seenAssetIDs.has(assetID)) throw new Error(`source provenance has duplicate asset ${JSON.stringify(assetID)}`);
    seenAssetIDs.add(assetID);
    if (!isLowerSHA256(asset.file_sha256)) {
      throw new Error(`source provenance asset ${JSON.stringify(assetID)} has an invalid file_sha256`);
    }
    const clipIDs = canonicalClipIDs(assetID, asset.clip_ids);
    const media = asset.media;
    if (!media || (media.width === undefined && media.height === undefined)) continue;
    if (media.width === undefined || media.height === undefined) {
      throw new Error(`source provenance asset ${JSON.stringify(assetID)} must provide both media width and height`);
    }
    if (!Number.isInteger(media.width) || !Number.isInteger(media.height) || media.width < 1 || media.height < 1) {
      throw new Error(`source provenance asset ${JSON.stringify(assetID)} media width and height must be positive integers`);
    }
    result.push({
      contract_version: SOURCE_PROVENANCE_CONTRACT_V1,
      asset_id: assetID,
      clip_ids: clipIDs,
      file_sha256: asset.file_sha256,
      source_bounds: { x: 0, y: 0, width: media.width, height: media.height },
    });
  }
  return result.sort((left, right) => left.asset_id.localeCompare(right.asset_id));
}

export function sourceProvenanceByAsset(manifest: RenderManifestV1): ReadonlyMap<string, CanonicalSourceProvenance> {
  return new Map(evaluateSourceProvenance(manifest).map((source) => [source.asset_id, source]));
}

function canonicalClipIDs(assetID: string, clipIDs: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const clipID of clipIDs) {
    if (!clipID.trim()) throw new Error(`source provenance asset ${JSON.stringify(assetID)} has an empty clip id`);
    if (clipID !== clipID.trim()) {
      throw new Error(`source provenance asset ${JSON.stringify(assetID)} clip id ${JSON.stringify(clipID)} must not have surrounding whitespace`);
    }
    if (seen.has(clipID)) throw new Error(`source provenance asset ${JSON.stringify(assetID)} has duplicate clip id ${JSON.stringify(clipID)}`);
    seen.add(clipID);
    result.push(clipID);
  }
  return result.sort();
}

function isLowerSHA256(value: string): boolean {
  return /^[a-f0-9]{64}$/.test(value);
}
