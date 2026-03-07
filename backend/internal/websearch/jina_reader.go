package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// JinaReader – fetches clean Markdown text from any URL via Jina Reader API.
//
// Jina Reader (https://r.jina.ai) converts any webpage into clean,
// LLM-friendly Markdown. Free, no API key required.
//
// Usage in the pipeline:
//   1. Search provider returns results with short snippets
//   2. JinaReader enriches the top N results with full page content
//   3. The enriched content is sent to the LLM for better summarisation
// ---------------------------------------------------------------------------

// JinaReader fetches clean Markdown content from URLs via the Jina Reader API.
type JinaReader struct {
	httpClient *http.Client
	maxLen     int // max characters to keep per page
}

// NewJinaReader creates a new JinaReader.
// maxLen controls how many characters of content to keep per page (0 = unlimited).
func NewJinaReader(maxLen int) *JinaReader {
	if maxLen <= 0 {
		maxLen = 3000 // sensible default: ~750 tokens per page
	}
	return &JinaReader{
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
		maxLen: maxLen,
	}
}

// FetchContent retrieves the Markdown content of a single URL.
func (j *JinaReader) FetchContent(ctx context.Context, pageURL string) (string, error) {
	// Validate and encode the URL before delegation
	parsed, err := url.Parse(pageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid URL: %s", pageURL)
	}
	endpoint := "https://r.jina.ai/" + parsed.String()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create jina request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "OmniLLM-Studio/1.0")
	// Request plain text output (no images, minimal formatting)
	req.Header.Set("X-Return-Format", "text")

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("jina request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jina returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read jina response: %w", err)
	}

	content := strings.TrimSpace(string(bodyBytes))

	// Truncate to maxLen to keep prompt sizes reasonable
	if j.maxLen > 0 && len(content) > j.maxLen {
		content = content[:j.maxLen] + "\n[...truncated]"
	}

	return content, nil
}

// EnrichResults fetches full-page content for the top N search results
// concurrently and updates each result's Snippet with the richer content.
// Results that fail to fetch keep their original snippet.
func (j *JinaReader) EnrichResults(ctx context.Context, results []SearchResult, topN int) []SearchResult {
	if topN <= 0 || topN > len(results) {
		topN = len(results)
	}
	if topN > 5 {
		topN = 5 // cap to avoid excessive requests
	}

	// Create a timeout context for all enrichment requests
	enrichCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	type enrichment struct {
		index   int
		content string
	}

	var wg sync.WaitGroup
	ch := make(chan enrichment, topN)

	for i := 0; i < topN; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			content, err := j.FetchContent(enrichCtx, results[idx].URL)
			if err != nil || content == "" {
				return // keep original snippet
			}
			ch <- enrichment{index: idx, content: content}
		}(i)
	}

	// Close channel when all goroutines finish
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect enrichments
	enriched := make([]SearchResult, len(results))
	copy(enriched, results)

	for e := range ch {
		// Prepend original snippet, then full content
		enriched[e.index].Snippet = enriched[e.index].Snippet + "\n\nFull content:\n" + e.content
	}

	return enriched
}
