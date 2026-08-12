package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type toolSandboxRuntime struct {
	createCount  int
	execCount    int
	destroyCount int
	lastExec     sandbox.ExecRequest
	stdout       string
}

func (r *toolSandboxRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{
		Name:                 "tool-test-runtime",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		ProcessTreeIsolation: true,
	}
}
func (r *toolSandboxRuntime) Create(context.Context, sandbox.RuntimeCreateRequest) (string, error) {
	r.createCount++
	return "runtime-session", nil
}
func (r *toolSandboxRuntime) Exec(_ context.Context, _ string, request sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	r.execCount++
	r.lastExec = request
	stdout := r.stdout
	if stdout == "" {
		stdout = "ok"
	}
	return &sandbox.ExecResult{ExecutionID: "exec-1", Stdout: stdout, ExitCode: 0}, nil
}
func (r *toolSandboxRuntime) Cancel(context.Context, string, string) error { return nil }
func (r *toolSandboxRuntime) Status(context.Context, string) (*sandbox.Status, error) {
	return &sandbox.Status{State: "ready"}, nil
}
func (r *toolSandboxRuntime) Destroy(context.Context, string) error {
	r.destroyCount++
	return nil
}

func TestCodeSandboxToolIssuesAndReusesOwnerBoundSession(t *testing.T) {
	runtime := &toolSandboxRuntime{}
	broker, err := sandbox.NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	tool := NewCodeSandboxTool(broker)
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{
		UserID:         "user-1",
		WorkspaceID:    "workspace-1",
		ConversationID: "conversation-1",
	})

	result, err := tool.Execute(ctx, json.RawMessage(`{"language":"python","code":"print('ok')"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	sessionID, _ := result.Metadata["session_id"].(string)
	if !strings.HasPrefix(sessionID, "sbx_") {
		t.Fatalf("session_id = %q", sessionID)
	}
	if runtime.createCount != 1 || runtime.execCount != 1 {
		t.Fatalf("runtime counts create=%d exec=%d", runtime.createCount, runtime.execCount)
	}

	args, _ := json.Marshal(codeSandboxArgs{Language: "python", Code: "print('again')", SessionID: sessionID})
	if _, err := tool.Execute(ctx, args); err != nil {
		t.Fatalf("reuse Execute() error = %v", err)
	}
	if runtime.createCount != 1 || runtime.execCount != 2 {
		t.Fatalf("reuse counts create=%d exec=%d", runtime.createCount, runtime.execCount)
	}

	otherCtx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-2"})
	if _, err := tool.Execute(otherCtx, args); err == nil || !strings.Contains(err.Error(), "owned") {
		t.Fatalf("cross-owner Execute() error = %v", err)
	}
	if runtime.execCount != 2 {
		t.Fatalf("cross-owner request reached runtime; exec=%d", runtime.execCount)
	}
}

func TestCodeSandboxToolRejectsNonBrokerSessionID(t *testing.T) {
	runtime := &toolSandboxRuntime{}
	broker, _ := sandbox.NewBroker(runtime)
	tool := NewCodeSandboxTool(broker)
	if err := tool.Validate(json.RawMessage(`{"language":"python","code":"print(1)","session_id":"caller-chosen"}`)); err == nil {
		t.Fatal("expected caller-chosen session id to be rejected")
	}
}

func TestPythonAnalysisToolUsesLazyDefaultBrokerAndDestroysSession(t *testing.T) {
	t.Setenv("OMNILLM_CODE_EXEC_ENABLED", "true")
	sandbox.SetDefaultBroker(nil)
	t.Cleanup(func() { sandbox.SetDefaultBroker(nil) })

	tool := NewPythonAnalysisTool()
	if tool.Definition().Enabled {
		t.Fatal("python_analysis should remain disabled before Broker composition")
	}

	runtime := &toolSandboxRuntime{stdout: `{"stdout":"done\n","result":42,"exit_code":0}`}
	broker, err := sandbox.NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	sandbox.SetDefaultBroker(broker)
	if !tool.Definition().Enabled {
		t.Fatal("python_analysis should become enabled after Broker composition")
	}

	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-1"})
	result, err := tool.Execute(ctx, json.RawMessage(`{"code":"result = 42"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Content, "done") || !strings.Contains(result.Content, "42") {
		t.Fatalf("result content = %q", result.Content)
	}
	if runtime.createCount != 1 || runtime.execCount != 1 || runtime.destroyCount != 1 {
		t.Fatalf("runtime counts create=%d exec=%d destroy=%d", runtime.createCount, runtime.execCount, runtime.destroyCount)
	}
	if runtime.lastExec.Language != "python" || !strings.Contains(runtime.lastExec.Code, "blocked_calls") {
		t.Fatalf("restricted wrapper missing from exec request: %#v", runtime.lastExec)
	}
}
