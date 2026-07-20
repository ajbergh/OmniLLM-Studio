# RAG Modernization Implementation Status

Branch: `feature/rag-modernization-v2`

## Runtime releases

| Area | Status | Current branch behavior |
|---|---|---|
| Embedding-space isolation | Complete | Provider/model/task/metric/schema routing fingerprints isolate physical chromem collections. |
| Chat/embedding decoupling | Complete | Embedding provider selection can be independent of the chat provider; advanced API configuration supports `Provider Profile::model`. |
| Batched embeddings | Complete | Bounded batches, validation, cosine normalization, retry/backoff, and dimensional consistency checks. |
| Shared parser | Complete | Text-like files, HTML, PDF, DOCX, XLSX, and PPTX use `internal/document`. |
| Deterministic structural chunks | Complete | Stable IDs plus heading/page/slide/sheet metadata and boundary-aware splitting. |
| Hybrid retrieval | Complete | Conversation RAG and File Library combine vector and SQLite FTS5/BM25 candidates using RRF and source diversity. |
| Context planning | Complete | Deduplication, source quotas, conservative token budgeting, stable labels, and untrusted-evidence boundaries. |
| Private evidence plus web | Complete | Request-scoped private evidence is preserved when grounded web summarization runs. |
| Safe attachment/conversation rebuild | Complete | Replacement vectors are built before relational activation; failed attachments retain prior searchable data. |
| Admin health and rebuild UI | Complete | Settings displays footprint counts and exposes full non-destructive rebuild actions. |

## Supporting and opt-in capabilities

| Capability | Status | Integration boundary |
|---|---|---|
| Query rewriting / multi-query planning | Implemented, opt-in API | `LLMQueryPlanner` exists but is not enabled by default in chat retrieval settings. |
| LLM reranking | Implemented, opt-in API | `LLMReranker` exists; RRF and diversity remain the default ranking path. |
| Exact `VectorIndex` | Implemented and tested | Reference in-memory generation backend; default runtime remains chromem-go. |
| HNSW `VectorIndex` | Implemented and tested | Pure-Go ANN with persistence and recall tests; no runtime selector is wired on this branch. |
| HTTP vector adapter | Implemented | Client/server adapter exists; Docker, Helm, discovery, and lifecycle wiring are not included. |
| Generation coordinator | Implemented | Atomic SQLite generation activation is available as a library component; default attachment reindex uses safe vector-first/chunk-replace activation instead. |
| Retrieval evaluation | Implemented | Recall@K, MRR, nDCG@K, hit rate, deltas, and stage telemetry helpers. |

## Administrative endpoint semantics

`POST /v1/rag/repair` and `POST /v1/rag/reindex-all` both rebuild every conversation that currently has persisted chunks. Neither endpoint currently restricts work to indexes proven inconsistent. Each response reports completed conversations, indexed chunks, and failures.

## Validation

See [RAG backend and frontend validation](RAG_BACKEND_VALIDATION.md) for the exact commands and latest branch results.
