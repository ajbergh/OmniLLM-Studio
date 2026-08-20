import { describe, expect, it } from 'vitest';
import { retrievalStateFrom } from './retrievalState';
import type { MessageMetadata } from './types';

const source = { index: 1, title: 't', url: 'https://x.test', source: 'x.test', publishedAt: '', snippet: 's' };

describe('retrievalStateFrom', () => {
  it('returns none for messages with no metadata', () => {
    expect(retrievalStateFrom(null)).toBe('none');
    expect(retrievalStateFrom(undefined)).toBe('none');
    expect(retrievalStateFrom({} as MessageMetadata)).toBe('none');
  });

  it('reports failure even when other retrieval fields are set', () => {
    // The regression this guards: a failed search used to fall through and the
    // answer rendered as if nothing had gone wrong.
    expect(
      retrievalStateFrom({
        search_attempted: true,
        search_failed: true,
        search_failure_reason: 'provider_error',
        web_search: true,
        sources: [source],
      }),
    ).toBe('failed');
  });

  it('flags a grounded answer that returned no citable sources', () => {
    // Native grounding returns an empty results array; `[] || fallback` yields
    // `[]` because an empty array is truthy, so this state is reachable.
    expect(retrievalStateFrom({ web_search: true, sources: [] })).toBe('grounded-no-sources');
    expect(retrievalStateFrom({ web_search: true })).toBe('grounded-no-sources');
  });

  it('separates verified freshness from unknown dates', () => {
    expect(retrievalStateFrom({ web_search: true, sources: [source] })).toBe('grounded-unverified');
    expect(
      retrievalStateFrom({ web_search: true, sources: [source], freshness_verified: true }),
    ).toBe('grounded-verified');
  });

  it('ignores retrieval state for answers that never searched', () => {
    expect(retrievalStateFrom({ search_attempted: true })).toBe('none');
  });
});
