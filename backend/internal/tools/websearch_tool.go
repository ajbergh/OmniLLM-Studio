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
				"description": "Limit results to a time window"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of results to return (default 5)"
			}
		},
		"required": ["query"]
	}`)

	return ToolDefinition{
		Name:        "web_search",
		Description: "Search the web for current information, news, and real-time data.",
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

// Execute performs a web search through the orchestrator and returns the
// summarised result as tool content.
func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a webSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	maxResults := a.MaxResults
	if maxResults == 0 {
		maxResults = 5
	}

	searchReq := websearch.SearchRequest{
		Query:      a.Query,
		TimeRange:  a.TimeRange,
		MaxResults: maxResults,
	}

	resp, err := t.orchestrator.DirectSearch(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Build a readable summary of the results for the LLM.
	content, err := json.Marshal(resp.Results)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}

	meta := map[string]interface{}{
		"query":        a.Query,
		"result_count": len(resp.Results),
		"sources":      resp.Results,
	}

	return &ToolResult{
		Content:  string(content),
		IsError:  false,
		Metadata: meta,
	}, nil
}
