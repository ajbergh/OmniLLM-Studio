package api

import (
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// ScopedToolPermissionHandler manages admin-defined user/workspace/conversation
// restrictions. Resolution remains monotonic in ScopedToolPermissionRepo.
type ScopedToolPermissionHandler struct {
	repo     *repository.ScopedToolPermissionRepo
	registry *tools.Registry
}

func NewScopedToolPermissionHandler(repo *repository.ScopedToolPermissionRepo, registry *tools.Registry) *ScopedToolPermissionHandler {
	return &ScopedToolPermissionHandler{repo: repo, registry: registry}
}

type scopedToolPermissionRequest struct {
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
	ToolName  string `json:"tool_name"`
	Policy    string `json:"policy"`
}

func (h *ScopedToolPermissionHandler) List(w http.ResponseWriter, r *http.Request) {
	scopeType := strings.TrimSpace(r.URL.Query().Get("scope_type"))
	scopeID := strings.TrimSpace(r.URL.Query().Get("scope_id"))
	if scopeType == "" || scopeID == "" {
		respondError(w, http.StatusBadRequest, "scope_type and scope_id are required")
		return
	}
	items, err := h.repo.List(scopeType, scopeID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if items == nil {
		items = []models.ScopedToolPermission{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *ScopedToolPermissionHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req scopedToolPermissionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.registry != nil {
		if _, ok := h.registry.Get(req.ToolName); !ok {
			respondError(w, http.StatusNotFound, "unknown tool: "+req.ToolName)
			return
		}
	}
	if err := h.repo.Upsert(strings.TrimSpace(req.ScopeType), strings.TrimSpace(req.ScopeID), strings.TrimSpace(req.ToolName), strings.TrimSpace(req.Policy)); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.repo.List(req.ScopeType, req.ScopeID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *ScopedToolPermissionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req scopedToolPermissionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.repo.Delete(strings.TrimSpace(req.ScopeType), strings.TrimSpace(req.ScopeID), strings.TrimSpace(req.ToolName)); err != nil {
		respondInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
