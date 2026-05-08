package news

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- Intent Detection Tests ----

func TestDetectNewsIntent(t *testing.T) {
	tests := []struct {
		query       string
		wantHandled bool
		wantSlug    string
		wantSearch  string
		wantReason  string
	}{
		{"What are the latest headlines?", true, "", "", ""},
		{"Latest AI news", true, "science-technology", "", ""},
		{"Show me climate headlines", true, "planet-climate", "", ""},
		{"What is happening with nuclear risk?", true, "existential-threats", "", ""},
		{"Latest public health news", true, "human-development", "", ""},
		{"Give me today's top global headlines", true, "", "", ""},
		{"What's happening in AI regulation", true, "science-technology", "", ""},
		{"Latest pandemic risk news", true, "existential-threats", "pandemic risk", ""},
		{"Show me important global stories", true, "", "", ""},

		// Sports should be rejected
		{"latest Cubs news", false, "", "", "sports-related prompt"},
		{"latest NBA news", false, "", "", "sports-related prompt"},
		{"what are the latest MLB standings", false, "", "", "sports-related prompt"},
		{"latest NFL scores", false, "", "", "sports-related prompt"},

		// Non-news creative prompts should be rejected
		{"write a fictional newspaper article about a Christmas market", false, "", "", "non-news creative prompt"},
		{"Write a fake newspaper story", false, "", "", "non-news creative prompt"},
		{"Design a newspaper layout", false, "", "", "non-news creative prompt"},

		// Non-news prompts should be rejected
		{"Explain climate change", false, "", "", "no news indicator found"},
		{"Tell me the history of newspapers", false, "", "", "non-news creative prompt"},
		{"Create a news-style landing page for my event", false, "", "", "non-news creative prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := DetectNewsIntent(tt.query)
			if got.Handled != tt.wantHandled {
				t.Fatalf("Handled = %v, want %v (reason: %s)", got.Handled, tt.wantHandled, got.Reason)
			}
			if got.Handled {
				if got.IssueSlug != tt.wantSlug {
					t.Fatalf("IssueSlug = %q, want %q", got.IssueSlug, tt.wantSlug)
				}
				if tt.wantSearch != "" && got.Search != tt.wantSearch {
					t.Fatalf("Search = %q, want %q", got.Search, tt.wantSearch)
				}
				if got.Confidence < 0.65 {
					t.Fatalf("Confidence = %f, want >= 0.65", got.Confidence)
				}
			} else if tt.wantReason != "" && got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestDetectNewsIntentPresentation(t *testing.T) {
	tests := []struct {
		query    string
		wantType NewsIntentType
		wantSize int
	}{
		{"Give me a front page of news", NewsIntentFrontPage, 10},
		{"Show me a newspaper edition", NewsIntentFrontPage, 10},
		{"Give me a quick news brief", NewsIntentBrief, 5},
		{"Give me a detailed news deep dive", NewsIntentDetailed, 8},
		{"Top 5 headlines today", NewsIntentFrontPage, 5},
		{"Latest headlines", NewsIntentFrontPage, 8},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := DetectNewsIntent(tt.query)
			if !got.Handled {
				t.Fatalf("expected handled for %q", tt.query)
			}
			if got.IntentType != tt.wantType {
				t.Fatalf("IntentType = %q, want %q", got.IntentType, tt.wantType)
			}
			if got.PageSize != tt.wantSize {
				t.Fatalf("PageSize = %d, want %d", got.PageSize, tt.wantSize)
			}
		})
	}
}

// ---- API Client Tests ----

func TestClientGetStories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stories" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("missing Accept header")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("missing User-Agent header")
		}

		// Check query params
		q := r.URL.Query()
		if q.Get("page") != "1" {
			t.Fatalf("page = %s, want 1", q.Get("page"))
		}
		if q.Get("pageSize") != "8" {
			t.Fatalf("pageSize = %s, want 8", q.Get("pageSize"))
		}

		resp := StoryPage{
			Data: []Story{
				{
					ID:        "1",
					Slug:      "test-story",
					Title:     "Test Story",
					Summary:   "This is a test story summary.",
					SourceURL: "https://example.com/test",
					Issue:     &IssueRef{Name: "Science & Technology", Slug: "science-technology"},
					Relevance: intPtr(8),
				},
			},
			Total:      1,
			Page:       1,
			PageSize:   8,
			TotalPages: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL + "/api",
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	page, err := client.GetStories(context.Background(), StoryQuery{Page: 1, PageSize: 8})
	if err != nil {
		t.Fatalf("GetStories failed: %v", err)
	}
	if page == nil {
		t.Fatal("page is nil")
	}
	if len(page.Data) != 1 {
		t.Fatalf("got %d stories, want 1", len(page.Data))
	}
	if page.Data[0].Title != "Test Story" {
		t.Fatalf("title = %q, want %q", page.Data[0].Title, "Test Story")
	}
}

