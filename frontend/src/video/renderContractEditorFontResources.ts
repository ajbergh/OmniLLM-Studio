import type { VideoAsset } from '../types/video';
import {
  evaluateVisualFrameState,
  type CanonicalVisualFrameState,
} from './renderContractFrameState';
import type { TimelineV2Document } from './renderContractTypes';

export const EDITOR_FONT_RESOURCE_BINDING_V1 = 'editor-font-resource-binding-v1' as const;
export const EDITOR_FONT_RESOURCE_DUPLICATE = 'EDITOR_FONT_RESOURCE_DUPLICATE' as const;
export const EDITOR_FONT_RESOURCE_MISSING = 'EDITOR_FONT_RESOURCE_MISSING' as const;

export interface EditorFontResourceBindingIssue {
  code: typeof EDITOR_FONT_RESOURCE_DUPLICATE | typeof EDITOR_FONT_RESOURCE_MISSING;
  font_resource_id: string;
  message: string;
  remediation: string;
}

export interface EditorFontResourceBindingIndex {
  contract_version: typeof EDITOR_FONT_RESOURCE_BINDING_V1;
  required_ids: string[];
  resources_by_id: ReadonlyMap<string, VideoAsset>;
  issue?: EditorFontResourceBindingIssue;
}

/**
 * Resolve mutable editor font assets by their declared font_resource_id.
 *
 * This is intentionally weaker than font-resource-provenance-v1: editor assets
 * do not carry immutable staged paths or enqueue-time hashes, so this index
 * proves only that one current project asset declares the requested resource
 * identity. It must never be treated as packaged-face provenance.
 */
export function indexEditorFontResources(
  assets: readonly VideoAsset[],
  requiredResourceIDs: Iterable<string>,
): EditorFontResourceBindingIndex {
  const requiredIDs = uniqueSortedResourceIDs(requiredResourceIDs);
  const resourcesByID = new Map<string, VideoAsset>();
  if (requiredIDs.length === 0) {
    return {
      contract_version: EDITOR_FONT_RESOURCE_BINDING_V1,
      required_ids: requiredIDs,
      resources_by_id: resourcesByID,
    };
  }

  for (const asset of assets) {
    const resourceID = editorFontResourceIDFromAsset(asset);
    if (!resourceID) continue;
    const existing = resourcesByID.get(resourceID);
    if (existing) {
      return {
        contract_version: EDITOR_FONT_RESOURCE_BINDING_V1,
        required_ids: requiredIDs,
        resources_by_id: resourcesByID,
        issue: {
          code: EDITOR_FONT_RESOURCE_DUPLICATE,
          font_resource_id: resourceID,
          message: `project declares font resource ${JSON.stringify(resourceID)} on both ${JSON.stringify(existing.file_name)} and ${JSON.stringify(asset.file_name)}`,
          remediation: 'keep exactly one font asset for each font_resource_id before using deterministic preview',
        },
      };
    }
    resourcesByID.set(resourceID, asset);
  }

  for (const resourceID of requiredIDs) {
    if (resourcesByID.has(resourceID)) continue;
    return {
      contract_version: EDITOR_FONT_RESOURCE_BINDING_V1,
      required_ids: requiredIDs,
      resources_by_id: resourcesByID,
      issue: {
        code: EDITOR_FONT_RESOURCE_MISSING,
        font_resource_id: resourceID,
        message: `timeline references font resource ${JSON.stringify(resourceID)} that the project does not provide`,
        remediation: 'upload or restore the exact font asset bound to this font_resource_id before using deterministic preview',
      },
    };
  }

  return {
    contract_version: EDITOR_FONT_RESOURCE_BINDING_V1,
    required_ids: requiredIDs,
    resources_by_id: resourcesByID,
  };
}

/** Read the canonical font_resource_id declaration from a current editor asset. */
export function editorFontResourceIDFromAsset(asset: VideoAsset): string | undefined {
  if (asset.kind !== 'font') return undefined;
  const rawMetadata = asset.metadata_json?.trim() ?? '';
  if (!rawMetadata) return undefined;
  try {
    const metadata = JSON.parse(rawMetadata) as { font_resource_id?: unknown };
    if (typeof metadata.font_resource_id !== 'string') return undefined;
    const resourceID = metadata.font_resource_id.trim();
    if (!/^[a-z0-9][a-z0-9._-]*$/.test(resourceID)) return undefined;
    return resourceID;
  } catch {
    return undefined;
  }
}

/**
 * Evaluate FrameState for the mutable editor after exact current project font
 * resource availability has been verified by the caller.
 *
 * Plain timeline evaluation intentionally rejects font_resource_id because it
 * has no resource context, while Render Manifest evaluation upgrades a verified
 * immutable face to packaged-resource provenance. Editor evaluation sits
 * between those two: it verifies current resource availability, temporarily
 * removes only the resource reference before canonical evaluation, then restores
 * that authored identity on the resulting text state. font_face_source remains
 * family-name-only/composition-default until a browser face is actually loaded;
 * no immutable provenance or glyph metrics are fabricated here.
 */
export function evaluateVisualFrameStateForEditor(
  document: TimelineV2Document,
  frameIndex: number,
  availableFontResourceIDs: ReadonlySet<string>,
): CanonicalVisualFrameState {
  const evaluationDocument = editorEvaluationDocument(document, availableFontResourceIDs);
  const state = evaluateVisualFrameState(evaluationDocument, frameIndex);

  for (const layer of state.layers) {
    const authoredText = document.tracks[layer.track_index]?.clips[layer.clip_index]?.text;
    const rawResourceID = authoredText?.font_resource_id;
    const resourceID = rawResourceID?.trim() ?? '';
    if (!resourceID) continue;
    if (!layer.text) {
      throw new Error(`canonical editor FrameState lost text while rebinding font resource ${JSON.stringify(resourceID)} for clip ${JSON.stringify(layer.clip_id)}`);
    }
    layer.text.font_resource_id = resourceID;
  }
  return state;
}

function editorEvaluationDocument(
  document: TimelineV2Document,
  availableFontResourceIDs: ReadonlySet<string>,
): TimelineV2Document {
  let changed = false;
  const tracks = document.tracks.map((track) => {
    let trackChanged = false;
    const clips = track.clips.map((clip) => {
      if (!clip.text?.font_resource_id) return clip;
      const rawResourceID = clip.text.font_resource_id;
      const resourceID = rawResourceID.trim();
      if (resourceID && rawResourceID !== resourceID) {
        throw new Error(`canonical text font_resource_id ${JSON.stringify(rawResourceID)} must not have surrounding whitespace`);
      }
      if (!resourceID) return clip;
      if (!availableFontResourceIDs.has(resourceID)) {
        throw new Error(`canonical editor text state for clip ${JSON.stringify(clip.id)} names font resource ${JSON.stringify(resourceID)} that current project assets do not provide`);
      }
      trackChanged = true;
      changed = true;
      const text = { ...clip.text };
      delete text.font_resource_id;
      return { ...clip, text };
    });
    return trackChanged ? { ...track, clips } : track;
  });
  return changed ? { ...document, tracks } : document;
}

function uniqueSortedResourceIDs(values: Iterable<string>): string[] {
  const ids = new Set<string>();
  for (const value of values) {
    const resourceID = value.trim();
    if (resourceID) ids.add(resourceID);
  }
  return [...ids].sort();
}
