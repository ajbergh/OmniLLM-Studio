package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/eval"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// llmAdapter adapts llm.Service to the eval.LLMClient interface.
type llmAdapter struct {
	svc *llm.Service
}

func (a *llmAdapter) ChatComplete(ctx context.Context, providerName, userMessage, systemPrompt string) (string, error) {
	msgs := []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	resp, err := a.svc.ChatComplete(ctx, llm.ChatRequest{
		Provider: providerName,
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// EvalHandler handles evaluation harness API endpoints.
type EvalHandler struct {
	repo   *repository.EvalRunRepo
	runner *eval.Runner
}

// NewEvalHandler creates a new EvalHandler.
func NewEvalHandler(repo *repository.EvalRunRepo, svc *llm.Service) *EvalHandler {
	runner := eval.NewRunner(&llmAdapter{svc: svc})
	return &EvalHandler{repo: repo, runner: runner}
}

// RunEval accepts a suite JSON payload with provider/model, runs the eval, and stores results.
func (h *EvalHandler) RunEval(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string          `json:"provider"`
		Model    string          `json:"model"`
		Suite    json.RawMessage `json:"suite"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Provider == "" {
		respondError(w, http.StatusBadRequest, "provider is required")
		return
	}

	suite, err := eval.ParseSuite(req.Suite)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.runner.RunSuite(r.Context(), *suite, req.Provider, req.Model)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	resultsJSON := eval.ReportToJSON(report)

	run := &models.EvalRun{
		SuiteName:   suite.Name,
		Provider:    req.Provider,
		Model:       req.Model,
		TotalScore:  &report.TotalScore,
		ResultsJSON: resultsJSON,
	}
	if err := h.repo.Create(run); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

// ListRuns returns eval runs, optionally filtered by ?suite query param.
func (h *EvalHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	suiteName := r.URL.Query().Get("suite")
	runs, err := h.repo.List(suiteName)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, runs)
}

// GetRun returns a single eval run by ID.
func (h *EvalHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run, err := h.repo.GetByID(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "eval run not found")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

// DeleteRun deletes an eval run by ID.
func (h *EvalHandler) DeleteRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
