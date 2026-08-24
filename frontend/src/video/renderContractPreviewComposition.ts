import type {
  VideoAsset,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
} from '../types/video';
import type {
  CanonicalFrameLayerState,
  CanonicalVisualFrameState,
} from './renderContractFrameState';
import {
  evaluateVisualFrameStateDiagnostic,
  type VisualFrameStateDiagnosticError,
} from './renderContractFrameStateDiagnostics';

export const PREVIEW_COMPOSITION_FRAME_V1 = 'preview-composition-frame-v1' as const;
export const PREVIEW_COMPOSITION_IDENTITY_MISMATCH = 'PREVIEW_COMPOSITION_IDENTITY_MISMATCH' as const;

/**
 * Runtime bridge from canonical FrameState back to the editor's persisted v1
 * objects. The canonical layer state owns semantic decisions; track/clip/asset
 * references are carried only so the existing preview painter can consume the
 * same identities while Phase 3 migrates incrementally.
 */
export interface CanonicalPreviewCompositionLayer {
  track_index: number;
  clip_index: number;
  track: VideoTimelineTrack;
  clip: VideoTimelineClip;
  asset?: VideoAsset;
  state: CanonicalFrameLayerState;
}

export interface CanonicalPreviewCompositionFrame {
  contract_version: typeof PREVIEW_COMPOSITION_FRAME_V1;
  frame_index: number;
  available: boolean;
  frame_state?: CanonicalVisualFrameState;
  layers?: CanonicalPreviewCompositionLayer[];
  error?: VisualFrameStateDiagnosticError;
}

/**
 * Build the canonical visual composition for one editor frame and bind each
 * canonical layer back to the exact v1 track/clip/asset object it came from.
 *
 * The v1 -> v2 adapter remains fail closed. If authored v1 state cannot be
 * represented canonically, callers receive an unavailable diagnostic instead
 * of a permissive preview-specific interpretation. The projection also checks
 * positional and ID identity so a future adapter that reorders tracks/clips
 * cannot silently bind canonical state to the wrong editor object.
 */
export function evaluateCanonicalPreviewCompositionFrame(
  document: VideoTimelineDocument,
  assets: readonly VideoAsset[],
  frameIndex: number,
): CanonicalPreviewCompositionFrame {
  const frame = Math.trunc(frameIndex);
  const diagnostic = evaluateVisualFrameStateDiagnostic(document, frame);
  if (!diagnostic.available || !diagnostic.state) {
    return {
      contract_version: PREVIEW_COMPOSITION_FRAME_V1,
      frame_index: frame,
      available: false,
      ...(diagnostic.error ? { error: diagnostic.error } : {}),
    };
  }

  const assetByID = new Map(assets.map((asset) => [asset.id, asset]));
  const layers: CanonicalPreviewCompositionLayer[] = [];
  for (const state of diagnostic.state.layers) {
    const track = document.tracks[state.track_index];
    const clip = track?.clips[state.clip_index];
    if (!track || !clip || track.id !== state.track_id || clip.id !== state.clip_id) {
      return {
        contract_version: PREVIEW_COMPOSITION_FRAME_V1,
        frame_index: frame,
        available: false,
        error: {
          code: PREVIEW_COMPOSITION_IDENTITY_MISMATCH,
          path: `tracks[${state.track_index}].clips[${state.clip_index}]`,
          message: `canonical layer ${JSON.stringify(state.clip_id)} no longer matches its editor track/clip identity`,
          remediation: 'keep the v1 -> v2 adapter positional until preview composition has an explicit stable identity mapping',
        },
      };
    }
    layers.push({
      track_index: state.track_index,
      clip_index: state.clip_index,
      track,
      clip,
      ...(clip.asset_id ? { asset: assetByID.get(clip.asset_id) } : {}),
      state,
    });
  }

  return {
    contract_version: PREVIEW_COMPOSITION_FRAME_V1,
    frame_index: frame,
    available: true,
    frame_state: diagnostic.state,
    layers,
  };
}
