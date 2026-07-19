package api

import (
	"net/http"
	"strconv"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/jobs"
	"github.com/go-chi/chi/v5"
)

// JobHandler exposes owned asynchronous job status and cancellation.
type JobHandler struct{ manager *jobs.Manager }

func NewJobHandler(manager *jobs.Manager) *JobHandler { return &JobHandler{manager: manager} }

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.manager.List(jobs.Scope{
		UserID:         auth.UserIDFromContext(r.Context()),
		WorkspaceID:    r.URL.Query().Get("workspace_id"),
		ConversationID: r.URL.Query().Get("conversation_id"),
	}, limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	job, err := h.manager.Get(chi.URLParam(r, "jobId"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	if job == nil || (job.UserID != "" && userID != job.UserID) {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}
	respondJSON(w, http.StatusOK, job)
}

func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.Cancel(chi.URLParam(r, "jobId"), auth.UserIDFromContext(r.Context())); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}
