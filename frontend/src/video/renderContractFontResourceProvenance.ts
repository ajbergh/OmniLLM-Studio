import type { RenderManifestV1 } from './renderContractTypes';

export const FONT_RESOURCE_PROVENANCE_CONTRACT_V1 = 'font-resource-provenance-v1' as const;

/**
 * One immutable static font face packaged with a Render Manifest snapshot.
 * This package/identity contract intentionally does not select a face for a
 * text layer or synthesize glyph metrics from a browser or system font.
 */
export interface CanonicalFontResourceProvenance {
  contract_version: typeof FONT_RESOURCE_PROVENANCE_CONTRACT_V1;
  font_resource_id: string;
  font_family: string;
  font_weight: number;
  font_style: 'normal' | 'italic';
  format: 'woff2' | 'woff' | 'ttf' | 'otf';
  staged_path: string;
  file_sha256: string;
  size_bytes: number;
}

/**
 * Validate and project immutable packaged font faces from a Render Manifest.
 * The evaluator never reads a file, resolves a system font, or guesses an
 * authored text-family-to-face binding.
 */
export function evaluateFontResourceProvenance(manifest: RenderManifestV1): CanonicalFontResourceProvenance[] {
  const seenIDs = new Set<string>();
  const result: CanonicalFontResourceProvenance[] = [];
  for (const resource of manifest.font_resources ?? []) {
    const resourceID = canonicalResourceID(resource.font_resource_id);
    if (seenIDs.has(resourceID)) throw new Error(`font resource provenance has duplicate font resource ${JSON.stringify(resourceID)}`);
    seenIDs.add(resourceID);
    const fontFamily = canonicalToken(`font resource ${JSON.stringify(resourceID)} font family`, resource.font_family);
    if (!Number.isInteger(resource.font_weight) || resource.font_weight < 1 || resource.font_weight > 1000) {
      throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} font weight must be between 1 and 1000`);
    }
    if (resource.font_style !== 'normal' && resource.font_style !== 'italic') {
      throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} has unsupported font style ${JSON.stringify(resource.font_style)}`);
    }
    if (!['woff2', 'woff', 'ttf', 'otf'].includes(resource.format)) {
      throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} has unsupported format ${JSON.stringify(resource.format)}`);
    }
    validateStagedPath(resourceID, resource.staged_path);
    if (!isLowerSHA256(resource.file_sha256)) {
      throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} has an invalid file_sha256`);
    }
    if (!Number.isInteger(resource.size_bytes) || resource.size_bytes < 1) {
      throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} size_bytes must be positive`);
    }
    result.push({
      contract_version: FONT_RESOURCE_PROVENANCE_CONTRACT_V1,
      font_resource_id: resourceID,
      font_family: fontFamily,
      font_weight: resource.font_weight,
      font_style: resource.font_style,
      format: resource.format,
      staged_path: resource.staged_path,
      file_sha256: resource.file_sha256,
      size_bytes: resource.size_bytes,
    });
  }
  return result.sort((left, right) => left.font_resource_id < right.font_resource_id ? -1 : left.font_resource_id > right.font_resource_id ? 1 : 0);
}

function canonicalResourceID(value: string): string {
  const resourceID = canonicalToken('font resource id', value);
  if (!/^[a-z0-9][a-z0-9._-]*$/.test(resourceID)) {
    throw new Error(`font resource provenance font resource id ${JSON.stringify(resourceID)} must use lowercase ASCII letters, digits, dots, underscores, or hyphens`);
  }
  return resourceID;
}

function canonicalToken(field: string, value: string): string {
  if (!value.trim()) throw new Error(`font resource provenance ${field} is required`);
  if (value !== value.trim()) throw new Error(`font resource provenance ${field} ${JSON.stringify(value)} must not have surrounding whitespace`);
  return value;
}

function validateStagedPath(resourceID: string, stagedPath: string): void {
  canonicalToken(`font resource ${JSON.stringify(resourceID)} staged path`, stagedPath);
  if (stagedPath.includes('\\') || stagedPath.startsWith('/') || stagedPath === '.' || stagedPath === '..' || stagedPath.startsWith('../') || stagedPath.split('/').some((part) => part === '' || part === '.' || part === '..')) {
    throw new Error(`font resource provenance font resource ${JSON.stringify(resourceID)} staged path must be a clean relative POSIX path`);
  }
}

function isLowerSHA256(value: string): boolean {
  return /^[a-f0-9]{64}$/.test(value);
}
