package news

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Service orchestrates news lookup: intent detection, API calls, caching, and formatting.
type Service struct {
	client *Client
	cache  *Cache
	cfg    Config
}

// NewService creates a new news lookup service.
func NewService(cfg Config) *Service {
	return &Service{
		client: NewClient(cfg),
		cache:  NewCache(cfg.CacheTTL),
		cfg:    cfg,
	}
}

// TryAnswer attempts to answer a user prompt with news data.
// Returns a LookupResult with Handled=false if the prompt is not a news request.
func (s *Service) TryAnswer(ctx context.Context, prompt string) (*LookupResult, error) {
	if !s.cfg.Enabled {
		return &LookupResult{Handled: false}, nil
	}

	intent := DetectNewsIntent(prompt)
	if !intent.Handled {
		return &LookupResult{Handled: false}, nil
	}

	start := time.Now()

	// Build query
	pageSize := intent.PageSize
	if pageSize <= 0 {
		pageSize = s.cfg.DefaultPageSize
	}
	if pageSize > s.cfg.MaxPageSize {
		pageSize = s.cfg.MaxPageSize
	}

	q := StoryQuery{
		Page:      1,
		PageSize:  pageSize,
		IssueSlug: intent.IssueSlug,
		Search:    intent.Search,
	}

	// Check cache
	if cached, ok := s.cache.Get(q); ok {
		if len(cached.Data) > 0 {
			content := FormatNewspaperEdition(EditionInput{
				Prompt:      prompt,
				Intent:      intent,
				Stories:     cached.Data,
				Total:       cached.Total,
				Broadened:   false,
				GeneratedAt: time.Now(),
			})
			durationMs := int(time.Since(start).Milliseconds())
			return &LookupResult{
				Handled: true,
				Content: content,
				Metadata: map[string]any{
					"provider":    "actually_relevant",
					"issue_slug":  intent.IssueSlug,
					"search":      intent.Search,
					"story_count": len(cached.Data),
					"from_cache":  true,
					"duration_ms": durationMs,
				},
			}, nil
		}
	}

	// Fetch from API
	page, err := s.client.GetStories(ctx, q)
	if err != nil {
		log.Printf("[news] API request failed: %v", err)
		return s.errorResult(intent, err), nil
	}

	if page == nil || len(page.Data) == 0 {
		// Try broadening: remove search, keep issue slug
		if q.Search != "" {
			q.Search = ""
			if broadenedPage, err := s.client.GetStories(ctx, q); err == nil && broadenedPage != nil && len(broadenedPage.Data) > 0 {
				page = broadenedPage
				s.cache.Set(q, page)
				content := FormatNewspaperEdition(EditionInput{
					Prompt:      prompt,
					Intent:      intent,
					Stories:     page.Data,
					Total:       page.Total,
					Broadened:   true,
					GeneratedAt: time.Now(),
				})
				durationMs := int(time.Since(start).Milliseconds())
				return &LookupResult{
					Handled: true,
					Content: content,
					Metadata: map[string]any{
						"provider":    "actually_relevant",
						"issue_slug":  intent.IssueSlug,
						"search":      intent.Search,
						"story_count": len(page.Data),
						"from_cache":  false,
						"broadened":   true,
						"duration_ms": durationMs,
					},
				}, nil
			}
		}

		// Try broadening: remove issue slug too
		if q.IssueSlug != "" {
			q.IssueSlug = ""
			if broadenedPage, err := s.client.GetStories(ctx, q); err == nil && broadenedPage != nil && len(broadenedPage.Data) > 0 {
				page = broadenedPage
				s.cache.Set(q, page)
				content := FormatNewspaperEdition(EditionInput{
					Prompt:      prompt,
					Intent:      intent,
					Stories:     page.Data,
					Total:       page.Total,
					Broadened:   true,
					GeneratedAt: time.Now(),
				})
				durationMs := int(time.Since(start).Milliseconds())
				return &LookupResult{
					Handled: true,
					Content: content,
					Metadata: map[string]any{
						"provider":    "actually_relevant",
						"issue_slug":  intent.IssueSlug,
						"search":      intent.Search,
						"story_count": len(page.Data),
						"from_cache":  false,
						"broadened":   true,
						"duration_ms": durationMs,
					},
				}, nil
			}
		}

		// No results at all
		content := FormatNewspaperEdition(EditionInput{
			Prompt:      prompt,
			Intent:      intent,
			Stories:     nil,
			Total:       0,
			Broadened:   false,
			GeneratedAt: time.Now(),
		})
		durationMs := int(time.Since(start).Milliseconds())
		return &LookupResult{
			Handled: true,
			Content: content,
			Metadata: map[string]any{
				"provider":    "actually_relevant",
				"issue_slug":  intent.IssueSlug,
				"search":      intent.Search,
				"story_count": 0,
				"from_cache":  false,
				"duration_ms": durationMs,
			},
		}, nil
	}

	// Cache and format results
	s.cache.Set(q, page)
	content := FormatNewspaperEdition(EditionInput{
		Prompt:      prompt,
		Intent:      intent,
		Stories:     page.Data,
		Total:       page.Total,
		Broadened:   false,
		GeneratedAt: time.Now(),
	})
	durationMs := int(time.Since(start).Milliseconds())

	return &LookupResult{
		Handled: true,
		Content: content,
		Metadata: map[string]any{
			"provider":    "actually_relevant",
			"issue_slug":  intent.IssueSlug,
			"search":      intent.Search,
			"story_count": len(page.Data),
			"from_cache":  false,
			"duration_ms": durationMs,
		},
	}, nil
}

// errorResult returns a user-facing error message when the API is unreachable.
func (s *Service) errorResult(intent NewsIntent, err error) *LookupResult {
	content := fmt.Sprintf(`# News lookup unavailable

I could not reach the Actually Relevant News API right now.

**What I tried:** %s
**Fallback:** I am not going to answer from memory because this is a current-news request.

*Error: %s*`, intent.Reason, err)

	return &LookupResult{
		Handled: true,
		Content: content,
		Metadata: map[string]any{
			"provider": "actually_relevant",
			"error":    err.Error(),
		},
	}
}
