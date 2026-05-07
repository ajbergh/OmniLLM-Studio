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
//  3. The first available embed-capable provider (canonical model)
//  4. Error
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

	// 1 & 2 — active provider supports embeddings
	if active, ok := byName[activeProviderName]; ok && active.Enabled {
		if canonical, supported := embedCapableTypes[strings.ToLower(active.Type)]; supported {
			selected := strings.TrimSpace(settings.RAGEmbeddingModel)
			if selected == "" {
				selected = canonical
			}
			return active.Name, selected, nil
		}
	}

	// 3 — fall back to any enabled embed-capable provider
	for _, p := range all {
		if !p.Enabled {
			continue
		}
		if canonical, ok := embedCapableTypes[strings.ToLower(p.Type)]; ok {
			selected := strings.TrimSpace(settings.RAGEmbeddingModel)
			if selected == "" {
				selected = canonical
			}
			return p.Name, selected, nil
		}
	}

	return "", "", fmt.Errorf("no embedding-capable provider configured; add an OpenAI, Mistral, Together, Ollama, or Gemini provider")
}
