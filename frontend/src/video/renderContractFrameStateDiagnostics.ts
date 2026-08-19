import type { VideoTimelineDocument } from '../types/video';
import { adaptTimelineV1ToV2, RenderContractAdapterError } from './renderContractAdapter';
import {
  evaluateVisualFrameState,
  type CanonicalVisualFrameState,
} from './renderContractFrameState';
import { TimelineV2RuntimeError } from './renderContractNormalize';

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

/**
 * Evaluate canonical visual FrameState from the persisted editor-compatible
 * Timeline v1 document without weakening the fail-closed v1→v2 adapter.
 *
 * Diagnostics are deliberately an envelope: timelines with semantics that are
 * not yet representable in Timeline v2 (for example ambiguous v1 transition
 * placement) return a structured unavailable result rather than a guessed
 * state. This keeps parity tooling observational and prevents it from becoming
 * a second, permissive contract adapter.
 */
export function evaluateVisualFrameStateDiagnostic(
  document: VideoTimelineDocument,
  frameIndex: number,
): VisualFrameStateDiagnostic {
  const frame = Math.trunc(frameIndex);
  try {
    const canonical = adaptTimelineV1ToV2(document);
    return {
      contract_version: VISUAL_FRAME_STATE_DIAGNOSTIC_V1,
      frame_index: frame,
      available: true,
      state: evaluateVisualFrameState(canonical, frame),
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
