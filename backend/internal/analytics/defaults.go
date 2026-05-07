package analytics

import (
	"log"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

// DefaultPricingRules returns a set of pre-configured pricing rules for
// well-known models.  Costs are in USD per 1M tokens as of mid-2025.
func DefaultPricingRules() []models.PricingRule {
	rules := []struct {
		providerType string
		pattern      string
		inputCost    float64
		outputCost   float64
	}{
		// ---- OpenAI ----
		// GPT-5 series
		{"openai", "gpt-5.5*", 6.00, 24.00},
		{"openai", "gpt-5.4-mini*", 0.20, 0.80},
		{"openai", "gpt-5.4-nano*", 0.08, 0.32},
		{"openai", "gpt-5.4*", 3.00, 12.00},
		{"openai", "gpt-5.2*", 3.00, 12.00},
		{"openai", "gpt-5-mini*", 0.20, 0.80},
		{"openai", "gpt-5-nano*", 0.08, 0.32},
		{"openai", "gpt-5*", 3.00, 12.00},
		// GPT-4.1 series
		{"openai", "gpt-4.1-mini*", 0.40, 1.60},
		{"openai", "gpt-4.1-nano*", 0.10, 0.40},
		{"openai", "gpt-4.1*", 2.00, 8.00},
		// GPT-4o series
		{"openai", "gpt-4o-mini*", 0.15, 0.60},
		{"openai", "gpt-4o*", 2.50, 10.00},
		// GPT-4 legacy
		{"openai", "gpt-4-turbo*", 10.00, 30.00},
		{"openai", "gpt-4*", 30.00, 60.00},
		// GPT-3.5
		{"openai", "gpt-3.5-turbo*", 0.50, 1.50},
		// o-series reasoning models
		{"openai", "o3-pro*", 20.00, 80.00},
		{"openai", "o4-mini*", 1.10, 4.40},
		{"openai", "o3-mini*", 1.10, 4.40},
		{"openai", "o3*", 10.00, 40.00},
		{"openai", "o1-mini*", 1.10, 4.40},
		{"openai", "o1*", 15.00, 60.00},

		// ---- Anthropic ----
		// Claude 4 series
		{"anthropic", "claude-opus-4*", 15.00, 75.00},
		{"anthropic", "claude-sonnet-4*", 3.00, 15.00},
		{"anthropic", "claude-haiku-4*", 0.80, 4.00},
		// Claude 3.7
		{"anthropic", "claude-3-7-sonnet*", 3.00, 15.00},
		// Claude 3.5
		{"anthropic", "claude-3-5-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-3-5-haiku*", 0.80, 4.00},
		// Claude 3 legacy
		{"anthropic", "claude-3-opus*", 15.00, 75.00},
		{"anthropic", "claude-3-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-3-haiku*", 0.25, 1.25},
		// Catch-all patterns
		{"anthropic", "claude-opus*", 15.00, 75.00},
		{"anthropic", "claude-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-haiku*", 0.80, 4.00},

		// ---- Google Gemini ----
		{"gemini", "gemini-2.5-flash*", 0.15, 0.60},
		{"gemini", "gemini-2.5-pro*", 1.25, 10.00},
		{"gemini", "gemini-2.0-flash*", 0.10, 0.40},
		{"gemini", "gemini-1.5-flash*", 0.075, 0.30},
		{"gemini", "gemini-1.5-pro*", 1.25, 5.00},
		{"gemini", "gemini-3.1-flash*", 0.15, 0.60},
		{"gemini", "gemini-3-ultra*", 5.00, 20.00},

		// ---- Groq (hosted inference) ----
		{"groq", "groq/compound*", 0.10, 0.20},
		{"groq", "llama-3*", 0.05, 0.08},
		{"groq", "llama3*", 0.05, 0.08},
		{"groq", "qwen*", 0.10, 0.10},
		{"groq", "mixtral*", 0.24, 0.24},
		{"groq", "gemma*", 0.05, 0.05},

		// ---- Together AI ----
		{"together", "llama-3*", 0.20, 0.20},
		{"together", "llama3*", 0.20, 0.20},
		{"together", "mixtral*", 0.60, 0.60},
		{"together", "qwen*", 0.30, 0.30},

		// ---- Mistral ----
		{"mistral", "mistral-large*", 2.00, 6.00},
		{"mistral", "mistral-medium*", 2.70, 8.10},
		{"mistral", "mistral-small*", 0.10, 0.30},
		{"mistral", "mistral-nemo*", 0.15, 0.15},
		{"mistral", "magistral*", 2.00, 6.00},
		{"mistral", "devstral*", 0.25, 0.75},
		{"mistral", "codestral*", 0.30, 0.90},
		{"mistral", "pixtral*", 2.00, 6.00},
		{"mistral", "open-mistral*", 0.20, 0.60},

		// ---- OpenRouter (pass-through cost with markup) ----
		// These are approximate; actual cost depends on the routed model.
		{"openrouter", "openai/gpt-5.5*", 6.00, 24.00},
		{"openrouter", "openai/gpt-5.4-mini*", 0.20, 0.80},
		{"openrouter", "openai/gpt-5.4*", 3.00, 12.00},
		{"openrouter", "openai/gpt-5*", 3.00, 12.00},
		{"openrouter", "openai/gpt-4.1-mini*", 0.40, 1.60},
		{"openrouter", "openai/gpt-4.1*", 2.00, 8.00},
		{"openrouter", "openai/gpt-4o-mini*", 0.15, 0.60},
		{"openrouter", "openai/gpt-4o*", 2.50, 10.00},
		{"openrouter", "anthropic/claude-opus-4*", 15.00, 75.00},
		{"openrouter", "anthropic/claude-sonnet-4*", 3.00, 15.00},
		{"openrouter", "anthropic/claude-3-5-sonnet*", 3.00, 15.00},
		{"openrouter", "anthropic/claude-3-5-haiku*", 0.80, 4.00},
		{"openrouter", "google/gemini-2.5-flash*", 0.15, 0.60},
		{"openrouter", "google/gemini-2.5-pro*", 1.25, 10.00},
		{"openrouter", "meta-llama/llama-3*", 0.20, 0.20},
		{"openrouter", "mistralai/mistral-large*", 2.00, 6.00},
		{"openrouter", "mistralai/mistral-small*", 0.10, 0.30},
		{"openrouter", "mistralai/mistral-medium*", 2.70, 8.10},
	}

	out := make([]models.PricingRule, len(rules))
	for i, r := range rules {
		out[i] = models.PricingRule{
			ID:                uuid.New().String(),
			ProviderType:      r.providerType,
			ModelPattern:      r.pattern,
			InputCostPerMTok:  r.inputCost,
			OutputCostPerMTok: r.outputCost,
			Currency:          "USD",
		}
	}
	return out
}

// SeedDefaults inserts default pricing rules that are not already present
// (matched by provider_type + model_pattern). This is additive: existing
// user-customized rules are never overwritten, and new model rules are
// added on every startup when missing.
func SeedDefaults(pricingRepo *repository.PricingRepo) {
	existing, err := pricingRepo.List()
	if err != nil {
		log.Printf("[analytics] warning: could not check existing pricing rules: %v", err)
		return
	}

	// Build a set of (provider_type, model_pattern) pairs that already exist.
	type key struct{ p, m string }
	present := make(map[key]struct{}, len(existing))
	for _, r := range existing {
		present[key{strings.ToLower(r.ProviderType), strings.ToLower(r.ModelPattern)}] = struct{}{}
	}

	defaults := DefaultPricingRules()
	added := 0
	for _, rule := range defaults {
		k := key{strings.ToLower(rule.ProviderType), strings.ToLower(rule.ModelPattern)}
		if _, ok := present[k]; ok {
			continue // already exists — don't overwrite user edits
		}
		if err := pricingRepo.Upsert(rule); err != nil {
			log.Printf("[analytics] warning: failed to seed pricing rule %s/%s: %v",
				rule.ProviderType, rule.ModelPattern, err)
		} else {
			added++
		}
	}
	if added > 0 {
		log.Printf("[analytics] seeded %d new default pricing rules", added)
	}
}
