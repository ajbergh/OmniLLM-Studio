package sandbox

import (
	"context"
	"testing"
)

func TestBrokerAcceptsCodeModeAndRejectsAmbiguousExecution(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := broker.Exec(context.Background(), owner, session.ID, ExecRequest{Language: "python", Code: "print(1)"}); err != nil {
		t.Fatalf("code-mode Exec() error = %v", err)
	}
	if runtime.execCount != 1 {
		t.Fatalf("runtime exec count = %d, want 1", runtime.execCount)
	}

	for _, request := range []ExecRequest{
		{},
		{Language: "python", Code: "print(1)", Command: "echo"},
		{Language: "python"},
		{Code: "print(1)"},
	} {
		if _, err := broker.Exec(context.Background(), owner, session.ID, request); err == nil {
			t.Fatalf("expected ambiguous/incomplete request %#v to be rejected", request)
		}
	}
	if runtime.execCount != 1 {
		t.Fatalf("runtime received invalid execution requests; count = %d", runtime.execCount)
	}
}
