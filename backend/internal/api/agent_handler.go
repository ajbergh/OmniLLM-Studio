package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
	"github.com/go-chi/chi/v5"
)

// AgentHandler exposes checkpointed Agent and Research mode endpoints.
type AgentHandler struct {
	runner    *agent.Runner
	runRepo   *repository.AgentRunRepo
	stepRepo  *repository.AgentStepRepo
	msgRepo   *repository.MessageRepo
	convoRepo *repository.ConversationRepo
}

func NewAgentHandler(runner *agent.Runner, runRepo *repository.AgentRunRepo, stepRepo *repository.AgentStepRepo, msgRepo *repository.MessageRepo, convoRepo *repository.ConversationRepo) *AgentHandler {
	return &AgentHandler{runner: runner, runRepo: runRepo, stepRepo: stepRepo, msgRepo: msgRepo, convoRepo: convoRepo}
}

// StartRun starts a new checkpointed run and streams lifecycle events. The
// execution context is detached from client disconnect so the persisted run can
// complete or be explicitly paused/cancelled.
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
	if req.Profile == "" {
		req.Profile = agent.ProfileAgent
	}
	if req.Profile != agent.ProfileAgent && req.Profile != agent.ProfileResearch && req.Profile != agent.ProfileChat {
		respondError(w, http.StatusBadRequest, "profile must be chat, research, or agent")
		return
	}
	convo, history, err := h.loadExecutionContext(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}
	resolveAgentProviderModel(convo, &req.Provider, &req.Model)

	flusher, onEvent, ok := prepareAgentSSE(w)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	ctx := tools.ContextWithInvocationScope(
		context.WithoutCancel(r.Context()),
		agentInvocationScope(r, convo, conversationID, ""),
	)
	run, runErr := h.runner.StartRunWithOptions(ctx, conversationID, req.Goal, req.Provider, req.Model, history, agent.RunOptions{Profile: req.Profile, Budgets: req.Budgets}, onEvent)
	finishAgentSSE(w, flusher, run, runErr)
}

func (h *AgentHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	runs, err := h.runRepo.ListByConversation(chi.URLParam(r, "conversationId"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, runs)
}

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
	if !verifyConversationAccessByID(w, r, h.convoRepo, run.ConversationID) {
		return
	}
	steps, err := h.stepRepo.ListByRun(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, agent.RunWithSteps{AgentRun: *run, Steps: steps})
}

func (h *AgentHandler) ApproveStep(w http.ResponseWriter, r *http.Request) {
	stepID := chi.URLParam(r, "stepId")
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
	var req agent.ApproveRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.runner.ApproveStep(runID, stepID, req.Approved); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "approved": req.Approved})
}

// CancelRun cancels by default. ?mode=pause checkpoints the run as resumable.
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
	if r.URL.Query().Get("mode") == "pause" {
		if err := h.runner.PauseRun(runID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"paused": true})
		return
	}
	if err := h.runner.CancelRun(runID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

// ResumeRun resumes the first incomplete persisted checkpoint and streams the
// same event protocol as a new run.
func (h *AgentHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
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
	convo, history, err := h.loadExecutionContext(run.ConversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	provider, model := "", ""
	resolveAgentProviderModel(convo, &provider, &model)
	var req struct {
		Provider string `json:"provider,omitempty"`
		Model    string `json:"model,omitempty"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Provider != "" {
			provider = req.Provider
		}
		if req.Model != "" {
			model = req.Model
		}
	}
	flusher, onEvent, ok := prepareAgentSSE(w)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	ctx := tools.ContextWithInvocationScope(
		context.WithoutCancel(r.Context()),
		agentInvocationScope(r, convo, run.ConversationID, run.ID),
	)
	resumed, resumeErr := h.runner.ResumeRun(ctx, runID, provider, model, history, onEvent)
	finishAgentSSE(w, flusher, resumed, resumeErr)
}

func (h *AgentHandler) loadExecutionContext(conversationID string) (*models.Conversation, []llm.ChatMessage, error) {
	convo, err := h.convoRepo.GetByID(conversationID)
	if err != nil || convo == nil {
		return convo, nil, err
	}
	messages, err := h.msgRepo.ListByConversation(conversationID)
	if err != nil {
		return nil, nil, err
	}
	history := make([]llm.ChatMessage, 0, len(messages))
	for _, message := range messages {
		history = append(history, llm.ChatMessage{Role: message.Role, Content: message.Content})
	}
	return convo, history, nil
}

func agentInvocationScope(r *http.Request, convo *models.Conversation, conversationID, runID string) tools.InvocationScope {
	scope := tools.InvocationScope{
		UserID:         auth.ScopeUserIDFromContext(r.Context()),
		ConversationID: conversationID,
		RunID:          runID,
	}
	if convo != nil && convo.WorkspaceID != nil {
		scope.WorkspaceID = *convo.WorkspaceID
	}
	return scope
}

func resolveAgentProviderModel(convo *models.Conversation, provider, model *string) {
	if convo == nil {
		return
	}
	if *provider == "" && convo.DefaultProvider != nil {
		*provider = *convo.DefaultProvider
	}
	if *model == "" && convo.DefaultModel != nil {
		*model = *convo.DefaultModel
	}
}

func prepareAgentSSE(w http.ResponseWriter) (http.Flusher, func(agent.Event), bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, nil, false
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	var writeMu sync.Mutex
	onEvent := func(event agent.Event) {
		data, _ := json.Marshal(event)
		writeMu.Lock()
		defer writeMu.Unlock()
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
		flusher.Flush()
	}
	return flusher, onEvent, true
}

func finishAgentSSE(w http.ResponseWriter, flusher http.Flusher, run *models.AgentRun, err error) {
	if err != nil {
		runID := ""
		if run != nil {
			runID = run.ID
		}
		event := agent.Event{Type: agent.EventError, RunID: runID, Data: map[string]string{"error": err.Error()}}
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", agent.EventError, data)
		flusher.Flush()
		return
	}
	data, _ := json.Marshal(run)
	_, _ = fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
	flusher.Flush()
}
