package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/githubrepo"
	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

const maxGitHubRepositoryBindingBodyBytes = int64(4096)

type GitHubRepositoryDiscoveryService interface {
	List(ctx context.Context, userID string, page, perPage int) (githubrepo.Page, error)
	Get(ctx context.Context, userID string, repositoryID int64) (githubrepo.Repository, error)
}

type GitHubRepositoryConnectionService interface {
	Status(userID string) (githubauth.Status, error)
}

type GitHubRepositoryBindingStore interface {
	List(ownerID string) ([]repository.GitHubRepositoryBinding, error)
	Upsert(ownerID string, binding repository.GitHubRepositoryBinding) error
	Delete(ownerID, localRepositoryID string) error
}

type LocalGitRepositoryCatalog interface {
	RepositoryIDs() []string
	HasRepository(repositoryID string) bool
}

// GitHubRepositoryHandler exposes user-scoped repository discovery and explicit
// GitHub-to-local bindings without accepting filesystem paths or remote URLs.
type GitHubRepositoryHandler struct {
	discovery  GitHubRepositoryDiscoveryService
	connection GitHubRepositoryConnectionService
	bindings   GitHubRepositoryBindingStore
	locals     LocalGitRepositoryCatalog
}

func NewGitHubRepositoryHandler(discovery GitHubRepositoryDiscoveryService, connection GitHubRepositoryConnectionService, bindings GitHubRepositoryBindingStore, locals LocalGitRepositoryCatalog) *GitHubRepositoryHandler {
	return &GitHubRepositoryHandler{discovery: discovery, connection: connection, bindings: bindings, locals: locals}
}

type githubRepositoryBindingRequest struct {
	GitHubRepositoryID int64 `json:"github_repository_id"`
}

type githubRepositoryBindingView struct {
	LocalRepositoryID  string `json:"local_repository_id"`
	GitHubUserID       int64  `json:"github_user_id"`
	GitHubRepositoryID int64  `json:"github_repository_id"`
	GitHubFullName     string `json:"github_full_name"`
	DefaultBranch      string `json:"default_branch"`
	Private            bool   `json:"private"`
	Fork               bool   `json:"fork"`
	Archived           bool   `json:"archived"`
	Disabled           bool   `json:"disabled"`
	AccountMatches     bool   `json:"account_matches"`
	LocalConfigured    bool   `json:"local_configured"`
}

type githubRepositoryBindingsResponse struct {
	LocalRepositories []string                      `json:"local_repositories"`
	Bindings          []githubRepositoryBindingView `json:"bindings"`
}

func (h *GitHubRepositoryHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.discovery == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub App authentication is not configured")
		return
	}
	page, err := positiveQueryInt(r, "page", 1)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	perPage, err := positiveQueryInt(r, "per_page", 30)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.discovery.List(r.Context(), auth.ScopeUserIDFromContext(r.Context()), page, perPage)
	if err != nil {
		handleGitHubRepositoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *GitHubRepositoryHandler) ListBindings(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.bindings == nil || h.locals == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub repository bindings are unavailable")
		return
	}
	ownerID := auth.ScopeUserIDFromContext(r.Context())
	bindings, err := h.bindings.List(ownerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "GitHub repository bindings could not be read")
		return
	}
	var currentGitHubUserID int64
	if h.connection != nil {
		status, statusErr := h.connection.Status(ownerID)
		if statusErr != nil {
			handleGitHubRepositoryError(w, statusErr)
			return
		}
		currentGitHubUserID = status.GitHubUserID
	}
	localIDs := h.locals.RepositoryIDs()
	views := make([]githubRepositoryBindingView, 0, len(bindings))
	for _, binding := range bindings {
		views = append(views, githubRepositoryBindingView{
			LocalRepositoryID:  binding.LocalRepositoryID,
			GitHubUserID:       binding.GitHubUserID,
			GitHubRepositoryID: binding.GitHubRepositoryID,
			GitHubFullName:     binding.GitHubFullName,
			DefaultBranch:      binding.DefaultBranch,
			Private:            binding.Private,
			Fork:               binding.Fork,
			Archived:           binding.Archived,
			Disabled:           binding.Disabled,
			AccountMatches:     currentGitHubUserID > 0 && currentGitHubUserID == binding.GitHubUserID,
			LocalConfigured:    h.locals.HasRepository(binding.LocalRepositoryID),
		})
	}
	respondJSON(w, http.StatusOK, githubRepositoryBindingsResponse{LocalRepositories: localIDs, Bindings: views})
}

