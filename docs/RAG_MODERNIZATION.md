# RAG Modernization Architecture

## Purpose

OmniLLM-Studio's RAG modernization keeps the backend pure Go while improving embedding correctness, document structure, hybrid retrieval, context safety, and operational visibility. Chat generation and embedding generation can use different configured provider profiles.

## Default runtime architecture

The application runtime used by conversation attachments and the File Library is:

```text
Attachment or File Library source
  -> shared structural parser
  -> deterministic structure-aware chunks
  -> SQLite document_chunks + FTS5/BM25
  -> batched, normalized embeddings
  -> embedding-space-isolated chromem collection
  -> vector candidates + lexical candidates
  -> reciprocal-rank fusion and source diversity
  -> token-budgeted untrusted-evidence context
  -> chat or grounded web summarization
```

SQLite remains authoritative for chunk text, provenance, lexical retrieval, RAG metadata, and operational records. Chromem-go remains the default embedded exact vector store. Physical collection names include a routing fingerprint so vectors produced by incompatible provider/model/task/schema contracts are never queried together.

## Reindex safety

Conversation and attachment reindexing does not delete the active chunk set first. It builds replacement vectors, atomically replaces the attachment's relational chunks, and then removes stale vector IDs. If extraction, embedding, or vector persistence fails before activation, the prior searchable data remains. A conversation rebuild may report partial attachment failures.

`POST /v1/rag/repair` and `POST /v1/rag/reindex-all` currently perform the same full rebuild over every conversation with persisted chunks. The repair endpoint does not yet perform a cheaper inconsistency-only scan.

## Embedding identity and provider selection

An embedding space is identified by provider profile, model, dimensions when known, distance metric, normalization behavior, task types, and schema version. The `rag_embedding_model` setting accepts either a model ID or the advanced `Provider Profile Name::model-id` form. Without an explicit provider pin, an enabled active chat provider is used when compatible; otherwise the first enabled embedding-capable provider in repository order is selected.

Embedding inputs are sent to the selected embedding provider. Use an Ollama or local compatible provider when document text must remain local.

## Parsing and chunking

The shared `internal/document` parser supports text-like formats, HTML, PDF, DOCX, XLSX, and PPTX without an external parser service. It preserves headings, pages when available, slides, sheets, tables, and code blocks in a common structural representation. The deterministic chunker uses content-derived IDs and records parser/chunker metadata for incremental comparison.

## Retrieval and context

SQLite FTS5/BM25 and semantic candidates are fused with reciprocal-rank fusion. Source diversity reduces repeated passages from one document. SQLite builds without FTS5 use a bounded tokenized `LIKE` fallback. Retrieved text is always wrapped as untrusted evidence, assigned stable source labels, deduplicated, and packed into a conservative token budget.

When live web search is triggered, request-scoped private evidence is preserved and supplied to the grounded summarizer rather than discarded.

## Supporting vector components

The branch also provides a backend-neutral `VectorIndex` contract with:

- `ExactVectorIndex`, an in-memory full-recall reference implementation.
- `HNSWVectorIndex`, a pure-Go approximate implementation with validation and gob persistence.
- `HTTPVectorIndex` and `NewVectorIndexHTTPHandler`, a bearer-token-capable HTTP transport for a dedicated index owner.
- `GenerationCoordinator`, which builds and validates immutable generations before changing an active-generation pointer in SQLite.

These components are implemented and tested as library capabilities. The normal desktop/server runtime still uses embedding-space-isolated chromem collections; HNSW, the HTTP adapter, and `GenerationCoordinator` are not selected by runtime configuration or wired into Docker/Helm deployment on this branch.

## Persistence

RAG v2 lazily creates `rag_embedding_spaces`, `rag_indexes`, `rag_index_generations`, `rag_ingest_jobs`, `rag_retrieval_events`, and the `document_chunks_fts` virtual table and triggers.

## Administration

Administrators can use:

- `GET /v1/rag/health` for current settings and relational/vector footprint counts.
- `POST /v1/rag/repair` to rebuild all indexed conversations non-destructively.
- `POST /v1/rag/reindex-all` to rebuild all indexed conversations non-destructively.
- Settings -> RAG for the same health snapshot and rebuild controls.

The health endpoint reports counts, not a proof that every relational chunk has a matching vector. Compare `chunks` and `vector_records` as an operational signal, accounting for multiple embedding spaces or legacy collections.

## Evaluation and benchmarks

The RAG package includes Recall@K, MRR, nDCG@K, hit rate, metric deltas, stage telemetry, exact-search tests, and HNSW recall tests. Run vector benchmarks with:

```bash
cd backend
go test -run '^$' -bench BenchmarkVectorIndexes -benchmem ./internal/rag
```

See [RAG modernization status](RAG_MODERNIZATION_STATUS.md) and [RAG validation](RAG_BACKEND_VALIDATION.md) for branch-specific completion and validation results.
