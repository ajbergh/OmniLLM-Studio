package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/api"
	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// Embedded frontend build — populated by the build script which copies
// frontend/dist/ into this directory before compiling.
//
//go:embed all:frontend_dist
var assets embed.FS

// Set via ldflags: -ldflags "-X main.version=1.0.0 -X main.commit=abc1234"
var (
	version = "0.2.0-dev"
	commit  = "dev"
)

// App exposes methods to the frontend via Wails bindings.
type App struct {
	ctx     context.Context
	apiBase string // e.g. "http://127.0.0.1:54321/v1"
}

func NewApp(apiBase string) *App { return &App{apiBase: apiBase} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// GetAPIBase returns the base URL for the local API server so the frontend
// can route fetch()/SSE calls to the real HTTP server (SSE streaming does
// not work through the Wails AssetServer handler).
func (a *App) GetAPIBase() string { return a.apiBase }

func main() {
	setDesktopDefaults()

	// In GUI mode (windowsgui subsystem), stderr is disconnected.
	// Route all logging to a file so errors are visible.
	logPath := filepath.Join(desktopDataDir(), "omnillm-studio.log")
	if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	cfg := config.Load()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close(database)

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	router := api.NewRouter(database, cfg, version, commit)

	// Start a real HTTP server on a random local port so that SSE streaming
	// works (the Wails AssetServer handler does not support http.Flusher).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	apiPort := ln.Addr().(*net.TCPAddr).Port
	apiBase := fmt.Sprintf("http://127.0.0.1:%d/v1", apiPort)
	log.Printf("[desktop] API server listening on %s", apiBase)

	srv := &http.Server{
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE needs unlimited write time
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[desktop] API server error: %v", err)
		}
	}()

	// Periodic cleanup of expired sessions (mirrors cmd/server behaviour).
	sessionRepo := repository.NewSessionRepo(database)
	stopCleanup := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if n, err := sessionRepo.DeleteExpired(); err != nil {
					log.Printf("[cleanup] expired-session delete: %v", err)
				} else if n > 0 {
					log.Printf("[cleanup] deleted %d expired session(s)", n)
				}
			case <-stopCleanup:
				return
			}
		}
	}()

	// The embedded FS root is "frontend_dist/"; extract the subfolder so
	// Wails can find index.html at the FS root.
	frontendFS, err := fs.Sub(assets, "frontend_dist")
	if err != nil {
		log.Fatalf("frontend assets: %v", err)
	}

	app := NewApp(apiBase)

	err = wails.Run(&options.App{
		Title:  "OmniLLM Studio",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets:  frontendFS, // serves the React SPA static files
			Handler: router,     // handles /v1/* requests from the WebView (e.g. <img> tags)
		},
		Bind:      []interface{}{app},
		OnStartup: app.startup,
		OnShutdown: func(ctx context.Context) {
			close(stopCleanup)
			srv.Shutdown(ctx)
		},
	})
	if err != nil {
		log.Fatalf("wails.Run: %v", err)
	}
}

// setDesktopDefaults configures OS-appropriate default paths for the database
// and attachments directory so a desktop user doesn't need env vars.
func setDesktopDefaults() {
	// In desktop mode all origins are same-app; allow all to avoid
	// WebView origin mismatches (wails://, http://wails.localhost, etc.).
	if os.Getenv("OMNILLM_CORS_ORIGINS") == "" {
		os.Setenv("OMNILLM_CORS_ORIGINS", "*")
	}

	dataDir := desktopDataDir()

	if os.Getenv("OMNILLM_DB_PATH") == "" {
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			log.Printf("warn: cannot create data dir %s: %v", dataDir, err)
		}
		os.Setenv("OMNILLM_DB_PATH", filepath.Join(dataDir, "omnillm-studio.db"))
	}
	if os.Getenv("OMNILLM_ATTACHMENTS_DIR") == "" {
		dir := filepath.Join(dataDir, "attachments")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("warn: cannot create attachments dir %s: %v", dir, err)
		}
		os.Setenv("OMNILLM_ATTACHMENTS_DIR", dir)
	}
}

// desktopDataDir returns the platform-appropriate user data directory.
func desktopDataDir() string {
	switch goruntime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "OmniLLM-Studio")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "OmniLLM-Studio")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "OmniLLM-Studio")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "OmniLLM-Studio")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "OmniLLM-Studio")
	}
}
