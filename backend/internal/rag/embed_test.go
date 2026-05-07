package rag

import (
	"context"
	"errors"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

type fakeEmbedSvc struct {
	calls    int
	failOnce error
	resp     [][]float32
}

func (f *fakeEmbedSvc) Embed(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	f.calls++
	if f.failOnce != nil {
		err := f.failOnce
		f.failOnce = nil
		return nil, err
	}
	dims := 0
	if len(f.resp) > 0 {
		dims = len(f.resp[0])
	}
	return &llm.EmbeddingResponse{
		Embeddings: f.resp,
		Model:      req.Model,
		Dimensions: dims,
	}, nil
}

func TestNewLLMEmbeddingFunc_Success(t *testing.T) {
	svc := &fakeEmbedSvc{resp: [][]float32{{0.1, 0.2, 0.3}}}
	fn := NewLLMEmbeddingFunc(svc, "openai", "text-embedding-3-small")

	got, err := fn(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != 0.1 {
		t.Fatalf("unexpected embedding: %v", got)
	}
	if svc.calls != 1 {
		t.Fatalf("expected 1 call, got %d", svc.calls)
	}
}

func TestNewLLMEmbeddingFunc_RetryOnTransient(t *testing.T) {
	svc := &fakeEmbedSvc{
		resp:     [][]float32{{0.5}},
		failOnce: errors.New("provider returned status 503: service unavailable"),
	}
	fn := NewLLMEmbeddingFunc(svc, "openai", "m")

	got, err := fn(context.Background(), "hi")
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if len(got) != 1 || got[0] != 0.5 {
		t.Fatalf("unexpected embedding: %v", got)
	}
	if svc.calls != 2 {
		t.Fatalf("expected 2 calls (1 fail + 1 retry), got %d", svc.calls)
	}
}

func TestNewLLMEmbeddingFunc_NonRetryable(t *testing.T) {
	svc := &fakeEmbedSvc{failOnce: errors.New("bad request: unknown model")}
	fn := NewLLMEmbeddingFunc(svc, "openai", "m")

	if _, err := fn(context.Background(), "x"); err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if svc.calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", svc.calls)
	}
}

func TestNewLLMEmbeddingFunc_EmptyResponse(t *testing.T) {
	svc := &fakeEmbedSvc{resp: [][]float32{}}
	fn := NewLLMEmbeddingFunc(svc, "openai", "m")

	if _, err := fn(context.Background(), "x"); err == nil {
		t.Fatal("expected error for empty embedding response")
	}
}
