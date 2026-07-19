# RAG Documentation and Go Comment Audit

Generated from `origin/main...feature/rag-modernization-v2` using the Go parser. This report is an audit input; semantic accuracy still requires review against implementation behavior.

## Changed files

- `backend/internal/api/attachment_text.go`
- `backend/internal/api/rag_admin_v2.go`
- `backend/internal/api/rag_handler.go`
- `backend/internal/api/rag_reindex_v2.go`
- `backend/internal/api/router.go`
- `backend/internal/document/document.go`
- `backend/internal/document/document_test.go`
- `backend/internal/filelibrary/extract.go`
- `backend/internal/filelibrary/rag_v2.go`
- `backend/internal/filelibrary/search.go`
- `backend/internal/rag/chunker.go`
- `backend/internal/rag/context_builder.go`
- `backend/internal/rag/embed.go`
- `backend/internal/rag/embed_resolver.go`
- `backend/internal/rag/rag_admin_v2.go`
- `backend/internal/rag/rag_chunk_context_v2.go`
- `backend/internal/rag/rag_embedding_v2.go`
- `backend/internal/rag/rag_eval_v2.go`
- `backend/internal/rag/rag_remote_v2.go`
- `backend/internal/rag/rag_retrieval_v2.go`
- `backend/internal/rag/rag_store_v2.go`
- `backend/internal/rag/rag_v2_test.go`
- `backend/internal/rag/rag_vector_v2.go`
- `backend/internal/rag/retriever_chromem.go`
- `backend/internal/rag/store.go`
- `backend/internal/repository/document_chunk.go`
- `backend/internal/repository/document_chunk_replace.go`
- `backend/internal/repository/rag_index.go`
- `backend/internal/repository/rag_v2_test.go`
- `backend/internal/websearch/orchestrator.go`
- `docs/RAG_BACKEND_VALIDATION.md`
- `docs/RAG_MODERNIZATION.md`
- `docs/RAG_MODERNIZATION_STATUS.md`
- `frontend/src/api.ts`
- `frontend/src/components/SettingsPanel.tsx`
- `frontend/src/types.ts`

## Package/file header comments

| File | Package comment immediately above `package` |
|---|---|
| `backend/internal/api/attachment_text.go` | _None_ |
| `backend/internal/api/rag_admin_v2.go` | _None_ |
| `backend/internal/api/rag_handler.go` | _None_ |
| `backend/internal/api/rag_reindex_v2.go` | _None_ |
| `backend/internal/api/router.go` | Package api provides HTTP handlers and routing for the OmniLLM-Studio backend. |
| `backend/internal/document/document.go` | _None_ |
| `backend/internal/document/document_test.go` | _None_ |
| `backend/internal/filelibrary/rag_v2.go` | _None_ |
| `backend/internal/rag/rag_admin_v2.go` | _None_ |
| `backend/internal/rag/rag_chunk_context_v2.go` | _None_ |
| `backend/internal/rag/rag_embedding_v2.go` | _None_ |
| `backend/internal/rag/rag_eval_v2.go` | _None_ |
| `backend/internal/rag/rag_remote_v2.go` | _None_ |
| `backend/internal/rag/rag_retrieval_v2.go` | _None_ |
| `backend/internal/rag/rag_store_v2.go` | _None_ |
| `backend/internal/rag/rag_v2_test.go` | _None_ |
| `backend/internal/rag/rag_vector_v2.go` | _None_ |
| `backend/internal/repository/document_chunk.go` | _None_ |
| `backend/internal/repository/document_chunk_replace.go` | _None_ |
| `backend/internal/repository/rag_index.go` | _None_ |
| `backend/internal/repository/rag_v2_test.go` | _None_ |
| `backend/internal/websearch/orchestrator.go` | _None_ |

## Exported declarations

