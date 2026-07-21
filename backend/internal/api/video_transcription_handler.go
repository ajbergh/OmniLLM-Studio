package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/go-chi/chi/v5"
)

// VideoTranscriptionHandler exposes the versioned provider-backed STT API.
type VideoTranscriptionHandler struct {
	service *video.VideoTranscriptionService
}

// NewVideoTranscriptionHandler performs startup recovery synchronously before
// exposing routes. This keeps composition deterministic and avoids racing
// SQLite in-memory test databases with an untracked background query.
func NewVideoTranscriptionHandler(service *video.VideoTranscriptionService) *VideoTranscriptionHandler {
	service.RecoverInterrupted()
	return &VideoTranscriptionHandler{service: service}
}

func (h *VideoTranscriptionHandler) Start(w http.ResponseWriter, r *http.Request) {
	var request video.TranscriptionRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Start(r.Context(), auth.UserIDFromContext(r.Context()), chi.URLParam(r, "projectId"), request)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, item)
}

func (h *VideoTranscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(auth.UserIDFromContext(r.Context()), chi.URLParam(r, "projectId"))
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *VideoTranscriptionHandler) Get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(auth.UserIDFromContext(r.Context()), chi.URLParam(r, "transcriptId"))
	if err != nil || item == nil {
		respondError(w, http.StatusNotFound, "transcript not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *VideoTranscriptionHandler) Captions(w http.ResponseWriter, r *http.Request) {
	clips, err := h.service.CaptionClips(auth.UserIDFromContext(r.Context()), chi.URLParam(r, "transcriptId"))
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"api_version": video.TranscriptionAPIVersion, "clips": clips})
}
