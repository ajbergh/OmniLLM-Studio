package api

import (
	"log"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// searchMechanism records how a turn actually obtained current information.
//
// This was previously implicit, spread across three booleans and two nil checks,
// which is how a turn could silently switch from provider-native grounding to a
// rate-limited HTML scraper with nothing in the logs or the UI to say so.
type searchMechanism string

const (
	// searchMechanismNone means no retrieval ran.
	searchMechanismNone searchMechanism = "none"
	// searchMechanismNative means the provider grounded its own answer. The
	// orchestrator owns the turn, so no tool loop runs.
	searchMechanismNative searchMechanism = "native"
	// searchMechanismLocal means the configured Brave/DuckDuckGo provider ran,
	// either owning the turn or as a preflight feeding the tool loop.
	searchMechanismLocal searchMechanism = "local"
	// searchMechanismFailed means retrieval was attempted and produced nothing.
	searchMechanismFailed searchMechanism = "failed"
)

const metaSearchMechanism = "search_mechanism"

// groundedToolLoopDirective is added when provider-native grounding travels with
// the tool catalog in the same request.
//
// The model has two ways to get information and needs to know which is which:
// its own search index covers the public web, while the tools reach things the
// web cannot see. Without this it tends to treat them as interchangeable and
// reaches for whichever it saw first.
const groundedToolLoopDirective = `SEARCH AND TOOLS ARE BOTH AVAILABLE THIS TURN.

You have built-in web search for public information, and separately a set of tools for everything the public web cannot answer: the user's own accounts, repositories, documents, and connected systems.

Use your own web search for public facts — prices, releases, benchmarks, news. Use a tool when the question is about the user's data. Use both when a question needs public data and then an action on it, and say which claims came from which. Do not answer a question about current public facts from memory when your search can check it.`

// withoutLocalWebSearch removes the local web_search tool from a catalog.
//
// Two situations call for it, and the reason is the same in both: the turn
// already has its search mechanism, and leaving a second, weaker one on the
// table invites the model to use it.
//
//  1. A preflight already retrieved evidence and injected it. Letting the model
//     re-search the same question through the local provider adds nothing and,
//     on DuckDuckGo, reliably fails — one observed turn spent five tool calls
//     re-searching, two of them refused outright.
//  2. The active provider grounds its own answers. The local scraper is the
//     fallback for providers that cannot, not a peer to offer alongside.
//
// Page-level tools are deliberately left in place. browser_* and fetch_url read a
// specific URL rather than querying an index, so they compose with grounding
// instead of competing with it.
func withoutLocalWebSearch(catalog []llm.Tool) []llm.Tool {
	filtered := make([]llm.Tool, 0, len(catalog))
	for _, tool := range catalog {
		if tool.Function.Name == "web_search" {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

// nativeGroundingAvailable reports whether the active provider and model can
// ground their own answer.
func (h *MessageHandler) nativeGroundingAvailable(provider, model string) bool {
	providerType, err := h.llmSvc.ResolveProviderType(provider)
	if err != nil {
		return false
	}
	return websearch.SupportsNativeSearch(providerType, model)
}

// logSearchMechanism records the resolved mechanism once per turn.
func logSearchMechanism(mechanism searchMechanism, provider, model string, ownsTurn bool) {
	log.Printf("[websearch] mechanism=%s provider=%s model=%s owns_turn=%v",
		mechanism, provider, model, ownsTurn)
}