func TestClientGetStoriesWithParams(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		resp := StoryPage{Data: []Story{}, Total: 0, Page: 1, PageSize: 8, TotalPages: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.GetStories(context.Background(), StoryQuery{
		Page:        1,
		PageSize:    8,
		IssueSlug:   "science-technology",
		Search:      "AI regulation",
		EmotionTags: []string{"positive", "important"},
	})
	if err != nil {
		t.Fatalf("GetStories failed: %v", err)
	}
	if !strings.Contains(capturedQuery, "issueSlug=science-technology") {
		t.Fatalf("missing issueSlug in query: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "search=AI+regulation") {
		t.Fatalf("missing search in query: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "emotionTags=positive%2Cimportant") {
		t.Fatalf("missing emotionTags in query: %s", capturedQuery)
	}
}

func TestClientNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.GetStories(context.Background(), StoryQuery{Page: 1, PageSize: 8})
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
	var apiErr *APIError
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should contain status code: %v", err)
	}
	_ = apiErr // avoid unused variable warning
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[],"total":0,"page":1,"pageSize":8,"totalPages":0}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 1 * time.Millisecond},
	}

	ctx := context.Background()
	_, err := client.GetStories(ctx, StoryQuery{Page: 1, PageSize: 8})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClientMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.GetStories(context.Background(), StoryQuery{Page: 1, PageSize: 8})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestClientMissingOptionalFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"1","slug":"test"}],"total":1,"page":1,"pageSize":8,"totalPages":1}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	page, err := client.GetStories(context.Background(), StoryQuery{Page: 1, PageSize: 8})
	if err != nil {
		t.Fatalf("GetStories failed: %v", err)
	}
	if len(page.Data) != 1 {
		t.Fatalf("got %d stories, want 1", len(page.Data))
	}
	if page.Data[0].ID != "1" {
		t.Fatalf("id = %q, want 1", page.Data[0].ID)
	}
	// Optional fields should be zero-valued, not cause errors
	if page.Data[0].Title != "" {
		t.Fatalf("expected empty title, got %q", page.Data[0].Title)
	}
}

