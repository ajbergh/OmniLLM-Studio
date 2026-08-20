package websearch

import (
	"context"
	"fmt"
	"time"
)

// Provider is the interface every web-search backend must implement.
// Swap FakeProvider for a real one (Brave Search, SearXNG, Tavily, etc.)
// by satisfying this interface.
type Provider interface {
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
	// Name identifies the provider in logs and telemetry. It must never include
	// credentials.
	Name() string
	// Capabilities describes what this provider can reliably deliver. The planner
	// bounds itself by these rather than assuming every provider behaves like an
	// API with a quota.
	Capabilities() ProviderCapabilities
}

// ProviderCapabilities describes the limits of a search backend.
//
// This exists because the two supported providers are not the same kind of
// thing. Brave is an API with a quota and real query parameters. DuckDuckGo is a
// scraped HTML endpoint that serves an anti-bot challenge after roughly one
// request per source address, so issuing an expanded three-query plan against it
// guarantees that queries two and three come back empty.
type ProviderCapabilities struct {
	// MaxQueriesPerTurn bounds how many searches one turn may issue.
	MaxQueriesPerTurn int
	// SupportsFreshnessFilter is true when the provider honors a real recency
	// parameter rather than approximating it with query text.
	SupportsFreshnessFilter bool
	// ProvidesPublicationDates is true when results carry dates that
	// ParsePublishedAt can read. Without them, freshness cannot be verified.
	ProvidesPublicationDates bool
}

// ---------------------------------------------------------------------------
// FakeProvider – returns canned results so the full pipeline can be tested
// end-to-end without any external API key or network call.
// ---------------------------------------------------------------------------

type FakeProvider struct{}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{}
}

// Name identifies the provider in logs.
func (f *FakeProvider) Name() string { return "fake" }

// Capabilities reports the canned provider as fully capable.
func (f *FakeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{MaxQueriesPerTurn: 5, SupportsFreshnessFilter: true, ProvidesPublicationDates: true}
}

func (f *FakeProvider) Search(_ context.Context, req SearchRequest) (*SearchResponse, error) {
	now := time.Now().UTC()
	results := []SearchResult{
		{
			Index:       1,
			Title:       fmt.Sprintf("Latest developments: %s - Reuters", req.Query),
			URL:         "https://www.reuters.com/example/1",
			Source:      "Reuters",
			PublishedAt: now.Add(-1 * time.Hour).Format(time.RFC3339),
			Snippet:     fmt.Sprintf("In a developing story, experts weigh in on %s. Multiple sources confirm significant activity in this area over the past 24 hours.", req.Query),
		},
		{
			Index:       2,
			Title:       fmt.Sprintf("%s: What you need to know today - AP News", req.Query),
			URL:         "https://apnews.com/example/2",
			Source:      "AP News",
			PublishedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
			Snippet:     fmt.Sprintf("An overview of the current situation regarding %s, with context and analysis from leading journalists.", req.Query),
		},
		{
			Index:       3,
			Title:       fmt.Sprintf("Breaking: Major update on %s - BBC News", req.Query),
			URL:         "https://www.bbc.com/news/example/3",
			Source:      "BBC News",
			PublishedAt: now.Add(-3 * time.Hour).Format(time.RFC3339),
			Snippet:     fmt.Sprintf("The BBC understands that new information has emerged regarding %s, prompting widespread discussion.", req.Query),
		},
		{
			Index:       4,
			Title:       fmt.Sprintf("Analysis: The impact of %s - The Guardian", req.Query),
			URL:         "https://www.theguardian.com/example/4",
			Source:      "The Guardian",
			PublishedAt: now.Add(-5 * time.Hour).Format(time.RFC3339),
			Snippet:     fmt.Sprintf("Our correspondents analyse the wider implications of %s and what it means going forward.", req.Query),
		},
		{
			Index:       5,
			Title:       fmt.Sprintf("Experts react to %s developments - NPR", req.Query),
			URL:         "https://www.npr.org/example/5",
			Source:      "NPR",
			PublishedAt: now.Add(-6 * time.Hour).Format(time.RFC3339),
			Snippet:     fmt.Sprintf("Leading researchers and policy experts share their perspectives on the latest %s developments.", req.Query),
		},
	}

	// Limit results to maxResults
	if req.MaxResults > 0 && req.MaxResults < len(results) {
		results = results[:req.MaxResults]
	}

	return &SearchResponse{
		Query:     req.Query,
		TimeRange: req.TimeRange,
		Results:   results,
		FetchedAt: now,
	}, nil
}
