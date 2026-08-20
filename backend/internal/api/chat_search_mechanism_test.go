package api

import (
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

func toolCatalog(names ...string) []llm.Tool {
	out := make([]llm.Tool, 0, len(names))
	for _, name := range names {
		var tool llm.Tool
		tool.Function.Name = name
		out = append(out, tool)
	}
	return out
}

// TestWithoutLocalWebSearchKeepsPageTools covers the distinction the filter
// relies on: web_search queries an index and competes with the turn's search
// mechanism, while browser_* and fetch_url read a specific URL and compose with
// it. Removing the page tools would break "look at this link" on a grounded turn.
func TestWithoutLocalWebSearchKeepsPageTools(t *testing.T) {
	catalog := toolCatalog(
		"web_search", "browser_navigate", "browser_screenshot",
		"fetch_url", "calculator", "mcp_jira_search_issues",
	)
	filtered := withoutLocalWebSearch(catalog)

	var names []string
	for _, tool := range filtered {
		names = append(names, tool.Function.Name)
	}
	joined := strings.Join(names, ",")

	if strings.Contains(joined, "web_search") {
		t.Error("the local index search must be removed")
	}
	for _, keep := range []string{"browser_navigate", "browser_screenshot", "fetch_url", "calculator", "mcp_jira_search_issues"} {
		if !strings.Contains(joined, keep) {
			t.Errorf("%q must survive: it reads a specific target rather than competing with the search mechanism", keep)
		}
	}
	if len(filtered) != len(catalog)-1 {
		t.Errorf("expected exactly one removal, got %d -> %d", len(catalog), len(filtered))
	}
}

func TestWithoutLocalWebSearchEmpty(t *testing.T) {
	if got := withoutLocalWebSearch(nil); len(got) != 0 {
		t.Errorf("nil catalog -> %d tools", len(got))
	}
	if got := withoutLocalWebSearch(toolCatalog("web_search")); len(got) != 0 {
		t.Errorf("a search-only catalog should empty out, got %d", len(got))
	}
}

// TestLocalWebSearchRemovalIsWired asserts both handler paths drop the tool once
// the turn has a search mechanism. Without this, a Gemini turn keeps a
// rate-limited scraper on the table next to its own working grounding.
func TestLocalWebSearchRemovalIsWired(t *testing.T) {
	text := readMessageHandlerSource(t)
	if strings.Count(text, "withoutLocalWebSearch(") < 2 {
		t.Error("streaming and non-streaming must both drop the local search tool once the turn has a mechanism")
	}
	if !strings.Contains(text, "preflight != nil || streamOwnership.NativeGrounding") {
		t.Error("streaming removal must trigger on preflight evidence or native grounding")
	}
	if !strings.Contains(text, "syncPreflight != nil || syncOwnership.NativeGrounding") {
		t.Error("non-streaming removal must trigger on the same two conditions")
	}
}

// TestNativeEscalationIsWired covers the recovery path. A failed local preflight
// on a native-capable model used to fall through to a training-data answer; a
// grounded answer without the follow-up tool is strictly better, and nothing has
// been streamed at that point so changing the turn's shape is safe.
func TestNativeEscalationIsWired(t *testing.T) {
	text := readMessageHandlerSource(t)
	if strings.Count(text, "escalating to native grounding") < 2 {
		t.Error("both paths must escalate to native grounding when the local preflight fails")
	}
	if !strings.Contains(text, "logSearchMechanism(") {
		t.Error("the resolved mechanism must be logged; it was previously invisible")
	}
	for _, key := range []string{"metaSearchMechanism"} {
		if strings.Count(text, key) < 3 {
			t.Errorf("%s must reach message metadata on every retrieval path", key)
		}
	}
}
