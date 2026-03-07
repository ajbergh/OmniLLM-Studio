package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// URLFetchTool retrieves the textual content of a URL.
type URLFetchTool struct {
	client *http.Client
}

// NewURLFetchTool creates a URLFetchTool with an SSRF-safe HTTP client.
func NewURLFetchTool() *URLFetchTool {
	return &URLFetchTool{
		client: NewSSRFSafeClient(15 * time.Second),
	}
}

type urlFetchArgs struct {
	URL string `json:"url"`
}

func (t *URLFetchTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch content from"
			}
		},
		"required": ["url"]
	}`)

	return ToolDefinition{
		Name:        "url_fetch",
		Description: "Fetch the textual content of a web page given its URL.",
		Parameters:  schema,
		Category:    "fetch",
		Enabled:     true,
	}
}

func (t *URLFetchTool) Validate(args json.RawMessage) error {
	var a urlFetchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(a.URL, "http://") && !strings.HasPrefix(a.URL, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}
	return nil
}

func (t *URLFetchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a urlFetchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "OmniLLM-Studio/1.0")
	req.Header.Set("Accept", "text/html, text/plain, application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Failed to fetch URL: %v", err),
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &ToolResult{
			Content: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
			IsError: true,
		}, nil
	}

	// Limit to 100KB to avoid overwhelming the context
	const maxBody = 100 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Failed to read response body: %v", err),
			IsError: true,
		}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	text := string(body)

	return &ToolResult{
		Content: text,
		Metadata: map[string]interface{}{
			"url":          a.URL,
			"status_code":  resp.StatusCode,
			"content_type": contentType,
			"byte_length":  len(body),
		},
	}, nil
}
