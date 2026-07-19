// Package agent provides autonomous execution capabilities for OmniLLM-Studio.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// Planner generates and repairs structured execution plans via an LLM.
type Planner struct {
	llmService   *llm.Service
	toolRegistry *tools.Registry
}

func NewPlanner(llmService *llm.Service, toolRegistry *tools.Registry) *Planner {
	return &Planner{llmService: llmService, toolRegistry: toolRegistry}
}

// GeneratePlan asks the LLM to produce a validated plan. Only enabled tools are
// supplied, and their complete input schemas and risk metadata are included.
func (p *Planner) GeneratePlan(ctx context.Context, provider, model, goal string, conversationHistory []llm.ChatMessage) ([]PlanStep, error) {
	toolDefs := p.selectTools(goal, conversationHistory)
	toolDescriptions := formatToolDescriptions(toolDefs)
	systemPrompt := fmt.Sprintf(`You are the planner for a local-first autonomous assistant.
Create a concise, executable plan for the user's goal using only the tools below.

Available tool contracts:
%s

Return a JSON array. Each step has:
- id: stable identifier such as "step-1"
- type: "think", "tool_call", "approval", or "message"
- description: user-understandable purpose of the step
- tool_name: required only for tool_call
- input_json: complete JSON object matching the selected tool schema
- depends_on: optional array of earlier step ids
- parallel_group: optional shared identifier for independent read-only calls
- retryable: whether a transient failure can be retried
- max_retries: 0-3
- requires_approval: true for consequential, destructive, external-write, or high-risk work

Rules:
- Use the minimum number of steps needed, usually 3-8.
- Do not invent tools or parameters.
- A tool call must contain valid arguments, not placeholders.
- Independent read-only calls may share a parallel_group.
- Add approval before external writes, deletes, process execution, or other high-risk actions.
- End with one message step that synthesizes evidence and clearly states limitations.
- Never include hidden chain-of-thought. A think step should describe the analysis objective, not reveal private reasoning.

Respond only with the JSON array.`, toolDescriptions)

	messages := make([]llm.ChatMessage, 0, 12)
	messages = append(messages, llm.ChatMessage{Role: "system", Content: systemPrompt})
	historyLimit := len(conversationHistory)
	if historyLimit > 10 {
		historyLimit = 10
	}
	if historyLimit > 0 {
		messages = append(messages, conversationHistory[len(conversationHistory)-historyLimit:]...)
	}
	messages = append(messages, llm.ChatMessage{Role: "user", Content: "Create an execution plan for: " + goal})

	resp, err := p.llmService.ChatComplete(ctx, llm.ChatRequest{Provider: provider, Model: model, Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("planner LLM call: %w", err)
	}

	steps, parseErr := parsePlanContent(resp.Content)
	if parseErr != nil {
		log.Printf("[agent/planner] plan parse failed, attempting repair: %v", parseErr)
		steps, err = p.repairPlan(ctx, provider, model, goal, resp.Content, []string{parseErr.Error()}, toolDescriptions)
		if err != nil {
			return fallbackPlan(goal), nil
		}
	}

	validated, validationErrors := p.validatePlan(steps)
	if len(validationErrors) > 0 {
		log.Printf("[agent/planner] plan validation failed, attempting repair: %s", strings.Join(validationErrors, "; "))
		repaired, repairErr := p.repairPlan(ctx, provider, model, goal, resp.Content, validationErrors, toolDescriptions)
		if repairErr == nil {
			validated, validationErrors = p.validatePlan(repaired)
		}
	}
	if len(validationErrors) > 0 || len(validated) == 0 {
		log.Printf("[agent/planner] invalid plan after repair; using safe fallback: %s", strings.Join(validationErrors, "; "))
		return fallbackPlan(goal), nil
	}
	return validated, nil
}

func (p *Planner) selectTools(goal string, history []llm.ChatMessage) []tools.ToolDefinition {
	terms := strings.Fields(strings.ToLower(goal))
	if len(history) > 0 {
		terms = append(terms, strings.Fields(strings.ToLower(history[len(history)-1].Content))...)
	}
	defs := p.toolRegistry.Select(terms, 12)
	if len(defs) == 0 {
		defs = p.toolRegistry.ListEnabled()
		if len(defs) > 12 {
			defs = defs[:12]
		}
	}
	return defs
}

func (p *Planner) repairPlan(ctx context.Context, provider, model, goal, original string, validationErrors []string, toolDescriptions string) ([]PlanStep, error) {
	messages := []llm.ChatMessage{
		{Role: "system", Content: `Repair an invalid autonomous-agent plan. Return only a valid JSON array. Preserve the user's goal, use only supplied tools, and correct every validation error.`},
		{Role: "user", Content: fmt.Sprintf("Goal:\n%s\n\nTool contracts:\n%s\n\nInvalid plan:\n%s\n\nValidation errors:\n- %s", goal, toolDescriptions, original, strings.Join(validationErrors, "\n- "))},
	}
	resp, err := p.llmService.ChatComplete(ctx, llm.ChatRequest{Provider: provider, Model: model, Messages: messages})
	if err != nil {
		return nil, err
	}
	return parsePlanContent(resp.Content)
}

func parsePlanContent(content string) ([]PlanStep, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var steps []PlanStep
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		return nil, fmt.Errorf("parse plan JSON: %w", err)
	}
	return steps, nil
}

