# RAG Modernization Implementation Status

Branch: `feature/rag-modernization-v2`

## Release A — Correctness foundation

| Work item | Status | Implementation |
|---|---|---|
| Embedding-space identity | Complete | Provider/model/task/metric/schema fingerprints and physical collection isolation |
| Chat/embedding decoupling | Complete | Explicit `Provider Profile::model` pinning plus provider-neutral embedding functions |
| Batched embeddings | Complete | Bounded 64-item batches, validation, normalization, retry/backoff/jitter |
| Durable RAG schema | Complete | Embedding spaces, indexes, generations, jobs, retrieval telemetry |
| Generation coordinator | Complete | Build, validate, mark ready, and atomically activate replacement generations |
| Legacy compatibility | Complete | Historical Retriever and VectorStore APIs remain available |
| Partial-index detection foundation | Complete | Expected/indexed counts and generation validation |

## Release B — Retrieval quality

| Work item | Status | Implementation |
|---|---|---|
| Shared parser | Complete | Text, Markdown, HTML, PDF, DOCX, XLSX, and PPTX through `internal/document` |
| Deterministic chunks | Complete | Content-hash IDs and chunker version metadata |
| Structure-aware chunking | Complete | Heading/page/slide/sheet preservation and boundary-aware splitting |
| Provider-neutral token budgeting | Complete | Conservative estimator abstraction and bounded context planner |
| FTS5/BM25 | Complete | External-content index, triggers, scoped queries, fallback lexical search |
| Hybrid conversation RAG | Complete | Vector + BM25 + RRF + source diversity |
| Hybrid File Library | Complete | Metadata no longer prefilters semantic candidates; RRF replaces fixed score weighting |
| Private RAG + web composition | Complete | Request-scoped evidence bridge and grounded web summarizer |
| Prompt-injection boundary | Complete | Retrieved evidence consistently marked untrusted |

## Release C — Capability

| Work item | Status | Implementation |
|---|---|---|
| Unified provenance model | Complete in RAG core | Page/section metadata and stable labels retained by context planning |
| Query rewriting | Complete, opt-in | Provider-neutral LLM standalone/multi-query planner with deterministic fallback |
| Optional reranking | Complete, opt-in | Any configured chat provider can rerank structured candidates |
| RRF/MMR default | Complete | Zero-cost deterministic ranking remains default |

## Release D — Scale

| Work item | Status | Implementation |
|---|---|---|
| Vector interface | Complete | Backend-neutral generation API |
| Exact reference backend | Complete | Pure-Go normalized dot-product search |
| HNSW backend | Complete | Pure-Go ANN, persistence, validation, and recall tests |
| Benchmarks | Complete | 1k/10k/100k exact and HNSW benchmarks |
| Incremental identity | Complete in core | Deterministic chunks and embedding-space/content hashes enable reuse |

## Release E — Validation and operations

| Work item | Status | Implementation |
|---|---|---|
| Retrieval metrics | Complete | Recall@K, MRR, nDCG, hit rate, deltas |
| Stage telemetry model | Complete | Embedding/vector/keyword/fusion/rerank/context timings |
| FTS integration test | Complete | Exact identifier and conversation-scope coverage |
| Parser tests | Complete | HTML trust/noise behavior and plain-text extraction |
| HNSW recall test | Complete | ANN compared against exact top-1 results |
| Administration UI | Deferred | Schema and health primitives exist; a dedicated UI is intentionally separated from core migration |
| Distributed index service | Deferred | Not required for the current single-process desktop/server architecture |

## Validation commands

```bash
cd backend
go test ./...
go test -race ./internal/rag ./internal/repository ./internal/filelibrary ./internal/document
go test -run '^$' -bench BenchmarkVectorIndexes -benchmem ./internal/rag
```

The branch is designed to remain CGO-free. No Python, ONNX Runtime, external vector database, or sidecar is required.

## Operational completion follow-up

- Reindexing now builds replacement vectors before atomically replacing relational chunks; failed attachments retain the previous searchable index.
- Added admin RAG health and repair endpoints.
- Added a token-protected pure-Go HTTP VectorIndex client/server adapter for dedicated index-owner and multi-replica deployments.
