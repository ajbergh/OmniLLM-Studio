package websearch

import "sort"

// rankByPreferredDomains promotes results hosted on a plan's preferred domains
// while preserving the provider's relative ordering within each group.
//
// This is deliberately a re-rank rather than a filter. AllowedDomains already
// exists for hard restriction; applying that to pricing or benchmark work would
// be too brittle, because the authoritative host differs per vendor and a
// missing entry silently drops the only good source. Ranking gets the official
// page in front of the aggregator without risking an empty result set.
//
// Indexes are renumbered afterwards because they are the citation markers the
// summarizer prompt refers to.
func rankByPreferredDomains(results []SearchResult, preferred []string) []SearchResult {
	if len(preferred) == 0 || len(results) < 2 {
		return renumber(results)
	}

	ranked := make([]SearchResult, len(results))
	copy(ranked, results)

	sort.SliceStable(ranked, func(i, j int) bool {
		iPreferred := matchesDomain(normalizeHost(ranked[i]), preferred)
		jPreferred := matchesDomain(normalizeHost(ranked[j]), preferred)
		return iPreferred && !jPreferred
	})

	return renumber(ranked)
}

// renumber makes Index contiguous and 1-based.
func renumber(results []SearchResult) []SearchResult {
	for i := range results {
		results[i].Index = i + 1
	}
	return results
}
