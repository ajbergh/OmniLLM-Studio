package mcpclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeToolResultTextContent(t *testing.T) {
	result := NormalizeToolResult(&CallToolResult{
		Content: []map[string]interface{}{
			{"type": "text", "text": "hello"},
			{"type": "text", "text": "world"},
		},
	})

	if result.IsError {
		t.Fatal("expected non-error result")
	}
	if result.Content != "hello\nworld" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestNormalizeToolResultStructuredFallback(t *testing.T) {
	result := NormalizeToolResult(&CallToolResult{
		StructuredContent: map[string]interface{}{"ok": true},
	})

	if result.Content != `{"ok":true}` {
		t.Fatalf("unexpected structured fallback: %q", result.Content)
	}
}

func TestNormalizeToolResultTruncatesLargeContent(t *testing.T) {
	result := NormalizeToolResult(&CallToolResult{
		Content: []map[string]interface{}{
			{"type": "text", "text": strings.Repeat("a", maxToolContentBytes+10)},
		},
	})

	if len(result.Content) <= maxToolContentBytes {
		t.Fatalf("expected truncation marker beyond content cap")
	}
	if !strings.Contains(result.Content, "[Truncated: 10 bytes]") {
		t.Fatalf("missing truncation marker: %q", result.Content[len(result.Content)-80:])
	}
}

func TestToolAdapterTreatsUnannotatedMCPToolAsSideEffecting(t *testing.T) {
	adapter := NewToolAdapter("server", "Server", "mcp_server_write", Tool{
		Name:        "write",
		Description: "Potentially mutating MCP tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, nil, nil)
	def := adapter.Definition()
	if def.ReadOnly {
		t.Fatal("unannotated MCP tool must not default to read-only")
	}
	if !def.SideEffecting {
		t.Fatal("unannotated MCP tool must be treated as potentially side-effecting")
	}
	if def.SupportsParallel {
		t.Fatal("unannotated MCP tool must not default to parallel execution")
	}
}

func TestToolAdapterHonorsMCPReadOnlyHint(t *testing.T) {
	adapter := NewToolAdapter("server", "Server", "mcp_server_read", Tool{
		Name:        "read",
		Description: "Read-only MCP tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Annotations: json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false}`),
	}, nil, nil)
	def := adapter.Definition()
	if !def.ReadOnly {
		t.Fatal("readOnlyHint=true should mark MCP tool read-only")
	}
	if def.SideEffecting {
		t.Fatal("readOnlyHint=true should not be side-effecting")
	}
	if !def.SupportsParallel {
		t.Fatal("read-only MCP tool should be eligible for parallel execution")
	}
}
