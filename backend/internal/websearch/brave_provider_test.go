package websearch

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const braveFixture = `{
  "web": {
    "results": [
      {
        "title": "Claude API pricing",
        "url": "https://www.anthropic.com/pricing",
        "description": "Per-million-token pricing for the Claude model family.",
        "page_age": "2026-08-18T00:00:00Z",
        "meta_url": {"hostname": "www.anthropic.com"}
      },
      {
        "title": "OpenAI API pricing",
        "url": "https://openai.com/api/pricing",
        "description": "Current model pricing.",
        "age": "3 days ago",
        "meta_url": {"hostname": "openai.com"}
      }
    ]
  },
  "news": {
    "results": [
      {
        "title": "New frontier model released",
        "url": "https://example.com/news/1",
        "description": "Coverage of the launch.",
        "age": "2 hours ago",
        "meta_url": {"hostname": "example.com"},
        "source": {"name": "Example Wire"}
      }
    ]
  }
}`

func newTestBraveProvider(t *testing.T, handler http.HandlerFunc) (*BraveProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &BraveProvider{
		apiKey:     "test-key",
		httpClient: srv.Client(),
		endpoint:   srv.URL,
	}, srv
}

// TestBraveSearchDecodesGzipResponse is the regression test for the transport
// defect that silently disabled Brave search. net/http only auto-decompresses
// when it owns the Accept-Encoding header, so a gzip body must still decode.
func TestBraveSearchDecodesGzipResponse(t *testing.T) {
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = gz.Write([]byte(braveFixture))
	})

	resp, err := provider.Search(context.Background(), SearchRequest{Query: "llm api pricing", MaxResults: 5})
	if err != nil {
		t.Fatalf("gzip response must decode, got: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results (1 news + 2 web), got %d", len(resp.Results))
	}
	if resp.Results[0].Source != "Example Wire" {
		t.Errorf("news results should come first with their source name, got %q", resp.Results[0].Source)
	}
	if resp.FetchedAt.IsZero() {
		t.Error("FetchedAt must be stamped")
	}
}

// TestBraveSearchDecodesGzipWithoutTransportHelp covers the exact shape of the
// original defect: a response whose Content-Encoding survives to the caller,
// which is what happens when Accept-Encoding is set by hand. The provider must
// decode it rather than handing gzip bytes to json.Unmarshal.
func TestBraveSearchDecodesGzipWithoutTransportHelp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = gz.Write([]byte(braveFixture))
	}))
	t.Cleanup(srv.Close)

	// DisableCompression stops the transport from negotiating or decoding gzip,
	// so Content-Encoding reaches readResponseBody exactly as it did before.
	provider := &BraveProvider{
		apiKey:     "test-key",
		httpClient: &http.Client{Transport: &http.Transport{DisableCompression: true}},
		endpoint:   srv.URL,
	}

	resp, err := provider.Search(context.Background(), SearchRequest{Query: "q"})
	if err != nil {
		t.Fatalf("undecoded gzip body must still parse, got: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results from the gzip fixture")
	}
}

func TestBraveSearchPlainJSONResponse(t *testing.T) {
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(braveFixture))
	})

	resp, err := provider.Search(context.Background(), SearchRequest{Query: "q", MaxResults: 2})
	if err != nil {
		t.Fatalf("plain JSON: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("MaxResults must trim, got %d", len(resp.Results))
	}
}

func TestBraveSearchMalformedBody(t *testing.T) {
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json at all"))
	})

	if _, err := provider.Search(context.Background(), SearchRequest{Query: "q"}); err == nil {
		t.Fatal("malformed body must return an error")
	}
}

func TestBraveSearchNonOKStatus(t *testing.T) {
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(strings.Repeat("x", 4000)))
	})

	_, err := provider.Search(context.Background(), SearchRequest{Query: "q"})
	if err == nil {
		t.Fatal("429 must return an error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should name the status: %v", err)
	}
	if len(err.Error()) > maxProviderErrorChars+128 {
		t.Errorf("error body must be truncated, got %d chars", len(err.Error()))
	}
}

func TestBraveSearchRequestParameters(t *testing.T) {
	var query url.Values
	var token string
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		token = r.Header.Get("X-Subscription-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(braveFixture))
	})

	_, err := provider.Search(context.Background(), SearchRequest{
		Query:      "current kubernetes release",
		TimeRange:  "7d",
		Region:     "US",
		Locale:     "en-GB",
		MaxResults: 6,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := query.Get("freshness"); got != "pw" {
		t.Errorf("7d must map to freshness=pw, got %q", got)
	}
	if got := query.Get("country"); got != "us" {
		t.Errorf("region must map to country=us, got %q", got)
	}
	if got := query.Get("count"); got != "6" {
		t.Errorf("count should be 6, got %q", got)
	}
	if token != "test-key" {
		t.Errorf("subscription token not sent, got %q", token)
	}
	// Phase 1.6: locale must reach Brave rather than dying in SearchRequest.
	if got := query.Get("search_lang"); got != "en" {
		t.Errorf("locale must map to search_lang=en, got %q", got)
	}
	if got := query.Get("ui_lang"); got != "en-GB" {
		t.Errorf("locale must map to ui_lang=en-GB, got %q", got)
	}
}

func TestBraveSearchEmptyTimeRangeOmitsFreshness(t *testing.T) {
	var query url.Values
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(braveFixture))
	})

	if _, err := provider.Search(context.Background(), SearchRequest{Query: "anthropic pricing"}); err != nil {
		t.Fatal(err)
	}
	if query.Has("freshness") {
		t.Error("an empty TimeRange must not send a freshness filter")
	}
}

func TestBraveSearchContextCancellation(t *testing.T) {
	provider, _ := newTestBraveProvider(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := provider.Search(ctx, SearchRequest{Query: "q"}); err == nil {
		t.Fatal("cancelled context must return an error")
	}
}
