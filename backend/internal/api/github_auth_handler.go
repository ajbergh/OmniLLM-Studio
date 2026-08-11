package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/githubauth"
)

// GitHubAuthService is the user-scoped, secret-free HTTP boundary around the
// GitHub App authentication service.
type GitHubAuthService interface {
	StartDeviceAuthorization(ctx context.Context, userID string) (githubauth.DeviceAuthorization, error)
	PollDeviceAuthorization(ctx context.Context, userID string) (githubauth.PollResult, error)
	Status(userID string) (githubauth.Status, error)
	Disconnect(userID string) error
}

// GitHubAuthHandler exposes device authorization without ever returning access
// tokens, refresh tokens, or provider device codes.
type GitHubAuthHandler struct {
	service GitHubAuthService
}

func NewGitHubAuthHandler(service GitHubAuthService) *GitHubAuthHandler {
	return &GitHubAuthHandler{service: service}
}

func (h *GitHubAuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.service == nil {
		respondJSON(w, http.StatusOK, githubauth.Status{Configured: false})
		return
	}
	status, err := h.service.Status(auth.ScopeUserIDFromContext(r.Context()))
	if err != nil {
		handleGitHubAuthError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *GitHubAuthHandler) StartDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.service == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub App authentication is not configured")
		return
	}
	result, err := h.service.StartDeviceAuthorization(r.Context(), auth.ScopeUserIDFromContext(r.Context()))
	if err != nil {
		handleGitHubAuthError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *GitHubAuthHandler) PollDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.service == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub App authentication is not configured")
		return
	}
	result, err := h.service.PollDeviceAuthorization(r.Context(), auth.ScopeUserIDFromContext(r.Context()))
	if err != nil {
		handleGitHubAuthError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *GitHubAuthHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.service == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.service.Disconnect(auth.ScopeUserIDFromContext(r.Context())); err != nil {
		handleGitHubAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func setGitHubAuthNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

func handleGitHubAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, githubauth.ErrNotConfigured):
		respondError(w, http.StatusServiceUnavailable, "GitHub App authentication is not configured")
	case errors.Is(err, githubauth.ErrReauthorizationRequired):
		respondError(w, http.StatusConflict, "GitHub reauthorization is required")
	case errors.Is(err, githubauth.ErrNotConnected):
		respondError(w, http.StatusConflict, "GitHub account is not connected")
	default:
		respondError(w, http.StatusBadGateway, "GitHub authentication request failed")
	}
}
