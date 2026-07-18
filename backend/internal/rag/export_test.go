package rag

import (
	chromem "github.com/philippgille/chromem-go"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// NewChromemRetrieverWithDeps constructs a ChromemRetriever without a real
// *llm.Service. Test-only — production code uses NewChromemRetriever.
func NewChromemRetrieverWithDeps(chunkRepo *repository.ChunkRepo, vectorStore *VectorStore) *ChromemRetriever {
	return &ChromemRetriever{
		chunkRepo:   chunkRepo,
		vectorStore: vectorStore,
	}
}

// WithEmbedFuncForTest injects a deterministic embedding function so tests
// don't need a real *llm.Service. Stores the func in a package-level variable
// keyed off the retriever — see retrieveTestEmbedFn.
func (r *ChromemRetriever) WithEmbedFuncForTest(fn chromem.EmbeddingFunc) *ChromemRetriever {
	r.testEmbedFn = fn
	return r
}
