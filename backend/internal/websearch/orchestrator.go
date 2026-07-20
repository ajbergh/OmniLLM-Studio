package websearch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/turncontext"
)

// Orchestrator chooses the cheapest capable retrieval path for the active model,
// falling back to the configured local web-search provider when provider-native
// grounding is unavailable or fails.
type Orchestrator struct {
	mu         sync.RWMutex
	provider   Provider
	llmSvc     *llm.Service
	jinaReader *JinaReader
}

func NewOrchestrator(provider Provider, llmSvc *llm.Service, jinaReader *JinaReader) *Orchestrator {
	return &Orchestrator{provider: provider, llmSvc: llmSvc, jinaReader: jinaReader}
}

func (o *Orchestrator) Reconfigure(provider Provider, jinaReader *JinaReader) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.provider = provider
	o.jinaReader = jinaReader
}

func (o *Orchestrator) snapshot() (Provider, *JinaReader) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.provider, o.jinaReader
}

// Process handles a non-streaming current-information turn. Native grounding is
// preferred because it avoids a separate search request and summarization call.
func (o *Orchestrator) Process(
	ctx context.Context,
	userText string,
	history []llm.ChatMessage,
	provider, model string,
) (*OrchestratorResult, error) {
	tc := turncontext.FromContext(ctx)
	plan := BuildSearchPlan(userText, tc.Now, tc.Timezone)
	if !plan.NeedsWeb {
		return nil, nil
	}
	toolCall := toolCallForPlan(plan, tc)

	providerType, _ := o.llmSvc.ResolveProviderType(provider)
	if SupportsNativeSearch(providerType, model) {
		nativeReq := buildNativeSearchRequest(provider, model, history, userText, plan, tc)
		resp, err := o.llmSvc.ChatComplete(ctx, nativeReq)
		if err == nil {
			if ok, _ := ValidateAnswer(plan, resp.Content); ok {
				return &OrchestratorResult{
					Content:    resp.Content,
					WebSearch:  true,
					ToolCall:   toolCall,
					Provider:   resp.Provider,
					Model:      resp.Model,
					TokenInput: resp.TokenInput,
					TokenOut:   resp.TokenOutput,
				}, nil
			}
		}
	}

	searchResp, err := o.searchWithPlan(ctx, plan, tc)
	if err != nil || searchResp == nil || len(searchResp.Results) == 0 {
		return &OrchestratorResult{WebSearch: true, SearchFailed: true, ToolCall: toolCall}, nil
	}

	req := buildLocalSummarizerRequest(provider, model, userText, plan, tc, searchResp.Results)
	resp, err := o.llmSvc.ChatComplete(ctx, req)
	if err != nil {
		return &OrchestratorResult{
			Content:   directFailureMessage(plan),
			Sources:   searchResp.Results,
			WebSearch: true,
			ToolCall:  toolCall,
			Provider:  provider,
			Model:     model,
		}, nil
	}
	if ok, _ := ValidateAnswer(plan, resp.Content); !ok {
		return &OrchestratorResult{
			Content:   directFailureMessage(plan),
			Sources:   searchResp.Results,
			WebSearch: true,
			ToolCall:  toolCall,
			Provider:  resp.Provider,
			Model:     resp.Model,
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

// ProcessStream returns a streaming-ready request. Native providers receive an
// internal marker that the llm transport converts to OpenAI web_search_options,
// Gemini Google Search grounding, or OpenRouter's server-side web search tool.
func (o *Orchestrator) ProcessStream(
	ctx context.Context,
	userText, provider, model string,
) (*SearchResponse, *llm.ChatRequest, *ToolCall, error) {
	tc := turncontext.FromContext(ctx)
	plan := BuildSearchPlan(userText, tc.Now, tc.Timezone)
	if !plan.NeedsWeb {
		return nil, nil, nil, nil
	}
	toolCall := toolCallForPlan(plan, tc)

	providerType, _ := o.llmSvc.ResolveProviderType(provider)
	if SupportsNativeSearch(providerType, model) {
		req := buildNativeSearchRequest(provider, model, nil, userText, plan, tc)
		return &SearchResponse{
			Query:     toolCall.Arguments.Query,
			TimeRange: plan.TimeRange,
			FetchedAt: time.Now().UTC(),
		}, &req, toolCall, nil
	}

	searchResp, err := o.searchWithPlan(ctx, plan, tc)
	if err != nil || searchResp == nil || len(searchResp.Results) == 0 {
		if err == nil {
			err = fmt.Errorf("web search returned no results")
		}
		return nil, nil, toolCall, err
	}
	req := buildLocalSummarizerRequest(provider, model, userText, plan, tc, searchResp.Results)
	return searchResp, &req, toolCall, nil
}

// ProcessStreamFallback bypasses provider-native grounding. It is used
// only when a native streaming request is rejected before emitting content.
func (o *Orchestrator) ProcessStreamFallback(
	ctx context.Context,
	userText, provider, model string,
) (*SearchResponse, *llm.ChatRequest, *ToolCall, error) {
	tc := turncontext.FromContext(ctx)
	plan := BuildSearchPlan(userText, tc.Now, tc.Timezone)
	if !plan.NeedsWeb {
		return nil, nil, nil, nil
	}
	toolCall := toolCallForPlan(plan, tc)
	searchResp, err := o.searchWithPlan(ctx, plan, tc)
	if err != nil || searchResp == nil || len(searchResp.Results) == 0 {
		if err == nil {
			err = fmt.Errorf("web search returned no results")
		}
		return nil, nil, toolCall, err
	}
	req := buildLocalSummarizerRequest(provider, model, userText, plan, tc, searchResp.Results)
	return searchResp, &req, toolCall, nil
}

func (o *Orchestrator) DirectSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	provider, _ := o.snapshot()
	if provider == nil {
		return nil, fmt.Errorf("web search is disabled")
	}
	return provider.Search(ctx, req)
}

func (o *Orchestrator) searchWithPlan(ctx context.Context, plan SearchPlan, tc turncontext.TurnContext) (*SearchResponse, error) {
	provider, jinaReader := o.snapshot()
	if provider == nil {
		return nil, fmt.Errorf("web search provider is disabled")
	}
	if len(plan.Queries) == 0 {
		return nil, fmt.Errorf("search plan contained no queries")
	}

	iterations := plan.MaxIterations
	if iterations <= 0 || iterations > len(plan.Queries) {
		iterations = len(plan.Queries)
	}
	seen := map[string]bool{}
	combined := make([]SearchResult, 0, plan.MaxResults*iterations)

	for i := 0; i < iterations; i++ {
		response, err := provider.Search(ctx, SearchRequest{
			Query:      plan.Queries[i],
			TimeRange:  plan.TimeRange,
			Region:     firstNonEmptySearch(tc.Country, "US"),
			Locale:     firstNonEmptySearch(tc.Locale, "en-US"),
			MaxResults: plan.MaxResults,
		})
		if err != nil {
			if len(combined) == 0 && i == iterations-1 {
				return nil, err
			}
			continue
		}

		newResults := make([]SearchResult, 0, len(response.Results))
		for _, result := range response.Results {
			key := strings.ToLower(strings.TrimSpace(result.URL))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			result.Index = len(combined) + len(newResults) + 1
			newResults = append(newResults, result)
		}
		if jinaReader != nil && len(newResults) > 0 {
			enrichCount := 2
			if plan.AnswerShape == AnswerShapeResearch {
				enrichCount = 5
			}
			newResults = jinaReader.EnrichResults(ctx, newResults, enrichCount)
		}
		combined = append(combined, newResults...)
		if ResultsLikelyAnswerable(plan, combined) {
			break
		}
	}

	if len(combined) == 0 {
		return nil, fmt.Errorf("web search returned no results")
	}
	return &SearchResponse{
		Query:     plan.Queries[0],
		TimeRange: plan.TimeRange,
		Results:   combined,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func buildNativeSearchRequest(
	provider, model string,
	history []llm.ChatMessage,
	userText string,
	plan SearchPlan,
	tc turncontext.TurnContext,
) llm.ChatRequest {
	messages := append([]llm.ChatMessage{{Role: "system", Content: nativeSearchDirective(plan, tc)}}, trimHistory(history, userText)...)
	searchPlugin := llm.NativeSearchPlugin(NativeSearchConfigForPlan(plan, tc))
	req := llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: messages,
	}
	if searchPlugin.ID != "" {
		req.Plugins = []llm.Plugin{searchPlugin}
	}
	if plan.AnswerShape == AnswerShapeDirect {
		maxTokens := 180
		temperature := 0.1
		req.MaxTokens = &maxTokens
		req.Temperature = &temperature
	}
	return req
}

func buildLocalSummarizerRequest(
	provider, model, userText string,
	plan SearchPlan,
	tc turncontext.TurnContext,
	results []SearchResult,
) llm.ChatRequest {
	req := llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: localSummarizerPrompt(plan, tc, results, userText)},
			{Role: "user", Content: userText},
		},
	}
	if plan.AnswerShape == AnswerShapeDirect {
		maxTokens := 180
		temperature := 0.1
		req.MaxTokens = &maxTokens
		req.Temperature = &temperature
	}
	return req
}

