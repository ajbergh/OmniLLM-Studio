package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// NormalizeToolCalls applies provider-neutral invariants before tool calls enter
// the Chat Studio orchestration loop. Providers and OpenAI-compatible proxies
// differ on whether they populate IDs and type fields, so the application must
// not rely on either being present upstream.
func NormalizeToolCalls(provider string, calls []ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, len(calls))
	for i := range calls {
		out[i] = NormalizeToolCall(provider, calls[i], i)
	}
	return out
}

// NormalizeToolCall returns a copy with a stable call ID, function type, index,
// and syntactically valid argument payload. Invalid or empty argument fragments
// are represented as an observable wrapper object so downstream validation can
// return a normal tool error instead of failing the provider protocol.
func NormalizeToolCall(provider string, call ToolCall, fallbackIndex int) ToolCall {
	call.Index = fallbackIndex
	call.Type = strings.TrimSpace(call.Type)
	if call.Type == "" {
		call.Type = "function"
	}
	call.Function.Name = strings.TrimSpace(call.Function.Name)
	call.Function.Arguments = strings.TrimSpace(call.Function.Arguments)
	if call.Function.Arguments == "" {
		call.Function.Arguments = "{}"
	} else if !json.Valid([]byte(call.Function.Arguments)) {
		call.Function.Arguments = fmt.Sprintf(`{"_provider_arguments":%q}`, call.Function.Arguments)
	}
	if strings.TrimSpace(call.ID) == "" {
		call.ID = stableToolCallID(provider, call, fallbackIndex)
	}
	return call
}

func stableToolCallID(provider string, call ToolCall, index int) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.ToLower(strings.TrimSpace(provider)),
		call.Function.Name,
		call.Function.Arguments,
		fmt.Sprintf("%d", index),
	}, "\x00")))
	return "call_" + hex.EncodeToString(sum[:8])
}

// ProviderRequestError is the normalized error returned when a provider rejects
// or cannot complete a chat request. It intentionally keeps the raw response
// body out of Error() so API layers can expose the error safely.
type ProviderRequestError struct {
	Provider          string
	StatusCode        int
	Code              string
	RequestID         string
	UpstreamRequestID string
	Retryable         bool
	Cause             error
}

func (e *ProviderRequestError) Error() string {
	if e == nil {
		return "provider request failed"
	}
	message := "provider request failed"
	if e.Provider != "" {
		message = e.Provider + " request failed"
	}
	if e.StatusCode > 0 {
		message += fmt.Sprintf(" with status %d", e.StatusCode)
	}
	if e.Code != "" {
		message += " (" + e.Code + ")"
	}
	if e.RequestID != "" {
		message += " [request_id=" + e.RequestID + "]"
	}
	return message
}

func (e *ProviderRequestError) Unwrap() error { return e.Cause }

// IsRetryableProviderStatus reports whether an HTTP response is safe to retry
// before any streaming output has been delivered.
func IsRetryableProviderStatus(status int) bool {
	switch status {
	case 408, 409, 425, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// ProviderRequestID extracts the common request-correlation headers used by
// OpenAI, Anthropic, OpenRouter, and compatible gateways.
func ProviderRequestID(headers map[string][]string) string {
	for _, key := range []string{"x-request-id", "request-id", "openai-request-id", "x-trace-id"} {
		for header, values := range headers {
			if strings.EqualFold(header, key) && len(values) > 0 && strings.TrimSpace(values[0]) != "" {
				return strings.TrimSpace(values[0])
			}
		}
	}
	return ""
}

func newProviderRequestID() string {
	return fmt.Sprintf("llm_%d", time.Now().UnixNano())
}

func retryableProviderTransportError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

// doProviderRequestWithRetry retries only failures that occur before any chat
// response body is consumed. This makes non-streaming provider retries safe
// while avoiding duplicate streamed tokens or tool calls.
func doProviderRequestWithRetry(ctx context.Context, client *http.Client, request *http.Request, body []byte, provider, requestID string) (*http.Response, int, error) {
	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		clone := request.Clone(ctx)
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.ContentLength = int64(len(body))
		resp, err := client.Do(clone)
		if err != nil {
			retryable := retryableProviderTransportError(err) && ctx.Err() == nil
			if retryable && attempt < maxAttempts {
				continue
			}
			return nil, attempt, &ProviderRequestError{
				Provider:  provider,
				Code:      "PROVIDER_TRANSPORT_ERROR",
				RequestID: requestID,
				Retryable: retryable,
				Cause:     err,
			}
		}
		if IsRetryableProviderStatus(resp.StatusCode) && attempt < maxAttempts {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			continue
		}
		return resp, attempt, nil
	}
	return nil, maxAttempts, &ProviderRequestError{Provider: provider, Code: "PROVIDER_RETRY_EXHAUSTED", RequestID: requestID, Retryable: false}
}
