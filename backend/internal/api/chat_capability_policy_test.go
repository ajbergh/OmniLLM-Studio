package api

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type chatCapabilityTestTool struct{ name string }

func (t chatCapabilityTestTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        t.name,
		Description: "capability policy test tool",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Enabled:     true,
		ReadOnly:    true,
	}
}

func (t chatCapabilityTestTool) Validate(json.RawMessage) error { return nil }

func (t chatCapabilityTestTool) Execute(context.Context, json.RawMessage) (*tools.ToolResult, error) {
	return &tools.ToolResult{Content: "ok"}, nil
}

func newCapabilityPolicyHandler(policy *string) *MessageHandler {
	registry := tools.NewRegistry()
	registry.MustRegister(chatCapabilityTestTool{name: "sports_lookup"})
	executor := tools.NewExecutor(registry, func(name string) string {
		if name == "sports_lookup" {
			return *policy
		}
		return "allow"
	}, 0)
	return &MessageHandler{toolRegistry: registry, toolExecutor: executor}
}

func TestChatPreflightOnlyExecutesAllow(t *testing.T) {
	policy := "allow"
	h := newCapabilityPolicyHandler(&policy)
	if !h.chatPreflightAllowed("sports_lookup") {
		t.Fatal("allow policy should permit deterministic preflight")
	}

	policy = "ask"
	if h.chatPreflightAllowed("sports_lookup") {
		t.Fatal("ask policy must fall through to the approval-aware tool loop")
	}

	policy = "deny"
	if h.chatPreflightAllowed("sports_lookup") {
		t.Fatal("deny policy must block deterministic preflight")
	}
}

func TestCapabilityDirectiveTracksDiscoveryPolicy(t *testing.T) {
	policy := "allow"
	h := newCapabilityPolicyHandler(&policy)
	convo := &models.Conversation{}

	req := h.buildLLMRequest(convo, nil, nil)
	if len(req.Messages) == 0 || !strings.Contains(req.Messages[0].Content, sportsLookupSystemDirective) {
		t.Fatal("sports directive should be advertised while sports_lookup is available")
	}

	policy = "deny"
	req = h.buildLLMRequest(convo, nil, nil)
	if len(req.Messages) == 0 || strings.Contains(req.Messages[0].Content, sportsLookupSystemDirective) {
		t.Fatal("sports directive must disappear when sports_lookup is denied")
	}
}

func TestLegacyChatCapabilityPathsUseUnifiedPolicyGate(t *testing.T) {
	data, err := os.ReadFile("message_handler.go")
	if err != nil {
		t.Fatalf("read message_handler.go: %v", err)
	}
	source := string(data)
	checks := []string{
		`h.chatPreflightAllowed("fetch_url_context")`,
		`h.chatPreflightAllowed("sports_lookup")`,
		`h.chatPreflightAllowed("file_search")`,
		`h.chatPreflightAllowed("web_search")`,
		`h.chatPreflightAllowed("image_generate")`,
		`h.chatPreflightAllowed("generate_word_doc")`,
		`h.chatPreflightAllowed("artifact_generate")`,
	}
	for _, check := range checks {
		if !strings.Contains(source, check) {
			t.Fatalf("message_handler.go missing unified capability gate %s", check)
		}
	}
	if strings.Contains(composeSystemPrompt(""), sportsLookupSystemDirective) {
		t.Fatal("composeSystemPrompt must not advertise sports independently of runtime policy")
	}
}
