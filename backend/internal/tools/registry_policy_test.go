package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type registryPolicyTestTool struct {
	name string
}

func (t registryPolicyTestTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        t.name,
		Description: "Registry policy test tool " + t.name,
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Category:    "registry-policy-test",
		Enabled:     true,
		ReadOnly:    true,
	}
}

func (t registryPolicyTestTool) Validate(json.RawMessage) error { return nil }
func (t registryPolicyTestTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	return &ToolResult{Content: "ok"}, nil
}

func TestRegistryDiscoveryExcludesHardDeniedTools(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(registryPolicyTestTool{name: "allowed_registry_tool"})
	registry.MustRegister(registryPolicyTestTool{name: "denied_registry_tool"})
	registry.MustRegister(registryPolicyTestTool{name: "ask_registry_tool"})

	policies := map[string]string{
		"allowed_registry_tool": "allow",
		"denied_registry_tool":  "deny",
		"ask_registry_tool":     "ask",
	}
	_ = NewExecutor(registry, func(name string) string { return policies[name] }, 0)

	if !registry.IsAvailable("allowed_registry_tool") {
		t.Fatal("allowed tool should be available")
	}
	if registry.IsAvailable("denied_registry_tool") {
		t.Fatal("denied tool should not be available")
	}
	if !registry.IsAvailable("ask_registry_tool") {
		t.Fatal("ask tool should remain discoverable so approval can be requested")
	}

	enabled := registry.ListEnabled()
	if containsToolDefinition(enabled, "denied_registry_tool") {
		t.Fatal("ListEnabled advertised a denied tool")
	}
	if !containsToolDefinition(enabled, "ask_registry_tool") {
		t.Fatal("ListEnabled should advertise ask tools")
	}

	all := registry.List()
	if !containsToolDefinition(all, "denied_registry_tool") {
		t.Fatal("List should retain denied tools for Settings and diagnostics")
	}
}

func TestRegistryDiscoveryTracksPolicyChangesWithoutReregistration(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(registryPolicyTestTool{name: "mutable_registry_tool"})
	policy := "allow"
	_ = NewExecutor(registry, func(name string) string {
		if name == "mutable_registry_tool" {
			return policy
		}
		return "allow"
	}, 0)

	if !registry.IsAvailable("mutable_registry_tool") {
		t.Fatal("tool should initially be available")
	}
	policy = "deny"
	if registry.IsAvailable("mutable_registry_tool") {
		t.Fatal("tool should become unavailable immediately after policy changes to deny")
	}
	policy = "ask"
	if !registry.IsAvailable("mutable_registry_tool") {
		t.Fatal("ask policy should make tool discoverable again")
	}
}

func containsToolDefinition(definitions []ToolDefinition, name string) bool {
	for _, definition := range definitions {
		if definition.Name == name {
			return true
		}
	}
	return false
}
