package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration.
type Config struct {
	Port            int
	BindAddress     string // Network interface to bind; default "127.0.0.1" (localhost only).
	DatabasePath    string
	AttachmentsDir  string
	CORSOrigins     []string
	AllowPublicReg  bool   // When false (default), only the first user can register.
	ChromemDir      string // Directory for chromem-go persistent vector files.
	ChromemCompress bool   // Enable gzip compression for chromem data files.

	BrowserEnabled     bool
	BrowserExecPath    string
	BrowserCacheDir    string
	BrowserMaxSessions int
	BrowserSessionTTL  time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := 8080
	if p := os.Getenv("OMNILLM_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	dbPath := "omnillm-studio.db"
	if p := os.Getenv("OMNILLM_DB_PATH"); p != "" {
		dbPath = p
	}

	attachDir := "attachments"
	if p := os.Getenv("OMNILLM_ATTACHMENTS_DIR"); p != "" {
		attachDir = p
	}

	bindAddr := "127.0.0.1"
	if addr := os.Getenv("OMNILLM_BIND_ADDRESS"); addr != "" {
		bindAddr = addr
	}

	allowPublicReg := os.Getenv("OMNILLM_ALLOW_PUBLIC_REGISTRATION") == "true"

	chromemDir := os.Getenv("OMNILLM_CHROMEM_DIR")
	if chromemDir == "" {
		dbDir := filepath.Dir(dbPath)
		if dbDir == "" || dbDir == "." {
			chromemDir = "chromem"
		} else {
			chromemDir = filepath.Join(dbDir, "chromem")
		}
	}

	chromemCompress := os.Getenv("OMNILLM_CHROMEM_COMPRESS") == "true"

	browserEnabled := strings.EqualFold(os.Getenv("OMNILLM_BROWSER_ENABLED"), "true")
	browserExecPath := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_EXEC_PATH"))

	browserCacheDir := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_CACHE_DIR"))
	if browserCacheDir == "" {
		dbDir := filepath.Dir(dbPath)
		if dbDir == "" || dbDir == "." {
			browserCacheDir = "chromium-cache"
		} else {
			browserCacheDir = filepath.Join(dbDir, "chromium-cache")
		}
	}

	browserMaxSessions := 3
	if v := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_MAX_SESSIONS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			browserMaxSessions = n
		}
	}

	browserSessionTTL := 30 * time.Minute
	if v := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_SESSION_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			browserSessionTTL = d
		}
	}

	origins := []string{"http://localhost:5173", "http://localhost:3000"}
	if corsEnv := os.Getenv("OMNILLM_CORS_ORIGINS"); corsEnv != "" {
		origins = strings.Split(corsEnv, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
	}

	return &Config{
		Port:               port,
		BindAddress:        bindAddr,
		DatabasePath:       dbPath,
		AttachmentsDir:     attachDir,
		CORSOrigins:        origins,
		AllowPublicReg:     allowPublicReg,
		ChromemDir:         chromemDir,
		ChromemCompress:    chromemCompress,
		BrowserEnabled:     browserEnabled,
		BrowserExecPath:    browserExecPath,
		BrowserCacheDir:    browserCacheDir,
		BrowserMaxSessions: browserMaxSessions,
		BrowserSessionTTL:  browserSessionTTL,
	}
}
