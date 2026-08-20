package websearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ddgFixture mirrors the subset of DuckDuckGo HTML markup the parser depends on.
// If DuckDuckGo renames result__body / result__a / result__snippet, the parser
// silently returns nothing in production — this fixture makes that a CI failure.
const ddgFixture = `<!DOCTYPE html>
<html><body>
<div class="results">
  <div class="result results_links results_links_deep web-result">
    <div class="result__body links_main">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.anthropic.com%2Fpricing&amp;rut=abc">Claude API pricing</a>
      </h2>
      <a class="result__snippet" href="https://www.anthropic.com/pricing">Per-million-token pricing for the Claude model family.</a>
    </div>
  </div>
  <div class="result results_links results_links_deep web-result">
    <div class="result__body links_main">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="https://openai.com/api/pricing">OpenAI API pricing</a>
      </h2>
      <a class="result__snippet" href="https://openai.com/api/pricing">Current model pricing per token.</a>
    </div>
  </div>
  <div class="result results_links">
    <div class="result__body links_main">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="https://kubernetes.io/releases/">Kubernetes releases</a>
      </h2>
      <a class="result__snippet" href="https://kubernetes.io/releases/">Current supported Kubernetes versions.</a>
    </div>
  </div>
</div>
</body></html>`

func newTestDDGProvider(t *testing.T, handler http.HandlerFunc) *DuckDuckGoProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &DuckDuckGoProvider{httpClient: srv.Client(), endpoint: srv.URL}
}

