package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// WorkspaceHandler handles workspace API requests.
type WorkspaceHandler struct {
	workspaceRepo *repository.WorkspaceRepo
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(workspaceRepo *repository.WorkspaceRepo) *WorkspaceHandler {
	return &WorkspaceHandler{workspaceRepo: workspaceRepo}
}

// List handles GET /v1/workspaces
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaces, err := h.workspaceRepo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if workspaces == nil {
		workspaces = []models.Workspace{}
	}
	respondJSON(w, http.StatusOK, workspaces)
}

// Create handles POST /v1/workspaces
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input repository.CreateWorkspaceInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if input.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	workspace, err := h.workspaceRepo.Create(input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, workspace)
}

// Get handles GET /v1/workspaces/{id}
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	workspace, err := h.workspaceRepo.GetByID(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if workspace == nil {
		respondError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respondJSON(w, http.StatusOK, workspace)
}

// Update handles PATCH /v1/workspaces/{id}
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input repository.UpdateWorkspaceInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	workspace, err := h.workspaceRepo.Update(id, input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if workspace == nil {
		respondError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respondJSON(w, http.StatusOK, workspace)
}

// Delete handles DELETE /v1/workspaces/{id}
func (h *WorkspaceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.workspaceRepo.Delete(id); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// GetStats handles GET /v1/workspaces/{id}/stats
func (h *WorkspaceHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	stats, err := h.workspaceRepo.GetStats(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}