func nativeSearchDirective(plan SearchPlan, tc turncontext.TurnContext) string {
	location := localContextLine(tc)
	switch plan.AnswerShape {
	case AnswerShapeDirect:
		return fmt.Sprintf(`Use native web search or grounding to answer this current lookup.
%s
Answer the exact question in the first sentence. For one event, give the matchup and local start time in no more than two sentences. Convert times to the supplied IANA timezone. Do not explain how to find the answer, provide background, list generic websites, add headings, or add a Key Takeaways section. Cite the source used.`, location)
	case AnswerShapeBrief:
		return fmt.Sprintf("Use native web search for current information. %s Answer first, stay concise, and cite factual claims.", location)
	case AnswerShapeResearch:
		return fmt.Sprintf("Use native web search iteratively for a thorough current answer. %s Synthesize reliable sources and cite claims.", location)
	default:
		return fmt.Sprintf("Use native web search when needed. %s Start with a direct answer, then add only useful support with citations.", location)
	}
}

func localSummarizerPrompt(plan SearchPlan, tc turncontext.TurnContext, results []SearchResult, userText string) string {
	resultsBlock := formatResultsForPrompt(results)
	location := localContextLine(tc)
	switch plan.AnswerShape {
	case AnswerShapeDirect:
		return fmt.Sprintf(`You are answering a simple, time-sensitive fact lookup using retrieved evidence.
%s

STRICT RULES:
- Answer the exact question in the first sentence.
- For one event, use: "<team> vs. <team> starts at <time> <timezone>."
- Convert the time to the supplied IANA timezone when evidence includes an absolute time or offset.
- Use no heading and no more than two short sentences.
- Do not explain how to check, list websites, provide background, or add Key Takeaways.
- Use only the evidence below. Cite the supporting result as [1], [2], etc.
- If evidence lacks a verifiable event and start time, say only: "I couldn't verify today's start time from the available sources."

EVIDENCE:
%s

USER QUESTION: %s`, location, resultsBlock, userText)
	case AnswerShapeBrief:
		return fmt.Sprintf(`Answer this current-information question briefly using only the evidence below. %s Start with the answer. Use bullets only when multiple items are necessary and cite claims inline.

EVIDENCE:
%s

USER QUESTION: %s`, location, resultsBlock, userText)
	case AnswerShapeResearch:
		return fmt.Sprintf(`Prepare a thorough, source-grounded answer. %s Synthesize the evidence, distinguish uncertainty, and cite every material factual claim. Use clear Markdown structure appropriate to the question.

EVIDENCE:
%s

USER QUESTION: %s`, location, resultsBlock, userText)
	default:
		return fmt.Sprintf(`Answer the question using only the evidence below. %s Start with a direct answer, then provide concise supporting detail. Cite factual claims inline as [1], [2], etc. Do not add a Key Takeaways section unless it materially improves a complex answer.

EVIDENCE:
%s

USER QUESTION: %s`, location, resultsBlock, userText)
	}
}

