package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/browser"
	"github.com/go-chi/chi/v5"
)

// BrowserHandler exposes browser session management endpoints.
type BrowserHandler struct {
	mgr *browser.Manager
}

func NewBrowserHandler(mgr *browser.Manager) *BrowserHandler {
	return &BrowserHandler{mgr: mgr}
}

func (h *BrowserHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.mgr == nil {
		respondJSON(w, http.StatusOK, browser.Status{})
		return
	}
	respondJSON(w, http.StatusOK, h.mgr.Status())
}

func (h *BrowserHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.mgr == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, h.mgr.ListSessions(auth.UserIDFromContext(r.Context())))
}

func (h *BrowserHandler) CloseSession(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.mgr == nil {
		respondError(w, http.StatusNotFound, "browser manager not available")
		return
	}
	id := chi.URLParam(r, "sessionId")
	if id == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}
	if err := h.mgr.CloseSession(r.Context(), auth.UserIDFromContext(r.Context()), id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
