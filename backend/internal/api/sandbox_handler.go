package api

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/sandbox"
	"github.com/go-chi/chi/v5"
)

// SandboxHandler exposes safe sandbox runtime/workspace administration surfaces.
// Physical workspace roots are accepted only by the tightly gated grant endpoint
// and are never returned by any response.
type SandboxHandler struct {
	registry *sandbox.WorkspaceRegistry
}

func NewSandboxHandler(database *sql.DB) (*SandboxHandler, error) {
	registry, err := sandbox.NewWorkspaceRegistry(database)
	if err != nil {
		return nil, err
	}
	// Reuse the application's existing SQLite connection instead of letting the
	// transitional default registry open a second database handle.
	sandbox.SetDefaultWorkspaceRegistry(registry)
	return &SandboxHandler{registry: registry}, nil
}

type sandboxStatusResponse struct {
	Configured             bool                        `json:"configured"`
	Capabilities           sandbox.RuntimeCapabilities `json:"capabilities"`
	ExtensionSandboxMode   string                      `json:"extension_sandbox_mode"`
	PathGrantsConfigured   bool                        `json:"path_grants_configured"`
	PathGrantAvailableHere bool                        `json:"path_grant_available_here"`
}

type sandboxWorkspaceResponse struct {
	ID        string            `json:"id"`
	Mode      sandbox.MountMode `json:"mode"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type sandboxWorkspaceChangeResponse struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	RelativePath string    `json:"relative_path"`
	Operation    string    `json:"operation"`
	BeforeExists bool      `json:"before_exists"`
	BeforeSHA256 string    `json:"before_sha256,omitempty"`
	AfterExists  bool      `json:"after_exists"`
	AfterSHA256  string    `json:"after_sha256,omitempty"`
	Revertable   bool      `json:"revertable"`
	CreatedAt    time.Time `json:"created_at"`
}

type createSandboxWorkspaceRequest struct {
	ID       string            `json:"id"`
	RootPath string            `json:"root_path"`
	Mode     sandbox.MountMode `json:"mode"`
}

// Status reports only non-secret runtime capabilities and effective policy.
func (h *SandboxHandler) Status(w http.ResponseWriter, r *http.Request) {
	broker := sandbox.DefaultBroker()
	capabilities := sandbox.RuntimeCapabilities{}
	if broker != nil {
		capabilities = broker.Capabilities()
	}
	mode, err := sandbox.CurrentExtensionSandboxMode()
	if err != nil {
		mode = sandbox.ExtensionSandboxMode("invalid")
	}
	pathConfigured := sandboxPathGrantsConfigured()
	respondJSON(w, http.StatusOK, sandboxStatusResponse{
		Configured:             broker != nil,
		Capabilities:           capabilities,
		ExtensionSandboxMode:   string(mode),
		PathGrantsConfigured:   pathConfigured,
		PathGrantAvailableHere: pathConfigured && requestIsLoopback(r),
	})
}

func (h *SandboxHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondJSON(w, http.StatusOK, []sandboxWorkspaceResponse{})
		return
	}
	items, err := h.registry.List(auth.ScopeUserIDFromContext(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	out := make([]sandboxWorkspaceResponse, 0, len(items))
	for _, item := range items {
		out = append(out, safeSandboxWorkspace(item))
	}
	respondJSON(w, http.StatusOK, out)
}

// CreateWorkspace creates/replaces one explicit owner-scoped filesystem grant.
// Router composition additionally protects this endpoint with RequireRole(admin).
func (h *SandboxHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondError(w, http.StatusServiceUnavailable, "sandbox workspace registry is unavailable")
		return
	}
	if !sandboxPathGrantsConfigured() {
		respondError(w, http.StatusForbidden, "sandbox path grants are disabled by the operator")
		return
	}
	if !requestIsLoopback(r) {
		respondError(w, http.StatusForbidden, "sandbox path grants are restricted to loopback/desktop requests")
		return
	}

	var request createSandboxWorkspaceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid sandbox workspace request")
		return
	}
	workspace, err := h.registry.Register(
		auth.ScopeUserIDFromContext(r.Context()),
		strings.TrimSpace(request.ID),
		strings.TrimSpace(request.RootPath),
		request.Mode,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, safeSandboxWorkspace(*workspace))
}

// DeleteWorkspace revokes a grant without deleting files from disk.
func (h *SandboxHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondError(w, http.StatusServiceUnavailable, "sandbox workspace registry is unavailable")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "workspaceId"))
	if id == "" {
		respondError(w, http.StatusBadRequest, "workspace ID is required")
		return
	}
	if err := h.registry.Remove(auth.ScopeUserIDFromContext(r.Context()), id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SandboxHandler) ListWorkspaceChanges(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondJSON(w, http.StatusOK, []sandboxWorkspaceChangeResponse{})
		return
	}
	workspaceID := strings.TrimSpace(chi.URLParam(r, "workspaceId"))
	if workspaceID == "" {
		respondError(w, http.StatusBadRequest, "workspace ID is required")
		return
	}
	// Resolve the owner-scoped workspace first so an unknown/cross-owner ID does
	// not become a change-history existence oracle.
	if _, err := h.registry.Get(auth.ScopeUserIDFromContext(r.Context()), workspaceID); err != nil {
		respondError(w, http.StatusNotFound, "sandbox workspace not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.registry.ListWorkspaceChanges(
		r.Context(),
		auth.ScopeUserIDFromContext(r.Context()),
		workspaceID,
		limit,
	)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	out := make([]sandboxWorkspaceChangeResponse, 0, len(items))
	for _, item := range items {
		out = append(out, safeSandboxWorkspaceChange(item))
	}
	respondJSON(w, http.StatusOK, out)
}

func safeSandboxWorkspace(workspace sandbox.FileWorkspace) sandboxWorkspaceResponse {
	return sandboxWorkspaceResponse{
		ID:        workspace.ID,
		Mode:      workspace.Mode,
		CreatedAt: workspace.CreatedAt,
		UpdatedAt: workspace.UpdatedAt,
	}
}

func safeSandboxWorkspaceChange(change sandbox.WorkspaceChange) sandboxWorkspaceChangeResponse {
	return sandboxWorkspaceChangeResponse{
		ID:           change.ID,
		WorkspaceID:  change.WorkspaceID,
		RelativePath: change.RelativePath,
		Operation:    change.Operation,
		BeforeExists: change.BeforeExists,
		BeforeSHA256: change.BeforeSHA256,
		AfterExists:  change.AfterExists,
		AfterSHA256:  change.AfterSHA256,
		Revertable:   change.Revertable,
		CreatedAt:    change.CreatedAt,
	}
}

func sandboxPathGrantsConfigured() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_ALLOW_PATH_GRANTS")), "true")
}

func requestIsLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := strings.TrimSpace(r.RemoteAddr)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func registerSandboxRoutes(r chi.Router, handler *SandboxHandler) {
	if r == nil || handler == nil {
		return
	}
	r.Get("/sandbox/status", handler.Status)
	r.Get("/sandbox/workspaces", handler.ListWorkspaces)
	r.Get("/sandbox/workspaces/{workspaceId}/changes", handler.ListWorkspaceChanges)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/sandbox/workspaces", handler.CreateWorkspace)
		r.Delete("/sandbox/workspaces/{workspaceId}", handler.DeleteWorkspace)
	})
}
