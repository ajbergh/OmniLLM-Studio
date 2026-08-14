> **Archived — superseded security notes.** Current owner-scoped File Library and RAG boundaries are documented in active runtime references.

# File Library Security Notes (WIP)

Last updated: 2026-05-09
Status: Draft, actively implemented

## Security Goals

- Prevent cross-user and cross-workspace file data leakage.
- Prevent prompt-injection instructions in file content from overriding system/tool policies.
- Prevent unsafe file/path handling in ingestion and retrieval flows.

## Baseline Controls Implemented

- `library_files` rows include ownership and scope fields:
  - `owner_user_id`
  - `workspace_id`
  - `conversation_id`
  - `scope`
- Repository query methods enforce owner filtering for list/search/checksum lookups.
- Deterministic file-intent routing logic is isolated in `internal/filelibrary` to reduce accidental tool bypass for file-grounded prompts.
- Conversation shortcut endpoints enforce `verifyConversationAccess` checks before search/ingest.
- Solo mode support: owner-scoped queries correctly resolve rows where `owner_user_id IS NULL`.
- Message preflight now injects file excerpts as explicitly untrusted content and requires source citations when file context is used.
- Hybrid retrieval path now combines vector and keyword candidates while retaining owner/scope checks in file/chunk access paths.
- Summarize/compare flows require explicit file IDs and resolve file ownership before building prompt context.
- Lifecycle flows (`UpdateFile`, `DeleteFile`, `ReindexFile`) resolve file ownership before mutation.
- Delete/reindex removes associated chunk rows and vector docs for the relevant scoped collection.
- API lifecycle routes are now available with auth middleware coverage under `/v1/file-library/files/{fileId}`.

## Controls Planned Next

1. Per-request tool execution scoping for multi-user mode (current registrations still use conservative static defaults).
2. Tool execution guardrails to ensure explicit conversation/workspace binding on model-triggered calls.
3. Extend lifecycle tool tests for unauthorized-owner and cross-scope abuse cases.
  - Current tool set includes `file_search`, `file_fetch`, `file_ingest`, `file_summarize`, `file_compare`, `file_delete`, and `file_reindex`.
4. Strict path safety (`SafeJoin`) and file-type allow/deny checks in ingestion.
5. Prompt wrapping of retrieved file chunks as untrusted evidence.
6. Metadata sanitization to avoid exposing server absolute paths in responses.
