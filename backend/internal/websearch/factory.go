package websearch

import (
	"log"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// NewProviderFromSettings reads "web_search_provider" and related keys from
// the settings repo and returns the appropriate Provider implementation.
//
// Supported values for "web_search_provider":
//   - "brave"  → BraveProvider (requires "brave_api_key")
//   - "ddg"    → DuckDuckGoProvider (no API key, zero config)
//   - "none"   → nil (web search disabled)
//   - ""       → auto: uses Brave if key is set, else DuckDuckGo
//
// Falls back to DuckDuckGoProvider on any error.
func NewProviderFromSettings(settingsRepo *repository.SettingsRepo) Provider {
	providerVal, _ := settingsRepo.Get("web_search_provider")
	providerVal = strings.TrimSpace(strings.Trim(providerVal, `"`))

	switch strings.ToLower(providerVal) {
	case "none", "disabled", "off":
		log.Println("[websearch] provider disabled by settings")
		return nil

	case "brave":
		key := getSettingString(settingsRepo, "brave_api_key")
		if key == "" {
			log.Println("[websearch] brave selected but no API key; falling back to DuckDuckGo")
			return NewDuckDuckGoProvider()
		}
		log.Println("[websearch] using Brave Search provider")
		return NewBraveProvider(key)

	case "ddg", "duckduckgo":
		log.Println("[websearch] using DuckDuckGo provider")
		return NewDuckDuckGoProvider()

	default:
		// Auto-detect: prefer Brave if key present, else DDG
		key := getSettingString(settingsRepo, "brave_api_key")
		if key != "" {
			log.Println("[websearch] auto-detected Brave API key; using Brave Search")
			return NewBraveProvider(key)
		}
		log.Println("[websearch] no search API key configured; using DuckDuckGo (zero-config)")
		return NewDuckDuckGoProvider()
	}
}

// getSettingString reads a setting, strips surrounding quotes, and decrypts
// sensitive keys (e.g. brave_api_key) transparently.
func getSettingString(repo *repository.SettingsRepo, key string) string {
	val, err := repo.Get(key)
	if err != nil || val == "" {
		return ""
	}
	val = strings.TrimSpace(strings.Trim(val, `"`))

	// Decrypt sensitive keys stored encrypted at rest
	if key == "brave_api_key" {
		val = crypto.DecryptOrPlaintext(val)
	}

	return val
}

// NewJinaReaderFromSettings returns a JinaReader if enabled in settings,
// or nil if disabled/ not configured.
//
// Settings keys:
//   - "jina_reader_enabled"  → "true" to enable (default: enabled)
//   - "jina_reader_max_len"  → max chars per page (default: 3000)
func NewJinaReaderFromSettings(settingsRepo *repository.SettingsRepo) *JinaReader {
	enabled := getSettingString(settingsRepo, "jina_reader_enabled")
	if strings.EqualFold(enabled, "false") || strings.EqualFold(enabled, "off") || enabled == "0" {
		log.Println("[websearch] Jina Reader disabled by settings")
		return nil
	}

	maxLen := 3000 // default
	if v := getSettingString(settingsRepo, "jina_reader_max_len"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxLen = n
		}
	}

	log.Printf("[websearch] Jina Reader enabled (maxLen=%d chars per page)\n", maxLen)
	return NewJinaReader(maxLen)
}
