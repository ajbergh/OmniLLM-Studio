package llm

import (
	"net/http"
	"testing"
)

func TestNormalizeToolCallsProviderMatrix(t *testing.T) {
	providers := []string{"openai", "anthropic", "gemini", "openrouter", "ollama", "custom-openai"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			call := ToolCall{}
			call.Function.Name = " calculator "
			call.Function.Arguments = `{"expression":"2+2"}`

			got := NormalizeToolCalls(provider, []ToolCall{call})
			if len(got) != 1 {
				t.Fatalf("NormalizeToolCalls() returned %d calls, want 1", len(got))
			}
			if got[0].ID == "" {
				t.Fatal("normalized call ID is empty")
			}
			if got[0].Type != "function" {
				t.Fatalf("normalized type = %q, want function", got[0].Type)
			}
			if got[0].Function.Name != "calculator" {
				t.Fatalf("normalized name = %q, want calculator", got[0].Function.Name)
			}
			if got[0].Index != 0 {
				t.Fatalf("normalized index = %d, want 0", got[0].Index)
			}
		})
	}
}

func TestNormalizeToolCallGeneratesStableMissingID(t *testing.T) {
	call := ToolCall{}
	call.Function.Name = "weather"
	call.Function.Arguments = `{"location":"Madison, WI"}`

	first := NormalizeToolCall("gemini", call, 2)
	second := NormalizeToolCall("gemini", call, 2)
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("stable IDs differ: %q vs %q", first.ID, second.ID)
	}
}

func TestNormalizeToolCallContainsMalformedArguments(t *testing.T) {
	call := ToolCall{}
	call.Function.Name = "calculator"
	call.Function.Arguments = `{"expression":`

	got := NormalizeToolCall("openrouter", call, 0)
	if got.Function.Arguments == call.Function.Arguments {
		t.Fatal("malformed arguments were not contained")
	}
	if got.Function.Arguments == "{}" {
		t.Fatal("malformed provider payload should remain observable")
	}
}

func TestIsRetryableProviderStatus(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusConflict, 425, http.StatusTooManyRequests, 500, 502, 503, 504} {
		if !IsRetryableProviderStatus(status) {
			t.Fatalf("status %d should be retryable", status)
		}
	}
	for _, status := range []int{400, 401, 403, 404, 422} {
		if IsRetryableProviderStatus(status) {
			t.Fatalf("status %d should not be retryable", status)
		}
	}
}

func TestProviderRequestID(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Request-ID", "req_123")
	if got := ProviderRequestID(headers); got != "req_123" {
		t.Fatalf("ProviderRequestID() = %q, want req_123", got)
	}
}
