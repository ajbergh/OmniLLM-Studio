package api

import (
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// AssistantProfileHandler exposes owner-scoped reusable Agent profiles and Skills.
type AssistantProfileHandler struct {
	profiles *repository.AssistantProfileRepo
	skills   *repository.SkillRepo
}

func NewAssistantProfileHandler(profiles *repository.AssistantProfileRepo, skills *repository.SkillRepo) *AssistantProfileHandler {
	return &AssistantProfileHandler{profiles: profiles, skills: skills}
}

func assistantOwnerID(r *http.Request) string { return auth.ScopeUserIDFromContext(r.Context()) }

func (h *AssistantProfileHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	items, err := h.profiles.List(assistantOwnerID(r))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if items == nil {
		items = []models.AssistantProfile{}
	}
	respondJSON(w, http.StatusOK, items)
}
func (h *AssistantProfileHandler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	var profile models.AssistantProfile
	if err := decodeJSON(r, &profile); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(profile.Name)) > 100 || len(profile.SystemPrompt) > 20000 {
		respondError(w, http.StatusBadRequest, "profile name or system prompt is too long")
		return
	}
	item, err := h.profiles.Save(assistantOwnerID(r), profile)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}
func (h *AssistantProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	ok, err := h.profiles.Delete(assistantOwnerID(r), chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !ok {
		respondError(w, http.StatusNotFound, "assistant profile not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AssistantProfileHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	items, err := h.skills.List(assistantOwnerID(r), false)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if items == nil {
		items = []models.Skill{}
	}
	respondJSON(w, http.StatusOK, items)
}
func (h *AssistantProfileHandler) GetSkill(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.Get(assistantOwnerID(r), chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if item == nil {
		respondError(w, http.StatusNotFound, "skill not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}
func (h *AssistantProfileHandler) SaveSkill(w http.ResponseWriter, r *http.Request) {
	var skill models.Skill
	if err := decodeJSON(r, &skill); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(skill.Name)) > 100 || len(skill.BodyMarkdown) > 50000 {
		respondError(w, http.StatusBadRequest, "skill name or body is too long")
		return
	}
	skill.Enabled = true
	item, err := h.skills.Save(assistantOwnerID(r), skill)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}
func (h *AssistantProfileHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	ok, err := h.skills.Delete(assistantOwnerID(r), chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !ok {
		respondError(w, http.StatusNotFound, "skill not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
