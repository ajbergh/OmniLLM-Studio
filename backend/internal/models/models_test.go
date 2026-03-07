package models_test

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestDefaultAppSettings(t *testing.T) {
	s := models.DefaultAppSettings()

	if s.WebSearchProvider != "auto" {
		t.Errorf("expected web_search_provider=auto, got %q", s.WebSearchProvider)
	}
	if s.BraveAPIKey != "" {
		t.Errorf("expected empty brave_api_key, got %q", s.BraveAPIKey)
	}
	if !s.JinaReaderEnabled {
		t.Error("expected jina_reader_enabled=true")
	}
	if s.JinaReaderMaxLen != 3000 {
		t.Errorf("expected jina_reader_max_len=3000, got %d", s.JinaReaderMaxLen)
	}
}

func TestAppSettingsRoundTrip(t *testing.T) {
	original := models.AppSettings{
		WebSearchProvider: "brave",
		BraveAPIKey:       "sk-test-key",
		JinaReaderEnabled: false,
		JinaReaderMaxLen:  5000,
	}

	m := original.ToMap()
	restored := models.AppSettingsFromMap(m)

	if restored.WebSearchProvider != original.WebSearchProvider {
		t.Errorf("WebSearchProvider: got %q, want %q", restored.WebSearchProvider, original.WebSearchProvider)
	}
	if restored.BraveAPIKey != original.BraveAPIKey {
		t.Errorf("BraveAPIKey: got %q, want %q", restored.BraveAPIKey, original.BraveAPIKey)
	}
	if restored.JinaReaderEnabled != original.JinaReaderEnabled {
		t.Errorf("JinaReaderEnabled: got %v, want %v", restored.JinaReaderEnabled, original.JinaReaderEnabled)
	}
	if restored.JinaReaderMaxLen != original.JinaReaderMaxLen {
		t.Errorf("JinaReaderMaxLen: got %d, want %d", restored.JinaReaderMaxLen, original.JinaReaderMaxLen)
	}
}

func TestAppSettingsFromMap_LegacyQuotedValues(t *testing.T) {
	// The old frontend sent JSON-encoded strings like "\"brave\"".
	// AppSettingsFromMap should strip surrounding quotes for backward compat.
	m := map[string]string{
		"web_search_provider": `"brave"`,
		"brave_api_key":       `"my-key"`,
		"jina_reader_enabled": `"false"`,
		"jina_reader_max_len": `"4000"`,
	}

	s := models.AppSettingsFromMap(m)

	if s.WebSearchProvider != "brave" {
		t.Errorf("expected brave, got %q", s.WebSearchProvider)
	}
	if s.BraveAPIKey != "my-key" {
		t.Errorf("expected my-key, got %q", s.BraveAPIKey)
	}
	if s.JinaReaderEnabled {
		t.Error("expected jina_reader_enabled=false")
	}
	if s.JinaReaderMaxLen != 4000 {
		t.Errorf("expected 4000, got %d", s.JinaReaderMaxLen)
	}
}

func TestAppSettingsFromMap_EmptyMap(t *testing.T) {
	s := models.AppSettingsFromMap(map[string]string{})

	defaults := models.DefaultAppSettings()
	if s.WebSearchProvider != defaults.WebSearchProvider {
		t.Errorf("expected default provider %q, got %q", defaults.WebSearchProvider, s.WebSearchProvider)
	}
	if !s.JinaReaderEnabled {
		t.Error("expected jina_reader_enabled=true (default)")
	}
}

func TestAppSettingsToMap_BoolValues(t *testing.T) {
	s := models.AppSettings{JinaReaderEnabled: true}
	m := s.ToMap()
	if m["jina_reader_enabled"] != "true" {
		t.Errorf("expected 'true', got %q", m["jina_reader_enabled"])
	}

	s.JinaReaderEnabled = false
	m = s.ToMap()
	if m["jina_reader_enabled"] != "false" {
		t.Errorf("expected 'false', got %q", m["jina_reader_enabled"])
	}
}
