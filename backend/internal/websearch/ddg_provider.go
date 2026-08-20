package websearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ---------------------------------------------------------------------------
// DuckDuckGoProvider – zero-config web search that requires no API key.
// Uses DuckDuckGo's HTML-only endpoint and parses the results.
// This is the default fallback when no Brave API key is configured.
// ---------------------------------------------------------------------------

const ddgSearchEndpoint = "https://html.duckduckgo.com/html/"

// DuckDuckGoProvider performs web searches via DuckDuckGo HTML.
type DuckDuckGoProvider struct {
	httpClient *http.Client
	endpoint   string
}

// NewDuckDuckGoProvider creates a new DuckDuckGoProvider.
func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		endpoint: ddgSearchEndpoint,
	}
}

// Name identifies the provider in logs.
func (d *DuckDuckGoProvider) Name() string { return "duckduckgo" }

// searchEndpoint returns the configured endpoint, falling back to the public
// DuckDuckGo HTML endpoint. Tests override it; production never does.
func (d *DuckDuckGoProvider) searchEndpoint() string {
	if strings.TrimSpace(d.endpoint) == "" {
		return ddgSearchEndpoint
	}
	return d.endpoint
}

type ddgResult struct {
	title   string
	url     string
	snippet string
}

func (d *DuckDuckGoProvider) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Build query: append time-related terms for freshness.
	// DuckDuckGo HTML has no freshness parameter, so temporal terms are the only
	// available signal. An empty TimeRange means the caller deliberately wants no
	// recency bias (schedule lookups, pricing pages, reference material).
	q := req.Query
	switch req.TimeRange {
	case "24h":
		if !containsAny(strings.ToLower(q), "today", "latest", "breaking", "now") {
			q = q + " latest today"
		}
	case "7d":
		if !containsAny(strings.ToLower(q), "this week", "recent", "latest") {
			q = q + " this week"
		}
	case "30d":
		if !containsAny(strings.ToLower(q), "this month", "recent", "latest") {
			q = q + " this month"
		}
	}

	params := url.Values{}
	params.Set("q", q)

	// Set region
	if req.Region != "" {
		params.Set("kl", strings.ToLower(req.Region)+"-en")
	}

	endpoint := d.searchEndpoint() + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create ddg request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "OmniLLM-Studio/1.0")
	httpReq.Header.Set("Accept", "text/html")

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ddg request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readResponseBody(resp)
		return nil, fmt.Errorf("ddg returned status %d: %s", resp.StatusCode, truncateProviderError(string(bodyBytes)))
	}

	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read ddg response: %w", err)
	}

	raw, err := parseDDGHTML(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("parse ddg html: %w", err)
	}
	if len(raw) == 0 {
		// DuckDuckGo HTML is scraped, not an API. A markup change yields a 200
		// with zero parsable results, which must surface as a provider failure
		// rather than a successful empty search.
		return nil, fmt.Errorf("ddg returned no parsable results (markup may have changed)")
	}

	// Convert to normalized results
	var results []SearchResult
	for i, r := range raw {
		if i >= req.MaxResults && req.MaxResults > 0 {
			break
		}
		results = append(results, SearchResult{
			Index:       i + 1,
			Title:       r.title,
			URL:         r.url,
			Source:      extractDomain(r.url),
			PublishedAt: "",
			Snippet:     r.snippet,
		})
	}

	return &SearchResponse{
		Query:     req.Query,
		TimeRange: req.TimeRange,
		Results:   results,
		FetchedAt: time.Now().UTC(),
	}, nil
}

// parseDDGHTML parses the DuckDuckGo HTML search results page.
func parseDDGHTML(body io.Reader) ([]ddgResult, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	var results []ddgResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "result__body") {
					r := extractResult(n)
					if r.url != "" && r.title != "" {
						results = append(results, r)
					}
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results, nil
}

// extractResult pulls title, URL, and snippet from a DDG result__body div.
func extractResult(n *html.Node) ddgResult {
	var r ddgResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "class" {
					switch {
					case strings.Contains(a.Val, "result__a"):
						r.title = getTextContent(n)
						r.url = getHref(n)
					case strings.Contains(a.Val, "result__snippet"):
						r.snippet = getTextContent(n)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	// DDG sometimes uses redirect URLs; extract the actual URL
	if strings.Contains(r.url, "duckduckgo.com") {
		r.url = extractDDGRedirectURL(r.url)
	}

	return r
}

func getTextContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

func getHref(n *html.Node) string {
	for _, a := range n.Attr {
		if a.Key == "href" {
			return a.Val
		}
	}
	return ""
}

var ddgRedirectRe = regexp.MustCompile(`uddg=([^&]+)`)

func extractDDGRedirectURL(rawURL string) string {
	m := ddgRedirectRe.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return rawURL
	}
	decoded, err := url.QueryUnescape(m[1])
	if err != nil {
		return rawURL
	}
	return decoded
}
