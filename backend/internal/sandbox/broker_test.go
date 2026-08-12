package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRuntime struct {
	capabilities RuntimeCapabilities
	created      RuntimeCreateRequest
	execCount    int
	destroyCount int
}

func (f *fakeRuntime) Capabilities() RuntimeCapabilities { return f.capabilities }

func (f *fakeRuntime) Create(_ context.Context, request RuntimeCreateRequest) (string, error) {
	f.created = request
	return "runtime-session", nil
}

func (f *fakeRuntime) Exec(_ context.Context, _ string, request ExecRequest) (*ExecResult, error) {
	f.execCount++
	return &ExecResult{ExecutionID: "exec-1", Stdout: request.Command, ExitCode: 0}, nil
}

func (f *fakeRuntime) Cancel(context.Context, string, string) error { return nil }

func (f *fakeRuntime) Status(context.Context, string) (*Status, error) {
	return &Status{State: "ready"}, nil
}

func (f *fakeRuntime) Destroy(context.Context, string) error {
	f.destroyCount++
	return nil
}

func TestBrokerIssuesSessionAndBindsOwner(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{
		Name:                 "fake",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		ProcessTreeIsolation: true,
	}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1", WorkspaceID: "workspace-1", ConversationID: "conversation-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{
		Mounts: []WorkspaceMount{{WorkspaceID: "workspace-1", Mode: MountReadWriteNoDelete}},
		Requirements: RuntimeRequirements{
			OSIsolation:          true,
			FilesystemIsolation:  true,
			NetworkIsolation:     true,
			ProcessTreeIsolation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(session.ID, "sbx_") || session.ID == "sbx_" {
		t.Fatalf("session id = %q, want application-issued sbx_ id", session.ID)
	}
	if runtime.created.SessionID != session.ID || !runtime.created.Owner.Equal(owner) {
		t.Fatalf("runtime create request = %#v", runtime.created)
	}
	if runtime.created.Spec.Network.Mode != NetworkNone {
		t.Fatalf("default network mode = %q, want none", runtime.created.Spec.Network.Mode)
	}

	if _, err := broker.Exec(context.Background(), OwnerScope{UserID: "other-user"}, session.ID, ExecRequest{Command: "echo"}); err == nil {
		t.Fatal("expected cross-owner execution to be rejected")
	}
	if runtime.execCount != 0 {
		t.Fatalf("runtime exec count = %d after rejected owner", runtime.execCount)
	}

	result, err := broker.Exec(context.Background(), owner, session.ID, ExecRequest{Command: "echo"})
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if result.Stdout != "echo" || runtime.execCount != 1 {
		t.Fatalf("unexpected exec result %#v count=%d", result, runtime.execCount)
	}
}

func TestBrokerFailsClosedOnMissingRuntimeCapability(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "compat-host"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	_, err = broker.Create(context.Background(), OwnerScope{UserID: "user-1"}, CreateRequest{
		Requirements: RuntimeRequirements{OSIsolation: true, NetworkIsolation: true},
	})
	if err == nil || !strings.Contains(err.Error(), "os_isolation") || !strings.Contains(err.Error(), "network_isolation") {
		t.Fatalf("Create() error = %v, want missing capability failure", err)
	}
}

func TestBrokerRejectsExpiredSessionBeforeRuntimeExecution(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	broker.now = func() time.Time { return now }
	owner := OwnerScope{UserID: "user-1"}
	session, err := broker.Create(context.Background(), owner, CreateRequest{TTLSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, err := broker.Exec(context.Background(), owner, session.ID, ExecRequest{Command: "echo"}); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Exec() error = %v, want expired session", err)
	}
	if runtime.execCount != 0 {
		t.Fatalf("runtime exec count = %d, want 0", runtime.execCount)
	}
}

func TestBrokerRejectsEmptyOwnerAndUnsafeMounts(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Create(context.Background(), OwnerScope{}, CreateRequest{}); err == nil {
		t.Fatal("expected empty owner to be rejected")
	}
	if _, err := broker.Create(context.Background(), OwnerScope{UserID: "user-1"}, CreateRequest{
		Mounts: []WorkspaceMount{{WorkspaceID: "workspace-1", Mode: "host_root"}},
	}); err == nil {
		t.Fatal("expected unsupported mount mode to be rejected")
	}
}
