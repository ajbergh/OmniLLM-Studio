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

// Capabilities reports DuckDuckGo honestly, which means reporting it as weak.
//
// It is a scraped HTML endpoint, not an API. Measured behaviour: the first
// request from a source address returns results, and subsequent requests return
// HTTP 202 with a challenge page and zero parsable results — for minutes. So one
// query per turn is the only volume that reliably works, and issuing the
// planner's expanded three-query set against it guarantees two empty results.
//
// It also has no freshness parameter and returns no publication dates, so
// recency can only be hinted at in the query text and can never be verified.
func (d *DuckDuckGoProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		MaxQueriesPerTurn:        1,
		SupportsFreshnessFilter:  false,
		ProvidesPublicationDates: false,
	}
}

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

	// HTTP 202 with an HTML body is DuckDuckGo's anti-bot challenge, not a
	// transient error. It is returned per source address for minutes after about
	// one successful request, so retrying inside the same turn cannot help.
	if resp.StatusCode == http.StatusAccepted {
		return nil, ErrSearchProviderRateLimited
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrSearchProviderRateLimited
	}
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
		// A 200 with no parsable results is ambiguous: either the markup changed,
		// or this is a challenge page served with a 200. Distinguish them, because
		// the previous single message blamed the parser for what was almost always
		// a rate limit and sent readers to the wrong place.
		if looksLikeDDGChallenge(bodyBytes) {
			return nil, ErrSearchProviderRateLimited
		}
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

// ddgChallengeMarkers are phrases taken from an observed DuckDuckGo challenge
// page. The live body reads:
//
//	"Unfortunately, bots use DuckDuckGo too. Please complete the following
//	 challenge to confirm this search was made by a human. Select all squares
//	 containing a duck"
//
// Detection is by explicit marker only. An earlier version also guessed from body
// size and the absence of result markup, which misclassified a genuine markup
// change as a rate limit — a wrong diagnosis is worse than an unspecific one,
// because it sends the reader to the wrong file.
var ddgChallengeMarkers = []string{
	"bots use duckduckgo",
	"confirm this search was made by a human",
	"complete the following challenge",
	"squares containing a duck",
	"unusual traffic",
	"captcha",
	"anomaly",
}

// looksLikeDDGChallenge reports whether a 200 body is an anti-bot challenge
// rather than a results page.
func looksLikeDDGChallenge(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, marker := range ddgChallengeMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
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
