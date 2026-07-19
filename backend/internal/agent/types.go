// Package agent defines persistent multi-step execution for OmniLLM-Studio.
package agent

import (
	"encoding/json"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// Run status constants.
const (
	RunStatusPlanning         = "planning"
	RunStatusRunning          = "running"
	RunStatusAwaitingApproval = "awaiting_approval"
	RunStatusPaused           = "paused"
	RunStatusCompleted        = "completed"
	RunStatusFailed           = "failed"
	RunStatusCancelled        = "cancelled"
)

// IsTerminalRunStatus returns true for statuses that indicate a run is finished.
func IsTerminalRunStatus(s string) bool {
	return s == RunStatusCompleted || s == RunStatusFailed || s == RunStatusCancelled
}

// Step status constants.
const (
	StepStatusPending          = "pending"
	StepStatusRunning          = "running"
	StepStatusAwaitingApproval = "awaiting_approval"
	StepStatusCompleted        = "completed"
	StepStatusFailed           = "failed"
	StepStatusSkipped          = "skipped"
)

// Step type constants.
const (
	StepTypeThink    = "think"
	StepTypeToolCall = "tool_call"
	StepTypeApproval = "approval"
	StepTypeMessage  = "message"
)

var ValidStepTypes = map[string]bool{
	StepTypeThink: true, StepTypeToolCall: true, StepTypeApproval: true, StepTypeMessage: true,
}

// RunProfile selects orchestration budgets and presentation behavior.
type RunProfile string

const (
	ProfileChat     RunProfile = "chat"
	ProfileResearch RunProfile = "research"
	ProfileAgent    RunProfile = "agent"
)

const (
	DefaultMaxSteps      = 20
	DefaultMaxDuration   = 10 * time.Minute
	DefaultMaxModelCalls = 30
	DefaultMaxToolCalls  = 40
	DefaultMaxRetries    = 2
	DefaultMaxParallel   = 4
)

// RunnerConfig holds configurable limits for agent execution.
type RunnerConfig struct {
	MaxSteps      int
	MaxDuration   time.Duration
	MaxModelCalls int
	MaxToolCalls  int
	MaxRetries    int
	MaxParallel   int
	MaxCostUSD    float64
}

func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		MaxSteps:      DefaultMaxSteps,
		MaxDuration:   DefaultMaxDuration,
		MaxModelCalls: DefaultMaxModelCalls,
		MaxToolCalls:  DefaultMaxToolCalls,
		MaxRetries:    DefaultMaxRetries,
		MaxParallel:   DefaultMaxParallel,
	}
}

// PlanStep describes one planned action. Dependency fields are optional so old
// persisted plans remain compatible.
type PlanStep struct {
	ID               string          `json:"id,omitempty"`
	Type             string          `json:"type"`
	Description      string          `json:"description"`
	ToolName         string          `json:"tool_name,omitempty"`
	InputJSON        json.RawMessage `json:"input_json,omitempty"`
	DependsOn        []string        `json:"depends_on,omitempty"`
	ParallelGroup    string          `json:"parallel_group,omitempty"`
	Retryable        bool            `json:"retryable,omitempty"`
	MaxRetries       int             `json:"max_retries,omitempty"`
	RequiresApproval bool            `json:"requires_approval,omitempty"`
}

// RunWithSteps bundles an agent run with its steps for API responses.
type RunWithSteps struct {
	models.AgentRun
	Steps []models.AgentStep `json:"steps"`
}

// RunBudgets allows callers to lower, but not exceed, server defaults.
type RunBudgets struct {
	MaxSteps      int     `json:"max_steps,omitempty"`
	MaxDurationMS int     `json:"max_duration_ms,omitempty"`
	MaxModelCalls int     `json:"max_model_calls,omitempty"`
	MaxToolCalls  int     `json:"max_tool_calls,omitempty"`
	MaxCostUSD    float64 `json:"max_cost_usd,omitempty"`
}

// StartRunRequest is the API request body for starting an agent run.
type StartRunRequest struct {
	Goal     string      `json:"goal"`
	Provider string      `json:"provider,omitempty"`
	Model    string      `json:"model,omitempty"`
	Profile  RunProfile  `json:"profile,omitempty"`
	Budgets  *RunBudgets `json:"budgets,omitempty"`
}

// ApproveRequest is the API request body for approving/rejecting a step.
type ApproveRequest struct {
	Approved bool            `json:"approved"`
	Feedback string          `json:"feedback,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func ParsePlan(planJSON string) ([]PlanStep, error) {
	var steps []PlanStep
	if err := json.Unmarshal([]byte(planJSON), &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func EncodePlan(steps []PlanStep) (string, error) {
	data, err := json.Marshal(steps)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
