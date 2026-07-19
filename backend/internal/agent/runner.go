// Package agent provides persistent autonomous execution capabilities for OmniLLM-Studio.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
	"github.com/google/uuid"
)

type EventType string

const (
	EventPlan             EventType = "agent_plan"
	EventStepStart        EventType = "agent_step_start"
	EventStepComplete     EventType = "agent_step_complete"
	EventApprovalRequired EventType = "agent_approval_required"
	EventToken            EventType = "agent_token"
	EventRetry            EventType = "agent_retry"
	EventReplan           EventType = "agent_replan"
	EventCheckpoint       EventType = "agent_checkpoint"
	EventTool             EventType = "agent_tool"
	EventComplete         EventType = "agent_complete"
	EventError            EventType = "agent_error"
)

var errToolApprovalRejected = errors.New("tool approval rejected")

type Event struct {
	Type   EventType   `json:"type"`
	RunID  string      `json:"run_id"`
	StepID string      `json:"step_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

type RunOptions struct {
	Profile RunProfile
	Budgets *RunBudgets
}

type runContext struct {
	cancel   context.CancelFunc
	approval chan bool
}

type Runner struct {
	planner      *Planner
	llmService   *llm.Service
	toolExecutor *tools.Executor
	runRepo      *repository.AgentRunRepo
	stepRepo     *repository.AgentStepRepo
	msgRepo      *repository.MessageRepo
	config       RunnerConfig

	mu   sync.Mutex
	runs map[string]*runContext
}

func NewRunner(planner *Planner, llmService *llm.Service, toolExecutor *tools.Executor, runRepo *repository.AgentRunRepo, stepRepo *repository.AgentStepRepo, msgRepo *repository.MessageRepo) *Runner {
	r := &Runner{
		planner: planner, llmService: llmService, toolExecutor: toolExecutor,
		runRepo: runRepo, stepRepo: stepRepo, msgRepo: msgRepo,
		config: DefaultRunnerConfig(), runs: make(map[string]*runContext),
	}
	if runRepo != nil {
		if count, err := runRepo.MarkInterruptedPaused(); err != nil {
			log.Printf("[agent] startup recovery: %v", err)
		} else if count > 0 {
			log.Printf("[agent] marked %d interrupted run(s) paused", count)
		}
	}
	if stepRepo != nil {
		if count, err := stepRepo.ResetAllInterrupted(); err != nil {
			log.Printf("[agent] reset interrupted steps: %v", err)
		} else if count > 0 {
			log.Printf("[agent] reset %d interrupted step(s)", count)
		}
	}
	return r
}

func (r *Runner) registerRun(runID string, cancel context.CancelFunc) *runContext {
	rc := &runContext{cancel: cancel, approval: make(chan bool, 1)}
	r.mu.Lock()
	r.runs[runID] = rc
	r.mu.Unlock()
	return rc
}

func (r *Runner) deregisterRun(runID string) {
	r.mu.Lock()
	delete(r.runs, runID)
	r.mu.Unlock()
}

func (r *Runner) getRunContext(runID string) *runContext {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[runID]
}

func (r *Runner) StartRun(ctx context.Context, conversationID, goal, provider, model string, history []llm.ChatMessage, onEvent func(Event)) (*models.AgentRun, error) {
	return r.StartRunWithOptions(ctx, conversationID, goal, provider, model, history, RunOptions{Profile: ProfileAgent}, onEvent)
}

func (r *Runner) StartRunWithOptions(ctx context.Context, conversationID, goal, provider, model string, history []llm.ChatMessage, options RunOptions, onEvent func(Event)) (*models.AgentRun, error) {
	if r.runRepo == nil || r.stepRepo == nil || r.planner == nil {
		return nil, fmt.Errorf("agent runtime is not fully configured")
	}
	run, err := r.runRepo.Create(conversationID, goal)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	cfg := r.resolveConfig(options)
	execCtx, cancel := context.WithCancel(ctx)
	if cfg.MaxDuration > 0 {
		var timeoutCancel context.CancelFunc
		execCtx, timeoutCancel = context.WithTimeout(execCtx, cfg.MaxDuration)
		defer timeoutCancel()
	}
	rc := r.registerRun(run.ID, cancel)
	defer func() {
		cancel()
		r.deregisterRun(run.ID)
	}()

	planningGoal := goal
	if options.Profile == ProfileResearch {
		planningGoal = "Research this goal using multiple independent, citation-ready sources before synthesizing a report: " + goal
	}
	plan, err := r.planner.GeneratePlan(execCtx, provider, model, planningGoal, history)
	if err != nil {
		return r.failRun(run.ID, "generate plan: "+err.Error(), onEvent)
	}
	if cfg.MaxSteps > 0 && len(plan) > cfg.MaxSteps {
		plan = plan[:cfg.MaxSteps]
		if plan[len(plan)-1].Type != StepTypeMessage {
			plan[len(plan)-1] = PlanStep{ID: plan[len(plan)-1].ID, Type: StepTypeMessage, Description: "Synthesize the completed work into the final response"}
		}
	}
	if err := r.persistInitialPlan(run.ID, plan); err != nil {
		return r.failRun(run.ID, "persist plan: "+err.Error(), onEvent)
	}
	emit(onEvent, Event{Type: EventPlan, RunID: run.ID, Data: map[string]interface{}{"steps": plan, "profile": options.Profile}})
	return r.executePersisted(execCtx, rc, run, plan, provider, model, history, cfg, onEvent)
}

func (r *Runner) ResumeRun(ctx context.Context, runID, provider, model string, history []llm.ChatMessage, onEvent func(Event)) (*models.AgentRun, error) {
	if r.getRunContext(runID) != nil {
		return nil, fmt.Errorf("run is already executing")
	}
	run, err := r.runRepo.GetByID(runID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("run not found")
	}
	if IsTerminalRunStatus(run.Status) {
		return nil, fmt.Errorf("run cannot be resumed (status: %s)", run.Status)
	}
	plan, err := ParsePlan(run.PlanJSON)
	if err != nil || len(plan) == 0 {
		plan, err = r.planner.GeneratePlan(ctx, provider, model, run.Goal, history)
		if err != nil {
			return nil, fmt.Errorf("regenerate missing plan: %w", err)
		}
		if err := r.persistInitialPlan(run.ID, plan); err != nil {
			return nil, err
		}
	}
	if _, err := r.stepRepo.ResetInterrupted(runID); err != nil {
		return nil, err
	}
	cfg := r.config
	execCtx, cancel := context.WithCancel(ctx)
	if cfg.MaxDuration > 0 {
		var timeoutCancel context.CancelFunc
		execCtx, timeoutCancel = context.WithTimeout(execCtx, cfg.MaxDuration)
		defer timeoutCancel()
	}
	rc := r.registerRun(run.ID, cancel)
	defer func() {
		cancel()
		r.deregisterRun(run.ID)
	}()
	emit(onEvent, Event{Type: EventPlan, RunID: run.ID, Data: map[string]interface{}{"steps": plan, "resumed": true}})
	return r.executePersisted(execCtx, rc, run, plan, provider, model, history, cfg, onEvent)
}

func (r *Runner) resolveConfig(options RunOptions) RunnerConfig {
	cfg := r.config
	if options.Budgets == nil {
		return cfg
	}
	b := options.Budgets
	if b.MaxSteps > 0 && (cfg.MaxSteps <= 0 || b.MaxSteps < cfg.MaxSteps) {
		cfg.MaxSteps = b.MaxSteps
	}
	if b.MaxDurationMS > 0 {
		requested := time.Duration(b.MaxDurationMS) * time.Millisecond
		if cfg.MaxDuration <= 0 || requested < cfg.MaxDuration {
			cfg.MaxDuration = requested
		}
	}
	if b.MaxModelCalls > 0 && (cfg.MaxModelCalls <= 0 || b.MaxModelCalls < cfg.MaxModelCalls) {
		cfg.MaxModelCalls = b.MaxModelCalls
	}
	if b.MaxToolCalls > 0 && (cfg.MaxToolCalls <= 0 || b.MaxToolCalls < cfg.MaxToolCalls) {
		cfg.MaxToolCalls = b.MaxToolCalls
	}
	if b.MaxCostUSD > 0 && (cfg.MaxCostUSD <= 0 || b.MaxCostUSD < cfg.MaxCostUSD) {
		cfg.MaxCostUSD = b.MaxCostUSD
	}
	return cfg
}

func (r *Runner) persistInitialPlan(runID string, plan []PlanStep) error {
	planJSON, err := EncodePlan(plan)
	if err != nil {
		return err
	}
	if err := r.runRepo.UpdatePlan(runID, planJSON); err != nil {
		return err
	}
	steps := make([]models.AgentStep, 0, len(plan))
	for i, planned := range plan {
		input := "{}"
		if len(planned.InputJSON) > 0 {
			input = string(planned.InputJSON)
		}
		steps = append(steps, models.AgentStep{
			RunID: runID, StepIndex: i, Type: planned.Type,
			Description: planned.Description, InputJSON: input,
			ToolName: strPtr(planned.ToolName),
		})
	}
	return r.stepRepo.CreateBatch(steps)
}

type budgetState struct {
	mu         sync.Mutex
	config     RunnerConfig
	modelCalls int
	toolCalls  int
	costUSD    float64
}

func (b *budgetState) reserveModel() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.config.MaxModelCalls > 0 && b.modelCalls >= b.config.MaxModelCalls {
		return fmt.Errorf("agent model-call budget exhausted")
	}
	b.modelCalls++
	return nil
}

func (b *budgetState) reserveTools(count int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.config.MaxToolCalls > 0 && b.toolCalls+count > b.config.MaxToolCalls {
		return fmt.Errorf("agent tool-call budget exhausted")
	}
	b.toolCalls += count
	return nil
}

func (r *Runner) executePersisted(ctx context.Context, rc *runContext, run *models.AgentRun, plan []PlanStep, provider, model string, history []llm.ChatMessage, cfg RunnerConfig, onEvent func(Event)) (*models.AgentRun, error) {
	if err := r.runRepo.UpdateStatus(run.ID, RunStatusRunning); err != nil {
		log.Printf("[agent] set running: %v", err)
	}
	steps, err := r.stepRepo.ListByRun(run.ID)
	if err != nil {
		return r.failRun(run.ID, "load steps: "+err.Error(), onEvent)
	}
	history = appendCompletedStepHistory(history, steps)
	budget := &budgetState{config: cfg}
	replanned := false

	for index := 0; index < len(steps); {
		if ctx.Err() != nil {
			return r.finishInterrupted(run.ID, ctx.Err(), onEvent)
		}
		step := &steps[index]
		if step.Status == StepStatusCompleted || step.Status == StepStatusSkipped {
			index++
			continue
		}
		planned := planStepAt(plan, index, step)

		if step.Type == StepTypeApproval {
			approved, interrupted := r.executeApprovalStep(ctx, rc, run, step, onEvent)
			if interrupted {
				return r.finishInterrupted(run.ID, ctx.Err(), onEvent)
			}
			if !approved {
				final, _ := r.runRepo.GetByID(run.ID)
				return final, nil
			}
			history = append(history, llm.ChatMessage{Role: "assistant", Content: fmt.Sprintf("[Step %d: approval] User approved proceeding.", step.StepIndex+1)})
			emit(onEvent, Event{Type: EventCheckpoint, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"step_index": step.StepIndex}})
			index++
			continue
		}

		if end := r.parallelGroupEnd(plan, steps, index, cfg.MaxParallel); end > index+1 {
			outputs, groupErr := r.executeParallelGroup(ctx, rc, run, plan[index:end], steps[index:end], budget, onEvent)
			if groupErr == nil {
				for offset, output := range outputs {
					history = append(history, llm.ChatMessage{Role: "assistant", Content: fmt.Sprintf("[Step %d: tool_call] %s", index+offset+1, output)})
					steps[index+offset].Status = StepStatusCompleted
				}
				index = end
				continue
			}
			if !replanned && !errors.Is(groupErr, errToolApprovalRejected) {
				newPlan, newSteps, replanErr := r.replanFromFailure(ctx, run, plan, index, provider, model, history, groupErr, cfg, onEvent)
				if replanErr == nil {
					plan, steps, replanned = newPlan, newSteps, true
					continue
				}
			}
			return r.failStepAndRun(run.ID, steps[index].ID, groupErr, onEvent)
		}

		emit(onEvent, Event{Type: EventStepStart, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{
			"type": step.Type, "description": step.Description, "step_index": step.StepIndex,
		}})
		if err := r.stepRepo.UpdateStatus(step.ID, StepStatusRunning); err != nil {
			log.Printf("[agent] set step running: %v", err)
		}
		started := time.Now()
		output, execErr := r.executeWithRetry(ctx, rc, run, step, planned, provider, model, history, budget, onEvent)
		durationMS := int(time.Since(started).Milliseconds())
		if execErr != nil {
			if errors.Is(execErr, errToolApprovalRejected) {
				return r.handleApprovalRejection(run.ID, step, durationMS, onEvent)
			}
			if !replanned && ctx.Err() == nil {
				newPlan, newSteps, replanErr := r.replanFromFailure(ctx, run, plan, index, provider, model, history, execErr, cfg, onEvent)
				if replanErr == nil {
					plan, steps, replanned = newPlan, newSteps, true
					continue
				}
				log.Printf("[agent] adaptive replan failed: %v", replanErr)
			}
			return r.failStepAndRun(run.ID, step.ID, execErr, onEvent)
		}

		outputJSON, _ := json.Marshal(map[string]string{"output": output})
		if err := r.stepRepo.UpdateOutput(step.ID, string(outputJSON), durationMS); err != nil {
			log.Printf("[agent] store step output: %v", err)
		}
		if step.Type == StepTypeMessage {
			r.persistConversationMessage(run, step, provider, model, output)
		}
		displayOutput := output
		if step.Type == StepTypeThink {
			displayOutput = "Analysis checkpoint completed"
		}
		emit(onEvent, Event{Type: EventStepComplete, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"output": displayOutput, "duration_ms": durationMS}})
		emit(onEvent, Event{Type: EventCheckpoint, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"step_index": step.StepIndex, "status": StepStatusCompleted}})
		history = append(history, llm.ChatMessage{Role: "assistant", Content: fmt.Sprintf("[Step %d: %s] %s", step.StepIndex+1, step.Type, output)})
		steps[index].Status = StepStatusCompleted
		steps[index].OutputJSON = string(outputJSON)
		index++
	}

	summary := "Agent run completed."
	if err := budget.reserveModel(); err == nil {
		summary = r.generateSummary(ctx, provider, model, run.Goal, history)
	} else {
		log.Printf("[agent] summary skipped: %v", err)
	}
	if err := r.runRepo.UpdateResult(run.ID, summary); err != nil {
		log.Printf("[agent] store result summary: %v", err)
	}
	if err := r.runRepo.UpdateStatus(run.ID, RunStatusCompleted); err != nil {
		log.Printf("[agent] complete run: %v", err)
	}
	emit(onEvent, Event{Type: EventComplete, RunID: run.ID, Data: map[string]interface{}{"summary": summary, "model_calls": budget.modelCalls, "tool_calls": budget.toolCalls}})
	final, _ := r.runRepo.GetByID(run.ID)
	return final, nil
}

func (r *Runner) executeWithRetry(ctx context.Context, rc *runContext, run *models.AgentRun, step *models.AgentStep, planned PlanStep, provider, model string, history []llm.ChatMessage, budget *budgetState, onEvent func(Event)) (string, error) {
	maxRetries := 0
	if planned.Retryable {
		maxRetries = planned.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 1
		}
		if r.config.MaxRetries > 0 && maxRetries > r.config.MaxRetries {
			maxRetries = r.config.MaxRetries
		}
	}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * 500 * time.Millisecond
			emit(onEvent, Event{Type: EventRetry, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"attempt": attempt + 1, "max_attempts": maxRetries + 1, "delay_ms": delay.Milliseconds(), "error": lastErr.Error()}})
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		output, err := r.executeStep(ctx, rc, run, step, provider, model, history, budget, onEvent)
		if err == nil {
			return output, nil
		}
		if errors.Is(err, errToolApprovalRejected) || ctx.Err() != nil {
			return "", err
		}
		lastErr = err
	}
	return "", lastErr
}

func (r *Runner) executeStep(ctx context.Context, rc *runContext, run *models.AgentRun, step *models.AgentStep, provider, model string, history []llm.ChatMessage, budget *budgetState, onEvent func(Event)) (string, error) {
	switch step.Type {
	case StepTypeThink:
		if err := budget.reserveModel(); err != nil {
			return "", err
		}
		return r.executeThink(ctx, provider, model, step.Description, history)
	case StepTypeToolCall:
		if err := budget.reserveTools(1); err != nil {
			return "", err
		}
		return r.executeToolCall(ctx, rc, run, step, onEvent)
	case StepTypeMessage:
		if err := budget.reserveModel(); err != nil {
			return "", err
		}
		return r.executeMessage(ctx, run, provider, model, step.Description, history)
	default:
		return "", fmt.Errorf("unknown step type: %s", step.Type)
	}
}

func (r *Runner) executeThink(ctx context.Context, provider, model, description string, history []llm.ChatMessage) (string, error) {
	messages := []llm.ChatMessage{{Role: "system", Content: "Perform the requested analysis and return a concise decision summary with assumptions, evidence needed, and next action. Do not reveal private chain-of-thought or hidden reasoning."}}
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: "Analysis objective: " + description})
	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{Provider: provider, Model: model, Messages: messages})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (r *Runner) executeToolCall(ctx context.Context, rc *runContext, run *models.AgentRun, step *models.AgentStep, onEvent func(Event)) (string, error) {
	toolName := ""
	if step.ToolName != nil {
		toolName = *step.ToolName
	}
	if toolName == "" {
		return "", fmt.Errorf("tool_call step missing tool_name")
	}
	args := json.RawMessage(`{}`)
	if step.InputJSON != "" && step.InputJSON != "{}" {
		args = json.RawMessage(step.InputJSON)
	}
	execCtx := tools.ContextWithInvocationScope(ctx, tools.InvocationScope{ConversationID: run.ConversationID, RunID: run.ID})
	execCtx = tools.ContextWithEventSink(execCtx, func(event tools.ToolEvent) {
		if event.Type == tools.ToolEventQueued || event.Type == tools.ToolEventApprovalRequired || event.Type == tools.ToolEventApprovalResolved {
			return
		}
		emit(onEvent, Event{Type: EventTool, RunID: run.ID, StepID: step.ID, Data: event})
	})
	execCtx = tools.ContextWithApprovalHandler(execCtx, func(approvalCtx context.Context, req tools.ApprovalRequest) (bool, error) {
		return r.requestToolApproval(approvalCtx, run, step, rc, req, onEvent)
	})
	result := r.toolExecutor.Execute(execCtx, tools.ToolCall{ID: step.ID, Name: toolName, Arguments: args})
	if result.IsError {
		if status, _ := result.Metadata[tools.ApprovalStatusMetadataKey].(string); status == "rejected" {
			return "", errToolApprovalRejected
		}
		return "", fmt.Errorf("tool error: %s", result.Content)
	}
	return result.Content, nil
}

func (r *Runner) executeMessage(ctx context.Context, _ *models.AgentRun, provider, model, description string, history []llm.ChatMessage) (string, error) {
	messages := []llm.ChatMessage{{Role: "system", Content: "Compose the final user-facing answer. Synthesize completed steps, distinguish facts from inference, cite supplied sources where available, and state any incomplete work or limitations."}}
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: "Compose a response for: " + description})
	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{Provider: provider, Model: model, Messages: messages})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (r *Runner) executeApprovalStep(ctx context.Context, rc *runContext, run *models.AgentRun, step *models.AgentStep, onEvent func(Event)) (bool, bool) {
	_ = r.stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval)
	_ = r.runRepo.UpdateStatus(run.ID, RunStatusAwaitingApproval)
	emit(onEvent, Event{Type: EventApprovalRequired, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"description": step.Description, "step_index": step.StepIndex}})
	approved, err := r.waitForApproval(ctx, rc)
	if err != nil {
		return false, true
	}
	if !approved {
		_ = r.stepRepo.UpdateStatus(step.ID, StepStatusSkipped)
		_ = r.runRepo.UpdateStatus(run.ID, RunStatusCancelled)
		return false, false
	}
	_ = r.stepRepo.UpdateOutput(step.ID, `{"output":"User approved"}`, 0)
	_ = r.runRepo.UpdateStatus(run.ID, RunStatusRunning)
	emit(onEvent, Event{Type: EventStepComplete, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"output": "User approved", "duration_ms": 0}})
	return true, false
}

func (r *Runner) requestToolApproval(ctx context.Context, run *models.AgentRun, step *models.AgentStep, rc *runContext, req tools.ApprovalRequest, onEvent func(Event)) (bool, error) {
	_ = r.stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval)
	_ = r.runRepo.UpdateStatus(run.ID, RunStatusAwaitingApproval)
	arguments := string(req.Arguments)
	if arguments == "" {
		arguments = "{}"
	}
	description := "Approve tool " + req.ToolName
	if req.Description != "" {
		description += ": " + req.Description
	}
	emit(onEvent, Event{Type: EventApprovalRequired, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"description": description, "tool_name": req.ToolName, "arguments": arguments, "risk": req.Risk, "read_only": req.ReadOnly, "step_index": step.StepIndex}})
	approved, err := r.waitForApproval(ctx, rc)
	if err != nil {
		return false, err
	}
	if !approved {
		_ = r.stepRepo.UpdateStatus(step.ID, StepStatusSkipped)
		_ = r.runRepo.UpdateStatus(run.ID, RunStatusCancelled)
		return false, nil
	}
	_ = r.stepRepo.UpdateStatus(step.ID, StepStatusRunning)
	_ = r.runRepo.UpdateStatus(run.ID, RunStatusRunning)
	return true, nil
}

func (r *Runner) waitForApproval(ctx context.Context, rc *runContext) (bool, error) {
	select {
	case approved := <-rc.approval:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

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
	rc := r.getRunContext(runID)
	if rc == nil {
		return fmt.Errorf("run is not currently executing")
	}
	select {
	case rc.approval <- approved:
	default:
	}
	return nil
}

func (r *Runner) CancelRun(runID string) error {
	run, err := r.runRepo.GetByID(runID)
	if err != nil || run == nil {
		return fmt.Errorf("run not found")
	}
	if IsTerminalRunStatus(run.Status) {
		return fmt.Errorf("run cannot be cancelled (status: %s)", run.Status)
	}
	if rc := r.getRunContext(runID); rc != nil {
		rc.cancel()
	}
	return r.runRepo.UpdateStatus(runID, RunStatusCancelled)
}

func (r *Runner) PauseRun(runID string) error {
	run, err := r.runRepo.GetByID(runID)
	if err != nil || run == nil {
		return fmt.Errorf("run not found")
	}
	if IsTerminalRunStatus(run.Status) {
		return fmt.Errorf("run cannot be paused (status: %s)", run.Status)
	}
	if err := r.runRepo.UpdateStatus(runID, RunStatusPaused); err != nil {
		return err
	}
	if rc := r.getRunContext(runID); rc != nil {
		rc.cancel()
	}
	return nil
}

func (r *Runner) parallelGroupEnd(plan []PlanStep, steps []models.AgentStep, start, maxParallel int) int {
	if start >= len(plan) || start >= len(steps) || maxParallel < 2 || r.toolExecutor == nil {
		return start + 1
	}
	group := plan[start].ParallelGroup
	if group == "" || steps[start].Type != StepTypeToolCall || plan[start].RequiresApproval {
		return start + 1
	}
	end := start
	for end < len(plan) && end < len(steps) && end-start < maxParallel {
		planned := plan[end]
		step := steps[end]
		if planned.ParallelGroup != group || step.Type != StepTypeToolCall || planned.RequiresApproval || step.Status != StepStatusPending {
			break
		}
		def, ok := r.toolExecutor.Definition(planned.ToolName)
		if !ok || !def.ReadOnly || !def.SupportsParallel || r.toolExecutor.Policy(planned.ToolName) != "allow" {
			break
		}
		end++
	}
	if end-start < 2 {
		return start + 1
	}
	return end
}

func (r *Runner) executeParallelGroup(ctx context.Context, rc *runContext, run *models.AgentRun, planned []PlanStep, steps []models.AgentStep, budget *budgetState, onEvent func(Event)) ([]string, error) {
	if err := budget.reserveTools(len(steps)); err != nil {
		return nil, err
	}
	outputs := make([]string, len(steps))
	errs := make([]error, len(steps))
	var wg sync.WaitGroup
	wg.Add(len(steps))
	for i := range steps {
		step := &steps[i]
		emit(onEvent, Event{Type: EventStepStart, RunID: run.ID, StepID: step.ID, Data: map[string]interface{}{"type": step.Type, "description": step.Description, "step_index": step.StepIndex, "parallel_group": planned[i].ParallelGroup}})
		_ = r.stepRepo.UpdateStatus(step.ID, StepStatusRunning)
		go func(index int, current *models.AgentStep) {
			defer wg.Done()
			started := time.Now()
			outputs[index], errs[index] = r.executeToolCall(ctx, rc, run, current, onEvent)
			duration := int(time.Since(started).Milliseconds())
			if errs[index] != nil {
				_ = r.stepRepo.UpdateStatus(current.ID, StepStatusFailed)
				return
			}
			encoded, _ := json.Marshal(map[string]string{"output": outputs[index]})
			_ = r.stepRepo.UpdateOutput(current.ID, string(encoded), duration)
			emit(onEvent, Event{Type: EventStepComplete, RunID: run.ID, StepID: current.ID, Data: map[string]interface{}{"output": outputs[index], "duration_ms": duration, "parallel_group": planned[index].ParallelGroup}})
			emit(onEvent, Event{Type: EventCheckpoint, RunID: run.ID, StepID: current.ID, Data: map[string]interface{}{"step_index": current.StepIndex}})
		}(i, step)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return outputs, err
		}
	}
	return outputs, nil
}

func (r *Runner) replanFromFailure(ctx context.Context, run *models.AgentRun, currentPlan []PlanStep, fromIndex int, provider, model string, history []llm.ChatMessage, cause error, cfg RunnerConfig, onEvent func(Event)) ([]PlanStep, []models.AgentStep, error) {
	if r.planner == nil || fromIndex >= len(currentPlan) {
		return nil, nil, fmt.Errorf("planner unavailable")
	}
	recoveryGoal := fmt.Sprintf("Continue the original goal %q from the failed checkpoint. The failed step was %q and returned: %v. Do not repeat completed work. Produce only the remaining recovery steps and final response.", run.Goal, currentPlan[fromIndex].Description, cause)
	replacement, err := r.planner.GeneratePlan(ctx, provider, model, recoveryGoal, history)
	if err != nil {
		return nil, nil, err
	}
	remaining := cfg.MaxSteps - fromIndex
	if remaining > 0 && len(replacement) > remaining {
		replacement = replacement[:remaining]
	}
	replacement = namespacePlan(replacement, fmt.Sprintf("repair-%d-", fromIndex+1))
	merged := append(append([]PlanStep{}, currentPlan[:fromIndex]...), replacement...)
	encoded, err := EncodePlan(merged)
	if err != nil {
		return nil, nil, err
	}
	modelsToInsert := make([]models.AgentStep, 0, len(replacement))
	for _, planned := range replacement {
		input := "{}"
		if len(planned.InputJSON) > 0 {
			input = string(planned.InputJSON)
		}
		modelsToInsert = append(modelsToInsert, models.AgentStep{Type: planned.Type, Description: planned.Description, InputJSON: input, ToolName: strPtr(planned.ToolName)})
	}
	if err := r.stepRepo.ReplaceFromIndex(run.ID, fromIndex, modelsToInsert); err != nil {
		return nil, nil, err
	}
	if err := r.runRepo.UpdatePlan(run.ID, encoded); err != nil {
		return nil, nil, err
	}
	steps, err := r.stepRepo.ListByRun(run.ID)
	if err != nil {
		return nil, nil, err
	}
	emit(onEvent, Event{Type: EventReplan, RunID: run.ID, Data: map[string]interface{}{"from_step_index": fromIndex, "reason": cause.Error(), "replacement_steps": replacement}})
	return merged, steps, nil
}

func namespacePlan(plan []PlanStep, prefix string) []PlanStep {
	mapping := make(map[string]string, len(plan))
	for i := range plan {
		old := plan[i].ID
		if old == "" {
			old = fmt.Sprintf("step-%d", i+1)
		}
		mapping[old] = prefix + old
		plan[i].ID = mapping[old]
	}
	for i := range plan {
		for j, dependency := range plan[i].DependsOn {
			if replacement, ok := mapping[dependency]; ok {
				plan[i].DependsOn[j] = replacement
			}
		}
	}
	return plan
}

func (r *Runner) persistConversationMessage(run *models.AgentRun, step *models.AgentStep, provider, model, content string) {
	if r.msgRepo == nil || content == "" {
		return
	}
	message := &models.Message{ID: uuid.NewString(), ConversationID: run.ConversationID, Role: "assistant", Content: content, Provider: strPtr(provider), Model: strPtr(model), CreatedAt: time.Now().UTC()}
	if _, err := r.msgRepo.Create(message); err != nil {
		log.Printf("[agent] persist final conversation message: %v", err)
		return
	}
	if err := r.stepRepo.UpdateMessageID(step.ID, message.ID); err != nil {
		log.Printf("[agent] link step message: %v", err)
	}
}

func appendCompletedStepHistory(history []llm.ChatMessage, steps []models.AgentStep) []llm.ChatMessage {
	for _, step := range steps {
		if step.Status != StepStatusCompleted || step.OutputJSON == "" || step.OutputJSON == "{}" {
			continue
		}
		var payload map[string]interface{}
		if json.Unmarshal([]byte(step.OutputJSON), &payload) != nil {
			continue
		}
		output, _ := payload["output"].(string)
		if output != "" {
			history = append(history, llm.ChatMessage{Role: "assistant", Content: fmt.Sprintf("[Completed step %d: %s] %s", step.StepIndex+1, step.Type, output)})
		}
	}
	return history
}

func planStepAt(plan []PlanStep, index int, step *models.AgentStep) PlanStep {
	if index < len(plan) {
		return plan[index]
	}
	planned := PlanStep{ID: fmt.Sprintf("step-%d", index+1), Type: step.Type, Description: step.Description, InputJSON: json.RawMessage(step.InputJSON)}
	if step.ToolName != nil {
		planned.ToolName = *step.ToolName
	}
	return planned
}

func (r *Runner) handleApprovalRejection(runID string, step *models.AgentStep, durationMS int, onEvent func(Event)) (*models.AgentRun, error) {
	encoded, _ := json.Marshal(map[string]string{"output": "Tool approval rejected"})
	_ = r.stepRepo.UpdateOutput(step.ID, string(encoded), durationMS)
	_ = r.stepRepo.UpdateStatus(step.ID, StepStatusSkipped)
	_ = r.runRepo.UpdateStatus(runID, RunStatusCancelled)
	emit(onEvent, Event{Type: EventStepComplete, RunID: runID, StepID: step.ID, Data: map[string]interface{}{"output": "Tool approval rejected", "duration_ms": durationMS}})
	final, _ := r.runRepo.GetByID(runID)
	return final, nil
}

func (r *Runner) failStepAndRun(runID, stepID string, cause error, onEvent func(Event)) (*models.AgentRun, error) {
	encoded, _ := json.Marshal(map[string]string{"error": cause.Error()})
	_ = r.stepRepo.UpdateOutput(stepID, string(encoded), 0)
	_ = r.stepRepo.UpdateStatus(stepID, StepStatusFailed)
	_ = r.runRepo.UpdateStatus(runID, RunStatusFailed)
	emit(onEvent, Event{Type: EventError, RunID: runID, StepID: stepID, Data: map[string]interface{}{"error": cause.Error()}})
	final, _ := r.runRepo.GetByID(runID)
	return final, nil
}

func (r *Runner) failRun(runID, reason string, onEvent func(Event)) (*models.AgentRun, error) {
	_ = r.runRepo.UpdateStatus(runID, RunStatusFailed)
	emit(onEvent, Event{Type: EventError, RunID: runID, Data: map[string]interface{}{"error": reason}})
	final, _ := r.runRepo.GetByID(runID)
	return final, fmt.Errorf("%s", reason)
}

func (r *Runner) finishInterrupted(runID string, cause error, onEvent func(Event)) (*models.AgentRun, error) {
	run, _ := r.runRepo.GetByID(runID)
	if run != nil && run.Status != RunStatusPaused && run.Status != RunStatusCancelled {
		status := RunStatusPaused
		if errors.Is(cause, context.DeadlineExceeded) {
			status = RunStatusFailed
		}
		_ = r.runRepo.UpdateStatus(runID, status)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		emit(onEvent, Event{Type: EventError, RunID: runID, Data: map[string]interface{}{"error": "agent run exceeded maximum duration"}})
	}
	final, _ := r.runRepo.GetByID(runID)
	return final, nil
}

func (r *Runner) generateSummary(ctx context.Context, provider, model, goal string, history []llm.ChatMessage) string {
	if r.llmService == nil {
		return "Agent run completed."
	}
	messages := []llm.ChatMessage{{Role: "system", Content: "Summarize what was accomplished in one or two user-facing sentences without exposing hidden reasoning."}}
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{Role: "user", Content: "Summarize the results for goal: " + goal})
	resp, err := r.llmService.ChatComplete(ctx, llm.ChatRequest{Provider: provider, Model: model, Messages: messages})
	if err != nil {
		return "Agent run completed."
	}
	return resp.Content
}

func emit(callback func(Event), event Event) {
	PublishEvent(event)
	if callback != nil {
		callback(event)
	}
}

func strPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
