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

// Planner generates structured execution plans via LLM.
type Planner struct {
	llmService   *llm.Service
	toolRegistry *tools.Registry
}

// NewPlanner creates a Planner.
func NewPlanner(llmService *llm.Service, toolRegistry *tools.Registry) *Planner {
	return &Planner{
		llmService:   llmService,
		toolRegistry: toolRegistry,
	}
}

// GeneratePlan asks the LLM to produce a structured plan for the given goal.
func (p *Planner) GeneratePlan(ctx context.Context, provider, model, goal string, conversationHistory []llm.ChatMessage) ([]PlanStep, error) {
	toolDefs := p.toolRegistry.List()
	toolDescriptions := formatToolDescriptions(toolDefs)
	toolNames := make(map[string]bool, len(toolDefs))
	for _, d := range toolDefs {
		toolNames[d.Name] = true
	}

	systemPrompt := fmt.Sprintf(`You are an autonomous agent planner. Given a user's goal, generate a structured execution plan.

Available tools:
%s

Respond with a JSON array of steps. Each step has:
- "type": one of "think", "tool_call", "approval", "message"
- "description": what this step does
- "tool_name": (only for tool_call) which tool to use
- "input_json": (optional for tool_call) pre-filled arguments as a JSON object

Step type guidelines:
- "think": internal reasoning / analysis step (no side-effects)
- "tool_call": executes a registered tool — must include "tool_name"
- "approval": pauses execution and asks the user for explicit approval before continuing
- "message": generates a response to the user, typically used for the final step

Plan guidelines:
- Start with a "think" step to analyze the goal
- Use "tool_call" steps when external data is needed
- Add an "approval" step before any destructive or irreversible actions
- End with a "message" step to present the final result
- Keep plans concise (typically 3-7 steps)
- Only use tools that are listed above

Respond ONLY with a valid JSON array, no markdown fences or explanation.`, toolDescriptions)

	messages := make([]llm.ChatMessage, 0, len(conversationHistory)+2)
	messages = append(messages, llm.ChatMessage{Role: "system", Content: systemPrompt})
	// Include last few messages for context (max 10)
	historyLimit := len(conversationHistory)
	if historyLimit > 10 {
		historyLimit = 10
	}
	messages = append(messages, conversationHistory[len(conversationHistory)-historyLimit:]...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: fmt.Sprintf("Create a plan for this goal: %s", goal)})

	resp, err := p.llmService.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("planner LLM call: %w", err)
	}

	// Parse the JSON response
	content := strings.TrimSpace(resp.Content)
	// Strip markdown fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var steps []PlanStep
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		// Log the parse failure — don't silently swallow it.
		log.Printf("[agent/planner] failed to parse plan JSON (falling back to 2-step plan): %v — raw response: %.200s", err, content)
		return []PlanStep{
			{Type: StepTypeThink, Description: "Analyze the user's request"},
			{Type: StepTypeMessage, Description: "Respond to the user's goal: " + goal},
		}, nil
	}

	// Validate and sanitise steps.
	validated := make([]PlanStep, 0, len(steps))
	for _, s := range steps {
		if !ValidStepTypes[s.Type] {
			log.Printf("[agent/planner] dropping step with unknown type %q", s.Type)
			continue
		}
		// Validate tool_name for tool_call steps.
		if s.Type == StepTypeToolCall && s.ToolName != "" && !toolNames[s.ToolName] {
			log.Printf("[agent/planner] dropping tool_call step with unknown tool %q", s.ToolName)
			continue
		}
		// Validate input_json is valid JSON if present.
		if len(s.InputJSON) > 0 && !json.Valid(s.InputJSON) {
			log.Printf("[agent/planner] stripping invalid input_json for step %q", s.Description)
			s.InputJSON = nil
		}
		validated = append(validated, s)
	}

	if len(validated) == 0 {
		log.Printf("[agent/planner] all steps were invalid — using 2-step fallback")
		return []PlanStep{
			{Type: StepTypeThink, Description: "Analyze the user's request"},
			{Type: StepTypeMessage, Description: "Respond to the user's goal: " + goal},
		}, nil
	}

	return validated, nil
}

// formatToolDescriptions formats tool definitions for the system prompt.
func formatToolDescriptions(defs []tools.ToolDefinition) string {
	if len(defs) == 0 {
		return "(no tools available)"
	}
	var sb strings.Builder
	for _, d := range defs {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", d.Name, d.Description))
	}
	return sb.String()
}
