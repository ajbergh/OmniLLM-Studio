package api

import (
	"net/http"
	"strconv"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// AgentEventHandler exposes cursor-based replay for disconnected clients.
type AgentEventHandler struct {
	events   *repository.AgentEventRepo
	runs     *repository.AgentRunRepo
	convos   *repository.ConversationRepo
}

func NewAgentEventHandler(events *repository.AgentEventRepo, runs *repository.AgentRunRepo, convos *repository.ConversationRepo) *AgentEventHandler {
	return &AgentEventHandler{events: events, runs: runs, convos: convos}
}

func (h *AgentEventHandler) List(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	run, err := h.runs.GetByID(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "agent run not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convos, run.ConversationID) {
		return
	}
	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.events.ListAfter(runID, afterID, limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	nextCursor := afterID
	if len(events) > 0 {
		nextCursor = events[len(events)-1].ID
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events":      events,
		"next_cursor": nextCursor,
	})
}
