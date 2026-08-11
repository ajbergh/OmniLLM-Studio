package api

import (
	"database/sql"
	"errors"
	"log"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// NewGitHubAuthHandlerFromEnvironment composes the encrypted credential store
// with the GitHub App device-flow service. Missing operator configuration is a
// supported state: the handler remains mounted and reports configured=false.
func NewGitHubAuthHandlerFromEnvironment(database *sql.DB) *GitHubAuthHandler {
	store := repository.NewGitHubAppConnectionRepo(database)
	service, err := githubauth.NewServiceFromEnvironment(store)
	if err == nil {
		return NewGitHubAuthHandler(service)
	}
	if !errors.Is(err, githubauth.ErrNotConfigured) {
		log.Printf("WARN: GitHub App authentication unavailable: %v", err)
	}
	return NewGitHubAuthHandler(nil)
}

// MountGitHubAuthRoutes mounts user-scoped GitHub connection routes. The caller
// must mount this inside the existing authenticated /v1 route group.
func MountGitHubAuthRoutes(r chi.Router, handler *GitHubAuthHandler) {
	if r == nil {
		return
	}
	if handler == nil {
		handler = NewGitHubAuthHandler(nil)
	}
	r.Get("/github/auth", handler.Status)
	r.Post("/github/auth/device/start", handler.StartDeviceAuthorization)
	r.Post("/github/auth/device/poll", handler.PollDeviceAuthorization)
	r.Delete("/github/auth", handler.Disconnect)
}
