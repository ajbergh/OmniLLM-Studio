package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newSandboxTaskQueueForTest(t *testing.T) *SandboxTaskQueue {
	t.Helper()
	// Mirror the production SQLite concurrency settings that matter to the
	// durable task worker. The worker claims/completes on background goroutines
	// while tests poll through the same sql.DB; without WAL + a busy timeout the
	// raw modernc connection can return SQLITE_BUSY even though production
	// db.Open waits for the writer and permits concurrent readers.
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "sandbox-task-test.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = db.Close() })
	queue, err := NewSandboxTaskQueue(db)
	if err != nil {
		t.Fatal(err)
	}
	return queue
}

func sandboxTaskFixture() (OwnerScope, CreateRequest, ExecRequest) {
	return OwnerScope{UserID: "user-1", WorkspaceID: "workspace-1", ConversationID: "conversation-1", TaskID: "scheduled-1"},
		CreateRequest{Network: NetworkPolicy{Mode: NetworkNone}, TTLSeconds: 300},
		ExecRequest{Language: "shell", Code: "echo hello", TimeoutMS: 5000}
}

func TestSandboxTaskQueueDefaultsToNoReplayAndPreallocatesAttemptExecutionID(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	owner, create, execReq := sandboxTaskFixture()
	execReq.ExecutionID = "exec_caller_should_not_survive"
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, "")
	if err != nil {
		t.Fatal(err)
	}
	if task.RetryPolicy != SandboxRetryNever || task.Status != SandboxTaskQueued {
		t.Fatalf("unexpected task policy/status: %#v", task)
	}
	if task.Exec.ExecutionID != "" {
		t.Fatalf("persisted template execution id = %q", task.Exec.ExecutionID)
	}
	claimed, attempt, err := queue.Claim(context.Background(), "worker-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || attempt == nil {
		t.Fatal("expected task claim")
	}
	if claimed.ExecutionID == "" || claimed.ExecutionID != attempt.ExecutionID {
		t.Fatalf("execution id not durably preallocated: task=%q attempt=%q", claimed.ExecutionID, attempt.ExecutionID)
	}
	if claimed.LeaseToken == "" || claimed.LeaseOwner != "worker-1" || claimed.AttemptCount != 1 {
		t.Fatalf("unexpected claim state: %#v", claimed)
	}
}

func TestSandboxTaskQueueExpiredNonRetryableLeaseFailsClosed(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return now }
	owner, create, execReq := sandboxTaskFixture()
	first, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever)
	if err != nil {
		t.Fatal(err)
	}
	claimed, _, err := queue.Claim(context.Background(), "worker-1", time.Second)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %#v %v", claimed, err)
	}
	now = now.Add(2 * time.Second)
	second, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := queue.Claim(context.Background(), "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != second.ID {
		t.Fatalf("expired non-retryable task was replayed; next=%#v want=%s", next, second.ID)
	}
	stored, err := queue.Get(context.Background(), first.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != SandboxTaskFailed || stored.CompletedAt == nil {
		t.Fatalf("expired non-retryable task state = %#v", stored)
	}
}

func TestSandboxTaskQueueExpiredIdempotentLeaseGetsNewAttempt(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return now }
	owner, create, execReq := sandboxTaskFixture()
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryIdempotent)
	if err != nil {
		t.Fatal(err)
	}
	first, firstAttempt, err := queue.Claim(context.Background(), "worker-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	second, secondAttempt, err := queue.Claim(context.Background(), "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || second.ID != task.ID || second.AttemptCount != 2 {
		t.Fatalf("idempotent task did not retry as new attempt: %#v", second)
	}
	if first.ExecutionID == second.ExecutionID || firstAttempt.ID == secondAttempt.ID {
		t.Fatalf("retry reused prior execution identity: first=%#v second=%#v", firstAttempt, secondAttempt)
	}
}

func TestSandboxTaskQueueLeaseTokenProtectsMutationAndCompletion(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	owner, create, execReq := sandboxTaskFixture()
	if _, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever); err != nil {
		t.Fatal(err)
	}
	task, attempt, err := queue.Claim(context.Background(), "worker-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.BindRuntime(context.Background(), task.ID, "worker-1", "wrong-token", attempt.ID, "rt_1"); err == nil {
		t.Fatal("runtime binding accepted wrong lease token")
	}
	if err := queue.BindRuntime(context.Background(), task.ID, "worker-1", task.LeaseToken, attempt.ID, "rt_1"); err != nil {
		t.Fatal(err)
	}
	if err := queue.Complete(context.Background(), task.ID, "worker-1", "wrong-token", attempt.ID, &ExecResult{ExecutionID: task.ExecutionID}, nil); err == nil {
		t.Fatal("completion accepted wrong lease token")
	}
	result := &ExecResult{ExecutionID: task.ExecutionID, Stdout: "ok", ExitCode: 0}
	if err := queue.Complete(context.Background(), task.ID, "worker-1", task.LeaseToken, attempt.ID, result, nil); err != nil {
		t.Fatal(err)
	}
	stored, err := queue.Get(context.Background(), task.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != SandboxTaskSucceeded || stored.CompletedAt == nil || len(stored.Result) == 0 {
		t.Fatalf("unexpected completed task: %#v", stored)
	}
}

func TestSandboxTaskQueueOwnerScopeOnRead(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	owner, create, execReq := sandboxTaskFixture()
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Get(context.Background(), task.ID, OwnerScope{UserID: "other"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-user read err = %v", err)
	}
	if _, err := queue.Get(context.Background(), task.ID, OwnerScope{UserID: owner.UserID, WorkspaceID: "other-workspace"}); err == nil {
		t.Fatal("cross-workspace read unexpectedly succeeded")
	}
}