func (h *GitHubRepositoryHandler) Bind(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.discovery == nil || h.connection == nil || h.bindings == nil || h.locals == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub repository bindings are unavailable")
		return
	}
	localRepositoryID := strings.TrimSpace(chi.URLParam(r, "localRepositoryId"))
	if !gitrepo.ValidRepositoryID(localRepositoryID) || !h.locals.HasRepository(localRepositoryID) {
		respondError(w, http.StatusNotFound, "Local Git repository is not configured")
		return
	}
	var request githubRepositoryBindingRequest
	if err := decodeGitHubRepositoryBindingRequest(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.GitHubRepositoryID <= 0 {
		respondError(w, http.StatusBadRequest, "github_repository_id must be a positive integer")
		return
	}
	ownerID := auth.ScopeUserIDFromContext(r.Context())
	discovered, err := h.discovery.Get(r.Context(), ownerID, request.GitHubRepositoryID)
	if err != nil {
		handleGitHubRepositoryError(w, err)
		return
	}
	status, err := h.connection.Status(ownerID)
	if err != nil {
		handleGitHubRepositoryError(w, err)
		return
	}
	if status.GitHubUserID <= 0 {
		respondError(w, http.StatusConflict, "GitHub account is not connected")
		return
	}
	binding := repository.GitHubRepositoryBinding{
		LocalRepositoryID:  localRepositoryID,
		GitHubUserID:       status.GitHubUserID,
		GitHubRepositoryID: discovered.ID,
		GitHubFullName:     discovered.FullName,
		DefaultBranch:      discovered.DefaultBranch,
		Private:            discovered.Private,
		Fork:               discovered.Fork,
		Archived:           discovered.Archived,
		Disabled:           discovered.Disabled,
	}
	if err := h.bindings.Upsert(ownerID, binding); err != nil {
		respondError(w, http.StatusInternalServerError, "GitHub repository binding could not be saved")
		return
	}
	respondJSON(w, http.StatusOK, githubRepositoryBindingView{
		LocalRepositoryID:  binding.LocalRepositoryID,
		GitHubUserID:       binding.GitHubUserID,
		GitHubRepositoryID: binding.GitHubRepositoryID,
		GitHubFullName:     binding.GitHubFullName,
		DefaultBranch:      binding.DefaultBranch,
		Private:            binding.Private,
		Fork:               binding.Fork,
		Archived:           binding.Archived,
		Disabled:           binding.Disabled,
		AccountMatches:     true,
		LocalConfigured:    true,
	})
}

func (h *GitHubRepositoryHandler) DeleteBinding(w http.ResponseWriter, r *http.Request) {
	setGitHubAuthNoStore(w)
	if h == nil || h.bindings == nil {
		respondError(w, http.StatusServiceUnavailable, "GitHub repository bindings are unavailable")
		return
	}
	localRepositoryID := strings.TrimSpace(chi.URLParam(r, "localRepositoryId"))
	if !gitrepo.ValidRepositoryID(localRepositoryID) {
		respondError(w, http.StatusBadRequest, "Invalid local Git repository ID")
		return
	}
	if err := h.bindings.Delete(auth.ScopeUserIDFromContext(r.Context()), localRepositoryID); err != nil {
		respondError(w, http.StatusInternalServerError, "GitHub repository binding could not be deleted")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeGitHubRepositoryBindingRequest(r *http.Request, target *githubRepositoryBindingRequest) error {
	if r == nil || r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxGitHubRepositoryBindingBodyBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request body")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain one JSON object")
	}
	return nil
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func handleGitHubRepositoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, githubauth.ErrNotConfigured):
		respondError(w, http.StatusServiceUnavailable, "GitHub App authentication is not configured")
	case errors.Is(err, githubauth.ErrNotConnected):
		respondError(w, http.StatusConflict, "GitHub account is not connected")
	case errors.Is(err, githubauth.ErrReauthorizationRequired):
		respondError(w, http.StatusConflict, "GitHub reauthorization is required")
	case errors.Is(err, githubrepo.ErrRepositoryNotFound):
		respondError(w, http.StatusNotFound, "GitHub repository is not available")
	default:
		respondError(w, http.StatusBadGateway, "GitHub repository request failed")
	}
}
