package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	chromem "github.com/philippgille/chromem-go"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	// EmbeddingSchemaVersion is incremented when the persisted meaning of an
	// embedding space changes. Collections from different schema versions must
	// never be mixed.
	EmbeddingSchemaVersion = 2

	defaultDistanceMetric = "cosine"
	collectionSpaceMarker = "@@space-"
)

// EmbeddingSpace describes one immutable vector space. Two vectors are safe to
// compare only when their embedding-space fingerprints match.
type EmbeddingSpace struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Dimensions       int    `json:"dimensions,omitempty"`
	DistanceMetric   string `json:"distance_metric"`
	Normalize        bool   `json:"normalize"`
	DocumentTaskType string `json:"document_task_type,omitempty"`
	QueryTaskType    string `json:"query_task_type,omitempty"`
	SchemaVersion    int    `json:"schema_version"`
}

// Canonical returns a normalized copy suitable for persistence and hashing.
func (s EmbeddingSpace) Canonical() EmbeddingSpace {
	s.Provider = strings.TrimSpace(strings.ToLower(s.Provider))
	s.Model = strings.TrimSpace(s.Model)
	s.DistanceMetric = strings.TrimSpace(strings.ToLower(s.DistanceMetric))
	if s.DistanceMetric == "" {
		s.DistanceMetric = defaultDistanceMetric
	}
	s.DocumentTaskType = strings.TrimSpace(strings.ToLower(s.DocumentTaskType))
	s.QueryTaskType = strings.TrimSpace(strings.ToLower(s.QueryTaskType))
	if s.SchemaVersion <= 0 {
		s.SchemaVersion = EmbeddingSchemaVersion
	}
	return s
}

// Validate verifies that the space has enough identity to isolate a collection.
func (s EmbeddingSpace) Validate() error {
	s = s.Canonical()
	if s.Provider == "" {
		return fmt.Errorf("embedding provider is required")
	}
	if s.Model == "" {
		return fmt.Errorf("embedding model is required")
	}
	if s.Dimensions < 0 {
		return fmt.Errorf("embedding dimensions cannot be negative")
	}
	if s.DistanceMetric != "cosine" && s.DistanceMetric != "dot" && s.DistanceMetric != "euclidean" {
		return fmt.Errorf("unsupported embedding distance metric %q", s.DistanceMetric)
	}
	return nil
}

