package agent

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// --------------- Test helpers ---------------

// newTestDB creates an in-memory SQLite database with all migrations applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// newTestRunner creates a Runner with real repos (in-memory SQLite) but nil
// for LLM/planner/tools (not used in state-machine tests).
func newTestRunner(t *testing.T) (*Runner, *repository.AgentRunRepo, *repository.AgentStepRepo, *repository.ConversationRepo) {
	t.Helper()
	database := newTestDB(t)
	runRepo := repository.NewAgentRunRepo(database)
	stepRepo := repository.NewAgentStepRepo(database)
	convoRepo := repository.NewConversationRepo(database)

	r := &Runner{
		runRepo:  runRepo,
		stepRepo: stepRepo,
		config:   DefaultRunnerConfig(),
		runs:     make(map[string]*runContext),
	}
	return r, runRepo, stepRepo, convoRepo
}

// createTestConversation creates a conversation for FK constraint satisfaction.
func createTestConversation(t *testing.T, convoRepo *repository.ConversationRepo) string {
	t.Helper()
	convo, err := convoRepo.Create("", "Test Conversation", nil, nil, nil)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return convo.ID
}

// createTestRun creates an agent run for testing.
func createTestRun(t *testing.T, runRepo *repository.AgentRunRepo, conversationID string) *models.AgentRun {
	t.Helper()
	run, err := runRepo.Create(conversationID, "Test goal")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}

// createTestStep creates a step for testing via the step repo.
func createTestStep(t *testing.T, stepRepo *repository.AgentStepRepo, runID, stepType string, index int) *models.AgentStep {
	t.Helper()
	step, err := stepRepo.Create(runID, index, stepType, fmt.Sprintf("Test step %d", index))
	if err != nil {
		t.Fatalf("create step: %v", err)
	}
	return step
}

// --------------- registerRun / deregisterRun / getRunContext ---------------

func TestRegisterRunCreatesContext(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	rc := runner.registerRun("run-1", cancel)
	if rc == nil {
		t.Fatal("registerRun returned nil")
	}
	if rc.approval == nil {
		t.Error("approval channel should not be nil")
	}
	if rc.cancel == nil {
		t.Error("cancel func should not be nil")
	}
}

func TestGetRunContextReturnsRegistered(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner.registerRun("run-1", cancel)

	rc := runner.getRunContext("run-1")
	if rc == nil {
		t.Fatal("getRunContext returned nil for registered run")
	}

	// Non-existent run should return nil.
	rc2 := runner.getRunContext("run-nonexistent")
	if rc2 != nil {
		t.Error("getRunContext should return nil for unregistered run")
	}
}

func TestDeregisterRunRemovesContext(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner.registerRun("run-1", cancel)
	runner.deregisterRun("run-1")

	rc := runner.getRunContext("run-1")
	if rc != nil {
		t.Error("getRunContext should return nil after deregisterRun")
	}
}

func TestDeregisterNonexistentRunNoOp(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	// Should not panic.
	runner.deregisterRun("nonexistent")
}

// --------------- waitForApproval ---------------

func TestWaitForApprovalReceivesApproved(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	rc := runner.registerRun("run-1", cancel)

	// Send approval in a goroutine.
	go func() {
		time.Sleep(10 * time.Millisecond)
		rc.approval <- true
	}()

	approved, err := runner.waitForApproval(ctx, rc)
	if err != nil {
		t.Fatalf("waitForApproval error: %v", err)
	}
	if !approved {
		t.Error("expected approved = true")
	}
}

func TestWaitForApprovalReceivesRejected(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	rc := runner.registerRun("run-1", cancel)

	go func() {
		time.Sleep(10 * time.Millisecond)
		rc.approval <- false
	}()

	approved, err := runner.waitForApproval(ctx, rc)
	if err != nil {
		t.Fatalf("waitForApproval error: %v", err)
	}
	if approved {
		t.Error("expected approved = false")
	}
}

func TestWaitForApprovalContextCancelled(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	ctx, cancel := context.WithCancel(context.Background())

	rc := runner.registerRun("run-1", cancel)

	// Cancel the context after a short delay.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := runner.waitForApproval(ctx, rc)
	if err == nil {
		t.Error("waitForApproval should return error when context cancelled")
	}
}

