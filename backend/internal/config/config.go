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

	DesktopAPIToken string
	MaxUploadBytes  int64

	BrowserEnabled     bool
	BrowserExecPath    string
	BrowserCacheDir    string
	BrowserMaxSessions int
	BrowserSessionTTL  time.Duration
	// BrowserNoSandbox is an emergency compatibility override. The Chromium
	// sandbox remains enabled by default and disabling it requires an explicit
	// OMNILLM_BROWSER_NO_SANDBOX=true setting.
	BrowserNoSandbox bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := 8080
	if value := strings.TrimSpace(os.Getenv("OMNILLM_PORT")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 65535 {
			port = parsed
		}
	}

	dbPath := strings.TrimSpace(os.Getenv("OMNILLM_DB_PATH"))
	if dbPath == "" {
		dbPath = "omnillm-studio.db"
	}
	attachmentsDir := strings.TrimSpace(os.Getenv("OMNILLM_ATTACHMENTS_DIR"))
	if attachmentsDir == "" {
		attachmentsDir = "attachments"
	}
	bindAddress := strings.TrimSpace(os.Getenv("OMNILLM_BIND_ADDRESS"))
	if bindAddress == "" {
		bindAddress = "127.0.0.1"
	}

	chromemDir := strings.TrimSpace(os.Getenv("OMNILLM_CHROMEM_DIR"))
	if chromemDir == "" {
		dbDir := filepath.Dir(dbPath)
		if dbDir == "" || dbDir == "." {
			chromemDir = "chromem"
		} else {
			chromemDir = filepath.Join(dbDir, "chromem")
		}
	}

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
	if value := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_MAX_SESSIONS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			browserMaxSessions = parsed
		}
	}
	browserSessionTTL := 30 * time.Minute
	if value := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_SESSION_TTL")); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			browserSessionTTL = parsed
		}
	}
	maxUploadBytes := int64(500 << 20)
	if value := strings.TrimSpace(os.Getenv("OMNILLM_MAX_UPLOAD_BYTES")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			maxUploadBytes = parsed
		}
	}

	origins := []string{"http://localhost:5173", "http://localhost:3000"}
	if configured := strings.TrimSpace(os.Getenv("OMNILLM_CORS_ORIGINS")); configured != "" {
		origins = strings.Split(configured, ",")
		cleaned := origins[:0]
		for _, origin := range origins {
			if origin = strings.TrimSpace(origin); origin != "" {
				cleaned = append(cleaned, origin)
			}
		}
		origins = cleaned
	}

	return &Config{
		Port:               port,
		BindAddress:        bindAddress,
		DatabasePath:       dbPath,
		AttachmentsDir:     attachmentsDir,
		CORSOrigins:        origins,
		AllowPublicReg:     strings.EqualFold(os.Getenv("OMNILLM_ALLOW_PUBLIC_REGISTRATION"), "true"),
		ChromemDir:         chromemDir,
		ChromemCompress:    strings.EqualFold(os.Getenv("OMNILLM_CHROMEM_COMPRESS"), "true"),
		DesktopAPIToken:    strings.TrimSpace(os.Getenv("OMNILLM_DESKTOP_API_TOKEN")),
		MaxUploadBytes:     maxUploadBytes,
		BrowserEnabled:     strings.EqualFold(os.Getenv("OMNILLM_BROWSER_ENABLED"), "true"),
		BrowserExecPath:    strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_EXEC_PATH")),
		BrowserCacheDir:    browserCacheDir,
		BrowserMaxSessions: browserMaxSessions,
		BrowserSessionTTL:  browserSessionTTL,
		BrowserNoSandbox:   strings.EqualFold(os.Getenv("OMNILLM_BROWSER_NO_SANDBOX"), "true"),
	}
}
