package urlcontext

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service is the URL context resolver. It detects URLs in user messages,
// fetches source context, and prepares prompt packs for the LLM.
type Service struct {
	cfg        *Config
	fetcher    *Fetcher
	inspector  *GitHubInspector
	cache      *Cache
	browserMgr BrowserNavigator
}

// BrowserNavigator is the minimum interface needed for browser fallback.
type BrowserNavigator interface {
	Navigate(ctx context.Context, url string) (text string, title string, err error)
}

const minUsableChars = 100

// NewService creates a URL context service with the given configuration.
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	fetcher := NewFetcher(cfg)
	return &Service{
		cfg:       cfg,
		fetcher:   fetcher,
		inspector: NewGitHubInspector(fetcher, cfg),
		cache:     NewCache(cfg.CacheTTL),
	}
}

// SetBrowserManager enables browser fallback for JS-rendered pages.
func (s *Service) SetBrowserManager(m BrowserNavigator) {
	s.browserMgr = m
}

// Resolve is the main entry point. It extracts URLs from the user message,
// determines if URL context is required, fetches source content, and
// returns a ResolveResult ready to be applied to the LLM request.
func (s *Service) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResult, error) {
	if !s.cfg.Enabled {
		return &ResolveResult{Handled: false}, nil
	}

	// Extract URLs
	urls := req.URLs
	if len(urls) == 0 {
		urls = ExtractURLs(req.UserMessage, s.cfg.MaxURLs)
	}
	if len(urls) == 0 {
		return &ResolveResult{Handled: false}, nil
	}

	// Classify intent
	needed, goal := RequiresURLContext(req.UserMessage, urls, s.cfg.ForceOnURL)
	if !needed && !req.Force {
		return &ResolveResult{Handled: false}, nil
	}

	// Notify frontend that URLs were detected
	s.emitStatus(req.StreamStatus, "detected", map[string]any{
		"url_count": len(urls),
	})

	// Resolve each URL
	var resolvedSources []ResolvedSource
	var warnings []string
	var sources []SourceRef

	for _, rawURL := range urls {
		kind := ClassifyURL(rawURL)

		s.emitStatus(req.StreamStatus, "fetching", map[string]any{
			"url":  rawURL,
			"kind": string(kind),
		})

		resolved, err := s.resolveOne(ctx, rawURL, kind, goal, req.ConversationID, req.StreamStatus)
		if err != nil {
			// Hard errors (blocked host, private repo) propagate up
			if IsRequiredContextError(err) {
				return nil, err
			}
			log.Printf("WARN urlcontext: resolve %s: %v", rawURL, err)
			warnings = append(warnings, fmt.Sprintf("Could not resolve %s: %v", rawURL, err))
			continue
		}

		resolvedSources = append(resolvedSources, *resolved)
		sources = append(sources, resolved.SourceRef)
		warnings = append(warnings, resolved.Warnings...)
	}

	if len(resolvedSources) == 0 {
		// Intent was triggered but every fetch failed. Return a hard error so the
		// caller generates a canned message without invoking the LLM at all. Using a
		// prompt directive here is insufficient — small models ignore grounding
		// instructions and hallucinate content from training data.
		s.emitStatus(req.StreamStatus, "complete", map[string]any{})
		return nil, ErrNoUsableContent
	}

	// Build prompt context
	promptContext := BuildPromptContext(resolvedSources)

	// Switch to compact context + RAG ingest when content exceeds the threshold.
	usedRAG := len(promptContext) > s.cfg.RAGThresholdChars
	if usedRAG {
		promptContext = BuildCompactPromptContext(resolvedSources)
	}

	s.emitStatus(req.StreamStatus, "indexed", map[string]any{
		"source_count": len(resolvedSources),
		"used_rag":     usedRAG,
	})

	s.emitStatus(req.StreamStatus, "complete", map[string]any{})

	return &ResolveResult{
		Handled:               true,
		RequiresLLM:           true,
		UsedDirectContext:     !usedRAG,
		UsedRAG:               usedRAG,
		PromptContext:         promptContext,
		Sources:               sources,
		ResolvedSources:       resolvedSources,
		Warnings:              warnings,
		Metadata:              map[string]any{"goal": string(goal)},
		ShouldBypassWebSearch: true,
	}, nil
}

