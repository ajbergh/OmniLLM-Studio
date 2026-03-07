package api

import (
	"fmt"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// ConversationHandler handles conversation API endpoints.
type ConversationHandler struct {
	repo *repository.ConversationRepo
}

// NewConversationHandler creates a new ConversationHandler.
func NewConversationHandler(repo *repository.ConversationRepo) *ConversationHandler {
	return &ConversationHandler{repo: repo}
}

func parseConversationKind(raw string) (string, error) {
	if raw == "" {
		return models.ConversationKindChat, nil
	}
	switch raw {
	case models.ConversationKindChat, models.ConversationKindImage:
		return raw, nil
	default:
		return "", fmt.Errorf("invalid conversation kind %q", raw)
	}
}

func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	workspaceID := r.URL.Query().Get("workspace_id")
	kind, err := parseConversationKind(r.URL.Query().Get("kind"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var convos []models.Conversation
	if workspaceID != "" {
		convos, err = h.repo.ListByKind(userID, includeArchived, kind, workspaceID)
	} else {
		convos, err = h.repo.ListByKind(userID, includeArchived, kind)
	}
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convos == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, convos)
}

func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "conversationId")
	convo, err := h.repo.GetByIDForUser(id, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}
	respondJSON(w, http.StatusOK, convo)
}

type createConversationRequest struct {
	Title           string  `json:"title"`
	DefaultProvider *string `json:"default_provider,omitempty"`
	DefaultModel    *string `json:"default_model,omitempty"`
	SystemPrompt    *string `json:"system_prompt,omitempty"`
	WorkspaceID     *string `json:"workspace_id,omitempty"`
	Kind            string  `json:"kind,omitempty"`
}

func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req createConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kind, err := parseConversationKind(req.Kind)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	title := req.Title
	if title == "" {
		if kind == models.ConversationKindImage {
			title = "Untitled Session"
		} else {
			title = "New Conversation"
		}
	}

	convo, err := h.repo.CreateWithKind(userID, title, kind, req.DefaultProvider, req.DefaultModel, req.SystemPrompt, req.WorkspaceID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, convo)
}

func (h *ConversationHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "conversationId")

	var upd repository.ConversationUpdate
	if err := decodeJSON(r, &upd); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	convo, err := h.repo.Update(id, userID, upd)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}
	respondJSON(w, http.StatusOK, convo)
}

func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "conversationId")
	if err := h.repo.Delete(id, userID); err != nil {
		respondInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ConversationHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	q := r.URL.Query().Get("q")
	if q == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' required")
		return
	}
	kind, err := parseConversationKind(r.URL.Query().Get("kind"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	convos, err := h.repo.SearchByKind(userID, q, kind)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, convos)
}
