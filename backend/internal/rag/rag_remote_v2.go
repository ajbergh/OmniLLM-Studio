package rag

// File overview: adapts VectorIndex to a bearer-token-protected standard-library HTTP transport.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HTTPVectorIndex is a pure-Go remote VectorIndex adapter for deployments
// where one process owns mutable index generations and API replicas query it.
type HTTPVectorIndex struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

// NewHTTPVectorIndex creates a remote adapter with a 60-second default HTTP timeout.
func NewHTTPVectorIndex(baseURL, token string) *HTTPVectorIndex {
	return &HTTPVectorIndex{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (h *HTTPVectorIndex) call(ctx context.Context, path string, requestBody, responseBody any) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		request.Header.Set("Authorization", "Bearer "+h.Token)
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("remote vector index returned status %d", response.StatusCode)
	}
	if responseBody != nil {
		return json.NewDecoder(response.Body).Decode(responseBody)
	}
	return nil
}

// CreateGeneration asks the remote index owner to create an immutable generation.
func (h *HTTPVectorIndex) CreateGeneration(ctx context.Context, spec IndexSpec) error {
	return h.call(ctx, "/generations/create", spec, nil)
}

// UpsertBatch sends vector records to an existing remote generation.
func (h *HTTPVectorIndex) UpsertBatch(ctx context.Context, generationID string, records []VectorRecord) error {
	return h.call(ctx, "/generations/upsert", map[string]any{"generation_id": generationID, "records": records}, nil)
}

// Search queries one remote generation and returns its ranked nearest-neighbor hits.
func (h *HTTPVectorIndex) Search(ctx context.Context, generationID string, query []float32, topK int) ([]VectorHit, error) {
	var hits []VectorHit
	err := h.call(ctx, "/generations/search", map[string]any{"generation_id": generationID, "query": query, "top_k": topK}, &hits)
	return hits, err
}

// Delete removes the supplied record IDs from one remote generation.
func (h *HTTPVectorIndex) Delete(ctx context.Context, generationID string, ids []string) error {
	return h.call(ctx, "/generations/delete", map[string]any{"generation_id": generationID, "ids": ids}, nil)
}

// Validate requests structural statistics for one remote generation.
func (h *HTTPVectorIndex) Validate(ctx context.Context, generationID string) (IndexStats, error) {
	var stats IndexStats
	err := h.call(ctx, "/generations/validate", map[string]string{"generation_id": generationID}, &stats)
	return stats, err
}

// Drop removes one remote generation and all of its vector records.
func (h *HTTPVectorIndex) Drop(ctx context.Context, generationID string) error {
	return h.call(ctx, "/generations/drop", map[string]string{"generation_id": generationID}, nil)
}

// NewVectorIndexHTTPHandler exposes a VectorIndex as a token-protected
// standard-library HTTP service suitable for a dedicated index owner.
func NewVectorIndexHTTPHandler(backend VectorIndex, token string) http.Handler {
	mux := http.NewServeMux()
	authorize := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	decode := func(r *http.Request, target any) error { return json.NewDecoder(r.Body).Decode(target) }
	write := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("/generations/create", authorize(func(w http.ResponseWriter, r *http.Request) {
		var spec IndexSpec
		if decode(r, &spec) != nil || backend.CreateGeneration(r.Context(), spec) != nil {
			http.Error(w, "create failed", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/generations/upsert", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationID string         `json:"generation_id"`
			Records      []VectorRecord `json:"records"`
		}
		if decode(r, &body) != nil || backend.UpsertBatch(r.Context(), body.GenerationID, body.Records) != nil {
			http.Error(w, "upsert failed", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/generations/search", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationID string    `json:"generation_id"`
			Query        []float32 `json:"query"`
			TopK         int       `json:"top_k"`
		}
		if decode(r, &body) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		hits, err := backend.Search(r.Context(), body.GenerationID, body.Query, body.TopK)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		write(w, hits)
	}))
	mux.HandleFunc("/generations/delete", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationID string   `json:"generation_id"`
			IDs          []string `json:"ids"`
		}
		if decode(r, &body) != nil || backend.Delete(r.Context(), body.GenerationID, body.IDs) != nil {
			http.Error(w, "delete failed", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/generations/validate", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationID string `json:"generation_id"`
		}
		if decode(r, &body) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		stats, err := backend.Validate(r.Context(), body.GenerationID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		write(w, stats)
	}))
	mux.HandleFunc("/generations/drop", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationID string `json:"generation_id"`
		}
		if decode(r, &body) != nil || backend.Drop(r.Context(), body.GenerationID) != nil {
			http.Error(w, "drop failed", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	return mux
}
