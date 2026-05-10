package rag

import (
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// providerLister is the subset of *repository.ProviderRepo that the resolver
// needs. Defined as an interface for testability.
type providerLister interface {
	List() ([]models.ProviderProfile, error)
}

// embedCapableTypes lists provider types known to expose embedding endpoints.
var embedCapableTypes = map[string]string{
	"openai":     "text-embedding-3-small",
	"openrouter": "openai/text-embedding-3-small",
	"mistral":    "mistral-embed",
	"together":   "togethercomputer/m2-bert-80M-8k-base",
	"ollama":     "nomic-embed-text",
	"gemini":     "text-embedding-004",
}

// modelCompatibleWithProvider returns true when the pinned embedding model
// looks usable for the given provider type.
func modelCompatibleWithProvider(providerType, model string) bool {
	pt := strings.ToLower(strings.TrimSpace(providerType))
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return true
	}

	switch pt {
	case "openai":
		return strings.HasPrefix(m, "text-embedding-")
	case "gemini":
		return strings.Contains(m, "text-embedding-004")
	case "mistral":
		return strings.HasPrefix(m, "mistral-embed")
	case "together":
		return strings.HasPrefix(m, "togethercomputer/") || strings.HasPrefix(m, "whereisai/")
	case "openrouter":
		// OpenRouter supports provider-prefixed model IDs from multiple vendors.
		return true
	case "ollama":
		// Ollama model names vary by local install; llm.Service also normalizes
		// common OpenAI defaults to nomic-embed-text.
		return true
	default:
		return false
	}
}

// ProviderHasEmbeddings returns true if the given provider type supports embeddings.
func ProviderHasEmbeddings(providerType string) bool {
	_, ok := embedCapableTypes[strings.ToLower(strings.TrimSpace(providerType))]
	return ok
}

// ResolveEmbeddingProvider chooses a provider profile name + model to use for
// embeddings, given the conversation's active provider profile name and the
// app's RAG settings.
//
// Resolution order:
//  1. settings.RAGEmbeddingModel pinned to the active provider, if it supports embeddings
//  2. The active provider with its canonical embed model, if it supports embeddings
//  3. If a pinned model exists, the first enabled provider compatible with that model
//  4. The first available embed-capable provider (canonical model)
//  5. Error
func ResolveEmbeddingProvider(
	activeProviderName string,
	settings models.AppSettings,
	repo providerLister,
) (provider, model string, err error) {
	all, err := repo.List()
	if err != nil {
		return "", "", fmt.Errorf("list providers for embedding resolution: %w", err)
	}

	byName := make(map[string]models.ProviderProfile, len(all))
	for _, p := range all {
		byName[p.Name] = p
	}
	pinned := strings.TrimSpace(settings.RAGEmbeddingModel)

	// 1 & 2 — active provider supports embeddings
	if active, ok := byName[activeProviderName]; ok && active.Enabled {
		if canonical, supported := embedCapableTypes[strings.ToLower(active.Type)]; supported {
			if modelCompatibleWithProvider(active.Type, pinned) {
				selected := pinned
				if selected == "" {
					selected = canonical
				}
				return active.Name, selected, nil
			}
			return active.Name, canonical, nil
		}
	}

	// 3 — if pinned model is set, prefer any enabled provider that can use it
	if pinned != "" {
		for _, p := range all {
			if !p.Enabled {
				continue
			}
			if _, ok := embedCapableTypes[strings.ToLower(p.Type)]; !ok {
				continue
			}
			if modelCompatibleWithProvider(p.Type, pinned) {
				return p.Name, pinned, nil
			}
		}
	}

	// 4 — fall back to any enabled embed-capable provider
	for _, p := range all {
		if !p.Enabled {
			continue
		}
		if canonical, ok := embedCapableTypes[strings.ToLower(p.Type)]; ok {
			return p.Name, canonical, nil
		}
	}

	return "", "", fmt.Errorf("no embedding-capable provider configured; add an OpenAI, Mistral, Together, Ollama, or Gemini provider")
}
