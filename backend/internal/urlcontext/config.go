package urlcontext

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the urlcontext service.
type Config struct {
	Enabled              bool
	ForceOnURL           bool
	MaxURLs              int
	FetchTimeout         time.Duration
	MaxBytesPerSource    int64
	DirectContextMaxChars int
	RAGThresholdChars    int
	CacheTTL             time.Duration
	AllowPrivateNetworks bool
	AllowedSchemes       []string
	UserAgent            string

	// GitHub
	GitHubEnabled     bool
	GitHubToken       string // never log or expose
	GitHubUseAPI      bool
	GitHubMaxFiles    int
	GitHubMaxBytesPerFile int64
	GitHubMaxTreeEntries  int
	GitHubIncludeGlobs   []string
	GitHubExcludeGlobs   []string

	// RAG
	RAGIngestEnabled    bool
	RAGScope            string
	RAGTopK             int
	RAGMaxChunksPerSrc  int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:               true,
		ForceOnURL:            true,
		MaxURLs:               5,
		FetchTimeout:          15 * time.Second,
		MaxBytesPerSource:     750_000,
		DirectContextMaxChars: 60_000,
		RAGThresholdChars:     60_000,
		CacheTTL:              15 * time.Minute,
		AllowPrivateNetworks:  false,
		AllowedSchemes:        []string{"https", "http"},
		UserAgent:             "OmniLLM-Studio URLContextResolver/1.0",

		GitHubEnabled:        true,
		GitHubUseAPI:         true,
		GitHubMaxFiles:       80,
		GitHubMaxBytesPerFile: 120_000,
		GitHubMaxTreeEntries: 100_000,
		GitHubIncludeGlobs: []string{
			"README.md", "README*", "docs/**", "*.md",
			"backend/**/*.go", "frontend/src/**/*.ts", "frontend/src/**/*.tsx",
			"package.json", "go.mod", "go.sum",
			"backend/go.mod", "backend/go.sum",
		},
		GitHubExcludeGlobs: []string{
			".git/**", "node_modules/**", "dist/**", "build/**",
			"coverage/**", "vendor/**",
			"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg",
			"*.ico", "*.pdf", "*.zip", "*.tar", "*.gz",
			"*.exe", "*.dll", "*.so", "*.dylib", "*.lock",
		},

		RAGIngestEnabled:   true,
		RAGScope:           "ephemeral_conversation",
		RAGTopK:            12,
		RAGMaxChunksPerSrc: 120,
	}
}

// ConfigFromEnv loads configuration from environment variables, falling back to defaults.
func ConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if v := os.Getenv("URL_CONTEXT_ENABLED"); v != "" {
		cfg.Enabled = parseBool(v, cfg.Enabled)
	}
	if v := os.Getenv("URL_CONTEXT_FORCE_ON_URL"); v != "" {
		cfg.ForceOnURL = parseBool(v, cfg.ForceOnURL)
	}
	if v := os.Getenv("URL_CONTEXT_MAX_URLS"); v != "" {
		cfg.MaxURLs = parseInt(v, cfg.MaxURLs)
	}
	if v := os.Getenv("URL_CONTEXT_FETCH_TIMEOUT_SECONDS"); v != "" {
		if n := parseInt(v, 0); n > 0 {
			cfg.FetchTimeout = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("URL_CONTEXT_MAX_BYTES_PER_SOURCE"); v != "" {
		cfg.MaxBytesPerSource = parseInt64(v, cfg.MaxBytesPerSource)
	}
	if v := os.Getenv("URL_CONTEXT_DIRECT_CONTEXT_MAX_CHARS"); v != "" {
		cfg.DirectContextMaxChars = parseInt(v, cfg.DirectContextMaxChars)
	}
	if v := os.Getenv("URL_CONTEXT_RAG_THRESHOLD_CHARS"); v != "" {
		cfg.RAGThresholdChars = parseInt(v, cfg.RAGThresholdChars)
	}
	if v := os.Getenv("URL_CONTEXT_CACHE_TTL_SECONDS"); v != "" {
		if n := parseInt(v, 0); n > 0 {
			cfg.CacheTTL = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("URL_CONTEXT_ALLOW_PRIVATE_NETWORKS"); v != "" {
		cfg.AllowPrivateNetworks = parseBool(v, cfg.AllowPrivateNetworks)
	}
	if v := os.Getenv("URL_CONTEXT_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}

	if v := os.Getenv("GITHUB_CONTEXT_ENABLED"); v != "" {
		cfg.GitHubEnabled = parseBool(v, cfg.GitHubEnabled)
	}
	if v := os.Getenv("GITHUB_CONTEXT_TOKEN"); v != "" {
		cfg.GitHubToken = strings.TrimSpace(v)
	}
	if v := os.Getenv("GITHUB_CONTEXT_USE_API"); v != "" {
		cfg.GitHubUseAPI = parseBool(v, cfg.GitHubUseAPI)
	}
	if v := os.Getenv("GITHUB_CONTEXT_MAX_FILES"); v != "" {
		cfg.GitHubMaxFiles = parseInt(v, cfg.GitHubMaxFiles)
	}
	if v := os.Getenv("GITHUB_CONTEXT_MAX_BYTES_PER_FILE"); v != "" {
		cfg.GitHubMaxBytesPerFile = parseInt64(v, cfg.GitHubMaxBytesPerFile)
	}
	if v := os.Getenv("GITHUB_CONTEXT_MAX_TREE_ENTRIES"); v != "" {
		cfg.GitHubMaxTreeEntries = parseInt(v, cfg.GitHubMaxTreeEntries)
	}
	if v := os.Getenv("GITHUB_CONTEXT_INCLUDE_GLOBS"); v != "" {
		cfg.GitHubIncludeGlobs = splitComma(v)
	}
	if v := os.Getenv("GITHUB_CONTEXT_EXCLUDE_GLOBS"); v != "" {
		cfg.GitHubExcludeGlobs = splitComma(v)
	}

	if v := os.Getenv("URL_RAG_INGEST_ENABLED"); v != "" {
		cfg.RAGIngestEnabled = parseBool(v, cfg.RAGIngestEnabled)
	}
	if v := os.Getenv("URL_RAG_SCOPE"); v != "" {
		cfg.RAGScope = v
	}
	if v := os.Getenv("URL_RAG_TOP_K"); v != "" {
		cfg.RAGTopK = parseInt(v, cfg.RAGTopK)
	}
	if v := os.Getenv("URL_RAG_MAX_CHUNKS_PER_SOURCE"); v != "" {
		cfg.RAGMaxChunksPerSrc = parseInt(v, cfg.RAGMaxChunksPerSrc)
	}

	return cfg
}

func parseBool(s string, def bool) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}

func parseInt(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func parseInt64(s string, def int64) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return def
	}
	return n
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