| File | Line | Kind | Symbol | First comment line | Audit issue |
|---|---:|---|---|---|---|
| `backend/internal/api/rag_admin_v2.go` | 10 | func | `Health` | Health reports the effective RAG configuration and persisted index footprint. | — |
| `backend/internal/api/rag_admin_v2.go` | 52 | func | `Repair` | Repair non-destructively rebuilds all conversations that currently have chunks. | — |
| `backend/internal/api/rag_handler.go` | 14 | type | `RAGHandler` | RAGHandler handles RAG-related API endpoints. | — |
| `backend/internal/api/rag_handler.go` | 26 | func | `NewRAGHandler` | NewRAGHandler creates a new RAGHandler. | — |
| `backend/internal/api/rag_handler.go` | 49 | func | `ListChunks` | ListChunks returns all chunks for a conversation. | — |
| `backend/internal/api/rag_handler.go` | 67 | func | `ListAttachmentChunks` | ListAttachmentChunks returns all chunks for a specific attachment. | — |
| `backend/internal/api/rag_handler.go` | 94 | func | `Reindex` | Reindex re-chunks and re-embeds all text attachments for a conversation. | — |
| `backend/internal/api/rag_handler.go` | 108 | func | `IndexAttachment` | IndexAttachment chunks and embeds a single attachment. Called internally | — |
| `backend/internal/api/rag_handler.go` | 154 | func | `ReindexAll` | ReindexAll drops every chromem collection so the next query against each | — |
| `backend/internal/api/router.go` | 50 | func | `SecurityHeaders` | SecurityHeaders adds standard security headers to all responses. | — |
| `backend/internal/api/router.go` | 61 | func | `NewRouter` | NewRouter creates the main HTTP router with all API routes. | — |
| `backend/internal/api/router.go` | 68 | func | `NewRouterWithShutdown` | NewRouterWithShutdown creates the main HTTP router and returns a cleanup hook | — |
| `backend/internal/document/document.go` | 22 | type | `NodeType` | NodeType identifies a structural unit extracted from a source document. | — |
| `backend/internal/document/document.go` | 25 | const | `NodeDocument` | _None_ | missing |
| `backend/internal/document/document.go` | 26 | const | `NodePage` | _None_ | missing |
| `backend/internal/document/document.go` | 27 | const | `NodeSlide` | _None_ | missing |
| `backend/internal/document/document.go` | 28 | const | `NodeSheet` | _None_ | missing |
| `backend/internal/document/document.go` | 29 | const | `NodeHeading` | _None_ | missing |
| `backend/internal/document/document.go` | 30 | const | `NodeParagraph` | _None_ | missing |
| `backend/internal/document/document.go` | 31 | const | `NodeTable` | _None_ | missing |
| `backend/internal/document/document.go` | 32 | const | `NodeCode` | _None_ | missing |
| `backend/internal/document/document.go` | 36 | type | `Node` | Node preserves source structure before RAG chunking. | — |
| `backend/internal/document/document.go` | 47 | type | `ParsedDocument` | ParsedDocument is the canonical pure-Go parser output. | — |
| `backend/internal/document/document.go` | 57 | func | `ParseFile` | ParseFile parses supported text and office formats without CGO. | — |
| `backend/internal/document/document.go` | 79 | func | `ExtractFileText` | ExtractFileText renders a parsed document into structure-preserving Markdown | — |
| `backend/internal/document/document.go` | 91 | func | `NormalizeMIMEType` | _None_ | missing |
| `backend/internal/document/document.go` | 98 | func | `IsTextMIME` | _None_ | missing |
| `backend/internal/document/document.go` | 369 | func | `RenderMarkdown` | RenderMarkdown preserves structural boundaries using Markdown headings and | — |
| `backend/internal/document/document_test.go` | 10 | func | `TestParseHTMLPreservesHeadingsAndIgnoresNavigation` | _None_ | missing |
| `backend/internal/document/document_test.go` | 29 | func | `TestExtractPlainText` | _None_ | missing |
| `backend/internal/filelibrary/rag_v2.go` | 93 | func | `Search` | Search performs hybrid retrieval using SQLite FTS5/BM25, vector search, | — |
| `backend/internal/rag/rag_admin_v2.go` | 6 | func | `CollectionCounts` | CollectionCounts returns a stable snapshot of every physical collection. | — |
| `backend/internal/rag/rag_admin_v2.go` | 20 | func | `CollectionNames` | CollectionNames returns physical collection names in deterministic order. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 16 | const | `ChunkerSchemaVersion` | _None_ | missing |
| `backend/internal/rag/rag_chunk_context_v2.go` | 25 | type | `ChunkOptions` | ChunkOptions configures structure-aware chunking. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 31 | func | `DefaultChunkOptions` | _None_ | missing |
| `backend/internal/rag/rag_chunk_context_v2.go` | 37 | func | `ChunkText` | ChunkText splits plain text at nearby sentence/line boundaries where possible, | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 83 | func | `ChunkMarkdown` | ChunkMarkdown splits at heading boundaries and retains section/page/slide | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 162 | func | `DetectAndChunk` | _None_ | missing |
| `backend/internal/rag/rag_chunk_context_v2.go` | 304 | type | `ContextBlock` | ContextBlock is the formatted RAG context ready for injection into a prompt. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 312 | type | `SourceRef` | SourceRef is a lightweight reference to a retrieved chunk. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 326 | func | `BuildContext` | BuildContext creates a bounded, deduplicated ContextBlock using a conservative | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 333 | func | `BuildContextWithBudget` | BuildContextWithBudget creates a ContextBlock with an explicit retrieval token | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 419 | func | `SystemPrompt` | SystemPrompt wraps retrieved context in a consistent prompt-injection trust | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 441 | type | `TokenEstimator` | TokenEstimator estimates prompt tokens without coupling the core RAG system | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 447 | type | `ConservativeTokenEstimator` | ConservativeTokenEstimator is a provider-neutral fallback. It intentionally | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 449 | func | `Count` | _None_ | missing |
| `backend/internal/rag/rag_chunk_context_v2.go` | 466 | type | `Evidence` | Evidence is one candidate source for prompt context. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 478 | type | `ContextPlanConfig` | ContextPlanConfig controls evidence selection and prompt packing. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 487 | type | `ContextPlan` | ContextPlan is a provider-neutral, citation-ready context package. | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 496 | type | `ContextPlanner` | ContextPlanner deduplicates, diversifies, and packs evidence into a bounded | — |
| `backend/internal/rag/rag_chunk_context_v2.go` | 500 | func | `NewContextPlanner` | _None_ | missing |
| `backend/internal/rag/rag_chunk_context_v2.go` | 509 | func | `Plan` | Plan selects evidence in score order while enforcing source quotas and token | — |
| `backend/internal/rag/rag_embedding_v2.go` | 22 | const | `EmbeddingSchemaVersion` | EmbeddingSchemaVersion is incremented when the persisted meaning of an | — |
| `backend/internal/rag/rag_embedding_v2.go` | 30 | type | `EmbeddingSpace` | EmbeddingSpace describes one immutable vector space. Two vectors are safe to | — |
| `backend/internal/rag/rag_embedding_v2.go` | 42 | func | `Canonical` | Canonical returns a normalized copy suitable for persistence and hashing. | — |
| `backend/internal/rag/rag_embedding_v2.go` | 58 | func | `Validate` | Validate verifies that the space has enough identity to isolate a collection. | — |
| `backend/internal/rag/rag_embedding_v2.go` | 76 | func | `Fingerprint` | Fingerprint returns a stable SHA-256 identity for the embedding space. | — |
| `backend/internal/rag/rag_embedding_v2.go` | 96 | func | `RoutingFingerprint` | RoutingFingerprint identifies the provider/model/task contract used to route | — |
| `backend/internal/rag/rag_embedding_v2.go` | 115 | func | `PhysicalCollectionName` | PhysicalCollectionName maps a stable logical scope name to an embedding-space | — |
| `backend/internal/rag/rag_embedding_v2.go` | 135 | type | `BatchEmbeddingFunc` | BatchEmbeddingFunc embeds a batch in input order. | — |
| `backend/internal/rag/rag_embedding_v2.go` | 179 | func | `EmbeddingSpaceForFunc` | EmbeddingSpaceForFunc returns the immutable identity registered for an | — |
| `backend/internal/rag/rag_embedding_v2.go` | 191 | func | `NewLLMEmbeddingFunc` | NewLLMEmbeddingFunc returns a chromem-compatible single-text embedding | — |
| `backend/internal/rag/rag_embedding_v2.go` | 220 | func | `NewLLMBatchEmbeddingFunc` | NewLLMBatchEmbeddingFunc embeds multiple inputs in one provider request when | — |
| `backend/internal/rag/rag_embedding_v2.go` | 364 | func | `ProviderHasEmbeddings` | _None_ | missing |
| `backend/internal/rag/rag_embedding_v2.go` | 375 | func | `ParseEmbeddingSelection` | ParseEmbeddingSelection supports a backward-compatible provider pin inside | — |
| `backend/internal/rag/rag_embedding_v2.go` | 390 | func | `ResolveEmbeddingProvider` | ResolveEmbeddingProvider chooses a stable provider profile and model. An | — |
| `backend/internal/rag/rag_eval_v2.go` | 10 | type | `EvaluationCase` | EvaluationCase is one retrieval-quality benchmark query. | — |
| `backend/internal/rag/rag_eval_v2.go` | 18 | type | `EvaluationMetrics` | EvaluationMetrics summarizes retrieval quality over a corpus. | — |
| `backend/internal/rag/rag_eval_v2.go` | 27 | func | `EvaluateRetrieval` | EvaluateRetrieval computes recall@k, reciprocal rank, nDCG@k, and hit rate. | — |
| `backend/internal/rag/rag_eval_v2.go` | 86 | func | `CompareEvaluationMetrics` | CompareEvaluationMetrics returns deterministic metric deltas for release | — |
| `backend/internal/rag/rag_eval_v2.go` | 97 | func | `StableRankIDs` | StableRankIDs normalizes arbitrary scored candidates into deterministic IDs | — |
| `backend/internal/rag/rag_eval_v2.go` | 116 | type | `RetrievalTelemetry` | RetrievalTelemetry captures stage-level RAG latency and candidate counts. | — |
| `backend/internal/rag/rag_eval_v2.go` | 133 | func | `NewRetrievalTelemetry` | _None_ | missing |
| `backend/internal/rag/rag_eval_v2.go` | 137 | func | `TotalDuration` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 15 | type | `HTTPVectorIndex` | HTTPVectorIndex is a pure-Go remote VectorIndex adapter for deployments | — |
| `backend/internal/rag/rag_remote_v2.go` | 21 | func | `NewHTTPVectorIndex` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 60 | func | `CreateGeneration` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 63 | func | `UpsertBatch` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 66 | func | `Search` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 71 | func | `Delete` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 74 | func | `Validate` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 79 | func | `Drop` | _None_ | missing |
| `backend/internal/rag/rag_remote_v2.go` | 85 | func | `NewVectorIndexHTTPHandler` | NewVectorIndexHTTPHandler exposes a VectorIndex as a token-protected | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 22 | type | `RankedCandidate` | RankedCandidate is a provider-neutral retrieval candidate. | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 32 | type | `RankedList` | RankedList is one ordered retrieval channel such as BM25, vector, or title | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 41 | func | `ReciprocalRankFusion` | ReciprocalRankFusion combines independently ranked retrieval channels without | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 109 | func | `MMRSelect` | MMRSelect applies maximal marginal relevance to reduce near-duplicate chunks | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 175 | type | `QueryPlan` | QueryPlan is a provider-neutral retrieval plan. StandaloneQuery resolves | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 190 | type | `LLMQueryPlanner` | LLMQueryPlanner uses any configured chat provider to rewrite follow-up | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 194 | func | `NewLLMQueryPlanner` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 198 | func | `Plan` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 346 | func | `RegisterRequestEvidence` | RegisterRequestEvidence adds evidence gathered during a request. The router's | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 364 | func | `TakeRequestEvidence` | TakeRequestEvidence returns and removes evidence accumulated for the request. | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 380 | func | `ClearRequestEvidence` | ClearRequestEvidence discards any unconsumed evidence for a completed path. | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 399 | type | `RerankCandidate` | RerankCandidate is the minimum provider-neutral input required by a reranker. | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 408 | type | `RerankResult` | RerankResult preserves both the model relevance score and the original fused | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 415 | type | `Reranker` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 422 | type | `LLMReranker` | LLMReranker works with any provider supported by llm.Service. It is optional; | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 428 | func | `NewLLMReranker` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 432 | func | `Rerank` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 548 | type | `ChromemRetriever` | ChromemRetriever preserves the historical Retriever contract while using a | — |
| `backend/internal/rag/rag_retrieval_v2.go` | 558 | func | `NewChromemRetriever` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 562 | func | `WithLegacyEmbeddingRepo` | _None_ | missing |
| `backend/internal/rag/rag_retrieval_v2.go` | 570 | func | `Retrieve` | Retrieve runs vector and lexical candidate generation, combines ranks using | — |
| `backend/internal/rag/rag_store_v2.go` | 19 | type | `VectorStore` | VectorStore manages chromem-go collections for RAG vector storage. | — |
| `backend/internal/rag/rag_store_v2.go` | 26 | type | `QueryResult` | QueryResult is a normalized vector query hit from chromem. | — |
| `backend/internal/rag/rag_store_v2.go` | 33 | func | `NewVectorStore` | NewVectorStore opens (or creates) a persistent chromem-go database at the | — |
| `backend/internal/rag/rag_store_v2.go` | 46 | func | `NewInMemoryVectorStore` | NewInMemoryVectorStore returns a non-persistent VectorStore (used in tests). | — |
| `backend/internal/rag/rag_store_v2.go` | 62 | func | `Collection` | Collection returns (or creates) the physical collection for the logical scope | — |
| `backend/internal/rag/rag_store_v2.go` | 91 | func | `CollectionIfExists` | CollectionIfExists returns an existing legacy or physical collection. When | — |
| `backend/internal/rag/rag_store_v2.go` | 108 | func | `PhysicalCollections` | PhysicalCollections lists all collections currently associated with a logical | — |
| `backend/internal/rag/rag_store_v2.go` | 123 | func | `ExportToWriter` | ExportToWriter serializes the entire chromem DB. | — |
| `backend/internal/rag/rag_store_v2.go` | 131 | func | `ImportFromReader` | ImportFromReader replaces the chromem DB content with the supplied data. | — |
| `backend/internal/rag/rag_store_v2.go` | 140 | func | `DeleteCollection` | DeleteCollection removes every embedding-space generation associated with a | — |
| `backend/internal/rag/rag_store_v2.go` | 156 | func | `IndexChunks` | IndexChunks embeds and stores the given chunks. Hosted providers are embedded | — |
| `backend/internal/rag/rag_store_v2.go` | 237 | func | `DeleteDocuments` | DeleteDocuments removes the given chunk IDs from every physical | — |
| `backend/internal/rag/rag_store_v2.go` | 256 | func | `QuerySimilar` | QuerySimilar runs semantic search against the physical embedding space | — |
| `backend/internal/rag/rag_v2_test.go` | 14 | func | `TestContextPlannerBudgetAndTrustBoundary` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 31 | func | `TestContextPlannerDeduplicatesContent` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 47 | func | `Embed` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 58 | func | `TestVectorStoreBatchesHostedEmbeddingsAndIsolatesSpace` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 91 | func | `TestEmbeddingSpaceFingerprintIsolation` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 105 | func | `TestEmbeddingSpaceCanonicalDefaults` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 115 | func | `TestReciprocalRankFusionRewardsAgreement` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 131 | func | `TestMMRSelectDiversifiesSources` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 146 | func | `BenchmarkVectorIndexes` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 192 | func | `TestExactVectorIndexSearch` | _None_ | missing |
| `backend/internal/rag/rag_v2_test.go` | 214 | func | `TestHNSWVectorIndexRecallAgainstExact` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 17 | type | `VectorRecord` | VectorRecord is a backend-neutral vector document. | — |
| `backend/internal/rag/rag_vector_v2.go` | 24 | type | `VectorHit` | VectorHit is a backend-neutral nearest-neighbor result. | — |
| `backend/internal/rag/rag_vector_v2.go` | 31 | type | `IndexSpec` | IndexSpec identifies one immutable vector index generation. | — |
| `backend/internal/rag/rag_vector_v2.go` | 37 | type | `IndexStats` | IndexStats describes the health of an index generation. | — |
| `backend/internal/rag/rag_vector_v2.go` | 46 | type | `VectorIndex` | VectorIndex is the pluggable contract for exact, chromem, HNSW, or future | — |
| `backend/internal/rag/rag_vector_v2.go` | 58 | type | `ExactVectorIndex` | ExactVectorIndex is a pure-Go exact-search implementation intended for local | — |
| `backend/internal/rag/rag_vector_v2.go` | 69 | func | `NewExactVectorIndex` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 73 | func | `CreateGeneration` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 93 | func | `UpsertBatch` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 121 | func | `Search` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 161 | func | `Delete` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 174 | func | `Validate` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 194 | func | `Drop` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 221 | type | `HNSWConfig` | HNSWConfig configures the pure-Go approximate nearest-neighbor backend. | — |
| `backend/internal/rag/rag_vector_v2.go` | 247 | type | `HNSWVectorIndex` | HNSWVectorIndex is a pure-Go HNSW implementation. It favors predictable | — |
| `backend/internal/rag/rag_vector_v2.go` | 271 | func | `NewHNSWVectorIndex` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 280 | func | `CreateGeneration` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 301 | func | `UpsertBatch` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 395 | func | `Search` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 452 | func | `Delete` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 470 | func | `Validate` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 500 | func | `Drop` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 508 | func | `Save` | Save persists all generations using Go's gob format. | — |
| `backend/internal/rag/rag_vector_v2.go` | 520 | func | `Load` | Load replaces all in-memory generations from a gob snapshot. | — |
| `backend/internal/rag/rag_vector_v2.go` | 678 | func | `Len` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 679 | func | `Less` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 680 | func | `Swap` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 681 | func | `Push` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 682 | func | `Pop` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 691 | func | `Len` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 692 | func | `Less` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 693 | func | `Swap` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 694 | func | `Push` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 695 | func | `Pop` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 746 | type | `GenerationCoordinator` | GenerationCoordinator builds a replacement vector generation alongside the | — |
| `backend/internal/rag/rag_vector_v2.go` | 751 | func | `NewGenerationCoordinator` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 755 | type | `BuildGenerationRequest` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 768 | type | `BuildGenerationResult` | _None_ | missing |
| `backend/internal/rag/rag_vector_v2.go` | 774 | func | `BuildAndActivate` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 17 | type | `ChunkRepo` | ChunkRepo handles document chunk persistence and FTS5 lexical retrieval. | — |
| `backend/internal/repository/document_chunk.go` | 21 | func | `NewChunkRepo` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 28 | func | `Create` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 52 | func | `CreateBatch` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 95 | func | `ListByAttachment` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 104 | func | `ListByConversation` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 113 | func | `ListByLibraryFileID` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 125 | type | `KeywordChunkHit` | KeywordChunkHit is one BM25 or lexical-fallback candidate. Lower BM25 values | — |
| `backend/internal/repository/document_chunk.go` | 134 | func | `SearchFTSByLibraryFileIDs` | SearchFTSByLibraryFileIDs performs scoped FTS5/BM25 retrieval. It batches file | — |
| `backend/internal/repository/document_chunk.go` | 192 | func | `SearchFTSByConversation` | SearchFTSByConversation performs BM25 retrieval over all chunks linked to one | — |
| `backend/internal/repository/document_chunk.go` | 331 | func | `SearchByLibraryFileIDs` | SearchByLibraryFileIDs retains the historical API but now uses FTS5/BM25. | — |
| `backend/internal/repository/document_chunk.go` | 403 | func | `GetByIDs` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 433 | func | `DeleteByAttachment` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 441 | func | `DeleteByConversation` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 449 | func | `DeleteByLibraryFileID` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 457 | func | `DistinctConversationIDsWithChunks` | _None_ | missing |
| `backend/internal/repository/document_chunk.go` | 474 | func | `CountByAttachment` | _None_ | missing |
| `backend/internal/repository/document_chunk_replace.go` | 14 | func | `ReplaceAttachmentChunks` | ReplaceAttachmentChunks atomically replaces the relational chunk set | — |
| `backend/internal/repository/rag_index.go` | 162 | type | `RAGIndexRepo` | RAGIndexRepo persists embedding spaces, generations, jobs, and atomic active | — |
| `backend/internal/repository/rag_index.go` | 166 | func | `NewRAGIndexRepo` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 173 | type | `RAGEmbeddingSpaceRecord` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 187 | func | `UpsertEmbeddingSpace` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 225 | func | `EnsureIndex` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 252 | func | `BeginGeneration` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 270 | func | `UpdateGeneration` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 283 | func | `ActivateGeneration` | ActivateGeneration atomically switches the active pointer and supersedes the | — |
| `backend/internal/repository/rag_index.go` | 331 | func | `CreateIngestJob` | _None_ | missing |
| `backend/internal/repository/rag_index.go` | 342 | func | `UpdateIngestJob` | _None_ | missing |
| `backend/internal/repository/rag_v2_test.go` | 11 | func | `TestChunkRepoFTSFindsExactIdentifiersAndConversationScope` | _None_ | missing |
| `backend/internal/websearch/orchestrator.go` | 15 | type | `Orchestrator` | Orchestrator ties the web-search gate, provider, and LLM together. | — |
| `backend/internal/websearch/orchestrator.go` | 22 | func | `NewOrchestrator` | _None_ | missing |
| `backend/internal/websearch/orchestrator.go` | 26 | func | `Reconfigure` | _None_ | missing |
| `backend/internal/websearch/orchestrator.go` | 75 | func | `Process` | Process takes the user's latest message and the already assembled conversation | — |
| `backend/internal/websearch/orchestrator.go` | 117 | func | `ProcessStream` | ProcessStream preserves the historical method signature. Request-scoped RAG | — |
| `backend/internal/websearch/orchestrator.go` | 127 | func | `ProcessStreamWithHistory` | ProcessStreamWithHistory is the preferred streaming API for callers that can | — |
| `backend/internal/websearch/orchestrator.go` | 197 | func | `DirectSearch` | _None_ | missing |

