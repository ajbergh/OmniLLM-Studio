package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestBrokerAllocatesAndPreservesExecutionID(t *testing.T) {
	runtime := &fakeRuntime{}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := broker.Exec(context.Background(), owner, session.ID, ExecRequest{Command: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecutionID(result.ExecutionID); err != nil {
		t.Fatalf("Broker-generated execution id %q: %v", result.ExecutionID, err)
	}
	if runtime.lastExec.ExecutionID != result.ExecutionID {
		t.Fatalf("runtime request execution id = %q, result = %q", runtime.lastExec.ExecutionID, result.ExecutionID)
	}

	callerKnown := NewExecutionID()
	result, err = broker.Exec(context.Background(), owner, session.ID, ExecRequest{ExecutionID: callerKnown, Command: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionID != callerKnown || runtime.lastExec.ExecutionID != callerKnown {
		t.Fatalf("caller-known execution id was not preserved: request=%q result=%q", runtime.lastExec.ExecutionID, result.ExecutionID)
	}
}

func TestBrokerRejectsInvalidExecutionIDBeforeRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = broker.Exec(context.Background(), owner, session.ID, ExecRequest{ExecutionID: "exec-not-canonical", Command: "echo"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "execution id") {
		t.Fatalf("invalid execution id error = %v", err)
	}
	if runtime.execCount != 0 {
		t.Fatalf("runtime Exec called %d times for invalid execution id", runtime.execCount)
	}
}

type mismatchedExecutionRuntime struct{ *fakeRuntime }

func (r *mismatchedExecutionRuntime) Exec(_ context.Context, _ string, request ExecRequest) (*ExecResult, error) {
	r.execCount++
	r.lastExec = request
	return &ExecResult{ExecutionID: NewExecutionID(), ExitCode: 0}, nil
}

func TestBrokerFailsClosedOnRuntimeExecutionIDMismatch(t *testing.T) {
	runtime := &mismatchedExecutionRuntime{fakeRuntime: &fakeRuntime{}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}

	callerKnown := NewExecutionID()
	_, err = broker.Exec(context.Background(), owner, session.ID, ExecRequest{ExecutionID: callerKnown, Command: "echo"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "mismatched execution id") {
		t.Fatalf("runtime mismatch error = %v", err)
	}
}