// Fingerprint returns a stable SHA-256 identity for the embedding space.
func (s EmbeddingSpace) Fingerprint() string {
	s = s.Canonical()
	canonical := fmt.Sprintf(
		"provider=%s\nmodel=%s\ndimensions=%d\nmetric=%s\nnormalize=%t\ndocument_task=%s\nquery_task=%s\nschema=%d",
		s.Provider,
		s.Model,
		s.Dimensions,
		s.DistanceMetric,
		s.Normalize,
		s.DocumentTaskType,
		s.QueryTaskType,
		s.SchemaVersion,
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// RoutingFingerprint identifies the provider/model/task contract used to route
// queries to a physical collection. Dimensions are validated separately because
// some providers expose them only after the first request.
func (s EmbeddingSpace) RoutingFingerprint() string {
	s = s.Canonical()
	canonical := fmt.Sprintf(
		"provider=%s\nmodel=%s\nmetric=%s\nnormalize=%t\ndocument_task=%s\nquery_task=%s\nschema=%d",
		s.Provider,
		s.Model,
		s.DistanceMetric,
		s.Normalize,
		s.DocumentTaskType,
		s.QueryTaskType,
		s.SchemaVersion,
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// PhysicalCollectionName maps a stable logical scope name to an embedding-space
// specific physical collection. The short routing fingerprint keeps chromem
// directory names readable while retaining ample collision resistance.
func PhysicalCollectionName(logical string, space EmbeddingSpace) string {
	logical = strings.TrimSpace(logical)
	if logical == "" {
		return ""
	}
	fp := space.RoutingFingerprint()
	return logical + collectionSpaceMarker + fp[:20]
}

func isPhysicalCollectionFor(logical, candidate string) bool {
	return candidate == logical || strings.HasPrefix(candidate, logical+collectionSpaceMarker)
}

// embedService is the subset of *llm.Service used by the RAG embedding layer.
// It is intentionally small so tests can provide deterministic fakes.
type embedService interface {
	Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error)
}

// BatchEmbeddingFunc embeds a batch in input order.
type BatchEmbeddingFunc func(ctx context.Context, texts []string) ([][]float32, error)

type embeddingRuntime struct {
	space EmbeddingSpace
	batch BatchEmbeddingFunc
}

var embeddingRuntimeRegistry sync.Map // uintptr(function value) -> embeddingRuntime

// embeddingFunctionIdentity returns the runtime identity of a Go function
// value. Go function values are represented by a stable pointer to a funcval;
// copying the function preserves this pointer. This is deliberately contained
// here so the rest of the code does not depend on runtime representation.
//
// If a future Go release changes that representation, the fallback behavior is
// safe: unregistered functions continue using legacy logical collection names.
func embeddingFunctionIdentity(fn chromem.EmbeddingFunc) uintptr {
	if fn == nil {
		return 0
	}
	return *(*uintptr)(unsafe.Pointer(&fn)) // #nosec G103 -- process-local identity only
}

func registerEmbeddingRuntime(fn chromem.EmbeddingFunc, rt embeddingRuntime) {
	if id := embeddingFunctionIdentity(fn); id != 0 {
		embeddingRuntimeRegistry.Store(id, rt)
	}
}

func embeddingRuntimeFor(fn chromem.EmbeddingFunc) (embeddingRuntime, bool) {
	id := embeddingFunctionIdentity(fn)
	if id == 0 {
		return embeddingRuntime{}, false
	}
	value, ok := embeddingRuntimeRegistry.Load(id)
	if !ok {
		return embeddingRuntime{}, false
	}
	rt, ok := value.(embeddingRuntime)
	return rt, ok
}

// EmbeddingSpaceForFunc returns the immutable identity registered for an
// embedding function created by NewLLMEmbeddingFunc.
func EmbeddingSpaceForFunc(fn chromem.EmbeddingFunc) (EmbeddingSpace, bool) {
	rt, ok := embeddingRuntimeFor(fn)
	if !ok {
		return EmbeddingSpace{}, false
	}
	return rt.space, true
}

// NewLLMEmbeddingFunc returns a chromem-compatible single-text embedding
// function backed by the application's provider-neutral LLM service. The
// returned function also carries a process-local batch runtime and immutable
// embedding-space identity used by VectorStore.
func NewLLMEmbeddingFunc(svc embedService, provider, model string) chromem.EmbeddingFunc {
	batch := NewLLMBatchEmbeddingFunc(svc, provider, model)
	fn := chromem.EmbeddingFunc(func(ctx context.Context, text string) ([]float32, error) {
		vectors, err := batch(ctx, []string{text})
		if err != nil {
			return nil, err
		}
		if len(vectors) != 1 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("chromem embed: empty response from [%s/%s]", provider, model)
		}
		return vectors[0], nil
	})

	registerEmbeddingRuntime(fn, embeddingRuntime{
		space: EmbeddingSpace{
			Provider:       provider,
			Model:          model,
			DistanceMetric: defaultDistanceMetric,
			Normalize:      true,
			SchemaVersion:  EmbeddingSchemaVersion,
		},
		batch: batch,
	})
	return fn
}

// NewLLMBatchEmbeddingFunc embeds multiple inputs in one provider request when
// supported by the provider. Results are normalized for cosine similarity and
// returned in input order.
func NewLLMBatchEmbeddingFunc(svc embedService, provider, model string) BatchEmbeddingFunc {
	return func(ctx context.Context, texts []string) ([][]float32, error) {
		if len(texts) == 0 {
			return nil, nil
		}

		const maxAttempts = 3
		var lastErr error
		backoff := 250 * time.Millisecond

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			resp, err := svc.Embed(ctx, llm.EmbeddingRequest{
				Provider: provider,
				Model:    model,
				Input:    texts,
			})
			if err == nil {
				if len(resp.Embeddings) != len(texts) {
					return nil, fmt.Errorf(
						"embedding response count mismatch from [%s/%s]: got %d, want %d",
						provider,
						model,
						len(resp.Embeddings),
						len(texts),
					)
				}
				vectors := make([][]float32, len(resp.Embeddings))
				dimensions := 0
				for i, vector := range resp.Embeddings {
					if len(vector) == 0 {
						return nil, fmt.Errorf("empty embedding at input %d from [%s/%s]", i, provider, model)
					}
					if dimensions == 0 {
						dimensions = len(vector)
					} else if len(vector) != dimensions {
						return nil, fmt.Errorf(
							"embedding dimension mismatch from [%s/%s]: input %d has %d, want %d",
							provider,
							model,
							i,
							len(vector),
							dimensions,
						)
					}
					vectors[i] = normalizeVectorCopy(vector)
				}
				return vectors, nil
			}

			lastErr = err
			if !isRetryableEmbedError(err) || attempt == maxAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + time.Duration(attempt*37)*time.Millisecond):
			}
			backoff *= 2
		}

		return nil, fmt.Errorf("embedding batch [%s/%s]: %w", provider, model, lastErr)
	}
}

func normalizeVectorCopy(vector []float32) []float32 {
	out := append([]float32(nil), vector...)
	var sum float64
	for _, value := range out {
		v := float64(value)
		sum += v * v
	}
	if sum == 0 {
		return out
	}
	norm := float32(math.Sqrt(sum))
	for i := range out {
		out[i] /= norm
	}
	return out
}