func trimHistory(messages []llm.ChatMessage, userText string) []llm.ChatMessage {
	filtered := make([]llm.ChatMessage, 0, len(messages)+1)
	for _, message := range messages {
		if message.Role == "system" || message.Role == "tool" || strings.TrimSpace(message.Content) == "" {
			continue
		}
		filtered = append(filtered, message)
	}
	if len(filtered) > 8 {
		filtered = filtered[len(filtered)-8:]
	}
	if len(filtered) == 0 || filtered[len(filtered)-1].Role != "user" || strings.TrimSpace(filtered[len(filtered)-1].Content) != strings.TrimSpace(userText) {
		filtered = append(filtered, llm.ChatMessage{Role: "user", Content: userText})
	}
	return filtered
}

func toolCallForPlan(plan SearchPlan, tc turncontext.TurnContext) *ToolCall {
	query := ""
	if len(plan.Queries) > 0 {
		query = plan.Queries[0]
	}
	return &ToolCall{Name: "web_search", Arguments: SearchRequest{
		Query:      query,
		TimeRange:  plan.TimeRange,
		Region:     firstNonEmptySearch(tc.Country, "US"),
		Locale:     firstNonEmptySearch(tc.Locale, "en-US"),
		MaxResults: plan.MaxResults,
	}}
}

func localContextLine(tc turncontext.TurnContext) string {
	now := tc.Now
	if now.IsZero() {
		now = time.Now()
	}
	zone := firstNonEmptySearch(tc.Timezone, now.Location().String())
	return fmt.Sprintf("Current local date/time: %s. User timezone: %s. Locale: %s.", now.Format(time.RFC1123), zone, firstNonEmptySearch(tc.Locale, "en-US"))
}

func directFailureMessage(plan SearchPlan) string {
	if plan.AnswerShape == AnswerShapeDirect {
		return "I couldn't verify today's start time from the available sources."
	}
	return "I found web results but could not produce a sufficiently grounded answer."
}

func formatResultsForPrompt(results []SearchResult) string {
	var b strings.Builder
	for _, result := range results {
		fmt.Fprintf(&b, "--- Result [%d] ---\n", result.Index)
		fmt.Fprintf(&b, "Title: %s\n", result.Title)
		fmt.Fprintf(&b, "Source: %s\n", result.Source)
		if result.PublishedAt != "" {
			fmt.Fprintf(&b, "Published: %s\n", result.PublishedAt)
		}
		fmt.Fprintf(&b, "URL: %s\n", result.URL)
		fmt.Fprintf(&b, "Content: %s\n\n", result.Snippet)
	}
	return b.String()
}

func firstNonEmptySearch(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
