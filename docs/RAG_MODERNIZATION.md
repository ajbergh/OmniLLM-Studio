# RAG Modernization Architecture

## Goals

OmniLLM-Studio's RAG v2 architecture keeps the backend pure Go and CGO-free while improving correctness, retrieval quality, throughput, and scale. The implementation is provider-neutral: chat generation, embeddings, query planning, and reranking may use different configured providers.

## Core principles

1. SQLite remains the authoritative store for source metadata, chunks, lexical search, index state, jobs, and telemetry.
2. Embedding spaces are immutable and fingerprinted by provider, model, dimensions, metric, normalization, task type, and schema version.
3. Logical scopes and physical vector collections are separate. A conversation, workspace, or global scope can have multiple isolated embedding spaces without mixing incompatible vectors.
4. Rebuilds use index generations and atomic active-generation pointers rather than delete-first replacement.
5. Retrieval is hybrid: FTS5/BM25 and semantic candidates are fused with reciprocal-rank fusion and source diversity.
6. Retrieved content is always treated as untrusted evidence.
7. Vector backends implement one Go interface. Exact search is the reference; pure-Go HNSW is available for larger indexes.

## Implemented pipeline

```text
Document / URL / attachment
  -> shared pure-Go structural parser
  -> deterministic structure-aware chunks
  -> SQLite chunks + FTS5
  -> batched provider-neutral embeddings
  -> embedding-space-isolated vector collection
  -> vector + BM25 candidate retrieval
  -> reciprocal-rank fusion
  -> source diversity
  -> optional provider-neutral LLM query planning/reranking
  -> token-budgeted context planner
  -> chat generation with stable evidence labels
```

## Embedding space identity

The physical collection suffix is derived from an immutable routing fingerprint. Changing the chat model does not alter the embedding space. Changing the embedding provider/model creates a separate physical collection rather than querying old vectors with an incompatible query vector.

## Storage

RAG v2 lazily creates:

- `rag_embedding_spaces`
- `rag_indexes`
- `rag_index_generations`
- `rag_ingest_jobs`
- `rag_retrieval_events`
- `document_chunks_fts`

The FTS5 index uses external-content triggers over `document_chunks`. SQLite builds without FTS5 fall back to bounded tokenized `LIKE` retrieval.

## Vector backends

`VectorIndex` supports generation creation, batched upsert, search, delete, validation, and drop.

Implemented backends:

- `ExactVectorIndex`: deterministic full-recall reference backend.
- `HNSWVectorIndex`: pure-Go approximate backend with configurable M, construction/search breadth, deletion, validation, and gob snapshots.
- Existing chromem-go storage remains available through `VectorStore`, now isolated by embedding space and supplied with precomputed batch embeddings.

Run vector benchmarks with:

```bash
cd backend
go test -run '^$' -bench BenchmarkVectorIndexes -benchmem ./internal/rag
```

## Retrieval evaluation

The `rag` package provides Recall@K, MRR, nDCG, hit rate, and metric-delta helpers. Evaluation datasets should contain query IDs, relevant chunk IDs, and ranked retrieval results. Exact search is the ANN recall reference.

## Operational guidance

- Use exact search for small and medium local indexes until benchmarks show a latency problem.
- Enable HNSW only when its measured recall and memory profile meet the deployment target.
- Keep reranking optional. RRF + diversity remains the deterministic zero-cost default.
- Treat embedding model changes as new index generations.
- Never delete the active generation before replacement validation and activation.
