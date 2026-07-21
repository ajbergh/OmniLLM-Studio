package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const (
	maxChatToolDefinitions     = 16
	maxToolResultMetadataChars = 20000
)

func chatToolTerms(prompt string) []string {
	stop := map[string]struct{}{
		"about": {}, "after": {}, "again": {}, "also": {}, "and": {}, "are": {},
		"can": {}, "could": {}, "for": {}, "from": {}, "have": {}, "into": {},
		"please": {}, "that": {}, "the": {}, "then": {}, "this": {}, "using": {},
		"want": {}, "what": {}, "when": {}, "where": {}, "which": {}, "with": {},
		"would": {}, "you": {}, "your": {},
	}
	fields := strings.FieldsFunc(strings.ToLower(prompt), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})
	seen := map[string]struct{}{}
	terms := make([]string, 0, min(24, len(fields)))
	for _, field := range fields {
		if len(field) < 3 {
			continue
		}
		if _, blocked := stop[field]; blocked {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
		if len(terms) == 24 {
			break
		}
	}
	return terms
}

func containsAny(value string, terms ...string) bool {
	value = strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func preferredChatToolNames(prompt string) []string {
	var names []string
	add := func(values ...string) { names = append(names, values...) }
	if containsAny(prompt, "calculate", "math", "total", "percent", "sum", "multiply", "divide") {
		add("calculator")
	}
	if containsAny(prompt, "convert", "conversion", "units", "miles", "kilometers", "cups", "temperature") {
		add("unit_convert")
	}
	if containsAny(prompt, "currency", "exchange rate", "usd", "eur", "gbp", "cad", "aud") {
		add("currency_convert")
	}
	if containsAny(prompt, "weather", "forecast", "temperature", "rain", "snow") {
		add("weather_lookup")
	}
	if containsAny(prompt, "today", "tomorrow", "date", "time", "timezone") {
		add("date_time")
	}
	if containsAny(prompt, "latest", "current", "news", "search the web", "look up", "find online", "research") {
		add("web_search")
	}
	if containsAny(prompt, "file", "document", "library", "uploaded", "attachment") {
		add("file_search", "file_fetch", "file_summarize", "file_compare")
	}
	if containsAny(prompt, "website", "web page", "browser", "click", "screenshot", "navigate", "pdf") {
		add("browser_navigate", "browser_interact", "browser_screenshot", "browser_pdf", "browser_session")
	}
	if containsAny(prompt, "remember", "memory") {
		add("memory_search", "memory_save", "memory_delete")
	}
	if containsAny(prompt, "remind", "schedule", "task") {
		add("task_list", "task_create", "task_update")
	}
	if containsAny(prompt, "image", "picture", "illustration") {
		add("image_generate")
	}
	if containsAny(prompt, "music", "song", "soundtrack") {
		add("music_generate")
	}
	if containsAny(prompt, "video", "movie", "clip") {
		add("video_generate")
	}
	if containsAny(prompt, "artifact", "spreadsheet", "presentation", "document export") {
		add("artifact_generate")
	}
	if containsAny(prompt, "python", "analyze data", "dataset", "csv") {
		add("python_analysis")
	}
	if containsAny(prompt, "job", "generation status", "cancel generation") {
		add("job_status", "job_cancel")
	}
	if containsAny(prompt, "connected app", "connect app", "mcp") {
		add("app_catalog", "app_connections", "app_connect_mcp", "app_disconnect")
	}
	return names
}

func selectChatTools(registry *tools.Registry, executor *tools.Executor, prompt string) []llm.Tool {
	if registry == nil {
		return nil
	}
	defs := make([]tools.ToolDefinition, 0, maxChatToolDefinitions)
	seen := map[string]struct{}{}
	addDefinition := func(def tools.ToolDefinition) {
		if len(defs) >= maxChatToolDefinitions || !def.Enabled {
			return
		}
		if _, exists := seen[def.Name]; exists {
			return
		}
		if executor != nil && executor.Policy(def.Name) == "deny" {
			return
		}
		seen[def.Name] = struct{}{}
		defs = append(defs, def)
	}
	addName := func(name string) {
		if tool, ok := registry.Get(name); ok {
			addDefinition(tool.Definition().Normalized())
		}
	}

	// Intent-pinned definitions are added first so a broad MCP/plugin match cannot
	// evict the tool explicitly implied by the user's request.
	for _, name := range preferredChatToolNames(prompt) {
		addName(name)
	}
	for _, def := range registry.Select(chatToolTerms(prompt), maxChatToolDefinitions) {
		addDefinition(def)
	}
	if len(defs) == 0 {
		for _, name := range []string{"calculator", "date_time", "unit_convert"} {
			addName(name)
		}
	}

	out := make([]llm.Tool, 0, len(defs))
	for _, def := range defs {
		out = append(out, llm.Tool{
			Type: "function",
			Function: struct {
				Name        string          `json:"name"`
				Description string          `json:"description,omitempty"`
				Parameters  json.RawMessage `json:"parameters,omitempty"`
			}{Name: def.Name, Description: def.Description, Parameters: def.Parameters},
		})
	}
	return out
}

func orderedToolCalls(calls map[int]*llm.ToolCall) []llm.ToolCall {
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := make([]llm.ToolCall, 0, len(indexes))
	for _, index := range indexes {
		if call := calls[index]; call != nil {
			out = append(out, *call)
		}
	}
	return out
}

func classifyToolError(content string) (code, message string, retryable bool) {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "timed out"):
		return "TOOL_TIMEOUT", "The tool did not finish before its timeout.", true
	case strings.Contains(lower, "invalid arguments"):
		return "TOOL_INVALID_ARGUMENTS", "The model supplied invalid arguments for the tool.", false
	case strings.Contains(lower, "denied by policy"):
		return "TOOL_DENIED", "This tool is disabled by the current policy.", false
	case strings.Contains(lower, "was rejected by the user"):
		return "TOOL_REJECTED", "The tool call was rejected.", false
	case strings.Contains(lower, "requires user approval"):
		return "TOOL_APPROVAL_REQUIRED", "The tool is waiting for approval.", false
	case strings.Contains(lower, "unknown tool"):
		return "TOOL_NOT_FOUND", "The requested tool is not available.", false
	case strings.Contains(lower, "is disabled"):
		return "TOOL_DISABLED", "The requested tool is disabled.", false
	case strings.Contains(lower, "returned no result"):
		return "TOOL_EMPTY_RESULT", "The tool completed without returning a result.", true
	default:
		return "TOOL_EXECUTION_FAILED", "The tool failed before it could return a usable result.", true
	}
}

