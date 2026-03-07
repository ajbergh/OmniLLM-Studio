package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration.
type Config struct {
	Port           int
	BindAddress    string // Network interface to bind; default "127.0.0.1" (localhost only).
	DatabasePath   string
	AttachmentsDir string
	CORSOrigins    []string
	AllowPublicReg bool // When false (default), only the first user can register.
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

	origins := []string{"http://localhost:5173", "http://localhost:3000"}
	if corsEnv := os.Getenv("OMNILLM_CORS_ORIGINS"); corsEnv != "" {
		origins = strings.Split(corsEnv, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
	}

	return &Config{
		Port:           port,
		BindAddress:    bindAddr,
		DatabasePath:   dbPath,
		AttachmentsDir: attachDir,
		CORSOrigins:    origins,
		AllowPublicReg: allowPublicReg,
	}
}
