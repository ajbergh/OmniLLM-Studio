> **Archived — superseded follow-up list.** Verified remaining URL-context work is consolidated in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# URL Context — Remaining Follow-up Items

**Created:** 2026-05-08 | **Updated:** 2026-05-08  
**Phases 1–5 complete.** Items below are deferred to future work.

---

## ✅ Phase 2 — RAG Ingest for Large Sources — COMPLETE

Implemented 2026-05-08:
- `urlcontext/rag_ingest.go` — `SourceToRAGText()` converts ResolvedSource to flat text
- `urlcontext/prompt_pack.go` — `BuildCompactPromptContext()` for metadata-only context
- `urlcontext/service.go` — switches to compact context + `UsedRAG=true` when > threshold
- `api/message_handler.go` — `ingestURLContextSourcesToRAG()`, background goroutine in Create + Stream

---

## ✅ Phase 3 — Tool Registry Integration — COMPLETE

Implemented 2026-05-08:
- `tools/url_context_tools.go` — `FetchURLContextTool`, `GitHubRepoInspectTool`
- `api/router.go` — both tools registered in tool registry

---

## ✅ Phase 4 — UI Polish — COMPLETE

Implemented 2026-05-08:
- `frontend/src/types.ts` — `URLContextSourceRef`, extended `MessageMetadata`
- `frontend/src/components/URLContextSourcePanel.tsx` — collapsible source panel
- `frontend/src/components/ChatView.tsx` — panel rendered under assistant messages

---

## ✅ Phase 5 — Hardening — COMPLETE

Implemented 2026-05-08:
- ETag/If-None-Match caching in `fetcher.go` (`sync.Map` keyed by URL)
- 429 retry with Retry-After backoff (≤10s) in `fetcher.go`
- `X-RateLimit-Remaining` warning when < 10
- Improved PDF stub message

---

## ✅ Phase 6 — Hallucination Prevention — COMPLETE

Implemented 2026-05-08:
- `ErrNoUsableContent` (aggregate hard error) added to `errors.go`
- `IsRequiredContextError` now includes `ErrNoUsableContent` → caller returns canned message, LLM is NOT invoked
- `UserFacingErrorMessage` case for `ErrNoUsableContent` with JS-render guidance
- `service.go`: all-fetch-fail path now returns `nil, ErrNoUsableContent` instead of a prompt directive
- `readability.go`: JSON-LD structured data extraction from `<script type="application/ld+json">` blocks before script stripping — extracts headlines, descriptions, article bodies, item lists
- `readability.go`: Open Graph / meta description extraction (`og:title`, `og:description`, `name=description`, `twitter:description`)
- `readability.go`: `IsNavigationOnly()` — detects JS-skeleton pages (all short lines, < 2 substantive content lines)
- `service.go`: calls `IsNavigationOnly(doc.Text)` and returns `ErrInsufficientContent` for navigation-only pages — prevents hallucination from thin context

---

## Remaining Future Work

- PDF text extraction via `pdfcpu` Go library
- Optional headless JS rendering behind `URL_CONTEXT_HEADLESS_ENABLED` feature flag
- SQLite-backed ETag/cache persistence across restarts
- Workspace-persistent URL source collections (not just ephemeral per-conversation)
- `ingest_url_to_rag` explicit tool for agent-triggered RAG ingest
- `rag_search` tool with source-type filter

---

## Known Remaining Limitations

| Limitation | Impact | Notes |
|---|---|---|
| PDF text extraction | PDFs return guidance message | Needs `pdfcpu` |
| No headless JS rendering | JS-heavy pages return little text | Optional flag |
| ETag cache is in-process only | Lost on restart | SQLite persistence future work |
