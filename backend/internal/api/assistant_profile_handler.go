package api

import (
	"fmt"
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

// ExportProfile returns a versioned, portable profile bundle without local
// ownership, workspace, timestamps, or provider credentials.
func (h *AssistantProfileHandler) ExportProfile(w http.ResponseWriter, r *http.Request) {
	ownerID := assistantOwnerID(r)
	profile, err := h.profiles.Get(ownerID, chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if profile == nil {
		respondError(w, http.StatusNotFound, "assistant profile not found")
		return
	}

	bundle := models.AssistantProfileBundle{
		Schema:  models.AssistantProfileBundleSchema,
		Version: models.AssistantProfileBundleVersion,
		Profile: models.PortableAssistantProfile{
			Name:         profile.Name,
			Description:  profile.Description,
			Provider:     profile.Provider,
			Model:        profile.Model,
			SystemPrompt: profile.SystemPrompt,
			ToolNames:    append([]string(nil), profile.ToolNames...),
		},
		Skills: []models.PortableSkill{},
	}
	missingSkills := 0
	for _, skillID := range profile.SkillIDs {
		skill, err := h.skills.Get(ownerID, skillID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		if skill == nil {
			missingSkills++
			continue
		}
		bundle.Skills = append(bundle.Skills, models.PortableSkill{
			Name:         skill.Name,
			Description:  skill.Description,
			BodyMarkdown: skill.BodyMarkdown,
			Enabled:      skill.Enabled,
		})
	}
	if missingSkills > 0 {
		bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("%d attached Skill(s) no longer exist and were omitted", missingSkills))
	}
	respondJSON(w, http.StatusOK, bundle)
}

// ImportProfile validates a portable bundle and atomically creates fresh local
// Skill/profile records under the authenticated owner.
func (h *AssistantProfileHandler) ImportProfile(w http.ResponseWriter, r *http.Request) {
	var bundle models.AssistantProfileBundle
	if err := decodeJSON(r, &bundle); err != nil {
		respondError(w, http.StatusBadRequest, "invalid assistant profile bundle")
		return
	}
	if message := validateAssistantProfileBundle(bundle); message != "" {
		respondError(w, http.StatusBadRequest, message)
		return
	}
	item, err := h.profiles.ImportBundle(assistantOwnerID(r), bundle)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func validateAssistantProfileBundle(bundle models.AssistantProfileBundle) string {
	if bundle.Schema != models.AssistantProfileBundleSchema || bundle.Version != models.AssistantProfileBundleVersion {
		return "unsupported assistant profile bundle schema or version"
	}
	if strings.TrimSpace(bundle.Profile.Name) == "" {
		return "profile name is required"
	}
	if len(strings.TrimSpace(bundle.Profile.Name)) > 100 || len(bundle.Profile.Description) > 2000 || len(bundle.Profile.SystemPrompt) > 20000 {
		return "profile name, description, or system prompt is too long"
	}
	if len(bundle.Profile.Provider) > 200 || len(bundle.Profile.Model) > 500 {
		return "provider or model value is too long"
	}
	if len(bundle.Profile.ToolNames) > 256 {
		return "assistant profile bundle contains too many tools"
	}
	for _, name := range bundle.Profile.ToolNames {
		if len(strings.TrimSpace(name)) == 0 || len(name) > 200 {
			return "assistant profile bundle contains an invalid tool name"
		}
	}
	if len(bundle.Skills) > 50 {
		return "assistant profile bundle contains too many Skills"
	}
	for _, skill := range bundle.Skills {
		if strings.TrimSpace(skill.Name) == "" || len(strings.TrimSpace(skill.Name)) > 100 {
			return "assistant profile bundle contains an invalid Skill name"
		}
		if len(skill.Description) > 2000 || len(skill.BodyMarkdown) > 50000 || strings.TrimSpace(skill.BodyMarkdown) == "" {
			return "assistant profile bundle contains an invalid Skill body or description"
		}
	}
	return ""
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
