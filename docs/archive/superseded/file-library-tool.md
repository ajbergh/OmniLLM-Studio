> **Archived — superseded design draft.** File Library is implemented under the current RAG architecture.

# File Library Tool Design (WIP)

Last updated: 2026-05-09
Status: Implemented (core), hardening follow-ups tracked

## Scope

This document tracks implementation details for File Library tools:

- `file_search`
- `file_fetch`
- `file_ingest`
- `file_summarize`
- `file_compare`
- `file_delete`
- `file_reindex`

## Implemented So Far

- Database foundation migration (`V33`) for `library_files` and chunk metadata fields.
- Repository layer for durable file-library metadata (`LibraryFileRepo`).
- Deterministic file-intent detector (`internal/filelibrary/DetectFileIntent`) with unit tests.
- File-library service core:
	- attachment ingestion + chunk/vector indexing;
	- hybrid vector + keyword merged search with weighting/boosts/filters;
	- file/chunk fetch with optional full text.
- Summarize/compare service methods:
	- `Summarize` (citation-aware summary over selected files)
	- `Compare` (citation-aware compare over selected files)
- API endpoints wired:
	- `GET /v1/file-library/files`
	- `POST /v1/file-library/files/ingest`
	- `GET /v1/file-library/files/{fileId}`
	- `POST /v1/file-library/search`
	- `POST /v1/file-library/summarize`
	- `POST /v1/file-library/compare`
	- `PATCH /v1/file-library/files/{fileId}`
	- `DELETE /v1/file-library/files/{fileId}`
	- `POST /v1/file-library/files/{fileId}/reindex`
	- `POST /v1/conversations/{conversationId}/file-library/search`
	- `GET /v1/conversations/{conversationId}/file-library/files`
	- `POST /v1/conversations/{conversationId}/file-library/ingest-attachments`
- Message preflight integration:
	- non-streaming and streaming message paths run deterministic file-intent detection;
	- file search context is injected before normal LLM generation when file intent is detected;
	- streaming emits `file_search` and `file_search_results` events.
- Tool registry integration:
	- `file_search`
	- `file_fetch`
	- `file_ingest`
	- `file_summarize`
	- `file_compare`
	- `file_delete`
	- `file_reindex`
	- note: per-request user/workspace tool scoping is tracked as a follow-up for stricter multi-user isolation.

- Frontend integration completed:
	- `fileLibraryApi` client methods in `frontend/src/api.ts`.
	- `FileLibraryPanel` modal for list/ingest/summarize/compare/reindex/delete actions.
	- assistant message metadata tray renders `file_sources` citations.

## Planned Next

1. Harden per-request tool scoping in multi-user mode (current tool defaults are conservative but static).
2. Expand frontend UX for scope-specific filtering/edit metadata and richer citation drilldown.
3. Add deeper service/handler tests for lifecycle edge cases.

## Notes

- Existing conversation RAG remains active and backward compatible.
- File Library will reuse `rag.DetectAndChunk`, existing embedding resolution, and `rag.VectorStore`.