func TestWaitForApprovalContextTimeout(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	rc := runner.registerRun("run-1", cancel)

	// Don't send anything — let the context time out.
	_, err := runner.waitForApproval(ctx, rc)
	if err == nil {
		t.Error("waitForApproval should return error on timeout")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

// --------------- ApproveStep ---------------

func TestApproveStepSuccess(t *testing.T) {
	runner, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeApproval, 0)

	// Set step to awaiting_approval.
	if err := stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval); err != nil {
		t.Fatalf("update step status: %v", err)
	}

	// Register the run context (simulating a running run).
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.registerRun(run.ID, cancel)
	defer runner.deregisterRun(run.ID)

	// Call ApproveStep in a goroutine, read from channel in main.
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.ApproveStep(run.ID, step.ID, true)
	}()

	// Read the approval signal.
	rc := runner.getRunContext(run.ID)
	select {
	case approved := <-rc.approval:
		if !approved {
			t.Error("expected approval = true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval signal")
	}

	if err := <-errCh; err != nil {
		t.Errorf("ApproveStep error: %v", err)
	}
}

func TestApproveStepReject(t *testing.T) {
	runner, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeApproval, 0)

	if err := stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval); err != nil {
		t.Fatalf("update step status: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.registerRun(run.ID, cancel)
	defer runner.deregisterRun(run.ID)

	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.ApproveStep(run.ID, step.ID, false)
	}()

	rc := runner.getRunContext(run.ID)
	select {
	case approved := <-rc.approval:
		if approved {
			t.Error("expected approval = false (rejected)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rejection signal")
	}

	if err := <-errCh; err != nil {
		t.Errorf("ApproveStep error: %v", err)
	}
}

func TestApproveStepWrongRun(t *testing.T) {
	runner, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeApproval, 0)

	if err := stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval); err != nil {
		t.Fatalf("update step status: %v", err)
	}

	// Use a different run ID — ownership validation should fail.
	err := runner.ApproveStep("wrong-run-id", step.ID, true)
	if err == nil {
		t.Error("ApproveStep with wrong run ID should return error")
	}
}

func TestApproveStepNotAwaiting(t *testing.T) {
	runner, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeApproval, 0)

	// Step is still 'pending', not 'awaiting_approval'.
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.registerRun(run.ID, cancel)
	defer runner.deregisterRun(run.ID)

	err := runner.ApproveStep(run.ID, step.ID, true)
	if err == nil {
		t.Error("ApproveStep on non-awaiting step should return error")
	}
}

func TestApproveStepNotFound(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	err := runner.ApproveStep("run-1", "nonexistent-step", true)
	if err == nil {
		t.Error("ApproveStep with nonexistent step should return error")
	}
}

func TestApproveStepRunNotExecuting(t *testing.T) {
	runner, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeApproval, 0)

	if err := stepRepo.UpdateStatus(step.ID, StepStatusAwaitingApproval); err != nil {
		t.Fatalf("update step status: %v", err)
	}

	// Don't register the run — it's not currently executing.
	err := runner.ApproveStep(run.ID, step.ID, true)
	if err == nil {
		t.Error("ApproveStep on non-executing run should return error")
	}
}

// --------------- CancelRun ---------------

