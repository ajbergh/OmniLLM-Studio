package models

const (
	AssistantProfileBundleSchema  = "omnillm.assistant-profile"
	AssistantProfileBundleVersion = 1
)

// PortableAssistantProfile contains only profile settings that are safe and
// meaningful to move between OmniLLM-Studio installations. Owner/workspace IDs,
// timestamps, and provider credentials are never part of the bundle.
type PortableAssistantProfile struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	ToolNames    []string `json:"tool_names"`
}

// PortableSkill embeds one attached Markdown Skill without local ownership or
// database identifiers.
type PortableSkill struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	BodyMarkdown string `json:"body_markdown"`
	Enabled      bool   `json:"enabled"`
}

// AssistantProfileBundle is the versioned interchange format for reusable
// Assistant Profiles and their attached Skills.
type AssistantProfileBundle struct {
	Schema   string                   `json:"schema"`
	Version  int                      `json:"version"`
	Profile  PortableAssistantProfile `json:"profile"`
	Skills   []PortableSkill          `json:"skills"`
	Warnings []string                 `json:"warnings,omitempty"`
}
