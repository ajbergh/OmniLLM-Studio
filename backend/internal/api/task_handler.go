package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tasks"
	"github.com/go-chi/chi/v5"
)

// TaskHandler provides direct user management for scheduled agent tasks.
type TaskHandler struct {
	scheduler *tasks.Scheduler
	convos    *repository.ConversationRepo
}

func NewTaskHandler(scheduler *tasks.Scheduler, convos *repository.ConversationRepo) *TaskHandler {
	return &TaskHandler{scheduler: scheduler, convos: convos}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.scheduler.List(auth.ScopeUserIDFromContext(r.Context()), limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID  string           `json:"conversation_id"`
		Title           string           `json:"title"`
		Prompt          string           `json:"prompt"`
		Profile         agent.RunProfile `json:"profile,omitempty"`
		Timezone        string           `json:"timezone,omitempty"`
		ScheduleKind    string           `json:"schedule_kind"`
		NextRunAt       time.Time        `json:"next_run_at"`
		IntervalSeconds int64            `json:"interval_seconds,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convos, req.ConversationID) {
		return
	}
	item, err := h.scheduler.Create(tasks.CreateRequest{
		UserID: auth.ScopeUserIDFromContext(r.Context()), ConversationID: req.ConversationID,
		Title: req.Title, Prompt: req.Prompt, Profile: req.Profile, Timezone: req.Timezone,
		ScheduleKind: req.ScheduleKind, NextRunAt: req.NextRunAt, IntervalSeconds: req.IntervalSeconds,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *TaskHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.scheduler.SetStatus(chi.URLParam(r, "taskId"), auth.ScopeUserIDFromContext(r.Context()), req.Status); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": req.Status})
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.scheduler.Delete(chi.URLParam(r, "taskId"), auth.ScopeUserIDFromContext(r.Context())); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