func TestCancelRunSuccess(t *testing.T) {
	runner, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	// Set run to running status.
	if err := runRepo.UpdateStatus(run.ID, RunStatusRunning); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// Register context (simulating active run).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.registerRun(run.ID, cancel)
	defer runner.deregisterRun(run.ID)

	if err := runner.CancelRun(run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	// Verify context was cancelled.
	if ctx.Err() == nil {
		t.Error("context should be cancelled after CancelRun")
	}

	// Verify DB status is cancelled.
	updated, err := runRepo.GetByID(run.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != RunStatusCancelled {
		t.Errorf("status = %q, want %q", updated.Status, RunStatusCancelled)
	}
}

func TestCancelRunAlreadyCompleted(t *testing.T) {
	runner, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	if err := runRepo.UpdateStatus(run.ID, RunStatusCompleted); err != nil {
		t.Fatalf("update status: %v", err)
	}

	err := runner.CancelRun(run.ID)
	if err == nil {
		t.Error("CancelRun on completed run should return error")
	}
}

func TestCancelRunAlreadyFailed(t *testing.T) {
	runner, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	if err := runRepo.UpdateStatus(run.ID, RunStatusFailed); err != nil {
		t.Fatalf("update status: %v", err)
	}

	err := runner.CancelRun(run.ID)
	if err == nil {
		t.Error("CancelRun on failed run should return error")
	}
}

func TestCancelRunNotFound(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	err := runner.CancelRun("nonexistent-run")
	if err == nil {
		t.Error("CancelRun with nonexistent run should return error")
	}
}

func TestCancelRunAwaitingApproval(t *testing.T) {
	runner, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	// Set run to awaiting_approval (cancellable).
	if err := runRepo.UpdateStatus(run.ID, RunStatusAwaitingApproval); err != nil {
		t.Fatalf("update status: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.registerRun(run.ID, cancel)
	defer runner.deregisterRun(run.ID)

	if err := runner.CancelRun(run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	if ctx.Err() == nil {
		t.Error("context should be cancelled")
	}
	updated, _ := runRepo.GetByID(run.ID)
	if updated.Status != RunStatusCancelled {
		t.Errorf("status = %q, want %q", updated.Status, RunStatusCancelled)
	}
}

func TestCancelRunWithoutRegisteredContext(t *testing.T) {
	runner, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	// Run is in 'planning' status (not terminal), but no context registered.
	// Cancel should still succeed — it updates the DB.
	if err := runner.CancelRun(run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	updated, _ := runRepo.GetByID(run.ID)
	if updated.Status != RunStatusCancelled {
		t.Errorf("status = %q, want %q", updated.Status, RunStatusCancelled)
	}
}

// --------------- Run status transitions via DB ---------------

func TestRunStatusTransitions(t *testing.T) {
	_, runRepo, _, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	// Initial status should be 'planning'.
	if run.Status != RunStatusPlanning {
		t.Errorf("initial status = %q, want %q", run.Status, RunStatusPlanning)
	}

	transitions := []string{
		RunStatusRunning,
		RunStatusAwaitingApproval,
		RunStatusRunning,
		RunStatusCompleted,
	}
	for _, status := range transitions {
		if err := runRepo.UpdateStatus(run.ID, status); err != nil {
			t.Fatalf("UpdateStatus(%q): %v", status, err)
		}
		updated, err := runRepo.GetByID(run.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if updated.Status != status {
			t.Errorf("status = %q, want %q", updated.Status, status)
		}
	}
}

func TestStepStatusTransitions(t *testing.T) {
	_, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)
	step := createTestStep(t, stepRepo, run.ID, StepTypeThink, 0)

	// Initial status should be 'pending'.
	if step.Status != StepStatusPending {
		t.Errorf("initial status = %q, want %q", step.Status, StepStatusPending)
	}

	transitions := []string{
		StepStatusRunning,
		StepStatusCompleted,
	}
	for _, status := range transitions {
		if err := stepRepo.UpdateStatus(step.ID, status); err != nil {
			t.Fatalf("UpdateStatus(%q): %v", status, err)
		}
		updated, err := stepRepo.GetByID(step.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if updated.Status != status {
			t.Errorf("status = %q, want %q", updated.Status, status)
		}
	}
}

func TestStepBatchCreate(t *testing.T) {
	_, runRepo, stepRepo, convoRepo := newTestRunner(t)
	convoID := createTestConversation(t, convoRepo)
	run := createTestRun(t, runRepo, convoID)

	steps := []models.AgentStep{
		{RunID: run.ID, StepIndex: 0, Type: StepTypeThink, Description: "Think"},
		{RunID: run.ID, StepIndex: 1, Type: StepTypeToolCall, Description: "Tool", ToolName: strPtr("search")},
		{RunID: run.ID, StepIndex: 2, Type: StepTypeApproval, Description: "Approve"},
		{RunID: run.ID, StepIndex: 3, Type: StepTypeMessage, Description: "Reply"},
	}
	if err := stepRepo.CreateBatch(steps); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	loaded, err := stepRepo.ListByRun(run.ID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(loaded) != 4 {
		t.Fatalf("len(loaded) = %d, want 4", len(loaded))
	}

	// Verify order.
	for i, s := range loaded {
		if s.StepIndex != i {
			t.Errorf("step %d: StepIndex = %d, want %d", i, s.StepIndex, i)
		}
	}

	// Verify types.
	expectedTypes := []string{StepTypeThink, StepTypeToolCall, StepTypeApproval, StepTypeMessage}
	for i, s := range loaded {
		if s.Type != expectedTypes[i] {
			t.Errorf("step %d: Type = %q, want %q", i, s.Type, expectedTypes[i])
		}
	}
}

// --------------- RunnerConfig ---------------

func TestRunnerConfigCustom(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	runner.config = RunnerConfig{
		MaxSteps:    5,
		MaxDuration: 1 * time.Minute,
	}
	if runner.config.MaxSteps != 5 {
		t.Errorf("MaxSteps = %d, want 5", runner.config.MaxSteps)
	}
	if runner.config.MaxDuration != 1*time.Minute {
		t.Errorf("MaxDuration = %v, want 1m", runner.config.MaxDuration)
	}
}

// --------------- Concurrent access ---------------

func TestConcurrentRegisterDeregister(t *testing.T) {
	runner, _, _, _ := newTestRunner(t)
	done := make(chan struct{})

	// Register and deregister concurrently.
	for i := 0; i < 50; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			runID := fmt.Sprintf("run-%d", id)
			_, cancel := context.WithCancel(context.Background())
			runner.registerRun(runID, cancel)
			runner.getRunContext(runID)
			runner.deregisterRun(runID)
			cancel()
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}
