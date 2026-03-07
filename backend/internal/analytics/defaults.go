package analytics

import (
	"log"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

// DefaultPricingRules returns a set of pre-configured pricing rules for
// well-known models.  Costs are in USD per 1M tokens as of early 2025.
func DefaultPricingRules() []models.PricingRule {
	rules := []struct {
		providerType string
		pattern      string
		inputCost    float64
		outputCost   float64
	}{
		// OpenAI
		{"openai", "gpt-4o", 2.50, 10.00},
		{"openai", "gpt-4o-mini", 0.15, 0.60},
		{"openai", "gpt-4-turbo*", 10.00, 30.00},
		{"openai", "gpt-4", 30.00, 60.00},
		{"openai", "gpt-3.5-turbo*", 0.50, 1.50},
		{"openai", "o1*", 15.00, 60.00},
		{"openai", "o3-mini*", 1.10, 4.40},

		// Anthropic
		{"anthropic", "claude-3-5-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-3-5-haiku*", 0.80, 4.00},
		{"anthropic", "claude-3-opus*", 15.00, 75.00},
		{"anthropic", "claude-3-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-3-haiku*", 0.25, 1.25},
		{"anthropic", "claude-4-sonnet*", 3.00, 15.00},
		{"anthropic", "claude-4-opus*", 15.00, 75.00},

		// Google / Gemini
		{"gemini", "gemini-2.0-flash*", 0.10, 0.40},
		{"gemini", "gemini-1.5-flash*", 0.075, 0.30},
		{"gemini", "gemini-1.5-pro*", 1.25, 5.00},
		{"gemini", "gemini-2.5-pro*", 1.25, 10.00},

		// Groq (hosted inference)
		{"groq", "llama-3*", 0.05, 0.08},
		{"groq", "mixtral*", 0.24, 0.24},

		// Together AI
		{"together", "llama-3*", 0.20, 0.20},
		{"together", "mixtral*", 0.60, 0.60},

		// Mistral
		{"mistral", "mistral-large*", 2.00, 6.00},
		{"mistral", "mistral-small*", 0.20, 0.60},
		{"mistral", "mistral-medium*", 2.70, 8.10},
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

// SeedDefaults inserts default pricing rules only if the pricing_rules table
// is empty.  Safe to call on every startup.
func SeedDefaults(pricingRepo *repository.PricingRepo) {
	existing, err := pricingRepo.List()
	if err != nil {
		log.Printf("[analytics] warning: could not check existing pricing rules: %v", err)
		return
	}
	if len(existing) > 0 {
		return // user already has rules
	}

	defaults := DefaultPricingRules()
	log.Printf("[analytics] seeding %d default pricing rules", len(defaults))
	for _, rule := range defaults {
		if err := pricingRepo.Upsert(rule); err != nil {
			log.Printf("[analytics] warning: failed to seed pricing rule %s/%s: %v",
				rule.ProviderType, rule.ModelPattern, err)
		}
	}
}
