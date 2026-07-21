package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
	if !json.Valid([]byte(got.Function.Arguments)) {
		t.Fatalf("contained provider arguments are not valid JSON: %q", got.Function.Arguments)
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

func TestDoProviderRequestWithRetryRetriesBeforeBodyConsumption(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, []byte(`{"hello":"world"}`)) {
			t.Fatalf("request body = %q", string(body))
		}
		if r.Header.Get("X-OmniLLM-Request-ID") != "llm_test" {
			t.Fatalf("request correlation header = %q", r.Header.Get("X-OmniLLM-Request-ID"))
		}
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Request-ID", "upstream_123")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	body := []byte(`{"hello":"world"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-OmniLLM-Request-ID", "llm_test")

	resp, gotAttempts, err := doProviderRequestWithRetry(context.Background(), server.Client(), req, body, "openai", "llm_test")
	if err != nil {
		t.Fatalf("doProviderRequestWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if gotAttempts != 2 || attempts.Load() != 2 {
		t.Fatalf("attempts = %d server=%d, want 2", gotAttempts, attempts.Load())
	}
	if got := ProviderRequestID(resp.Header); got != "upstream_123" {
		t.Fatalf("upstream request ID = %q, want upstream_123", got)
	}
}

func TestDoProviderRequestWithRetryDoesNotRetryPermanentStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	body := []byte(`{}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, gotAttempts, err := doProviderRequestWithRetry(context.Background(), server.Client(), req, body, "openai", "llm_test")
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	defer resp.Body.Close()
	if gotAttempts != 1 || attempts.Load() != 1 {
		t.Fatalf("attempts = %d server=%d, want 1", gotAttempts, attempts.Load())
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}