func safeToolResultForMetadata(toolName string, result *tools.ToolResult) tools.ToolResult {
	if result == nil {
		result = &tools.ToolResult{Content: fmt.Sprintf("tool %q returned no result", toolName), IsError: true}
	}
	copyResult := *result
	copyResult.Metadata = map[string]interface{}{}
	for key, value := range result.Metadata {
		copyResult.Metadata[key] = value
	}
	if copyResult.IsError {
		code, message, retryable := classifyToolError(copyResult.Content)
		copyResult.Content = message
		copyResult.Metadata["error_code"] = code
		copyResult.Metadata["retryable"] = retryable
		copyResult.Metadata["tool_name"] = toolName
	}
	if len(copyResult.Content) > maxToolResultMetadataChars {
		copyResult.Content = copyResult.Content[:maxToolResultMetadataChars] + "\n\n[tool result truncated for display]"
		copyResult.Metadata["display_truncated"] = true
	}
	return copyResult
}

func sendToolEventSSE(w http.ResponseWriter, flusher http.Flusher, event tools.ToolEvent) {
	sendSSE(w, flusher, string(event.Type), map[string]interface{}{
		"type":         event.Type,
		"tool_call_id": event.ToolCallID,
		"tool_name":    event.ToolName,
		"data":         event.Data,
	})
}

func sendStreamFailure(w http.ResponseWriter, flusher http.Flusher, code, message string, retryable bool) {
	sendSSE(w, flusher, "error", map[string]interface{}{
		"code":      code,
		"error":     message,
		"retryable": retryable,
	})
}

func requiresComposableToolLoop(prompt string) bool {
	currentInformation := containsAny(prompt, "latest", "current", "today", "news", "search", "look up", "find online", "research")
	followUpAction := containsAny(prompt,
		"calculate", "convert", "save", "remember", "schedule", "remind", "create", "generate",
		"export", "download", "click", "screenshot", "interact", "open the page", "send to",
	)
	return currentInformation && followUpAction
}
