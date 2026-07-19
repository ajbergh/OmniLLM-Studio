package api

import (
	"encoding/json"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/apps"
	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/go-chi/chi/v5"
)

// AppHandler provides direct connected-app catalog and mapping management.
type AppHandler struct{ service *apps.Service }

func NewAppHandler(service *apps.Service) *AppHandler { return &AppHandler{service: service} }

func (h *AppHandler) Catalog(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, h.service.Catalog())
}

func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(auth.UserIDFromContext(r.Context()), r.URL.Query().Get("workspace_id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *AppHandler) ConnectMCP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string          `json:"workspace_id,omitempty"`
		AppKey      string          `json:"app_key"`
		DisplayName string          `json:"display_name,omitempty"`
		ServerID    string          `json:"server_id"`
		Scopes      []string        `json:"scopes"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	connection, err := h.service.ConnectMCP(auth.UserIDFromContext(r.Context()), req.WorkspaceID, req.AppKey, req.DisplayName, req.ServerID, req.Scopes, req.Metadata)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, connection)
}

func (h *AppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(chi.URLParam(r, "connectionId"), auth.UserIDFromContext(r.Context())); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
