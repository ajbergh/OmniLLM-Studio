// RetrievalStatus renders the current-information retrieval outcome for an
// assistant message.
//
// These states used to be indistinguishable in the UI: a failed search fell
// through to an ordinary-looking answer, and a natively-grounded answer showed
// no sources because the backend sent an empty array. Each state now renders
// differently so a reader can tell verified content from model recall.
import { AlertTriangle, Globe, ShieldCheck, ShieldAlert } from 'lucide-react';
import { SEARCH_FAILURE_LABELS, type MessageMetadata, type SearchFailureReason } from '../types';
import { retrievalStateFrom } from '../retrievalState';

function reasonLabel(reason: SearchFailureReason | undefined): string {
  if (reason && reason in SEARCH_FAILURE_LABELS) {
    return SEARCH_FAILURE_LABELS[reason];
  }
  return 'The search could not be completed.';
}

interface RetrievalStatusProps {
  metadata: MessageMetadata | null;
}

export function RetrievalStatus({ metadata }: RetrievalStatusProps) {
  const state = retrievalStateFrom(metadata);

  if (state === 'failed') {
    return (
      <div
        role="status"
        className="mt-2 flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5"
      >
        <AlertTriangle size={12} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
        <div className="min-w-0 text-[11px] leading-relaxed">
          <span className="font-medium text-amber-300">Current information could not be verified.</span>{' '}
          <span className="text-text-muted">
            {reasonLabel(metadata?.search_failure_reason)} This answer comes from the model&apos;s
            training data and may be out of date.
          </span>
        </div>
      </div>
    );
  }

  if (state === 'tool-skipped') {
    const toolName = typeof metadata?.tool_required === 'string' ? metadata.tool_required : null;
    return (
      <div
        role="status"
        className="mt-2 flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5"
      >
        <AlertTriangle size={12} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
        <div className="min-w-0 text-[11px] leading-relaxed">
          <span className="font-medium text-amber-300">
            {toolName ? `The ${toolName} tool was required but never ran.` : 'A required tool never ran.'}
          </span>{' '}
          <span className="text-text-muted">
            {metadata?.tool_enforced
              ? 'The provider was asked to require it and answered anyway, so this response is unverified.'
              : 'This provider does not support forced tool calls, so the requirement could not be enforced. Treat the answer as unverified.'}
          </span>
        </div>
      </div>
    );
  }

  if (state === 'grounded-no-sources') {
    return (
      <div
        role="status"
        className="mt-2 flex items-start gap-2 rounded-lg border border-border/40 bg-surface-hover/40 p-2.5"
      >
        <ShieldAlert size={12} className="mt-0.5 shrink-0 text-text-muted" aria-hidden="true" />
        <div className="min-w-0 text-[11px] leading-relaxed text-text-muted">
          <span className="font-medium text-text">Web search ran, but returned no citable sources.</span>{' '}
          Individual claims in this answer are not traceable to a source.
        </div>
      </div>
    );
  }

  return null;
}

/**
 * ClaimWarning renders the claim-support signal.
 *
 * Deliberately separate from RetrievalStatus and deliberately mild: claim
 * support is not decidable by string matching, so this is a caution about a
 * missing citation, not an assertion that the figures are wrong. It never
 * suppresses the answer.
 */
export function ClaimWarning({ metadata }: RetrievalStatusProps) {
  if (metadata?.claim_warning !== 'numeric_claims_without_citation') return null;
  return (
    <div
      role="note"
      className="mt-2 flex items-start gap-2 rounded-lg border border-border/40 bg-surface-hover/40 p-2.5"
    >
      <ShieldAlert size={12} className="mt-0.5 shrink-0 text-text-muted" aria-hidden="true" />
      <div className="min-w-0 text-[11px] leading-relaxed text-text-muted">
        <span className="font-medium text-text">This answer states figures without citing a source.</span>{' '}
        Check the numbers against the sources before relying on them.
      </div>
    </div>
  );
}

/**
 * Badge shown alongside the source count. Separated from RetrievalStatus so the
 * sources panel keeps ownership of its own disclosure button.
 */
export function FreshnessBadge({ metadata }: RetrievalStatusProps) {
  const state = retrievalStateFrom(metadata);
  if (state === 'grounded-verified') {
    return (
      <span
        className="inline-flex items-center gap-1 rounded bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-medium text-emerald-400"
        title={
          metadata?.answer_freshness
            ? `Newest source: ${metadata.answer_freshness}`
            : 'Sources fall inside the requested freshness window'
        }
      >
        <ShieldCheck size={9} aria-hidden="true" />
        freshness verified
      </span>
    );
  }
  if (state === 'grounded-unverified') {
    return (
      <span
        className="inline-flex items-center gap-1 rounded bg-surface-hover px-1.5 py-0.5 text-[10px] font-medium text-text-muted"
        title="Source publication dates were unavailable, so freshness could not be confirmed"
      >
        <Globe size={9} aria-hidden="true" />
        dates unknown
      </span>
    );
  }
  return null;
}
