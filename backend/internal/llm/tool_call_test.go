package llm

import (
	"encoding/json"
	"testing"
)

func TestToolCallPreservesGeminiThoughtSignatureEnvelope(t *testing.T) {
	raw := []byte(`{
		"index": 0,
		"id": "function-call-1",
		"type": "function",
		"extra_content": {
			"google": {
				"thought_signature": "signed-reasoning-state"
			}
		},
		"function": {
			"name": "github_repo_inspect",
			"arguments": "{\"url\":\"https://github.com/owner/repo\"}"
		}
	}`)

	var call ToolCall
	if err := json.Unmarshal(raw, &call); err != nil {
		t.Fatalf("unmarshal tool call: %v", err)
	}
	if got := call.GeminiThoughtSignature(); got != "signed-reasoning-state" {
		t.Fatalf("signature = %q, want signed-reasoning-state", got)
	}

	encoded, err := json.Marshal(call)
	if err != nil {
		t.Fatalf("marshal tool call: %v", err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("unmarshal round trip: %v", err)
	}
	if _, exists := roundTrip["thought_signature"]; exists {
		t.Fatal("Gemini signature was incorrectly flattened to the tool-call root")
	}
	extra, ok := roundTrip["extra_content"].(map[string]any)
	if !ok {
		t.Fatalf("extra_content missing from round trip: %s", encoded)
	}
	google, ok := extra["google"].(map[string]any)
	if !ok || google["thought_signature"] != "signed-reasoning-state" {
		t.Fatalf("Gemini signature envelope was not preserved: %s", encoded)
	}
}

func TestMergeToolCallDeltaPreservesLateGeminiSignature(t *testing.T) {
	var accumulated ToolCall
	first := ToolCall{Index: 0, ID: "function-call-1", Type: "function"}
	first.Function.Name = "github_repo_inspect"
	first.Function.Arguments = `{"url":"https://github.com/owner`
	MergeToolCallDelta(&accumulated, first)

	second := ToolCall{
		Index: 0,
		ExtraContent: &ToolCallExtraContent{
			Google: &GoogleToolCallExtraContent{ThoughtSignature: "late-signature"},
		},
	}
	second.Function.Arguments = `/repo"}`
	MergeToolCallDelta(&accumulated, second)

	// A later arguments-only fragment must not erase the signature.
	third := ToolCall{Index: 0}
	MergeToolCallDelta(&accumulated, third)

	if accumulated.ID != "function-call-1" || accumulated.Function.Name != "github_repo_inspect" {
		t.Fatalf("tool identity was not preserved: %#v", accumulated)
	}
	if got := accumulated.Function.Arguments; got != `{"url":"https://github.com/owner/repo"}` {
		t.Fatalf("arguments = %q", got)
	}
	if got := accumulated.GeminiThoughtSignature(); got != "late-signature" {
		t.Fatalf("signature = %q, want late-signature", got)
	}
}

func TestGeminiThoughtSignatureSupportsLegacyRootField(t *testing.T) {
	call := ToolCall{ThoughtSignature: "legacy-signature"}
	if got := call.GeminiThoughtSignature(); got != "legacy-signature" {
		t.Fatalf("signature = %q, want legacy-signature", got)
	}
}

func TestGeminiToolLoopHistoryResendsSignatureWithToolResult(t *testing.T) {
	call := ToolCall{
		Index: 0,
		ID:    "function-call-1",
		Type:  "function",
		ExtraContent: &ToolCallExtraContent{
			Google: &GoogleToolCallExtraContent{ThoughtSignature: "required-signature"},
		},
	}
	call.Function.Name = "github_repo_inspect"
	call.Function.Arguments = `{"url":"https://github.com/owner/repo"}`

	payload := map[string]any{
		"model": "gemini-3.5-flash",
		"messages": []ChatMessage{
			{Role: "user", Content: "Review owner/repo"},
			{Role: "assistant", ToolCalls: []ToolCall{call}},
			{Role: "tool", Name: call.Function.Name, ToolCallID: call.ID, Content: `{"repo":"context"}`},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal second Gemini request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode second Gemini request: %v", err)
	}
	messages, ok := decoded["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("unexpected messages: %s", encoded)
	}
	assistant := messages[1].(map[string]any)
	toolCalls := assistant["tool_calls"].([]any)
	toolCall := toolCalls[0].(map[string]any)
	extra := toolCall["extra_content"].(map[string]any)
	google := extra["google"].(map[string]any)
	if google["thought_signature"] != "required-signature" {
		t.Fatalf("assistant tool call lost signature: %s", encoded)
	}
	toolResult := messages[2].(map[string]any)
	if toolResult["tool_call_id"] != "function-call-1" || toolResult["name"] != "github_repo_inspect" {
		t.Fatalf("tool result is not correlated with its call: %s", encoded)
	}
}
