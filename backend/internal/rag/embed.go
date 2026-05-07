package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

// embedService is the subset of *llm.Service that NewLLMEmbeddingFunc needs.
// Defined as an interface so tests can swap in a fake.
type embedService interface {
	Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error)
}

// NewLLMEmbeddingFunc returns a chromem.EmbeddingFunc that delegates to the
// existing llm.Service.Embed pipeline so all provider routing (API key lookup,
// auth headers, base URLs) is reused. Includes bounded retry with backoff for
// transient failures from hosted providers.
func NewLLMEmbeddingFunc(svc embedService, provider, model string) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		const maxAttempts = 3
		var lastErr error
		backoff := 250 * time.Millisecond

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			resp, err := svc.Embed(ctx, llm.EmbeddingRequest{
				Provider: provider,
				Model:    model,
				Input:    []string{text},
			})
			if err == nil {
				if len(resp.Embeddings) == 0 || len(resp.Embeddings[0]) == 0 {
					return nil, fmt.Errorf("chromem embed: empty response from [%s/%s]", provider, model)
				}
				return resp.Embeddings[0], nil
			}

			lastErr = err
			if !isRetryableEmbedError(err) || attempt == maxAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		return nil, fmt.Errorf("chromem embed [%s/%s]: %w", provider, model, lastErr)
	}
}

// isRetryableEmbedError returns true for errors that look transient (rate
// limits, timeouts, 5xx). Best-effort string matching — the upstream service
// returns wrapped errors with the status text included.
func isRetryableEmbedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "status 429"),
		strings.Contains(msg, "status 500"),
		strings.Contains(msg, "status 502"),
		strings.Contains(msg, "status 503"),
		strings.Contains(msg, "status 504"),
		strings.Contains(msg, "timeout"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "eof"):
		return true
	}
	return false
}