func TestDDGSearchParsesFixture(t *testing.T) {
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(ddgFixture))
	})

	resp, err := provider.Search(context.Background(), SearchRequest{Query: "llm api pricing", MaxResults: 10})
	if err != nil {
		t.Fatalf("fixture must parse: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Claude API pricing" {
		t.Errorf("title: %q", resp.Results[0].Title)
	}
	// The first link is a DDG redirect and must be unwrapped.
	if resp.Results[0].URL != "https://www.anthropic.com/pricing" {
		t.Errorf("redirect URL not unwrapped: %q", resp.Results[0].URL)
	}
	if resp.Results[0].Source != "anthropic.com" {
		t.Errorf("source domain: %q", resp.Results[0].Source)
	}
	if resp.Results[0].Index != 1 || resp.Results[2].Index != 3 {
		t.Error("results must be 1-indexed in order")
	}
}

func TestDDGSearchRespectsMaxResults(t *testing.T) {
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ddgFixture))
	})

	resp, err := provider.Search(context.Background(), SearchRequest{Query: "q", MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
}

// TestDDGSearchMarkupDriftFails is the point of the fixture: a 200 response the
// parser cannot read must be an error, not a successful empty search that the
// orchestrator would treat as "nothing is happening in the world".
func TestDDGSearchMarkupDriftFails(t *testing.T) {
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div class="renamed__body"><a class="renamed__a" href="https://x.test">X</a></div></body></html>`))
	})

	_, err := provider.Search(context.Background(), SearchRequest{Query: "q", MaxResults: 5})
	if err == nil {
		t.Fatal("unparsable markup must return an error")
	}
	if !strings.Contains(err.Error(), "markup") {
		t.Errorf("error should point at markup drift: %v", err)
	}
}

func TestDDGSearchFreshnessTerms(t *testing.T) {
	cases := []struct {
		timeRange string
		query     string
		wantTerm  string
	}{
		{"24h", "kubernetes release", "latest today"},
		{"7d", "kubernetes release", "this week"},
		{"30d", "kubernetes release", "this month"},
	}
	for _, tc := range cases {
		t.Run(tc.timeRange, func(t *testing.T) {
			var sent url.Values
			provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
				sent = r.URL.Query()
				_, _ = w.Write([]byte(ddgFixture))
			})
			if _, err := provider.Search(context.Background(), SearchRequest{
				Query: tc.query, TimeRange: tc.timeRange, MaxResults: 3,
			}); err != nil {
				t.Fatal(err)
			}
			if got := sent.Get("q"); !strings.Contains(got, tc.wantTerm) {
				t.Errorf("query %q should contain %q", got, tc.wantTerm)
			}
		})
	}
}

func TestDDGSearchEmptyTimeRangeAddsNoTerms(t *testing.T) {
	var sent url.Values
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.URL.Query()
		_, _ = w.Write([]byte(ddgFixture))
	})
	if _, err := provider.Search(context.Background(), SearchRequest{Query: "anthropic pricing", MaxResults: 3}); err != nil {
		t.Fatal(err)
	}
	if got := sent.Get("q"); got != "anthropic pricing" {
		t.Errorf("no TimeRange must leave the query untouched, got %q", got)
	}
}

func TestDDGSearchRegionMapping(t *testing.T) {
	var sent url.Values
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.URL.Query()
		_, _ = w.Write([]byte(ddgFixture))
	})
	if _, err := provider.Search(context.Background(), SearchRequest{Query: "q", Region: "GB", MaxResults: 3}); err != nil {
		t.Fatal(err)
	}
	if got := sent.Get("kl"); got != "gb-en" {
		t.Errorf("region must map to kl, got %q", got)
	}
}

func TestDDGSearchNonOKStatus(t *testing.T) {
	provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("blocked"))
	})
	_, err := provider.Search(context.Background(), SearchRequest{Query: "q"})
	if err == nil {
		t.Fatal("403 must return an error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should name the status: %v", err)
	}
}

func TestExtractDDGRedirectURL(t *testing.T) {
	cases := map[string]string{
		"//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fa&rut=x": "https://example.com/a",
		"https://example.com/direct":                                   "https://example.com/direct",
	}
	for in, want := range cases {
		if got := extractDDGRedirectURL(in); got != want {
			t.Errorf("extractDDGRedirectURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDDGRateLimitIsDistinctFromMarkupDrift covers a misdiagnosis that reached a
// user's screen. DuckDuckGo answers roughly one request per source address and
// then serves an anti-bot challenge; the old code reported every unparsable body
// as "markup may have changed", which blamed the parser for a rate limit and
// invited a retry that could not succeed.
func TestDDGRateLimitIsDistinctFromMarkupDrift(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantLimited bool
	}{
		{
			name:        "202 challenge",
			status:      http.StatusAccepted,
			body:        `<!DOCTYPE html><html><head><title>DuckDuckGo</title></head><body></body></html>`,
			wantLimited: true,
		},
		{
			name:        "429",
			status:      http.StatusTooManyRequests,
			body:        "slow down",
			wantLimited: true,
		},
		{
			// Verbatim wording from an observed challenge body.
			name:   "200 challenge page",
			status: http.StatusOK,
			body: `<html><body><p>Unfortunately, bots use DuckDuckGo too. Please complete the ` +
				`following challenge to confirm this search was made by a human. Select all ` +
				`squares containing a duck:</p></body></html>`,
			wantLimited: true,
		},
		{
			// A real class rename must still be reported as a markup change, even
			// though it also yields zero parsable results. Guessing from body size
			// got this wrong and blamed a rate limit instead.
			name:        "genuine markup change",
			status:      http.StatusOK,
			body:        `<html><body><div class="renamed__body"><a class="renamed__a" href="https://x.test">X</a></div></body></html>`,
			wantLimited: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newTestDDGProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := provider.Search(context.Background(), SearchRequest{Query: "q", MaxResults: 5})
			if err == nil {
				t.Fatal("expected an error")
			}
			limited := errors.Is(err, ErrSearchProviderRateLimited)
			if limited != tc.wantLimited {
				t.Errorf("rate-limited = %v, want %v (err: %v)", limited, tc.wantLimited, err)
			}
			if !tc.wantLimited && !strings.Contains(err.Error(), "markup") {
				t.Errorf("a real markup change should still say so: %v", err)
			}
		})
	}
}

// TestDDGCapabilitiesAreHonest pins the declared limits. Reporting DuckDuckGo as
// more capable than it is caused the planner to issue expanded query sets that
// were guaranteed to be refused.
func TestDDGCapabilitiesAreHonest(t *testing.T) {
	capabilities := NewDuckDuckGoProvider().Capabilities()
	if capabilities.MaxQueriesPerTurn != 1 {
		t.Errorf("MaxQueriesPerTurn = %d; measured behaviour is one request per source address", capabilities.MaxQueriesPerTurn)
	}
	if capabilities.SupportsFreshnessFilter {
		t.Error("DuckDuckGo HTML has no freshness parameter")
	}
	if capabilities.ProvidesPublicationDates {
		t.Error("DuckDuckGo HTML returns no publication dates, so freshness cannot be verified")
	}
}