**Static comment findings:** 97

## Potentially stale wording

### `Deferred`

```text
docs/MCP_HOW_TO_FAQ.md:43:Deferred:
docs/RAG_MODERNIZATION_STATUS.md:59:| Administration UI | Deferred | Schema and health primitives exist; a dedicated UI is intentionally separated from core migration |
docs/RAG_MODERNIZATION_STATUS.md:60:| Distributed index service | Deferred | Not required for the current single-process desktop/server architecture |
```

### `chunks_created`

```text
backend/internal/filelibrary/types.go:162:	ChunksCreated    int    `json:"chunks_created"`
frontend/src/types.ts:479:  chunks_created: number;
```

### `embeddings_stored`

```text
backend/internal/filelibrary/types.go:163:	EmbeddingsStored int    `json:"embeddings_stored"`
frontend/src/types.ts:480:  embeddings_stored: number;
```

### `delete-first`

```text
docs/RAG_MODERNIZATION.md:12:4. Rebuilds use index generations and atomic active-generation pointers rather than delete-first replacement.
```

### `chromem-go) over uploaded files`

```text
docs/TECHNICAL_REFERENCE.md:56:| **RAG Pipeline** | Document chunking, embedding generation, and persistent vector retrieval (chromem-go) over uploaded files — auto-indexes attachments synchronously so context is available immediately |
```

