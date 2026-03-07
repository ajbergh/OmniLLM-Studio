package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// AgentHandler exposes agent mode endpoints.
type AgentHandler struct {
	runner    *agent.Runner
	runRepo   *repository.AgentRunRepo
	stepRepo  *repository.AgentStepRepo
	msgRepo   *repository.MessageRepo
	convoRepo *repository.ConversationRepo
}

// NewAgentHandler creates an AgentHandler.
func NewAgentHandler(
	runner *agent.Runner,
	runRepo *repository.AgentRunRepo,
	stepRepo *repository.AgentStepRepo,
	msgRepo *repository.MessageRepo,
	convoRepo *repository.ConversationRepo,
) *AgentHandler {
	return &AgentHandler{
		runner:    runner,
		runRepo:   runRepo,
		stepRepo:  stepRepo,
		msgRepo:   msgRepo,
		convoRepo: convoRepo,
	}
}

// StartRun starts a new agent run for a conversation. The run is executed
// synchronously and SSE events are streamed to the client.
func (h *AgentHandler) StartRun(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	var req agent.StartRunRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Goal == "" {
		respondError(w, http.StatusBadRequest, "goal is required")
		return
	}

	// Fall back to conversation defaults if provider/model not specified.
	convo, err := h.convoRepo.GetByID(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo != nil {
		if req.Provider == "" && convo.DefaultProvider != nil {
			req.Provider = *convo.DefaultProvider
		}
		if req.Model == "" && convo.DefaultModel != nil {
			req.Model = *convo.DefaultModel
		}
	}

	// Load conversation history for context
	messages, err := h.msgRepo.ListByConversation(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// Convert to LLM chat messages
	var history []llm.ChatMessage
	for _, m := range messages {
		history = append(history, llm.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Set up SSE streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Disable write deadline for this SSE connection (server has WriteTimeout set).
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	onEvent := func(evt agent.Event) {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
		flusher.Flush()
	}

	run, err := h.runner.StartRun(r.Context(), conversationID, req.Goal, req.Provider, req.Model, history, onEvent)
	if err != nil {
		// If we haven't written headers yet, send error JSON
		data, _ := json.Marshal(agent.Event{
			Type:  agent.EventError,
			RunID: "",
			Data:  map[string]string{"error": err.Error()},
		})
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", agent.EventError, data)
		flusher.Flush()
		return
	}

	// Send final done event with run data
	doneData, _ := json.Marshal(run)
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

// ListRuns lists all agent runs for a conversation.
func (h *AgentHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	runs, err := h.runRepo.ListByConversation(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, runs)
}

// GetRun retrieves a single agent run with its steps.
func (h *AgentHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	run, err := h.runRepo.GetByID(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "agent run not found")
		return
	}

	// Verify user owns the parent conversation
	if !verifyConversationAccessByID(w, r, h.convoRepo, run.ConversationID) {
		return
	}

	steps, err := h.stepRepo.ListByRun(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, agent.RunWithSteps{
		AgentRun: *run,
		Steps:    steps,
	})
}

// ApproveStep approves or rejects a step that's awaiting approval.
func (h *AgentHandler) ApproveStep(w http.ResponseWriter, r *http.Request) {
	stepID := chi.URLParam(r, "stepId")
	runID := chi.URLParam(r, "runId")

	// Verify user owns the parent conversation via the run
	run, err := h.runRepo.GetByID(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "agent run not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, run.ConversationID) {
		return
	}

	var req agent.ApproveRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.runner.ApproveStep(runID, stepID, req.Approved); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// CancelRun cancels a running agent.
func (h *AgentHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	run, err := h.runRepo.GetByID(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "agent run not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, run.ConversationID) {
		return
	}

	if err := h.runner.CancelRun(runID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

// ResumeRun is a no-op placeholder. Durable resume (re-executing from a
// specific step) is not yet implemented. Approval-based resume is handled
// automatically by the ApproveStep endpoint.
func (h *AgentHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	respondError(w, http.StatusNotImplemented, "resume is not yet supported — use approve/reject for approval steps")
}
