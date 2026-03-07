package websearch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

// Orchestrator ties the web-search gate, provider, and LLM together.
type Orchestrator struct {
	mu         sync.RWMutex
	provider   Provider
	llmSvc     *llm.Service
	jinaReader *JinaReader // optional – enriches search results with full page content
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(provider Provider, llmSvc *llm.Service, jinaReader *JinaReader) *Orchestrator {
	return &Orchestrator{
		provider:   provider,
		llmSvc:     llmSvc,
		jinaReader: jinaReader,
	}
}

// Reconfigure swaps the search provider and Jina reader at runtime (e.g. after settings change).
func (o *Orchestrator) Reconfigure(provider Provider, jinaReader *JinaReader) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.provider = provider
	o.jinaReader = jinaReader
}

// snapshot returns a consistent read of the mutable provider and jinaReader fields.
func (o *Orchestrator) snapshot() (Provider, *JinaReader) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.provider, o.jinaReader
}

// ---------------------------------------------------------------------------
// LLM prompts
// ---------------------------------------------------------------------------

const classifierPrompt = `You are a strict classifier. Decide whether the user's message requires a live web search to answer correctly.
Output ONLY valid JSON with no extra text:
{"needs_web": true/false, "query": "<optimised search query>", "timeRange": "24h|7d|30d", "reason": "<one sentence>"}

Rules:
- needs_web = true if the message asks about current/recent events, live data, today's news, scores, weather, stock prices, or asks user to verify/fact-check something.
- needs_web = false for general knowledge, coding help, math, creative writing, or anything an LLM can answer from training data.`

const summarizerPrompt = `You are a knowledgeable research assistant that provides comprehensive, well-structured answers using web search results.

TODAY'S DATE: %s

STRICT RULES:
1. ONLY use information from the search results below. NEVER invent, assume, or hallucinate facts not present in the results.
2. Cite EVERY factual claim with source indices: [1], [2][3]. Place citations immediately after the claim, not at the end of a paragraph.
3. If the search results are insufficient or contradictory, explicitly state what is uncertain and why.
4. If results conflict, present all perspectives and identify which sources disagree.

RESPONSE FORMAT GUIDELINES:
- Start with a brief 1-2 sentence direct answer to the question.
- Then provide detailed supporting information organized with clear headers (## or ###) where appropriate.
- Use bullet points for lists of items, comparisons, or multiple facts.
- Use bold (**text**) to highlight key terms, names, numbers, or dates.
- For time-sensitive topics, include dates and timestamps from the sources.
- For technical/how-to questions, provide step-by-step instructions.
- End with a brief "Key Takeaways" section if the response covers multiple aspects.

QUALITY GUIDELINES:
- Be thorough — aim for comprehensive coverage, not just surface-level summaries.
- Synthesize information across sources rather than summarizing each source separately.
- Quantify whenever possible (numbers, dates, percentages, prices).
- Distinguish between confirmed facts and speculation/opinions in the sources.
- If sources provide context or background, include it to make the answer more useful.

SEARCH RESULTS:
%s

USER QUESTION: %s

Respond now with a comprehensive, well-cited answer.`

// ---------------------------------------------------------------------------
// Process – main orchestration entry point
// ---------------------------------------------------------------------------

