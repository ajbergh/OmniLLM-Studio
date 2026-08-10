package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

type SkillListTool struct{ repo *repository.SkillRepo }

func NewSkillListTool(repo *repository.SkillRepo) *SkillListTool { return &SkillListTool{repo: repo} }

func (t *SkillListTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "skill_list",
		Description:      "List the user's available reusable skills by name and description without loading their full instructions.",
		Category:         "skills",
		Enabled:          true,
		ReadOnly:         true,
		SupportsParallel: true,
		Parameters:       json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	}
}

func (t *SkillListTool) Validate(args json.RawMessage) error {
	if len(args) == 0 {
		return nil
	}
	var v map[string]any
	return json.Unmarshal(args, &v)
}

func (t *SkillListTool) Execute(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	if t.repo == nil {
		return nil, fmt.Errorf("skill repository unavailable")
	}
	scope := InvocationScopeFromContext(ctx)
	skills, err := t.repo.List(scope.UserID, false)
	if err != nil {
		return nil, err
	}
	type item struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := make([]item, 0, len(skills))
	for _, skill := range skills {
		if skill.Enabled {
			out = append(out, item{ID: skill.ID, Name: skill.Name, Description: skill.Description})
		}
	}
	data, _ := json.Marshal(out)
	return &ToolResult{Content: string(data), Structured: data}, nil
}

type SkillReadTool struct{ repo *repository.SkillRepo }

func NewSkillReadTool(repo *repository.SkillRepo) *SkillReadTool { return &SkillReadTool{repo: repo} }

func (t *SkillReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "skill_read",
		Description: "Load the full Markdown instructions for one reusable skill by id or exact name.",
		Category:    "skills",
		Enabled:     true,
		ReadOnly:    true,
		Parameters:  json.RawMessage(`{"type":"object","properties":{"skill":{"type":"string","minLength":1}},"required":["skill"],"additionalProperties":false}`),
	}
}

func (t *SkillReadTool) Validate(args json.RawMessage) error {
	var in struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Skill) == "" {
		return fmt.Errorf("skill is required")
	}
	return nil
}

func (t *SkillReadTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t.repo == nil {
		return nil, fmt.Errorf("skill repository unavailable")
	}
	var in struct {
		Skill string `json:"skill"`
	}
	_ = json.Unmarshal(args, &in)
	scope := InvocationScopeFromContext(ctx)
	skill, err := t.repo.Get(scope.UserID, strings.TrimSpace(in.Skill))
	if err != nil {
		return nil, err
	}
	if skill == nil || !skill.Enabled {
		return nil, fmt.Errorf("skill not found")
	}
	structured, _ := json.Marshal(map[string]any{
		"id":            skill.ID,
		"name":          skill.Name,
		"description":   skill.Description,
		"body_markdown": skill.BodyMarkdown,
	})
	return &ToolResult{Content: skill.BodyMarkdown, Structured: structured}, nil
}
