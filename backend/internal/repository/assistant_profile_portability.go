package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ImportBundle atomically creates fresh owner-scoped Skill and profile records
// from a portable bundle. Local IDs, workspace ownership, and timestamps are
// always regenerated rather than trusted from imported data.
func (r *AssistantProfileRepo) ImportBundle(ownerUserID string, bundle models.AssistantProfileBundle) (*models.AssistantProfile, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("assistant profile repository is not configured")
	}
	if strings.TrimSpace(bundle.Profile.Name) == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	for _, skill := range bundle.Skills {
		if strings.TrimSpace(skill.Name) == "" {
			return nil, fmt.Errorf("skill name is required")
		}
		if strings.TrimSpace(skill.BodyMarkdown) == "" {
			return nil, fmt.Errorf("skill body_markdown is required")
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin assistant bundle import: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	skillIDs := make([]string, 0, len(bundle.Skills))
	for _, portable := range bundle.Skills {
		id := uuid.NewString()
		enabled := 0
		if portable.Enabled {
			enabled = 1
		}
		if _, err := tx.Exec(`INSERT INTO skills
			(id, owner_user_id, workspace_id, name, description, body_markdown, enabled, created_at, updated_at)
			VALUES (?, ?, '', ?, ?, ?, ?, ?, ?)`,
			id, ownerUserID, strings.TrimSpace(portable.Name), portable.Description, portable.BodyMarkdown, enabled, now, now); err != nil {
			return nil, fmt.Errorf("import skill %q: %w", portable.Name, err)
		}
		skillIDs = append(skillIDs, id)
	}

	profileID := uuid.NewString()
	toolNames := append([]string(nil), bundle.Profile.ToolNames...)
	toolsJSON, err := json.Marshal(toolNames)
	if err != nil {
		return nil, fmt.Errorf("encode imported tool names: %w", err)
	}
	skillsJSON, err := json.Marshal(skillIDs)
	if err != nil {
		return nil, fmt.Errorf("encode imported skill ids: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO assistant_profiles
		(id, owner_user_id, workspace_id, name, description, provider, model, system_prompt, tool_names_json, skill_ids_json, created_at, updated_at)
		VALUES (?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profileID,
		ownerUserID,
		strings.TrimSpace(bundle.Profile.Name),
		bundle.Profile.Description,
		strings.TrimSpace(bundle.Profile.Provider),
		strings.TrimSpace(bundle.Profile.Model),
		bundle.Profile.SystemPrompt,
		string(toolsJSON),
		string(skillsJSON),
		now,
		now,
	); err != nil {
		return nil, fmt.Errorf("import assistant profile: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit assistant bundle import: %w", err)
	}
	return r.Get(ownerUserID, profileID)
}
