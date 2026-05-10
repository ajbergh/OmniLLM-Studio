package rag

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

type fakeProviderRepo struct {
	providers []models.ProviderProfile
}

func (f *fakeProviderRepo) List() ([]models.ProviderProfile, error) {
	return f.providers, nil
}

func TestResolveEmbeddingProvider_ActiveSupportsEmbeddings(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "OpenAI", Type: "openai", Enabled: true},
	}}
	settings := models.AppSettings{RAGEmbeddingModel: "text-embedding-3-large"}

	p, m, err := ResolveEmbeddingProvider("OpenAI", settings, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "OpenAI" || m != "text-embedding-3-large" {
		t.Fatalf("unexpected provider/model: %s/%s", p, m)
	}
}

func TestResolveEmbeddingProvider_FallbackForAnthropic(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "Claude", Type: "anthropic", Enabled: true},
		{Name: "Local", Type: "ollama", Enabled: true},
	}}
	settings := models.AppSettings{}

	p, m, err := ResolveEmbeddingProvider("Claude", settings, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "Local" || m != "nomic-embed-text" {
		t.Fatalf("expected fallback to ollama, got %s/%s", p, m)
	}
}

func TestResolveEmbeddingProvider_NoCapableProvider(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "Claude", Type: "anthropic", Enabled: true},
		{Name: "Groq", Type: "groq", Enabled: true},
	}}
	if _, _, err := ResolveEmbeddingProvider("Claude", models.AppSettings{}, repo); err == nil {
		t.Fatal("expected error when no embed-capable provider configured")
	}
}

func TestResolveEmbeddingProvider_DisabledActiveFallsBack(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "OpenAI", Type: "openai", Enabled: false},
		{Name: "Mistral", Type: "mistral", Enabled: true},
	}}
	p, m, err := ResolveEmbeddingProvider("OpenAI", models.AppSettings{}, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "Mistral" || m != "mistral-embed" {
		t.Fatalf("expected mistral fallback, got %s/%s", p, m)
	}
}

func TestResolveEmbeddingProvider_IncompatiblePinnedUsesActiveCanonical(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "Gemini", Type: "gemini", Enabled: true},
	}}
	settings := models.AppSettings{RAGEmbeddingModel: "text-embedding-3-small"}

	p, m, err := ResolveEmbeddingProvider("Gemini", settings, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "Gemini" || m != "text-embedding-004" {
		t.Fatalf("expected gemini canonical fallback, got %s/%s", p, m)
	}
}

func TestResolveEmbeddingProvider_PinnedModelFindsCompatibleProvider(t *testing.T) {
	repo := &fakeProviderRepo{providers: []models.ProviderProfile{
		{Name: "Claude", Type: "anthropic", Enabled: true},
		{Name: "OpenAI", Type: "openai", Enabled: true},
	}}
	settings := models.AppSettings{RAGEmbeddingModel: "text-embedding-3-small"}

	p, m, err := ResolveEmbeddingProvider("Claude", settings, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "OpenAI" || m != "text-embedding-3-small" {
		t.Fatalf("expected OpenAI pinned model routing, got %s/%s", p, m)
	}
}

func TestProviderHasEmbeddings(t *testing.T) {
	cases := map[string]bool{
		"openai":     true,
		"OPENAI":     true,
		"anthropic":  false,
		"groq":       false,
		"ollama":     true,
		"unknown":    false,
		"":           false,
		"  openai  ": true,
	}
	for in, want := range cases {
		if got := ProviderHasEmbeddings(in); got != want {
			t.Errorf("ProviderHasEmbeddings(%q) = %v, want %v", in, got, want)
		}
	}
}
