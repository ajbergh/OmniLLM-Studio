package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type terminalRuntime struct {
	created sandbox.RuntimeCreateRequest
	exec    sandbox.ExecRequest
	destroy int
}

func (r *terminalRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{Name: "terminal-test", OSIsolation: true, FilesystemIsolation: true, NetworkIsolation: true, ProcessTreeIsolation: true}
}
func (r *terminalRuntime) Create(_ context.Context, request sandbox.RuntimeCreateRequest) (string, error) { r.created = request; return "runtime-1", nil }
func (r *terminalRuntime) Exec(_ context.Context, _ string, request sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	r.exec = request
	return &sandbox.ExecResult{ExecutionID: "exec-1", Stdout: "PASS\n", ExitCode: 0}, nil
}
func (r *terminalRuntime) Cancel(context.Context, string, string) error { return nil }
func (r *terminalRuntime) Status(context.Context, string) (*sandbox.Status, error) { return &sandbox.Status{State: "ready"}, nil }
func (r *terminalRuntime) Destroy(context.Context, string) error { r.destroy++; return nil }

func TestTerminalExecUsesReadOnlyWorkspaceMount(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "terminal.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = db.Close(database) })
	workspaceRegistry, err := sandbox.NewWorkspaceRegistry(database)
	if err != nil { t.Fatal(err) }
	sandbox.SetDefaultWorkspaceRegistry(workspaceRegistry)
	t.Cleanup(func() { sandbox.SetDefaultWorkspaceRegistry(nil) })
	root := t.TempDir()
	if _, err := workspaceRegistry.Register("user-1", "repo", root, sandbox.MountReadWrite); err != nil { t.Fatal(err) }

	runtime := &terminalRuntime{}
	broker, err := sandbox.NewBroker(runtime)
	if err != nil { t.Fatal(err) }
	sandbox.SetDefaultBroker(broker)
	t.Cleanup(func() { sandbox.SetDefaultBroker(nil) })

	tool := NewTerminalExecTool()
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-1", ConversationID: "conversation-1"})
	result, err := tool.Execute(ctx, json.RawMessage(`{"command":"go","args":["test","./..."],"workspace_id":"repo","directory":"backend"}`))
	if err != nil { t.Fatal(err) }
	if result.Content != "PASS\n" { t.Fatalf("result = %q", result.Content) }
	if len(runtime.created.ResolvedMounts) != 1 || runtime.created.ResolvedMounts[0].Mode != sandbox.MountReadOnly {
		t.Fatalf("resolved mounts = %#v", runtime.created.ResolvedMounts)
	}
	if runtime.created.ResolvedMounts[0].SourcePath != root {
		t.Fatalf("runtime mount source = %q, want %q", runtime.created.ResolvedMounts[0].SourcePath, root)
	}
	if runtime.exec.Command != "go" || len(runtime.exec.Args) != 2 || runtime.exec.Directory != "backend" {
		t.Fatalf("exec request = %#v", runtime.exec)
	}
	if runtime.destroy != 1 { t.Fatalf("destroy count = %d", runtime.destroy) }
}

func TestTerminalExecCannotUseAnotherUsersWorkspace(t *testing.T) {
	database, _ := db.Open(filepath.Join(t.TempDir(), "terminal.db"))
	t.Cleanup(func() { _ = db.Close(database) })
	registry, _ := sandbox.NewWorkspaceRegistry(database)
	sandbox.SetDefaultWorkspaceRegistry(registry)
	t.Cleanup(func() { sandbox.SetDefaultWorkspaceRegistry(nil) })
	_, _ = registry.Register("user-1", "repo", t.TempDir(), sandbox.MountReadWrite)
	runtime := &terminalRuntime{}
	broker, _ := sandbox.NewBroker(runtime)
	sandbox.SetDefaultBroker(broker)
	t.Cleanup(func() { sandbox.SetDefaultBroker(nil) })

	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-2"})
	if _, err := NewTerminalExecTool().Execute(ctx, json.RawMessage(`{"command":"ls","workspace_id":"repo"}`)); err == nil {
		t.Fatal("expected cross-owner terminal workspace to be rejected")
	}
}

func TestTerminalExecDefinitionIsHighRiskAndNoNetwork(t *testing.T) {
	def := NewTerminalExecTool().Definition().Normalized()
	if def.Risk != RiskHigh || !def.SideEffecting || def.RequiresNetwork {
		t.Fatalf("terminal definition = %#v", def)
	}
}