func (s *Service) resolveOne(
	ctx context.Context,
	rawURL string,
	kind URLKind,
	goal AnalysisGoal,
	conversationID string,
	streamStatus func(string, any),
) (*ResolvedSource, error) {
	switch kind {
	case URLKindGitHubRepo:
		return s.resolveGitHubRepo(ctx, rawURL, goal, streamStatus)
	case URLKindGitHubFile, URLKindGitHubRaw:
		return s.resolveGitHubFile(ctx, rawURL, kind)
	case URLKindGitHubDirectory:
		// Treat directory URLs as repo with a path filter
		return s.resolveGitHubRepo(ctx, rawURL, goal, streamStatus)
	case URLKindPDF:
		return s.resolveWebPageWithBrowserFallback(ctx, rawURL, streamStatus)
	default:
		return s.resolveWebPageWithBrowserFallback(ctx, rawURL, streamStatus)
	}
}

func (s *Service) resolveWebPageWithBrowserFallback(
	ctx context.Context,
	rawURL string,
	streamStatus func(string, any),
) (*ResolvedSource, error) {
	src, err := s.resolveWebPage(ctx, rawURL)
	if !errors.Is(err, ErrInsufficientContent) || s.browserMgr == nil {
		return src, err
	}

	if streamStatus != nil {
		streamStatus("url_context_browser_fallback", map[string]any{"url": rawURL})
	}
	text, title, browserErr := s.browserMgr.Navigate(ctx, rawURL)
	if browserErr != nil {
		return src, err
	}
	if len(text) < minUsableChars {
		return src, err
	}

	if title == "" {
		title = rawURL
	}
	resolved := &ResolvedSource{
		SourceRef: SourceRef{
			ID:               "urlsrc_" + uuid.New().String(),
			URL:              rawURL,
			FinalURL:         rawURL,
			Title:            title,
			Kind:             URLKindWebPage,
			FetchedAt:        time.Now().UTC(),
			LoadedViaBrowser: true,
		},
		ContentText:     text,
		ContentMarkdown: text,
	}
	s.cache.Set(URLKey(rawURL, GoalUnknown), resolved)
	return resolved, nil
}

func (s *Service) resolveGitHubRepo(
	ctx context.Context,
	rawURL string,
	goal AnalysisGoal,
	streamStatus func(string, any),
) (*ResolvedSource, error) {
	parsed := ParseGitHubURL(rawURL)
	if parsed == nil {
		return nil, fmt.Errorf("could not parse GitHub URL: %s", rawURL)
	}

	cacheKey := RepoKey(parsed.Owner, parsed.Repo, parsed.Ref, goal)
	if cached := s.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	repoCtx, err := s.inspector.InspectRepo(ctx, parsed, goal, streamStatus)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("%s/%s", parsed.Owner, parsed.Repo)
	ref := repoCtx.DefaultBranch
	if parsed.Ref != "" {
		ref = parsed.Ref
	}

	// Compute a content hash from commit SHA
	contentHash := ""
	if repoCtx.CommitSHA != "" {
		h := sha256.Sum256([]byte(repoCtx.CommitSHA))
		contentHash = fmt.Sprintf("sha256:%x", h[:8])
	}

	src := &ResolvedSource{
		SourceRef: SourceRef{
			ID:          "urlsrc_" + uuid.New().String(),
			URL:         rawURL,
			FinalURL:    fmt.Sprintf("https://github.com/%s/%s/tree/%s", parsed.Owner, parsed.Repo, ref),
			Title:       title,
			Kind:        URLKindGitHubRepo,
			FetchedAt:   time.Now().UTC(),
			ContentHash: contentHash,
		},
		Repo:     repoCtx,
		Warnings: repoCtx.Warnings,
	}

	s.cache.Set(cacheKey, src)
	return src, nil
}

