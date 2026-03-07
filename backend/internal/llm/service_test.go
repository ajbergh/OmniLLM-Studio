package llm

import "testing"

func TestOllamaAPIRoot(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "default with v1", baseURL: "http://localhost:11434/v1", want: "http://localhost:11434"},
		{name: "default no suffix", baseURL: "http://localhost:11434", want: "http://localhost:11434"},
		{name: "trailing slash", baseURL: "http://localhost:11434/v1/", want: "http://localhost:11434"},
		{name: "nested path", baseURL: "http://localhost:11434/custom/v1", want: "http://localhost:11434/custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ollamaAPIRoot(tt.baseURL)
			if got != tt.want {
				t.Fatalf("ollamaAPIRoot(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestOllamaModelNameMatches(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		requested string
		want      bool
	}{
		{name: "exact", installed: "nomic-embed-text", requested: "nomic-embed-text", want: true},
		{name: "installed latest", installed: "nomic-embed-text:latest", requested: "nomic-embed-text", want: true},
		{name: "requested latest", installed: "nomic-embed-text", requested: "nomic-embed-text:latest", want: true},
		{name: "different model", installed: "all-minilm", requested: "nomic-embed-text", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ollamaModelNameMatches(tt.installed, tt.requested)
			if got != tt.want {
				t.Fatalf("ollamaModelNameMatches(%q, %q) = %v, want %v", tt.installed, tt.requested, got, tt.want)
			}
		})
	}
}
