package models

import "time"

// AssistantProfile packages a reusable model configuration, instructions,
// permitted tools, and attached Skills for Agent Mode.
type AssistantProfile struct {
	ID           string    `json:"id"`
	OwnerUserID  string    `json:"owner_user_id"`
	WorkspaceID  string    `json:"workspace_id,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Provider     string    `json:"provider,omitempty"`
	Model        string    `json:"model,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	ToolNames    []string  `json:"tool_names"`
	SkillIDs     []string  `json:"skill_ids"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Skill is a reusable Markdown instruction package. The list API intentionally
// exposes only metadata; body_markdown is loaded on demand through skill_read.
type Skill struct {
	ID           string    `json:"id"`
	OwnerUserID  string    `json:"owner_user_id"`
	WorkspaceID  string    `json:"workspace_id,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	BodyMarkdown string    `json:"body_markdown,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
