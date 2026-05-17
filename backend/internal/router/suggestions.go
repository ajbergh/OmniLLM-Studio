package router

import (
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

func SuggestionsForProvider(provider models.ProviderProfile) SuggestionsResponse {
	pt := strings.ToLower(strings.TrimSpace(provider.Type))
	resp := SuggestionsResponse{
		Provider:     provider.Name,
		ProviderType: pt,
		Notes:        []string{"Model availability depends on the configured provider account.", "Keep local detector fallback enabled for sports routing rollout."},
	}
	if !llm.IsChatCapableProvider(pt) {
		resp.Notes = append(resp.Notes, "This provider type is not chat-capable and cannot be used as a router.")
		return resp
	}
	switch pt {
	case "openai":
		resp.Suggestions = []ModelSuggestion{
			{"gpt-4o-mini", "OpenAI GPT-4o Mini", "Fast, low-cost routing model.", "json_schema", "low", "high"},
			{"gpt-4.1-mini", "OpenAI GPT-4.1 Mini", "Good classifier and extractor for JSON schema output.", "json_schema", "low", "high"},
			{"gpt-4.1-nano", "OpenAI GPT-4.1 Nano", "Lowest-cost option when available.", "json_schema", "very_low", "medium"},
		}
	case "gemini":
		resp.Suggestions = []ModelSuggestion{
			{"gemini-2.5-flash-lite", "Gemini 2.5 Flash-Lite", "Fast Flash-Lite model for classification.", "json_schema", "low", "high"},
			{"gemini-2.5-flash", "Gemini 2.5 Flash", "Stronger Flash model for ambiguous extraction.", "json_schema", "low", "high"},
			{"gemini-2.0-flash-lite", "Gemini 2.0 Flash-Lite", "Cheap routing fallback if available.", "json_schema", "low", "medium"},
			{"gemini-2.0-flash", "Gemini 2.0 Flash", "Reliable JSON-following model if configured.", "json_schema", "low", "medium"},
		}
	case "openrouter":
		resp.Suggestions = []ModelSuggestion{
			{"openai/gpt-4o-mini", "OpenAI GPT-4o Mini via OpenRouter", "OpenAI mini model routed through OpenRouter.", "json_schema", "low", "high"},
			{"google/gemini-2.5-flash-lite", "Gemini 2.5 Flash-Lite via OpenRouter", "Fast Google model through OpenRouter.", "json_schema", "low", "medium"},
			{"google/gemini-2.5-flash", "Gemini 2.5 Flash via OpenRouter", "Stronger Google Flash model.", "json_schema", "low", "medium"},
			{"qwen/qwen3-14b", "Qwen 3 14B", "Low-cost structured extraction candidate.", "json_object", "low", "medium"},
			{"meta-llama/llama-3.1-8b-instruct", "Llama 3.1 8B Instruct", "Cheap fallback when JSON mode is supported.", "json_object", "low", "medium"},
		}
		resp.Notes = append(resp.Notes, "OpenRouter pricing and structured-output support vary by model.")
	case "ollama":
		resp.Suggestions = []ModelSuggestion{
			{"qwen3:4b", "Qwen 3 4B", "Small local classifier; validate strictly.", "prompted_json", "free_local", "medium"},
			{"qwen3:8b", "Qwen 3 8B", "Stronger local classifier.", "prompted_json", "free_local", "medium"},
			{"llama3.1:8b", "Llama 3.1 8B", "Common local fallback.", "prompted_json", "free_local", "medium"},
			{"mistral:7b", "Mistral 7B", "Small local JSON-following model.", "prompted_json", "free_local", "medium"},
		}
	default:
		resp.Suggestions = []ModelSuggestion{}
		resp.Notes = append(resp.Notes, "No curated router model list exists for this provider type; enter a chat-capable model manually.")
	}
	return resp
}
