package api

import (
	"os"
	"strings"
	"testing"
)

func TestMessageHandlerWiresOrderedGenericToolRuntime(t *testing.T) {
	source, err := os.ReadFile("message_handler.go")
	if err != nil {
		t.Fatalf("read message_handler.go: %v", err)
	}
	text := string(source)
	required := []string{
		"newChatToolExecution(h.toolExecutor, finalToolCalls)",
		"execution.genericRuntimeEligible()",
		"executeGenericChatToolRound(",
		"browser.WithProviderType(r.Context(), providerType)",
	}
	for _, marker := range required {
		if !strings.Contains(text, marker) {
			t.Fatalf("message handler is missing ordered-runtime marker %q", marker)
		}
	}
	if !strings.Contains(text, "for _, tc := range finalToolCalls") {
		t.Fatal("message handler no longer contains the browser-aware sequential fallback")
	}
}
