import type { VideoTimelineDocument } from '../types/video';
import { adaptTimelineV1ToV2, RenderContractAdapterError } from './renderContractAdapter';
import {
  evaluateVisualFrameStateForEditor,
} from './renderContractEditorFontResources';
import {
  evaluateVisualFrameState,
  type CanonicalVisualFrameState,
} from './renderContractFrameState';
import { TimelineV2RuntimeError } from './renderContractNormalize';
import type { TimelineV2ContentBounds } from './renderContractTypes';

export const VISUAL_FRAME_STATE_DIAGNOSTIC_V1 = 'visual-frame-state-diagnostic-v1' as const;
export const FRAME_STATE_EVALUATION_FAILED = 'FRAME_STATE_EVALUATION_FAILED' as const;

export interface VisualFrameStateDiagnosticError {
  code: string;
  path: string;
  message: string;
  remediation: string;
}

export interface VisualFrameStateDiagnostic {
  contract_version: typeof VISUAL_FRAME_STATE_DIAGNOSTIC_V1;
  frame_index: number;
  available: boolean;
  state?: CanonicalVisualFrameState;
  error?: VisualFrameStateDiagnosticError;
}

export interface VisualFrameStateDiagnosticOptions {
  /**
   * Persisted editor asset probe dimensions projected into the temporary
   * Timeline v2 evaluation document. These are content bounds only; they do
   * not claim immutable Render Manifest source provenance.
   */
  contentBoundsByAsset?: ReadonlyMap<string, TimelineV2ContentBounds>;
  /**
   * Current project font resource IDs that the editor has already resolved to
   * unique font assets. This verifies mutable resource availability only; it
   * does not claim packaged font provenance or deterministic glyph metrics.
   */
  availableFontResourceIDs?: ReadonlySet<string>;
}

/**
 * Evaluate canonical visual FrameState from the persisted editor-compatible
 * Timeline v1 document without weakening the fail-closed v1→v2 adapter.
 *
 * Diagnostics are deliberately an envelope: timelines with semantics that are
 * not yet representable in Timeline v2 (for example ambiguous v1 transition
 * placement) return a structured unavailable result rather than a guessed
 * state. This keeps parity tooling observational and prevents it from becoming
 * a second, permissive contract adapter.
 *
 * Callers may supply already-probed source dimensions as content bounds. The
 * projection mutates only the temporary adapted Timeline v2 document and never
 * fabricates source-provenance-v1 identity or hashes. When callers also supply
 * editor-verified font resource IDs, those IDs satisfy only current project
 * resource availability; immutable font provenance remains Render Manifest-only.
 */
export function evaluateVisualFrameStateDiagnostic(
  document: VideoTimelineDocument,
  frameIndex: number,
  options: VisualFrameStateDiagnosticOptions = {},
): VisualFrameStateDiagnostic {
  const frame = Math.trunc(frameIndex);
  try {
    const canonical = adaptTimelineV1ToV2(document);
    if (options.contentBoundsByAsset && options.contentBoundsByAsset.size > 0) {
      for (const track of canonical.tracks) {
        for (const clip of track.clips) {
          if (!clip.asset_id || clip.content_bounds) continue;
          const bounds = options.contentBoundsByAsset.get(clip.asset_id);
          if (bounds) clip.content_bounds = { ...bounds };
        }
      }
    }
    const state = options.availableFontResourceIDs
      ? evaluateVisualFrameStateForEditor(canonical, frame, options.availableFontResourceIDs)
      : evaluateVisualFrameState(canonical, frame);
    return {
      contract_version: VISUAL_FRAME_STATE_DIAGNOSTIC_V1,
      frame_index: frame,
      available: true,
      state,
    };
  } catch (reason) {
    return {
      contract_version: VISUAL_FRAME_STATE_DIAGNOSTIC_V1,
      frame_index: frame,
      available: false,
      error: diagnosticError(reason),
    };
  }
}

function diagnosticError(reason: unknown): VisualFrameStateDiagnosticError {
  if (reason instanceof RenderContractAdapterError || reason instanceof TimelineV2RuntimeError) {
    return {
      code: reason.code,
      path: reason.path,
      message: reason.message,
      remediation: reason.remediation,
    };
  }
  return {
    code: FRAME_STATE_EVALUATION_FAILED,
    path: '',
    message: reason instanceof Error ? reason.message : String(reason),
    remediation: 'inspect the canonical FrameState evaluator input and keep unsupported semantics fail-closed',
  };
}
