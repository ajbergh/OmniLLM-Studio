package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/search"
)

// SearchHandler handles search API requests.
type SearchHandler struct {
	searchService *search.Service
	convoRepo     *repository.ConversationRepo
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searchService *search.Service, convoRepo *repository.ConversationRepo) *SearchHandler {
	return &SearchHandler{searchService: searchService, convoRepo: convoRepo}
}

// Search handles GET /v1/search?q=...&mode=...&limit=...&conversation_id=...
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	mode := search.SearchMode(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = search.ModeHybrid
	}
	if mode != search.ModeHybrid && mode != search.ModeKeyword && mode != search.ModeSemantic {
		respondError(w, http.StatusBadRequest, "mode must be 'hybrid', 'keyword', or 'semantic'")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	conversationID := r.URL.Query().Get("conversation_id")
	kind, err := parseConversationKind(r.URL.Query().Get("kind"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := auth.UserIDFromContext(r.Context())

	// If a conversation filter is specified, verify the user owns it
	if conversationID != "" && userID != "" {
		convo, err := h.convoRepo.GetByIDForUser(conversationID, userID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		if convo == nil {
			respondError(w, http.StatusNotFound, "conversation not found")
			return
		}
		if convo.Kind != kind {
			respondError(w, http.StatusNotFound, "conversation not found")
			return
		}
	}

	results, err := h.searchService.Search(r.Context(), search.SearchOptions{
		Query:              q,
		Mode:               mode,
		Limit:              limit,
		ConversationFilter: conversationID,
		ConversationKind:   kind,
		UserID:             userID,
	})
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"query":   q,
		"mode":    string(mode),
		"kind":    kind,
	})
}

// Reindex handles POST /v1/search/reindex
func (h *SearchHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	status, err := h.searchService.Reindex(r.Context())
	if err != nil {
		log.Printf("ERROR: reindex: %v", err)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status": status,
			"error":  "reindex encountered an error",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": status,
	})
}
