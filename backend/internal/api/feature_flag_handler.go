package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// FeatureFlagHandler handles feature flag API endpoints.
type FeatureFlagHandler struct {
	repo *repository.FeatureFlagRepo
}

// NewFeatureFlagHandler creates a new FeatureFlagHandler.
func NewFeatureFlagHandler(repo *repository.FeatureFlagRepo) *FeatureFlagHandler {
	return &FeatureFlagHandler{repo: repo}
}

// List returns all feature flags.
func (h *FeatureFlagHandler) List(w http.ResponseWriter, r *http.Request) {
	flags, err := h.repo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if flags == nil {
		flags = []repository.FeatureFlag{}
	}
	respondJSON(w, http.StatusOK, flags)
}

// Update toggles a feature flag.
func (h *FeatureFlagHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", "feature flag key is required", nil)
		return
	}

	var body struct {
		Enabled  *bool   `json:"enabled"`
		Metadata *string `json:"metadata,omitempty"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", nil)
		return
	}
	if body.Enabled == nil {
		respondErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", "enabled field is required", nil)
		return
	}

	if body.Metadata != nil {
		if err := h.repo.SetWithMetadata(key, *body.Enabled, *body.Metadata); err != nil {
			respondInternalError(w, err)
			return
		}
	} else {
		if err := h.repo.Set(key, *body.Enabled); err != nil {
			respondInternalError(w, err)
			return
		}
	}

	flags, err := h.repo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, flags)
}