func (p *Planner) validatePlan(steps []PlanStep) ([]PlanStep, []string) {
	validated := make([]PlanStep, 0, len(steps)+1)
	knownIDs := map[string]bool{}
	var errs []string

	for index, step := range steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("step-%d", index+1)
		}
		if knownIDs[step.ID] {
			errs = append(errs, fmt.Sprintf("duplicate step id %q", step.ID))
			continue
		}
		if !ValidStepTypes[step.Type] {
			errs = append(errs, fmt.Sprintf("step %s has unknown type %q", step.ID, step.Type))
			continue
		}
		if strings.TrimSpace(step.Description) == "" {
			errs = append(errs, fmt.Sprintf("step %s is missing description", step.ID))
			continue
		}
		for _, dependency := range step.DependsOn {
			if !knownIDs[dependency] {
				errs = append(errs, fmt.Sprintf("step %s depends on unknown or later step %s", step.ID, dependency))
			}
		}
		if step.MaxRetries < 0 || step.MaxRetries > 3 {
			errs = append(errs, fmt.Sprintf("step %s max_retries must be 0-3", step.ID))
			step.MaxRetries = min(max(step.MaxRetries, 0), 3)
		}

		if step.Type == StepTypeToolCall {
			tool, ok := p.toolRegistry.Get(step.ToolName)
			if !ok || !tool.Definition().Normalized().Enabled {
				errs = append(errs, fmt.Sprintf("step %s references unavailable tool %q", step.ID, step.ToolName))
				continue
			}
			if len(step.InputJSON) == 0 {
				step.InputJSON = json.RawMessage(`{}`)
			}
			if !json.Valid(step.InputJSON) {
				errs = append(errs, fmt.Sprintf("step %s input_json is invalid JSON", step.ID))
				continue
			}
			if err := tool.Validate(step.InputJSON); err != nil {
				errs = append(errs, fmt.Sprintf("step %s arguments for %s are invalid: %v", step.ID, step.ToolName, err))
				continue
			}
			def := tool.Definition().Normalized()
			if def.SideEffecting || def.Risk == tools.RiskHigh || def.Risk == tools.RiskCritical {
				step.RequiresApproval = true
			}
			if step.MaxRetries == 0 && step.Retryable {
				step.MaxRetries = 1
			}
		}

		knownIDs[step.ID] = true
		validated = append(validated, step)
	}

	if len(validated) == 0 || validated[len(validated)-1].Type != StepTypeMessage {
		validated = append(validated, PlanStep{
			ID:          fmt.Sprintf("step-%d", len(validated)+1),
			Type:        StepTypeMessage,
			Description: "Synthesize the completed work into the final response",
		})
	}
	return validated, errs
}

func fallbackPlan(goal string) []PlanStep {
	return []PlanStep{
		{ID: "step-1", Type: StepTypeThink, Description: "Analyze the request and available evidence"},
		{ID: "step-2", Type: StepTypeMessage, Description: "Respond to the user's goal: " + goal},
	}
}

func formatToolDescriptions(defs []tools.ToolDefinition) string {
	if len(defs) == 0 {
		return "[]"
	}
	type plannerTool struct {
		Name             string          `json:"name"`
		Description      string          `json:"description"`
		Category         string          `json:"category"`
		Parameters       json.RawMessage `json:"parameters"`
		Risk             tools.RiskLevel `json:"risk"`
		ReadOnly         bool            `json:"read_only"`
		SideEffecting    bool            `json:"side_effecting"`
		SupportsParallel bool            `json:"supports_parallel"`
		Examples         []tools.ToolExample `json:"examples,omitempty"`
	}
	out := make([]plannerTool, 0, len(defs))
	for _, def := range defs {
		def = def.Normalized()
		out = append(out, plannerTool{
			Name: def.Name, Description: def.Description, Category: def.Category,
			Parameters: def.Parameters, Risk: def.Risk, ReadOnly: def.ReadOnly,
			SideEffecting: def.SideEffecting, SupportsParallel: def.SupportsParallel,
			Examples: def.Examples,
		})
	}
	encoded, _ := json.MarshalIndent(out, "", "  ")
	return string(encoded)
}