func (s *Service) resolveGitHubFile(ctx context.Context, rawURL string, kind URLKind) (*ResolvedSource, error) {
	parsed := ParseGitHubURL(rawURL)
	if parsed == nil {
		return nil, fmt.Errorf("could not parse GitHub file URL: %s", rawURL)
	}

	cacheKey := URLKey(rawURL, GoalUnknown)
	if cached := s.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Use raw.githubusercontent.com for direct file access when possible
	var fetchURL string
	if kind == URLKindGitHubRaw {
		fetchURL = rawURL
	} else if parsed.Path != "" && parsed.Ref != "" {
		fetchURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			parsed.Owner, parsed.Repo, parsed.Ref, parsed.Path)
	} else {
		// Fallback to Contents API
		fetchURL = rawURL
	}

	result, err := s.fetcher.Fetch(ctx, fetchURL)
	if err != nil {
		return nil, err
	}

	content := string(result.Body)
	if IsBinaryContent(result.Body) {
		return nil, fmt.Errorf("binary file, cannot extract text")
	}

	title := parsed.Path
	if title == "" {
		title = fmt.Sprintf("%s/%s", parsed.Owner, parsed.Repo)
	}

	var warnings []string
	if result.Truncated {
		warnings = append(warnings, "File content truncated at configured size limit.")
	}

	src := &ResolvedSource{
		SourceRef: SourceRef{
			ID:        "urlsrc_" + uuid.New().String(),
			URL:       rawURL,
			FinalURL:  result.FinalURL,
			Title:     title,
			Kind:      kind,
			Path:      parsed.Path,
			FetchedAt: time.Now().UTC(),
		},
		ContentText:     content,
		ContentMarkdown: content,
		Warnings:        warnings,
	}

	s.cache.Set(cacheKey, src)
	return src, nil
}

func (s *Service) resolveWebPage(ctx context.Context, rawURL string) (*ResolvedSource, error) {
	cacheKey := URLKey(rawURL, GoalUnknown)
	if cached := s.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	result, err := s.fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	// Check for PDF by content type
	ct := strings.ToLower(result.ContentType)
	if strings.Contains(ct, "application/pdf") {
		src := &ResolvedSource{
			SourceRef: SourceRef{
				ID:        "urlsrc_" + uuid.New().String(),
				URL:       rawURL,
				FinalURL:  result.FinalURL,
				Title:     rawURL,
				Kind:      URLKindPDF,
				FetchedAt: time.Now().UTC(),
			},
			ContentText:     fmt.Sprintf("PDF file detected at %s. Text extraction from PDFs is not yet supported. To analyse this document, copy and paste the text content directly into the chat.", rawURL),
			ContentMarkdown: fmt.Sprintf("**PDF file detected** at `%s`.\n\nText extraction from PDFs is not yet supported. To analyse this document, copy and paste the text content directly into the chat.", rawURL),
			Warnings:        []string{"PDF text extraction is not yet supported. Copy and paste the document text to continue."},
		}
		return src, nil
	}

	doc := ExtractReadable(result.FinalURL, result.Body, result.ContentType)

	var warnings []string
	if result.Truncated {
		warnings = append(warnings, "Web page content truncated at configured size limit.")
	}

	// Reject pages that returned no usable text — almost always JS-rendered.
	// Returning an empty source would let the LLM hallucinate from memory.
	if len(doc.Text) < minUsableChars {
		return nil, fmt.Errorf("%w: only %d chars extracted from %s (likely JavaScript-rendered or bot-protected)",
			ErrInsufficientContent, len(doc.Text), rawURL)
	}

	// Reject pages where all extracted text is navigation/menu items — a symptom of
	// JS-rendered pages that serve only the shell HTML. JSON-LD/OG extraction in
	// ExtractReadable would have added substantive content to doc.Text if any existed,
	// so this catches homepages that genuinely have no machine-readable article data.
	if IsNavigationOnly(doc.Text) {
		return nil, fmt.Errorf("%w: only navigation/menu text could be extracted from %s (likely JavaScript-rendered homepage)",
			ErrInsufficientContent, rawURL)
	}

	title := doc.Title
	if title == "" {
		title = rawURL
	}

	src := &ResolvedSource{
		SourceRef: SourceRef{
			ID:        "urlsrc_" + uuid.New().String(),
			URL:       rawURL,
			FinalURL:  result.FinalURL,
			Title:     title,
			Kind:      URLKindWebPage,
			FetchedAt: time.Now().UTC(),
		},
		ContentText:     doc.Text,
		ContentMarkdown: doc.Markdown,
		Warnings:        warnings,
	}

	s.cache.Set(cacheKey, src)
	return src, nil
}

// emitStatus sends an SSE event if a StreamStatus function is provided.
func (s *Service) emitStatus(fn func(string, any), status string, extra map[string]any) {
	if fn == nil {
		return
	}
	payload := map[string]any{"status": status}
	for k, v := range extra {
		payload[k] = v
	}
	fn("url_context", payload)
}

// IsEnabled returns whether URL context is enabled via config.
func (s *Service) IsEnabled() bool {
	return s.cfg != nil && s.cfg.Enabled
}