// Process takes the user's latest message and the full conversation history,
// decides whether to web-search, and returns the final response.
func (o *Orchestrator) Process(
	ctx context.Context,
	userText string,
	history []llm.ChatMessage,
	provider, model string,
) (*OrchestratorResult, error) {
	now := time.Now()
	tz := "UTC"

	searchProvider, jinaReader := o.snapshot()

	// ---- Step 1: deterministic gate ----
	triggered, toolCall := ShouldWebSearch(userText, now, tz)

	if !triggered || searchProvider == nil {
		// No web search needed (or provider disabled) – pass through to LLM normally.
		return nil, nil // nil result signals "no web search, use normal flow"
	}

	// ---- Step 2: execute web search ----
	searchResp, err := searchProvider.Search(ctx, toolCall.Arguments)
	if err != nil || len(searchResp.Results) == 0 {
		// Signal caller to fall through to normal LLM path (with warning metadata).
		return &OrchestratorResult{
			WebSearch:    true,
			SearchFailed: true,
			ToolCall:     toolCall,
			Sources:      nil,
		}, nil
	}

	// ---- Step 3: optional Jina Reader enrichment ----
	enrichedResults := searchResp.Results
	if jinaReader != nil {
		enrichedResults = jinaReader.EnrichResults(ctx, searchResp.Results, 5)
	}

	// ---- Step 4: build summarizer prompt with results ----
	resultsBlock := formatResultsForPrompt(enrichedResults)
	dateStr := time.Now().Format("Monday, January 2, 2006")
	sysPrompt := fmt.Sprintf(summarizerPrompt, dateStr, resultsBlock, userText)

	// Build messages: system + user question
	llmMessages := []llm.ChatMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userText},
	}

	llmReq := llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: llmMessages,
	}

	resp, err := o.llmSvc.ChatComplete(ctx, llmReq)
	if err != nil {
		return &OrchestratorResult{
			Content:   "I found some web results but failed to summarise them. Here are the raw sources.",
			Sources:   searchResp.Results,
			WebSearch: true,
			ToolCall:  toolCall,
		}, nil
	}

	return &OrchestratorResult{
		Content:    resp.Content,
		Sources:    searchResp.Results,
		WebSearch:  true,
		ToolCall:   toolCall,
		Provider:   resp.Provider,
		Model:      resp.Model,
		TokenInput: resp.TokenInput,
		TokenOut:   resp.TokenOutput,
	}, nil
}

// ProcessStream is like Process but returns results and a streaming-ready
// summarizer request so the caller can stream the LLM response.
// Returns (searchResponse, llmRequest-for-streaming, toolCall, error).
// If no web search is needed, all return values are nil.
func (o *Orchestrator) ProcessStream(
	ctx context.Context,
	userText string,
	provider, model string,
) (*SearchResponse, *llm.ChatRequest, *ToolCall, error) {
	now := time.Now()
	tz := "UTC"

	searchProvider, jinaReader := o.snapshot()

	triggered, toolCall := ShouldWebSearch(userText, now, tz)
	if !triggered || searchProvider == nil {
		return nil, nil, nil, nil
	}

	searchResp, err := searchProvider.Search(ctx, toolCall.Arguments)
	if err != nil || len(searchResp.Results) == 0 {
		return nil, nil, toolCall, fmt.Errorf("web search returned no results")
	}

	// Optional Jina Reader enrichment
	enrichedResults := searchResp.Results
	if jinaReader != nil {
		enrichedResults = jinaReader.EnrichResults(ctx, searchResp.Results, 5)
	}

	resultsBlock := formatResultsForPrompt(enrichedResults)
	dateStr := time.Now().Format("Monday, January 2, 2006")
	sysPrompt := fmt.Sprintf(summarizerPrompt, dateStr, resultsBlock, userText)

	llmReq := &llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userText},
		},
	}

	return searchResp, llmReq, toolCall, nil
}

// DirectSearch exposes the search provider for the /api/websearch endpoint.
func (o *Orchestrator) DirectSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	searchProvider, _ := o.snapshot()
	if searchProvider == nil {
		return nil, fmt.Errorf("web search is disabled")
	}
	return searchProvider.Search(ctx, req)
}

// formatResultsForPrompt turns search results into a structured text block
// optimised for LLM comprehension.
func formatResultsForPrompt(results []SearchResult) string {
	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "─── Result [%d] ───\n", r.Index)
		fmt.Fprintf(&b, "Title: %s\n", r.Title)
		fmt.Fprintf(&b, "Source: %s\n", r.Source)
		if r.PublishedAt != "" {
			fmt.Fprintf(&b, "Published: %s\n", r.PublishedAt)
		}
		fmt.Fprintf(&b, "URL: %s\n", r.URL)

		// Check if snippet contains full content from Jina Reader
		if strings.Contains(r.Snippet, "\n\nFull content:\n") {
			parts := strings.SplitN(r.Snippet, "\n\nFull content:\n", 2)
			fmt.Fprintf(&b, "Summary: %s\n", strings.TrimSpace(parts[0]))
			fmt.Fprintf(&b, "Full Content:\n%s\n", strings.TrimSpace(parts[1]))
		} else {
			fmt.Fprintf(&b, "Content: %s\n", r.Snippet)
		}
		b.WriteString("\n")
	}
	return b.String()
}
