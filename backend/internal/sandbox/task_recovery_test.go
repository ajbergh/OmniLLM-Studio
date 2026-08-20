package sandbox

import (
	"context"
	"testing"
	"time"
)

func TestCleanupExpiredRuntimeAssociationsDestroysRecordedRuntimeBeforeReplay(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return now }
	owner, create, execReq := sandboxTaskFixture()
	if _, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryIdempotent); err != nil {
		t.Fatal(err)
	}
	claimed, attempt, err := queue.Claim(context.Background(), "worker-1", time.Second)
	if err != nil || claimed == nil || attempt == nil {
		t.Fatalf("claim: task=%#v attempt=%#v err=%v", claimed, attempt, err)
	}
	if err := queue.BindRuntimeAssociation(
		context.Background(),
		claimed.ID,
		"worker-1",
		claimed.LeaseToken,
		attempt.ID,
		"sbx_recorded",
		"runtime-recorded",
	); err != nil {
		t.Fatal(err)
	}

	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := queue.CleanupExpiredRuntimeAssociations(context.Background(), broker); err != nil {
		t.Fatal(err)
	}
	if runtime.destroyCount != 1 {
		t.Fatalf("destroy count = %d, want 1", runtime.destroyCount)
	}

	replayed, nextAttempt, err := queue.Claim(context.Background(), "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if replayed == nil || nextAttempt == nil || replayed.ID != claimed.ID {
		t.Fatalf("expected cleaned idempotent task to become claimable: task=%#v attempt=%#v", replayed, nextAttempt)
	}
	if replayed.AttemptCount != 2 || nextAttempt.ExecutionID == attempt.ExecutionID {
		t.Fatalf("replay did not create a fresh attempt: task=%#v first=%#v next=%#v", replayed, attempt, nextAttempt)
	}
}
