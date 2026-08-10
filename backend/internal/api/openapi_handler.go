package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	openapiruntime "github.com/ajbergh/omnillm-studio/internal/openapi"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

type OpenAPIHandler struct {
	repo    *repository.OpenAPIServerRepo
	manager *openapiruntime.Manager
}

func NewOpenAPIHandler(repo *repository.OpenAPIServerRepo, manager *openapiruntime.Manager) *OpenAPIHandler {
	return &OpenAPIHandler{repo: repo, manager: manager}
}

type saveOpenAPIServerRequest struct {
	models.OpenAPIServer
	APIKey *string `json:"api_key,omitempty"`
}

func openAPIOwner(r *http.Request) string { return auth.ScopeUserIDFromContext(r.Context()) }

func (h *OpenAPIHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(openAPIOwner(r))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if items == nil {
		items = []models.OpenAPIServer{}
	}
	for i := range items {
		items[i].Tools = h.manager.Tools(items[i].ID)
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *OpenAPIHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req saveOpenAPIServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(req.Name)) > 100 || len(req.SpecJSON) > 1<<20 {
		respondError(w, http.StatusBadRequest, "OpenAPI server name or specification is too large")
		return
	}
	server, err := h.repo.Save(openAPIOwner(r), req.OpenAPIServer, req.APIKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	infos, err := h.manager.Refresh(context.Background(), server.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	server.Tools = infos
	respondJSON(w, http.StatusOK, server)
}

func (h *OpenAPIHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.repo.Get(openAPIOwner(r), id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if server == nil {
		respondError(w, http.StatusNotFound, "OpenAPI server not found")
		return
	}
	infos, err := h.manager.Refresh(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	server.Tools = infos
	respondJSON(w, http.StatusOK, server)
}

func (h *OpenAPIHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ok, err := h.repo.Delete(openAPIOwner(r), id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !ok {
		respondError(w, http.StatusNotFound, "OpenAPI server not found")
		return
	}
	h.manager.Remove(id)
	w.WriteHeader(http.StatusNoContent)
}
