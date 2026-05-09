package mcpclient

import (
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
