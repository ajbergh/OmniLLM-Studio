package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// AssistantProfileRepo persists owner-scoped reusable Agent Mode profiles.
type AssistantProfileRepo struct{ db *sql.DB }

func NewAssistantProfileRepo(db *sql.DB) *AssistantProfileRepo { return &AssistantProfileRepo{db: db} }

func (r *AssistantProfileRepo) List(ownerUserID string) ([]models.AssistantProfile, error) {
	rows, err := r.db.Query(`SELECT id, owner_user_id, workspace_id, name, description, provider, model,
		system_prompt, tool_names_json, skill_ids_json, created_at, updated_at
		FROM assistant_profiles WHERE owner_user_id = ? ORDER BY name`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AssistantProfile
	for rows.Next() {
		profile, err := scanAssistantProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (r *AssistantProfileRepo) Get(ownerUserID, id string) (*models.AssistantProfile, error) {
	row := r.db.QueryRow(`SELECT id, owner_user_id, workspace_id, name, description, provider, model,
		system_prompt, tool_names_json, skill_ids_json, created_at, updated_at
		FROM assistant_profiles WHERE owner_user_id = ? AND id = ?`, ownerUserID, id)
	profile, err := scanAssistantProfileRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *AssistantProfileRepo) Save(ownerUserID string, profile models.AssistantProfile) (*models.AssistantProfile, error) {
	if strings.TrimSpace(profile.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if profile.ID == "" {
		profile.ID = uuid.NewString()
	}
	profile.OwnerUserID = ownerUserID
	now := time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	toolsJSON, _ := json.Marshal(profile.ToolNames)
	skillsJSON, _ := json.Marshal(profile.SkillIDs)
	_, err := r.db.Exec(`INSERT INTO assistant_profiles
		(id, owner_user_id, workspace_id, name, description, provider, model, system_prompt, tool_names_json, skill_ids_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET workspace_id=excluded.workspace_id, name=excluded.name,
		description=excluded.description, provider=excluded.provider, model=excluded.model,
		system_prompt=excluded.system_prompt, tool_names_json=excluded.tool_names_json,
		skill_ids_json=excluded.skill_ids_json, updated_at=excluded.updated_at
		WHERE assistant_profiles.owner_user_id = excluded.owner_user_id`,
		profile.ID, ownerUserID, profile.WorkspaceID, profile.Name, profile.Description, profile.Provider, profile.Model,
		profile.SystemPrompt, string(toolsJSON), string(skillsJSON), profile.CreatedAt, profile.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r.Get(ownerUserID, profile.ID)
}

func (r *AssistantProfileRepo) Delete(ownerUserID, id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM assistant_profiles WHERE owner_user_id = ? AND id = ?`, ownerUserID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SkillRepo persists owner-scoped Markdown skills.
type SkillRepo struct{ db *sql.DB }

func NewSkillRepo(db *sql.DB) *SkillRepo { return &SkillRepo{db: db} }

func (r *SkillRepo) List(ownerUserID string, includeBody bool) ([]models.Skill, error) {
	body := "''"
	if includeBody {
		body = "body_markdown"
	}
	rows, err := r.db.Query(`SELECT id, owner_user_id, workspace_id, name, description, `+body+`, enabled, created_at, updated_at
		FROM skills WHERE owner_user_id = ? ORDER BY name`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Skill
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return out, rows.Err()
}

func (r *SkillRepo) Get(ownerUserID, idOrName string) (*models.Skill, error) {
	row := r.db.QueryRow(`SELECT id, owner_user_id, workspace_id, name, description, body_markdown, enabled, created_at, updated_at
		FROM skills WHERE owner_user_id = ? AND (id = ? OR name = ?) LIMIT 1`, ownerUserID, idOrName, idOrName)
	var skill models.Skill
	var enabled int
	err := row.Scan(&skill.ID, &skill.OwnerUserID, &skill.WorkspaceID, &skill.Name, &skill.Description, &skill.BodyMarkdown, &enabled, &skill.CreatedAt, &skill.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	skill.Enabled = enabled != 0
	return &skill, nil
}

func (r *SkillRepo) Save(ownerUserID string, skill models.Skill) (*models.Skill, error) {
	if strings.TrimSpace(skill.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(skill.BodyMarkdown) == "" {
		return nil, fmt.Errorf("body_markdown is required")
	}
	if skill.ID == "" {
		skill.ID = uuid.NewString()
	}
	skill.OwnerUserID = ownerUserID
	now := time.Now().UTC()
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	skill.UpdatedAt = now
	enabled := 0
	if skill.Enabled {
		enabled = 1
	}
	_, err := r.db.Exec(`INSERT INTO skills (id, owner_user_id, workspace_id, name, description, body_markdown, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET workspace_id=excluded.workspace_id, name=excluded.name,
		description=excluded.description, body_markdown=excluded.body_markdown, enabled=excluded.enabled,
		updated_at=excluded.updated_at WHERE skills.owner_user_id = excluded.owner_user_id`,
		skill.ID, ownerUserID, skill.WorkspaceID, skill.Name, skill.Description, skill.BodyMarkdown, enabled, skill.CreatedAt, skill.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r.Get(ownerUserID, skill.ID)
}

func (r *SkillRepo) Delete(ownerUserID, id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM skills WHERE owner_user_id = ? AND id = ?`, ownerUserID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

type assistantProfileScanner interface {
	Scan(dest ...any) error
}

func scanAssistantProfileRow(row assistantProfileScanner) (models.AssistantProfile, error) {
	var profile models.AssistantProfile
	var toolJSON, skillJSON string
	err := row.Scan(&profile.ID, &profile.OwnerUserID, &profile.WorkspaceID, &profile.Name, &profile.Description,
		&profile.Provider, &profile.Model, &profile.SystemPrompt, &toolJSON, &skillJSON, &profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		return profile, err
	}
	_ = json.Unmarshal([]byte(toolJSON), &profile.ToolNames)
	_ = json.Unmarshal([]byte(skillJSON), &profile.SkillIDs)
	if profile.ToolNames == nil {
		profile.ToolNames = []string{}
	}
	if profile.SkillIDs == nil {
		profile.SkillIDs = []string{}
	}
	return profile, nil
}

func scanAssistantProfile(rows *sql.Rows) (models.AssistantProfile, error) {
	return scanAssistantProfileRow(rows)
}

func scanSkill(rows *sql.Rows) (models.Skill, error) {
	var skill models.Skill
	var enabled int
	err := rows.Scan(&skill.ID, &skill.OwnerUserID, &skill.WorkspaceID, &skill.Name, &skill.Description, &skill.BodyMarkdown, &enabled, &skill.CreatedAt, &skill.UpdatedAt)
	skill.Enabled = enabled != 0
	return skill, err
}
