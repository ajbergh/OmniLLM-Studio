package llm

import "strings"

// Citation is a normalized provider-native grounding source.
//
// Native citations previously existed only as markdown appended to the message
// content — a "**Sources:**" block written into the answer text. That made them
// unusable for anything except human reading: the backend could not count them,
// could not check them against the plan's freshness window, and could not fill
// metadata.sources, so the source panel was empty for every natively-grounded
// answer while the answer itself claimed to cite sources.
type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// NormalizeCitations de-duplicates by URL, preserving first-seen order, and
// drops entries with no URL.
func NormalizeCitations(citations []Citation) []Citation {
	if len(citations) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(citations))
	out := make([]Citation, 0, len(citations))
	for _, citation := range citations {
		url := strings.TrimSpace(citation.URL)
		if url == "" {
			continue
		}
		if _, dup := seen[url]; dup {
			continue
		}
		seen[url] = struct{}{}
		title := strings.TrimSpace(citation.Title)
		if title == "" {
			title = url
		}
		out = append(out, Citation{URL: url, Title: title})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MergeCitations appends new citations onto an accumulator, de-duplicating.
func MergeCitations(accumulated []Citation, incoming []Citation) []Citation {
	if len(incoming) == 0 {
		return accumulated
	}
	return NormalizeCitations(append(append([]Citation(nil), accumulated...), incoming...))
}