// isRetryableEmbedError returns true for errors that look transient. The LLM
// service currently returns wrapped textual status errors, so matching remains
// intentionally conservative and provider-neutral.
func isRetryableEmbedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "status 408"),
		strings.Contains(msg, "status 409"),
		strings.Contains(msg, "status 425"),
		strings.Contains(msg, "status 429"),
		strings.Contains(msg, "status 500"),
		strings.Contains(msg, "status 502"),
		strings.Contains(msg, "status 503"),
		strings.Contains(msg, "status 504"),
		strings.Contains(msg, "timeout"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "temporarily unavailable"),
		strings.Contains(msg, "eof"):
		return true
	}
	return false
}

type providerLister interface {
	List() ([]models.ProviderProfile, error)
}

var embedCapableTypes = map[string]string{
	"openai":     "text-embedding-3-small",
	"openrouter": "openai/text-embedding-3-small",
	"mistral":    "mistral-embed",
	"together":   "togethercomputer/m2-bert-80M-8k-base",
	"ollama":     "nomic-embed-text",
	"gemini":     "gemini-embedding-001",
}

func modelCompatibleWithProvider(providerType, model string) bool {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return true
	}
	switch providerType {
	case "openai":
		return strings.HasPrefix(model, "text-embedding-")
	case "gemini":
		return strings.Contains(model, "gemini-embedding")
	case "mistral":
		return strings.HasPrefix(model, "mistral-embed")
	case "together":
		return strings.HasPrefix(model, "togethercomputer/") || strings.HasPrefix(model, "whereisai/")
	case "openrouter", "ollama":
		return true
	default:
		return false
	}
}

func ProviderHasEmbeddings(providerType string) bool {
	_, ok := embedCapableTypes[strings.ToLower(strings.TrimSpace(providerType))]
	return ok
}

// ParseEmbeddingSelection supports a backward-compatible provider pin inside
// the existing rag_embedding_model setting:
//
//	Provider Profile Name::embedding-model-id
//
// A plain model string continues to work exactly as before.
func ParseEmbeddingSelection(value string) (providerName, model string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	parts := strings.SplitN(value, "::", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", value
}

// ResolveEmbeddingProvider chooses a stable provider profile and model. An
// explicit provider::model selection is independent from the active chat model;
// legacy plain-model settings retain compatible behavior.
func ResolveEmbeddingProvider(
	activeProviderName string,
	settings models.AppSettings,
	repo providerLister,
) (provider, model string, err error) {
	all, err := repo.List()
	if err != nil {
		return "", "", fmt.Errorf("list providers for embedding resolution: %w", err)
	}
	byName := make(map[string]models.ProviderProfile, len(all))
	for _, profile := range all {
		byName[profile.Name] = profile
	}

	explicitProvider, pinnedModel := ParseEmbeddingSelection(settings.RAGEmbeddingModel)
	if explicitProvider != "" {
		profile, ok := byName[explicitProvider]
		if !ok || !profile.Enabled {
			return "", "", fmt.Errorf("configured RAG embedding provider %q is not enabled", explicitProvider)
		}
		canonical, supported := embedCapableTypes[strings.ToLower(profile.Type)]
		if !supported {
			return "", "", fmt.Errorf("configured RAG embedding provider %q does not support embeddings", explicitProvider)
		}
		if pinnedModel == "" {
			pinnedModel = canonical
		}
		if !modelCompatibleWithProvider(profile.Type, pinnedModel) {
			return "", "", fmt.Errorf("embedding model %q is incompatible with provider %q", pinnedModel, explicitProvider)
		}
		return profile.Name, pinnedModel, nil
	}

	// Backward compatibility: a compatible active embedding provider remains the
	// first choice when no independent provider has been explicitly selected.
	if active, ok := byName[activeProviderName]; ok && active.Enabled {
		if canonical, supported := embedCapableTypes[strings.ToLower(active.Type)]; supported {
			if modelCompatibleWithProvider(active.Type, pinnedModel) {
				if pinnedModel == "" {
					pinnedModel = canonical
				}
				return active.Name, pinnedModel, nil
			}
			return active.Name, canonical, nil
		}
	}

	if pinnedModel != "" {
		for _, profile := range all {
			if !profile.Enabled || !ProviderHasEmbeddings(profile.Type) {
				continue
			}
			if modelCompatibleWithProvider(profile.Type, pinnedModel) {
				return profile.Name, pinnedModel, nil
			}
		}
	}

	// Deterministic repository order is used as the provider-independent default
	// for chat providers that cannot embed (for example Anthropic).
	for _, profile := range all {
		if !profile.Enabled {
			continue
		}
		if canonical, ok := embedCapableTypes[strings.ToLower(profile.Type)]; ok {
			return profile.Name, canonical, nil
		}
	}

	return "", "", fmt.Errorf("no embedding-capable provider configured; add an OpenAI, OpenRouter, Mistral, Together, Ollama, or Gemini provider")
}
