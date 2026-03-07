package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/templates"
	"github.com/go-chi/chi/v5"
)

// TemplateHandler exposes prompt template CRUD endpoints.
type TemplateHandler struct {
	repo *repository.TemplateRepo
}

// NewTemplateHandler creates a TemplateHandler.
func NewTemplateHandler(repo *repository.TemplateRepo) *TemplateHandler {
	return &TemplateHandler{repo: repo}
}

// ListTemplates returns all templates, optionally filtered by ?category=.
func (h *TemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	list, err := h.repo.List(category)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if list == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// GetTemplate returns a single template by ID.
func (h *TemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.repo.GetByID(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if t == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}
	respondJSON(w, http.StatusOK, t)
}

// CreateTemplate creates a new user-defined template.
func (h *TemplateHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var input repository.CreateTemplateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" || input.TemplateBody == "" {
		respondError(w, http.StatusBadRequest, "name and template_body are required")
		return
	}
	// User-created templates are never system templates.
	input.IsSystem = false

	t, err := h.repo.Create(input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, t)
}

// UpdateTemplate updates an existing template by ID.
func (h *TemplateHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input repository.UpdateTemplateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t, err := h.repo.Update(id, input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if t == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}
	respondJSON(w, http.StatusOK, t)
}

// DeleteTemplate removes a user-created template. System templates cannot be deleted.
func (h *TemplateHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	deleted, err := h.repo.Delete(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !deleted {
		respondError(w, http.StatusNotFound, "template not found or is a system template")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// InterpolateTemplate applies variable values to a template and returns the
// interpolated text.
func (h *TemplateHandler) InterpolateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.repo.GetByID(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if t == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}

	var body struct {
		Values map[string]string `json:"values"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := templates.Interpolate(*t, body.Values)
	if len(result.MissingRequired) > 0 {
		respondErrorWithCode(w, http.StatusUnprocessableEntity, "MISSING_VARIABLES",
			"missing required variables", result.MissingRequired)
		return
	}
	respondJSON(w, http.StatusOK, result)
}
