package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// WebSearchTool wraps the existing websearch.Orchestrator as a Tool.
type WebSearchTool struct {
	orchestrator *websearch.Orchestrator
	provider     string // active LLM provider name for summarisation
	model        string // active LLM model for summarisation
}

// NewWebSearchTool creates a WebSearchTool backed by the given orchestrator.
// provider and model are used when the orchestrator needs to call the LLM
// for summarisation.
func NewWebSearchTool(orch *websearch.Orchestrator, provider, model string) *WebSearchTool {
	return &WebSearchTool{
		orchestrator: orch,
		provider:     provider,
		model:        model,
	}
}

// webSearchArgs mirrors the JSON arguments accepted by the tool.
type webSearchArgs struct {
	Query      string `json:"query"`
	TimeRange  string `json:"time_range,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// Definition returns the tool metadata and JSON-Schema for its parameters.
func (t *WebSearchTool) Definition() ToolDefinition {
	// time_range stays optional, but its description now states that the server
	// decides when it is omitted. Previously an omitted value meant the request
	// carried no freshness constraint at all, while the equivalent REST endpoint
	// applied defaults — the model-facing path was the weaker of the two.
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query to look up on the web"
			},
			"time_range": {
				"type": "string",
				"enum": ["24h", "7d", "30d"],
				"description": "Optional freshness window. Omit it unless the question is explicitly about a specific period; the server infers an appropriate window from the query intent, and picks no window at all for reference material such as pricing pages and release notes."
			},
			"max_results": {
				"type": "integer",
				"description": "Optional cap on returned results (1-20). The server chooses a sensible count per query intent when omitted."
			}
		},
		"required": ["query"]
	}`)

	return ToolDefinition{
		Name:        "web_search",
		Description: "Search the web for current information, news, and real-time data. Returns results with a retrieval timestamp and, where the provider supplies it, publication dates. Cite the sources you use by their index.",
		Parameters:  schema,
		Category:    "search",
		Enabled:     true,
	}
}

// Validate checks that the required 'query' argument is present.
func (t *WebSearchTool) Validate(args json.RawMessage) error {
	var a webSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// webSearchToolPayload is the JSON handed back to the model.
//
// The previous payload was a bare array of results, which gave the model no way
// to distinguish "these are from the last 24 hours" from "these are undated" —
// so it presented both as current.
type webSearchToolPayload struct {
	Query              string                   `json:"query"`
	QueriesRun         []string                 `json:"queries_run"`
	TimeRange          string                   `json:"time_range"`
	Region             string                   `json:"region"`
	Locale             string                   `json:"locale"`
	Intent             string                   `json:"intent"`
	FetchedAt          string                   `json:"fetched_at"`
	ResultCount        int                      `json:"result_count"`
	EvidenceSufficient bool                     `json:"evidence_sufficient"`
	Guidance           string                   `json:"guidance"`
	Results            []websearch.SearchResult `json:"results"`
}

// Execute performs a planner-backed web search and returns the results plus the
// retrieval metadata needed to describe them honestly.
func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a webSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	// PlannedSearch rather than DirectSearch: the server owns the freshness
	// window, region, locale, query expansion, and source ranking. A model that
	// omits those arguments gets server-chosen values instead of none.
	resp, err := t.orchestrator.PlannedSearch(ctx, a.Query, a.TimeRange, a.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	payload := webSearchToolPayload{
		Query:              resp.Query,
		QueriesRun:         resp.Queries,
		TimeRange:          freshnessLabel(resp.TimeRange),
		Region:             resp.Region,
		Locale:             resp.Locale,
		Intent:             string(resp.Intent),
		FetchedAt:          resp.FetchedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ResultCount:        len(resp.Results),
		EvidenceSufficient: resp.Sufficient,
		Guidance:           webSearchGuidance(resp),
		Results:            resp.Results,
	}

	content, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}

	meta := map[string]interface{}{
		"query":               resp.Query,
		"result_count":        len(resp.Results),
		"sources":             resp.Results,
		"fetched_at":          payload.FetchedAt,
		"time_range":          payload.TimeRange,
		"intent":              payload.Intent,
		"evidence_sufficient": resp.Sufficient,
	}

	return &ToolResult{
		Content:  string(content),
		IsError:  false,
		Metadata: meta,
	}, nil
}

// freshnessLabel makes an empty window explicit rather than absent, so the model
// does not read a missing field as "unknown recency".
func freshnessLabel(timeRange string) string {
	if timeRange == "" {
		return "none (reference material; results are not filtered by recency)"
	}
	return timeRange
}

// webSearchGuidance tells the model what the evidence does and does not support.
func webSearchGuidance(resp *websearch.PlannedSearchResult) string {
	base := "Cite sources by their index. A result with an empty publishedAt has no known publication date: do not describe it as current."
	if !resp.Sufficient {
		return base + " The retrieved evidence did not meet this query's corroboration requirement, so state which claims you could not verify rather than presenting them as established."
	}
	if resp.RequiresCitations {
		return base + " Every numeric claim in your answer must name the source it came from."
	}
	return base
}
