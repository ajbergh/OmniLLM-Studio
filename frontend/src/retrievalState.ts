// retrievalState derives the display state for current-information retrieval.
//
// Kept out of the component module so both the message renderer and any future
// telemetry/tests can import it without pulling in React.
import type { MessageMetadata } from './types';

export type RetrievalState =
  | 'none'
  | 'failed'
  | 'tool-skipped'
  | 'grounded-verified'
  | 'grounded-dated'
  | 'grounded-unverified'
  | 'grounded-no-sources';

/**
 * Derives the retrieval state from message metadata.
 *
 * Order matters: a failed retrieval and a skipped required tool both outrank the
 * grounded states, because in either case the answer came from model memory
 * regardless of what else is set.
 */
export function retrievalStateFrom(metadata: MessageMetadata | null | undefined): RetrievalState {
  if (!metadata) return 'none';
  if (metadata.search_failed) return 'failed';
  if (metadata.tool_requirement_unfulfilled) return 'tool-skipped';
  if (!metadata.web_search) return 'none';

  const sourceCount = metadata.sources?.length ?? 0;
  if (sourceCount === 0) return 'grounded-no-sources';
  if (metadata.freshness_verified) return 'grounded-verified';
  // Knowing the newest source date is a distinct state from knowing nothing.
  // `freshness_verified` requires a *requested* window, and the pricing,
  // release, and benchmark intents deliberately request none — so without this
  // branch those answers reported "dates unknown" while the backend had the
  // date in hand.
  if (metadata.answer_freshness) return 'grounded-dated';
  return 'grounded-unverified';
}
