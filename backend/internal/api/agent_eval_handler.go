package api

import (
	"context"
	"net/http"
	"time"

	evalsvc "github.com/ajbergh/omnillm-studio/internal/eval"
)

// AgentEvalHandler evaluates production planner/tool contracts without executing actions.
type AgentEvalHandler struct{ evaluator *evalsvc.AgentEvaluator }

func NewAgentEvalHandler(evaluator *evalsvc.AgentEvaluator) *AgentEvalHandler {
	return &AgentEvalHandler{evaluator: evaluator}
}

func (h *AgentEvalHandler) Scenarios(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, evalsvc.DefaultAgentScenarios())
}

func (h *AgentEvalHandler) Run(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider  string                  `json:"provider"`
		Model     string                  `json:"model"`
		Scenarios []evalsvc.AgentScenario `json:"scenarios,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Provider == "" || req.Model == "" {
		respondError(w, http.StatusBadRequest, "provider and model are required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	report, err := h.evaluator.Evaluate(ctx, req.Provider, req.Model, req.Scenarios)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, report)
}
