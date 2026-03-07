package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// BraveProvider – uses the Brave Search API (free tier: 2 000 queries/month)
//   Sign up: https://brave.com/search/api/
//   Docs:    https://api.search.brave.com/app/#/documentation
// ---------------------------------------------------------------------------

// BraveProvider performs real web searches via the Brave Search API.
type BraveProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewBraveProvider creates a new BraveProvider.
func NewBraveProvider(apiKey string) *BraveProvider {
	return &BraveProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// braveWebResponse models the relevant parts of the Brave Web Search JSON response.
type braveWebResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			PageAge     string `json:"page_age,omitempty"` // e.g. "2 hours ago"
			Age         string `json:"age,omitempty"`      // e.g. "2 hours ago"
			MetaURL     struct {
				Hostname string `json:"hostname"`
			} `json:"meta_url"`
		} `json:"results"`
	} `json:"web"`
	News *struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age,omitempty"`
			MetaURL     struct {
				Hostname string `json:"hostname"`
			} `json:"meta_url"`
			Source struct {
				Name string `json:"name"`
			} `json:"source,omitempty"`
		} `json:"results"`
	} `json:"news,omitempty"`
}

func (b *BraveProvider) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Build query parameters
	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("text_decorations", "false")
	params.Set("result_filter", "web,news")

	if req.MaxResults > 0 && req.MaxResults <= 20 {
		params.Set("count", fmt.Sprintf("%d", req.MaxResults))
	} else {
		params.Set("count", "10")
	}

	// Map timeRange to Brave "freshness" parameter
	switch req.TimeRange {
	case "24h":
		params.Set("freshness", "pd") // past day
	case "7d":
		params.Set("freshness", "pw") // past week
	case "30d":
		params.Set("freshness", "pm") // past month
	}

	// Map region
	if req.Region != "" {
		params.Set("country", strings.ToLower(req.Region))
	}

	endpoint := "https://api.search.brave.com/res/v1/web/search?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create brave request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Accept-Encoding", "gzip")
	httpReq.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("brave request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read brave response: %w", err)
	}

	var braveResp braveWebResponse
	if err := json.Unmarshal(bodyBytes, &braveResp); err != nil {
		return nil, fmt.Errorf("decode brave response: %w", err)
	}

	// Merge news results first (they're more relevant for news queries), then web results
	var results []SearchResult
	idx := 1
	seen := map[string]bool{}

	// Add news results first (if present)
	if braveResp.News != nil {
		for _, r := range braveResp.News.Results {
			if seen[r.URL] {
				continue
			}
			seen[r.URL] = true

			source := r.Source.Name
			if source == "" {
				source = extractDomain(r.URL)
			}

			results = append(results, SearchResult{
				Index:       idx,
				Title:       r.Title,
				URL:         r.URL,
				Source:      source,
				PublishedAt: r.Age,
				Snippet:     r.Description,
			})
			idx++
		}
	}

	// Add web results
	for _, r := range braveResp.Web.Results {
		if seen[r.URL] {
			continue
		}
		seen[r.URL] = true

		publishedAt := r.PageAge
		if publishedAt == "" {
			publishedAt = r.Age
		}

		results = append(results, SearchResult{
			Index:       idx,
			Title:       r.Title,
			URL:         r.URL,
			Source:      extractDomain(r.URL),
			PublishedAt: publishedAt,
			Snippet:     r.Description,
		})
		idx++
	}

	// Trim to maxResults
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	return &SearchResponse{
		Query:     req.Query,
		TimeRange: req.TimeRange,
		Results:   results,
		FetchedAt: time.Now().UTC(),
	}, nil
}

// extractDomain pulls a readable domain name from a URL.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	return host
}
