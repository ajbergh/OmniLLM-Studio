> **Archived — completed implementation plan.** File Library is part of the current RAG runtime; retained for original design decisions.

# File Library Implementation Plan

Last updated: 2026-05-09
Owner: GitHub Copilot (GPT-5.3-Codex)

## Objective
Deliver a first-class File Library and file-grounded tooling using OmniLLM-Studio's existing RAG architecture.

## Current Status

- Phase 0: Completed
- Phase 1: Completed (foundation)
- Phase 2: Completed (initial service + API)
- Phase 3: Completed (message preflight + SSE + hybrid retrieval)
- Phase 4: Completed (tool registry integration including summarize/compare)
- Phase 5: Completed (message preflight + SSE file-search metadata persisted to chat)
- Phase 6: Completed (frontend File Library panel + chat file source tray)
- Phase 7: Completed (lifecycle APIs/tools + tests + docs)

## Discovery Notes (Completed)

- Current schema versioned migrations run through V32 in `backend/internal/db/db.go`.
- RAG currently uses:
  - `document_chunks` table in SQLite.
  - `rag.VectorStore` with one collection per conversation ID.
  - `MessageHandler.autoIndexForRAG` for background attachment indexing.
- Message preflight order currently emphasizes URL context, then sports/news, then RAG/web search.
- No existing `library_files` schema, file-library repository, or file-library handler/tool set exists yet.

## Implementation Plan

1. Phase 0: baseline architecture review and integration points
2. Phase 1: schema + models + repositories + tests
3. Phase 2: ingestion service and extraction dispatcher
4. Phase 3: hybrid search + citation formatting
5. Phase 4: tool registry integration (`file_search`, `file_fetch`, `file_ingest`, `file_summarize`, `file_compare`)
6. Phase 5: MessageHandler preflight integration + SSE file-search events
7. Phase 6: frontend File Library panel and chat source tray
8. Phase 7: hardening, tests, docs updates

## Phase 1 Detailed File Targets

- `backend/internal/db/db.go`
- `backend/internal/models/models.go`
- `backend/internal/repository/library_file.go` (new)
- `backend/internal/repository/document_chunk.go`
- `backend/internal/repository/library_file_test.go` (new)
- `backend/internal/filelibrary/types.go` (new)
- `backend/internal/filelibrary/detector.go` (new)
- `backend/internal/filelibrary/detector_test.go` (new)
- `backend/internal/filelibrary/service.go` (new)
- `backend/internal/filelibrary/extract.go` (new)
- `backend/internal/filelibrary/search.go` (new)
- `backend/internal/filelibrary/fetch.go` (new)
- `backend/internal/api/file_library_handler.go` (new)
- `backend/internal/api/router.go` (updated routes/wiring)
- `backend/internal/api/message_handler.go` (file-intent preflight + SSE)

## Open Technical Decisions

- Chunk linkage strategy:
  - Preferred by prompt: extend `document_chunks` with `library_file_id` and metadata columns.
  - Alternative if risk emerges: mapping table (`library_file_chunk_refs`).
- Scope collection naming:
  - New helper should support `conversation:{id}`, `workspace:{id}`, `global:{user_id}`.
  - Existing calls must remain backwards compatible during migration.

## Progress Log

- 2026-05-09: Started implementation, completed architecture discovery, and created this tracked plan.
- 2026-05-09: Implemented migration V33 (`file_library_foundation`) with `library_files` table and document chunk file-library metadata columns.
- 2026-05-09: Added `LibraryFile` model and extended chunk model metadata fields for future citations/search filters.
- 2026-05-09: Added `LibraryFileRepo` with lifecycle and lookup methods (`Create`, `GetByID`, `ListByScope`, `SearchMetadata`, `UpdateStatus`, `MarkDeleted`, `Delete`, `GetByChecksum`).
- 2026-05-09: Added repository tests (`TestLibraryFileRepoCRUDAndStatus`, `TestLibraryFileRepoListSearchAndChecksum`).
- 2026-05-09: Added deterministic file-intent detector package (`internal/filelibrary`) with tests for attachment forcing, explicit grounding phrases, compare/summarize intent, and false-positive avoidance.
- 2026-05-09: Added `LibraryService` with `IngestFile`, `Search`, `Fetch`, and `ListFiles` methods; implemented attachment extraction/chunking/vector-index flow with status transitions.
- 2026-05-09: Added initial file-library API endpoints and conversation-scoped shortcut routes.
- 2026-05-09: Added chunk repository methods for library-file chunk search and retrieval.
- 2026-05-09: Integrated deterministic file-intent preflight into message create/stream paths with file-context prompt injection and `file_search` SSE events.
- 2026-05-09: Added file-library tools (`file_search`, `file_fetch`, `file_ingest`) and registered them in tool registry.
- 2026-05-09: Follow-up tracked: promote tool scoping to per-request user/workspace context for multi-user mode.
- 2026-05-09: Validation complete for current slice: `go test ./...` in `backend/` passed.
- 2026-05-10: Upgraded file-library search to hybrid vector+keyword merged ranking with weighting, boost tuning, and filter support.
- 2026-05-10: Added summarize/compare contracts and service methods in `internal/filelibrary`.
- 2026-05-10: Added API routes and handlers for `POST /v1/file-library/summarize` and `POST /v1/file-library/compare`.
- 2026-05-10: Added tool implementations and registry wiring for `file_summarize` and `file_compare`.
- 2026-05-10: Validation complete for this slice: focused and full backend tests passed.
- 2026-05-09: Re-validation pass complete after latest edits/sync; focused and full backend tests passed again.
- 2026-05-09: Added lifecycle operations: service methods (`UpdateFile`, `DeleteFile`, `ReindexFile`), new repository methods (`UpdateFields`, `DeleteByLibraryFileID`), API handlers/routes (`PATCH/DELETE/POST reindex` under `/v1/file-library/files/{fileId}`).
- 2026-05-09: Added lifecycle tools and router registration: `file_delete`, `file_reindex`.
- 2026-05-09: Added frontend file-library integration: `fileLibraryApi`, typed contracts, `FileLibraryPanel`, App tools integration, and chat message file-source rendering.
- 2026-05-09: Added tool validation tests in `backend/internal/tools/file_library_tools_test.go`.
- 2026-05-09: Validation complete for this slice: `go test ./...` (backend) and `npm run build` (frontend) passed.
