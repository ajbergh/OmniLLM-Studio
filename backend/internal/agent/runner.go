package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// EventType defines the types of events emitted during agent execution.
type EventType string

const (
	EventPlan             EventType = "agent_plan"
	EventStepStart        EventType = "agent_step_start"
	EventStepComplete     EventType = "agent_step_complete"
	EventApprovalRequired EventType = "agent_approval_required"
	EventToken            EventType = "agent_token"
	EventComplete         EventType = "agent_complete"
	EventError            EventType = "agent_error"
)

// Event is emitted during agent run execution.
type Event struct {
	Type   EventType   `json:"type"`
	RunID  string      `json:"run_id"`
	StepID string      `json:"step_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// runContext holds per-run state for cancellation and approval signalling.
type runContext struct {
	cancel   context.CancelFunc
	approval chan bool // true = approved, false = rejected
}

// Runner orchestrates agent run execution.
type Runner struct {
	planner      *Planner
	llmService   *llm.Service
	toolExecutor *tools.Executor
	runRepo      *repository.AgentRunRepo
	stepRepo     *repository.AgentStepRepo
	msgRepo      *repository.MessageRepo
	config       RunnerConfig

	mu   sync.Mutex
	runs map[string]*runContext // runID → live context
}

// NewRunner creates a Runner.
func NewRunner(
	planner *Planner,
	llmService *llm.Service,
	toolExecutor *tools.Executor,
	runRepo *repository.AgentRunRepo,
	stepRepo *repository.AgentStepRepo,
	msgRepo *repository.MessageRepo,
) *Runner {
	return &Runner{
		planner:      planner,
		llmService:   llmService,
		toolExecutor: toolExecutor,
		runRepo:      runRepo,
		stepRepo:     stepRepo,
		msgRepo:      msgRepo,
		config:       DefaultRunnerConfig(),
		runs:         make(map[string]*runContext),
	}
}

// registerRun creates a per-run context with cancellation and approval channel.
func (r *Runner) registerRun(runID string, cancel context.CancelFunc) *runContext {
	rc := &runContext{
		cancel:   cancel,
		approval: make(chan bool, 1),
	}
	r.mu.Lock()
	r.runs[runID] = rc
	r.mu.Unlock()
	return rc
}

// deregisterRun cleans up per-run state.
func (r *Runner) deregisterRun(runID string) {
	r.mu.Lock()
	delete(r.runs, runID)
	r.mu.Unlock()
}

// getRunContext returns the live run context, or nil if not running.
func (r *Runner) getRunContext(runID string) *runContext {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[runID]
}

// StartRun creates and begins executing an agent run. It calls onEvent for each
// execution event (plan, step start/complete, etc.) so the caller can stream
// them over SSE. This method blocks until the run completes or fails.
func (r *Runner) StartRun(ctx context.Context, conversationID, goal, provider, model string, history []llm.ChatMessage, onEvent func(Event)) (*models.AgentRun, error) {
	// 1. Create run record
	run, err := r.runRepo.Create(conversationID, goal)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	// Derive a cancellable context so CancelRun can stop in-flight work.
	ctx, cancel := context.WithCancel(ctx)
	rc := r.registerRun(run.ID, cancel)
	defer func() {
		cancel()
		r.deregisterRun(run.ID)
	}()

	// Apply max-duration timeout.
	if r.config.MaxDuration > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, r.config.MaxDuration)
		defer timeoutCancel()
	}

	// Helper: fail the run with an event and DB update.
	failRun := func(reason string) (*models.AgentRun, error) {
		if err := r.runRepo.UpdateStatus(run.ID, RunStatusFailed); err != nil {
			log.Printf("[agent] failed to update run status: %v", err)
		}
		onEvent(Event{Type: EventError, RunID: run.ID, Data: map[string]interface{}{
			"error": reason,
		}})
		final, _ := r.runRepo.GetByID(run.ID)
		return final, fmt.Errorf("%s", reason)
	}

	// 2. Generate plan via LLM
	planSteps, err := r.planner.GeneratePlan(ctx, provider, model, goal, history)
	if err != nil {
		return failRun(fmt.Sprintf("generate plan: %v", err))
	}

	// Enforce max-step guard at plan time.
	if r.config.MaxSteps > 0 && len(planSteps) > r.config.MaxSteps {
		planSteps = planSteps[:r.config.MaxSteps]
		log.Printf("[agent] plan truncated to %d steps (max)", r.config.MaxSteps)
	}

	planJSON, err := EncodePlan(planSteps)
	if err != nil {
		return failRun(fmt.Sprintf("encode plan: %v", err))
	}
	if err := r.runRepo.UpdatePlan(run.ID, planJSON); err != nil {
		log.Printf("[agent] failed to store plan: %v", err)
	}

	// 3. Create step records
	var stepModels []models.AgentStep
	for i, ps := range planSteps {
		inputJSON := "{}"
		if len(ps.InputJSON) > 0 {
			inputJSON = string(ps.InputJSON)
		}
		stepModels = append(stepModels, models.AgentStep{
			RunID:       run.ID,
			StepIndex:   i,
			Type:        ps.Type,
			Description: ps.Description,
			InputJSON:   inputJSON,
			ToolName:    strPtr(ps.ToolName),
		})
	}
	if err := r.stepRepo.CreateBatch(stepModels); err != nil {
		return failRun(fmt.Sprintf("create steps: %v", err))
	}

	// Reload steps (to get IDs)
	steps, err := r.stepRepo.ListByRun(run.ID)
	if err != nil {
		return failRun(fmt.Sprintf("list steps: %v", err))
	}

	// Emit plan event
	onEvent(Event{Type: EventPlan, RunID: run.ID, Data: map[string]interface{}{
		"steps": steps,
	}})

	// 4. Switch to running
	if err := r.runRepo.UpdateStatus(run.ID, RunStatusRunning); err != nil {
		log.Printf("[agent] failed to set running status: %v", err)
	}

	// 5. Execute steps sequentially
	for i := range steps {
		step := &steps[i]

		// Check cancellation / timeout between steps.
		if ctx.Err() != nil {
			status := RunStatusCancelled
			if ctx.Err() == context.DeadlineExceeded {
				status = RunStatusFailed
				onEvent(Event{Type: EventError, RunID: run.ID, Data: map[string]interface{}{
					"error": "agent run exceeded maximum duration",
				}})
			}
			if err := r.runRepo.UpdateStatus(run.ID, status); err != nil {
				log.Printf("[agent] failed to update run status on cancel: %v", err)
			}
			final, _ := r.runRepo.GetByID(run.ID)
			return final, nil
		}

		onEvent(Event{Type: EventStepStart, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
			"type":        step.Type,
			"description": step.Description,
			"step_index":  step.StepIndex,
		}})

		if err := r.stepRepo.UpdateStatus(step.ID, StepStatusRunning); err != nil {
			log.Printf("[agent] failed to set step running: %v", err)
		}

		// --- Handle approval steps specially: pause and wait ---
		if step.Type == StepTypeApproval {
			if err := r.stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval); err != nil {
				log.Printf("[agent] failed to set step awaiting_approval: %v", err)
			}
			if err := r.runRepo.UpdateStatus(run.ID, RunStatusAwaitingApproval); err != nil {
				log.Printf("[agent] failed to set run awaiting_approval: %v", err)
			}

			// Emit approval-required event so the frontend shows the UI.
			onEvent(Event{Type: EventApprovalRequired, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
				"description": step.Description,
				"step_index":  step.StepIndex,
			}})

			// Block until approval signal or cancellation.
			approved, err := r.waitForApproval(ctx, rc)
			if err != nil {
				// Context cancelled / timed out while waiting.
				if err := r.runRepo.UpdateStatus(run.ID, RunStatusCancelled); err != nil {
					log.Printf("[agent] failed to cancel run during approval wait: %v", err)
				}
				final, _ := r.runRepo.GetByID(run.ID)
				return final, nil
			}

			if approved {
				if err := r.stepRepo.UpdateStatus(step.ID, StepStatusCompleted); err != nil {
					log.Printf("[agent] failed to mark approved step completed: %v", err)
				}
			} else {
				if err := r.stepRepo.UpdateStatus(step.ID, StepStatusSkipped); err != nil {
					log.Printf("[agent] failed to mark rejected step skipped: %v", err)
				}
				// Rejection cancels the run.
				if err := r.runRepo.UpdateStatus(run.ID, RunStatusCancelled); err != nil {
					log.Printf("[agent] failed to cancel run after rejection: %v", err)
				}
				final, _ := r.runRepo.GetByID(run.ID)
				return final, nil
			}

			// Approval completed — resume running status.
			if err := r.runRepo.UpdateStatus(run.ID, RunStatusRunning); err != nil {
				log.Printf("[agent] failed to resume running after approval: %v", err)
			}

			onEvent(Event{Type: EventStepComplete, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
				"output":      "User approved",
				"duration_ms": 0,
			}})

			history = append(history, llm.ChatMessage{
				Role:    "assistant",
				Content: fmt.Sprintf("[Step %d: approval] User approved — proceeding", step.StepIndex+1),
			})
			continue
		}

		// --- Normal step execution ---
		start := time.Now()
		output, err := r.executeStep(ctx, run, step, provider, model, history)
		durationMs := int(time.Since(start).Milliseconds())

		if err != nil {
			outputJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			if err2 := r.stepRepo.UpdateOutput(step.ID, string(outputJSON), durationMs); err2 != nil {
				log.Printf("[agent] failed to store step error output: %v", err2)
			}
			if err2 := r.stepRepo.UpdateStatus(step.ID, StepStatusFailed); err2 != nil {
				log.Printf("[agent] failed to mark step failed: %v", err2)
			}
			log.Printf("[agent] step %d failed: %v", step.StepIndex, err)

			onEvent(Event{Type: EventError, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
				"error": err.Error(),
			}})

			// Fail the run on step failure
			if err2 := r.runRepo.UpdateStatus(run.ID, RunStatusFailed); err2 != nil {
				log.Printf("[agent] failed to mark run failed: %v", err2)
			}
			final, _ := r.runRepo.GetByID(run.ID)
			return final, nil
		}

		outputJSON, _ := json.Marshal(map[string]string{"output": output})
		if err2 := r.stepRepo.UpdateOutput(step.ID, string(outputJSON), durationMs); err2 != nil {
			log.Printf("[agent] failed to store step output: %v", err2)
		}

		onEvent(Event{Type: EventStepComplete, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
			"output":      output,
			"duration_ms": durationMs,
		}})

		// Append step output to history for subsequent steps
		history = append(history, llm.ChatMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("[Step %d: %s] %s", step.StepIndex+1, step.Type, output),
		})
	}

	// 6. Generate summary
	summary := r.generateSummary(ctx, provider, model, goal, history)
	if err := r.runRepo.UpdateResult(run.ID, summary); err != nil {
		log.Printf("[agent] failed to store result summary: %v", err)
	}
	if err := r.runRepo.UpdateStatus(run.ID, RunStatusCompleted); err != nil {
		log.Printf("[agent] failed to set completed status: %v", err)
	}

	onEvent(Event{Type: EventComplete, RunID: run.ID, Data: map[string]interface{}{
		"summary": summary,
	}})

	final, _ := r.runRepo.GetByID(run.ID)
	return final, nil
}

// waitForApproval blocks until an approval decision is received or the context
// is cancelled. Returns (approved, error). Error is non-nil only on cancellation.
func (r *Runner) waitForApproval(ctx context.Context, rc *runContext) (bool, error) {
	select {
	case approved := <-rc.approval:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// ApproveStep approves or rejects a step that's awaiting approval and wakes
// the blocked step loop.
func (r *Runner) ApproveStep(runID, stepID string, approved bool) error {
	step, err := r.stepRepo.GetByID(stepID)
	if err != nil || step == nil {
		return fmt.Errorf("step not found")
	}
	if step.RunID != runID {
		return fmt.Errorf("step does not belong to this run")
	}
	if step.Status != StepStatusAwaitingApproval {
		return fmt.Errorf("step is not awaiting approval (status: %s)", step.Status)
	}

	// Signal the blocked step loop.
	rc := r.getRunContext(runID)
	if rc == nil {
		return fmt.Errorf("run is not currently executing")
	}

	// Non-blocking send — buffer size 1.
	select {
	case rc.approval <- approved:
	default:
	}
	return nil
}

// CancelRun cancels a running or awaiting-approval agent run. It cancels the
// in-flight context so that LLM calls and step execution stop promptly.
func (r *Runner) CancelRun(runID string) error {
	run, err := r.runRepo.GetByID(runID)
	if err != nil || run == nil {
		return fmt.Errorf("run not found")
	}
	if IsTerminalRunStatus(run.Status) {
		return fmt.Errorf("run cannot be cancelled (status: %s)", run.Status)
	}

	// Cancel the in-flight context.
	if rc := r.getRunContext(runID); rc != nil {
		rc.cancel()
	}

	return r.runRepo.UpdateStatus(runID, RunStatusCancelled)
}

// executeStep runs a single step based on its type (excluding approval which
// is handled directly in the step loop).
func (r *Runner) executeStep(ctx context.Context, run *models.AgentRun, step *models.AgentStep, provider, model string, history []llm.ChatMessage) (string, error) {
	switch step.Type {
	case StepTypeThink:
		return r.executeThink(ctx, provider, model, step.Description, history)
	case StepTypeToolCall:
		return r.executeToolCall(ctx, step)
	case StepTypeMessage:
		return r.executeMessage(ctx, run, provider, model, step.Description, history)
	default:
		return "", fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeThink performs an LLM reasoning step (internal monologue).
func (r *Runner) executeThink(ctx context.Context, provider, model, description string, history []llm.ChatMessage) (string, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+2)
	messages = append(messages, llm.ChatMessage{
		Role:    "system",
		Content: "You are an autonomous agent performing a thinking/reasoning step. Analyze and reason about the task. Be concise.",
	})
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("Think about: %s", description),
	})

	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// executeToolCall executes a tool via the tool executor.
func (r *Runner) executeToolCall(ctx context.Context, step *models.AgentStep) (string, error) {
	toolName := ""
	if step.ToolName != nil {
		toolName = *step.ToolName
	}
	if toolName == "" {
		return "", fmt.Errorf("tool_call step missing tool_name")
	}

	var args json.RawMessage
	if step.InputJSON != "" && step.InputJSON != "{}" {
		args = json.RawMessage(step.InputJSON)
	} else {
		args = json.RawMessage(`{}`)
	}

	result := r.toolExecutor.Execute(ctx, tools.ToolCall{
		ID:        step.ID,
		Name:      toolName,
		Arguments: args,
	})

	if result.IsError {
		return "", fmt.Errorf("tool error: %s", result.Content)
	}
	return result.Content, nil
}

// executeMessage generates a response message to the user.
func (r *Runner) executeMessage(ctx context.Context, run *models.AgentRun, provider, model, description string, history []llm.ChatMessage) (string, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+2)
	messages = append(messages, llm.ChatMessage{
		Role:    "system",
		Content: "You are an autonomous agent composing a final response to the user. Synthesize all previous step results into a clear, comprehensive answer.",
	})
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("Compose a response for: %s", description),
	})

	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// generateSummary creates a brief summary of what the agent accomplished.
func (r *Runner) generateSummary(ctx context.Context, provider, model, goal string, history []llm.ChatMessage) string {
	messages := []llm.ChatMessage{
		{Role: "system", Content: "Summarize what was accomplished in 1-2 sentences."},
	}
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("Summarize the results for goal: %s", goal),
	})

	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return "Agent run completed."
	}
	return resp.Content
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
