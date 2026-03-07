package websearch

import "time"

// ToolCall represents a tool invocation decided by the orchestrator.
type ToolCall struct {
	Name      string        `json:"name"`
	Arguments SearchRequest `json:"arguments"`
}

// SearchRequest holds parameters for a web search query.
type SearchRequest struct {
	Query      string `json:"query"`
	TimeRange  string `json:"timeRange"` // "24h", "7d", "30d"
	Region     string `json:"region"`    // e.g. "US"
	Locale     string `json:"locale"`    // e.g. "en-US"
	MaxResults int    `json:"maxResults"`
}

// SearchResult is a single normalised result from a web search provider.
type SearchResult struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Source      string `json:"source"`
	PublishedAt string `json:"publishedAt"`
	Snippet     string `json:"snippet"`
}

// SearchResponse wraps the full response from a web search.
type SearchResponse struct {
	Query     string         `json:"query"`
	TimeRange string         `json:"timeRange"`
	Results   []SearchResult `json:"results"`
	FetchedAt time.Time      `json:"fetchedAt"`
}

// ClassifierOutput is the strict-JSON the LLM returns when used as a
// fallback classifier for ambiguous queries.
type ClassifierOutput struct {
	NeedsWeb  bool   `json:"needs_web"`
	Query     string `json:"query"`
	TimeRange string `json:"timeRange"`
	Reason    string `json:"reason"`
}

// OrchestratorResult is returned by the orchestrator to the HTTP handler.
type OrchestratorResult struct {
	Content      string         `json:"content"`
	Sources      []SearchResult `json:"sources,omitempty"`
	WebSearch    bool           `json:"web_search"`
	SearchFailed bool           `json:"search_failed,omitempty"`
	ToolCall     *ToolCall      `json:"tool_call,omitempty"`
	Provider     string         `json:"provider"`
	Model        string         `json:"model"`
	TokenInput   *int           `json:"token_input,omitempty"`
	TokenOut     *int           `json:"token_output,omitempty"`
}