func TestClientPageSizeClamping(t *testing.T) {
	// The client doesn't clamp - the service layer does.
	// This test verifies the client passes through whatever pageSize is given.
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		resp := StoryPage{Data: []Story{}, Total: 0, Page: 1, PageSize: 100, TotalPages: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		UserAgent:  "OmniLLM-Studio/Test",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.GetStories(context.Background(), StoryQuery{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("GetStories failed: %v", err)
	}
	if !strings.Contains(capturedQuery, "pageSize=100") {
		t.Fatalf("expected pageSize=100 in query, got: %s", capturedQuery)
	}
}

// ---- Cache Tests ----

func TestCacheHitAndMiss(t *testing.T) {
	cache := NewCache(5 * time.Minute)

	q1 := StoryQuery{Page: 1, PageSize: 8, IssueSlug: "science-technology"}
	q2 := StoryQuery{Page: 1, PageSize: 8, IssueSlug: "planet-climate"}

	// Miss on empty cache
	if _, ok := cache.Get(q1); ok {
		t.Fatal("expected cache miss on empty cache")
	}

	// Set and hit
	data := &StoryPage{Data: []Story{{ID: "1"}}, Total: 1}
	cache.Set(q1, data)

	if cached, ok := cache.Get(q1); !ok {
		t.Fatal("expected cache hit")
	} else if len(cached.Data) != 1 {
		t.Fatal("wrong cached data")
	}

	// Miss on different key
	if _, ok := cache.Get(q2); ok {
		t.Fatal("expected cache miss for different key")
	}
}

func TestCacheExpiry(t *testing.T) {
	cache := NewCache(1 * time.Millisecond)

	q := StoryQuery{Page: 1, PageSize: 8}
	cache.Set(q, &StoryPage{Data: []Story{{ID: "1"}}, Total: 1})

	time.Sleep(5 * time.Millisecond)

	if _, ok := cache.Get(q); ok {
		t.Fatal("expected cache miss after expiry")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(5 * time.Minute)

	q := StoryQuery{Page: 1, PageSize: 8}
	cache.Set(q, &StoryPage{Data: []Story{{ID: "1"}}, Total: 1})

	// Verify hit
	if _, ok := cache.Get(q); !ok {
		t.Fatal("expected cache hit before clear")
	}

	cache.Clear()

	// Stats should be zero after clear
	hits, misses := cache.Stats()
	if hits != 0 || misses != 0 {
		t.Fatalf("expected zero stats after clear, got hits=%d misses=%d", hits, misses)
	}

	// Get after clear should miss
	if _, ok := cache.Get(q); ok {
		t.Fatal("expected cache miss after clear")
	}
}

// ---- Formatter Tests ----

func TestFormatNewspaperEdition(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	stories := []Story{
		{
			ID:        "1",
			Title:     "Major Tech Breakthrough Announced",
			Summary:   "A major breakthrough in AI technology was announced today.",
			SourceURL: "https://example.com/tech-breakthrough",
			Issue:     &IssueRef{Name: "Science & Technology", Slug: "science-technology"},
			Relevance: intPtr(9),
		},
		{
			ID:        "2",
			Title:     "Climate Summit Reaches New Agreement",
			Summary:   "World leaders have reached a historic climate agreement.",
			SourceURL: "https://example.com/climate-summit",
			Issue:     &IssueRef{Name: "Planet & Climate", Slug: "planet-climate"},
			Relevance: intPtr(8),
		},
	}

	input := EditionInput{
		Prompt:      "Latest headlines",
		Intent:      NewsIntent{Handled: true, IntentType: NewsIntentFrontPage, IssueSlug: "science-technology"},
		Stories:     stories,
		Total:       2,
		Broadened:   false,
		GeneratedAt: now,
	}

	result := FormatNewspaperEdition(input)

	// Check key elements
	if !strings.Contains(result, "The OmniLLM Front Page") {
		t.Fatal("missing title")
	}
	if !strings.Contains(result, "Science & Technology Edition") {
		t.Fatal("missing topic edition")
	}
	if !strings.Contains(result, "Actually Relevant News") {
		t.Fatal("missing source attribution")
	}
	if !strings.Contains(result, "May 8, 2026") {
		t.Fatal("missing date")
	}
	if !strings.Contains(result, "Lead Story") {
		t.Fatal("missing lead story section")
	}
	if !strings.Contains(result, "Major Tech Breakthrough Announced") {
		t.Fatal("missing lead story title")
	}
	if !strings.Contains(result, "More Headlines") {
		t.Fatal("missing more headlines section")
	}
	if !strings.Contains(result, "Climate Summit Reaches New Agreement") {
		t.Fatal("missing headline title")
	}
	if !strings.Contains(result, "Source Notes") {
		t.Fatal("missing source notes")
	}
	if !strings.Contains(result, "https://example.com/tech-breakthrough") {
		t.Fatal("missing source URL")
	}
}

func TestFormatNewspaperEditionNoResults(t *testing.T) {
	input := EditionInput{
		Prompt:      "Latest news",
		Intent:      NewsIntent{Handled: true},
		Stories:     nil,
		Total:       0,
		Broadened:   false,
		GeneratedAt: time.Now(),
	}

	result := FormatNewspaperEdition(input)
	if !strings.Contains(result, "No matching curated stories found") {
		t.Fatal("expected no-results message")
	}
}

func TestFormatNewspaperEditionBroadened(t *testing.T) {
	stories := []Story{
		{
			ID:        "1",
			Title:     "Test Story",
			Summary:   "Test summary.",
			SourceURL: "https://example.com/test",
		},
	}

	input := EditionInput{
		Prompt:      "Latest news",
		Intent:      NewsIntent{Handled: true},
		Stories:     stories,
		Total:       1,
		Broadened:   true,
		GeneratedAt: time.Now(),
	}

	result := FormatNewspaperEdition(input)
	if !strings.Contains(result, "broadened the edition") {
		t.Fatal("expected broadened note")
	}
}

func TestFormatNewspaperEditionHTMLSafety(t *testing.T) {
	stories := []Story{
		{
			ID:        "1",
			Title:     "<script>alert('xss')</script>",
			Summary:   "Summary with <script>danger</script>",
			SourceURL: "https://example.com/test",
			Quote:     "Quote with <b>html</b>",
		},
	}

	input := EditionInput{
		Prompt:      "Latest news",
		Intent:      NewsIntent{Handled: true},
		Stories:     stories,
		Total:       1,
		Broadened:   false,
		GeneratedAt: time.Now(),
	}

	result := FormatNewspaperEdition(input)
	if strings.Contains(result, "<script>") {
		t.Fatal("HTML should be escaped, got raw <script> tag")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Fatal("expected escaped script tag")
	}
}

func TestSafeMarkdownLink(t *testing.T) {
	tests := []struct {
		text   string
		rawURL string
		want   string
	}{
		{"Test Story", "https://example.com/story", "[Test Story](https://example.com/story)"},
		{"Test Story", "http://example.com/story", "Test Story"},
		{"Test Story", "javascript:alert(1)", "Test Story"},
		{"Test Story", "", "Test Story"},
		{"Story [with] brackets", "https://example.com/s", "[Story \\[with\\] brackets](https://example.com/s)"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := safeMarkdownLink(tt.text, tt.rawURL)
			if got != tt.want {
				t.Fatalf("safeMarkdownLink = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatIssueDisplayName(t *testing.T) {
	tests := map[string]string{
		"science-technology":  "Science & Technology",
		"planet-climate":      "Planet & Climate",
		"existential-threats": "Existential Threats",
		"human-development":   "Human Development",
		"custom-topic":        "Custom Topic",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got := formatIssueDisplayName(input)
			if got != want {
				t.Fatalf("formatIssueDisplayName(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

// ---- Config Tests ----

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Fatal("expected enabled by default")
	}
	if cfg.BaseURL != "https://actually-relevant-api.onrender.com/api" {
		t.Fatalf("unexpected base URL: %s", cfg.BaseURL)
	}
	if cfg.Timeout != 8*time.Second {
		t.Fatalf("unexpected timeout: %v", cfg.Timeout)
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Fatalf("unexpected cache TTL: %v", cfg.CacheTTL)
	}
	if cfg.DefaultPageSize != 8 {
		t.Fatalf("unexpected default page size: %d", cfg.DefaultPageSize)
	}
	if cfg.MaxPageSize != 15 {
		t.Fatalf("unexpected max page size: %d", cfg.MaxPageSize)
	}
	if cfg.UserAgent != "OmniLLM-Studio/NewsLookup" {
		t.Fatalf("unexpected user agent: %s", cfg.UserAgent)
	}
}

// ---- Service Tests ----

func TestServiceDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	svc := NewService(cfg)

	result, err := svc.TryAnswer(context.Background(), "Latest headlines")
	if err != nil {
		t.Fatalf("TryAnswer failed: %v", err)
	}
	if result.Handled {
		t.Fatal("expected not handled when disabled")
	}
}

func TestServiceNonNewsPrompt(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewService(cfg)

	result, err := svc.TryAnswer(context.Background(), "Explain climate change")
	if err != nil {
		t.Fatalf("TryAnswer failed: %v", err)
	}
	if result.Handled {
		t.Fatal("expected not handled for non-news prompt")
	}
}

func TestServiceSportsPrompt(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewService(cfg)

	result, err := svc.TryAnswer(context.Background(), "latest Cubs news")
	if err != nil {
		t.Fatalf("TryAnswer failed: %v", err)
	}
	if result.Handled {
		t.Fatal("expected not handled for sports prompt")
	}
}

// ---- Helper ----

func intPtr(n int) *int {
	return &n
}
