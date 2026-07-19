package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	memorysvc "github.com/ajbergh/omnillm-studio/internal/memory"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// MemoryHandler provides direct user control over stored memories.
type MemoryHandler struct {
	service *memorysvc.Service
	convos  *repository.ConversationRepo
}

func NewMemoryHandler(service *memorysvc.Service, convos *repository.ConversationRepo) *MemoryHandler {
	return &MemoryHandler{service: service, convos: convos}
}

func (h *MemoryHandler) List(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.scopeFromRequest(w, r)
	if !ok {
		return
	}
	limit := 0
	if value := r.URL.Query().Get("limit"); value != "" {
		for _, char := range value {
			if char < '0' || char > '9' {
				respondError(w, http.StatusBadRequest, "limit must be numeric")
				return
			}
			limit = limit*10 + int(char-'0')
		}
	}
	items, err := h.service.List(scope, r.URL.Query().Get("query"), limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *MemoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req struct {
		WorkspaceID     string     `json:"workspace_id,omitempty"`
		ConversationID  string     `json:"conversation_id,omitempty"`
		Kind            string     `json:"kind"`
		Content         string     `json:"content"`
		SourceMessageID string     `json:"source_message_id,omitempty"`
		ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConversationID != "" && !verifyConversationAccessByID(w, r, h.convos, req.ConversationID) {
		return
	}
	item, err := h.service.Save(memorysvc.Scope{
		UserID: userID, WorkspaceID: strings.TrimSpace(req.WorkspaceID), ConversationID: strings.TrimSpace(req.ConversationID),
	}, req.Kind, req.Content, req.SourceMessageID, req.ExpiresAt)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *MemoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content   string     `json:"content"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Update(chi.URLParam(r, "memoryId"), auth.UserIDFromContext(r.Context()), req.Content, req.ExpiresAt)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *MemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(chi.URLParam(r, "memoryId"), auth.UserIDFromContext(r.Context())); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MemoryHandler) scopeFromRequest(w http.ResponseWriter, r *http.Request) (memorysvc.Scope, bool) {
	scope := memorysvc.Scope{
		UserID:         auth.UserIDFromContext(r.Context()),
		WorkspaceID:    strings.TrimSpace(r.URL.Query().Get("workspace_id")),
		ConversationID: strings.TrimSpace(r.URL.Query().Get("conversation_id")),
	}
	if scope.ConversationID != "" && !verifyConversationAccessByID(w, r, h.convos, scope.ConversationID) {
		return memorysvc.Scope{}, false
	}
	return scope, true
}
