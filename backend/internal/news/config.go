package news

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the news lookup feature.
type Config struct {
	Enabled         bool
	BaseURL         string
	Timeout         time.Duration
	CacheTTL        time.Duration
	DefaultPageSize int
	MaxPageSize     int
	UserAgent       string
}

// DefaultConfig returns the default configuration for news lookup.
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		BaseURL:         "https://actually-relevant-api.onrender.com/api",
		Timeout:         8 * time.Second,
		CacheTTL:        5 * time.Minute,
		DefaultPageSize: 8,
		MaxPageSize:     15,
		UserAgent:       "OmniLLM-Studio/NewsLookup",
	}
}

// LoadConfigFromEnv loads configuration from environment variables.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("NEWS_LOOKUP_ENABLED"); v == "false" || v == "0" {
		cfg.Enabled = false
	}

	if v := os.Getenv("NEWS_LOOKUP_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}

	if v := os.Getenv("NEWS_LOOKUP_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Timeout = time.Duration(n) * time.Second
		}
	}

	if v := os.Getenv("NEWS_LOOKUP_CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.CacheTTL = time.Duration(n) * time.Second
		}
	}

	if v := os.Getenv("NEWS_LOOKUP_DEFAULT_PAGE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DefaultPageSize = n
		}
	}

	if v := os.Getenv("NEWS_LOOKUP_MAX_PAGE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxPageSize = n
		}
	}

	return cfg
}
