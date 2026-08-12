package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type registrySandboxRuntime struct{}

func (registrySandboxRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{
		Name:                 "registry-test",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		ProcessTreeIsolation: true,
	}
}

func (registrySandboxRuntime) Create(context.Context, sandbox.RuntimeCreateRequest) (string, error) {
	return "runtime-session", nil
}

func (registrySandboxRuntime) Exec(context.Context, string, sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	return &sandbox.ExecResult{ExecutionID: "exec-1"}, nil
}

func (registrySandboxRuntime) Cancel(context.Context, string, string) error { return nil }
func (registrySandboxRuntime) Status(context.Context, string) (*sandbox.Status, error) {
	return &sandbox.Status{State: "ready"}, nil
}
func (registrySandboxRuntime) Destroy(context.Context, string) error { return nil }

func TestRegistrySandboxToolsFollowBrokerLifecycle(t *testing.T) {
	t.Setenv("OMNILLM_CODE_EXEC_ENABLED", "true")
	sandbox.SetDefaultBroker(nil)
	t.Cleanup(func() { sandbox.SetDefaultBroker(nil) })

	registry := NewRegistry()
	for _, name := range []string{"python_analysis", "terminal_exec", "sandbox_network_grant"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("sandbox tool %q is not registered", name)
		}
		if registry.IsAvailable(name) {
			t.Fatalf("sandbox tool %q is available before Broker composition", name)
		}
	}

	broker, err := sandbox.NewBroker(registrySandboxRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	sandbox.SetDefaultBroker(broker)

	for _, name := range []string{"python_analysis", "terminal_exec", "sandbox_network_grant"} {
		if !registry.IsAvailable(name) {
			t.Fatalf("sandbox tool %q did not become available after Broker composition", name)
		}
		if got := EffectivePolicy(mustToolDefinition(t, registry, name), ""); got != "ask" {
			t.Fatalf("default policy for %q = %q, want ask", name, got)
		}
	}
}

func mustToolDefinition(t *testing.T, registry *Registry, name string) ToolDefinition {
	t.Helper()
	tool, ok := registry.Get(name)
	if !ok {
		t.Fatalf("tool %q is not registered", name)
	}
	return tool.Definition().Normalized()
}
