package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/api"
	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// Set via ldflags: -ldflags "-X main.version=1.0.0 -X main.commit=abc1234"
var (
	version = "0.2.0-dev"
	commit  = "dev"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close(database)

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	router := api.NewRouter(database, cfg, version, commit)

	// Periodic cleanup of expired sessions (every 15 minutes).
	sessionRepo := repository.NewSessionRepo(database)
	stopCleanup := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if n, err := sessionRepo.DeleteExpired(); err != nil {
					log.Printf("[cleanup] expired-session delete failed: %v", err)
				} else if n > 0 {
					log.Printf("[cleanup] deleted %d expired session(s)", n)
				}
			case <-stopCleanup:
				return
			}
		}
	}()

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("OmniLLM-Studio server starting on %s:%d", cfg.BindAddress, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down server...")
	close(stopCleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
