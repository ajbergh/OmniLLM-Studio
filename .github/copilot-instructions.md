# OmniLLM-Studio — Copilot Instructions

## Architecture Overview

Local-first LLM chat app: **Go backend** (API + SQLite) + **React/TypeScript frontend** (Vite + Tailwind v4 + Zustand). All API routes live under `/v1/`. The Vite dev server proxies `/v1/*` to the Go backend at `:8080`.

### Backend (`backend/`)

- **Entry point:** `cmd/server/main.go` — opens SQLite DB, runs migrations, builds router, starts HTTP server with graceful shutdown.
- **Router:** `internal/api/router.go` — single `NewRouter()` function wires ALL repos → services → handlers → chi routes. This is the composition root; understand it first when tracing any feature.
- **Layered architecture:** `api/` handlers → domain services (`llm/`, `agent/`, `search/`, `analytics/`, `bundle/`, `rag/`, `tools/`, `templates/`, `plugins/`, `eval/`, `websearch/`, `auth/`) → `repository/` → `models/` → `db/`.
- **Database:** SQLite with WAL mode and single-writer safety. Numbered migrations live in `internal/db/db.go`; inspect the current highest migration before adding the next one. New tables use `CREATE TABLE IF NOT EXISTS`, new columns require backward-compatible defaults, and applied versions are tracked in `schema_versions`.
- **Auth:** Solo-mode by default (no users = middleware passthrough). Multi-user mode activates when first user registers. Auth middleware in `auth/auth.go` uses Bearer tokens (`Authorization` header).
- **Streaming:** SSE (Server-Sent Events) for LLM responses. `WriteTimeout: 0` on the HTTP server. SSE events use `event:` + `data:` format (e.g., `token`, `done`, `web_search_*`, `file_search`, `file_search_results`, `rag_indexing`, `url_context`, `tool_start`, `router`, `agent_*`). Anything the UI renders about *how* an answer was produced must appear in both the `done` payload and the saved message metadata, or a reloaded conversation disagrees with the live stream.

### Frontend (`frontend/src/`)

- **State:** Zustand stores in `stores/index.ts` — separate stores for conversations, messages, providers, settings, feature flags.
- **API client:** `api.ts` — typed `apiFetch<T>()` wrapper with auto-attached auth token. Namespaced API objects (`api.*`, `branchApi.*`, `agentApi.*`, `templateApi.*`, etc.).
- **Components:** Single-file components in `components/`. No routing library for navigation — relies on conditional rendering in `App.tsx` with modal overlays (framer-motion `AnimatePresence`).
- **Styling:** Tailwind CSS v4 via `@tailwindcss/vite` plugin. Dark theme with indigo/purple accents. Utility-first inline classes directly in JSX.

## Development Workflow

```bash
# Backend (requires Go 1.25+; Linux desktop builds also require GTK/WebKit2GTK)
cd backend && go run ./cmd/server

# Frontend (Node.js 24 is the CI and container toolchain)
cd frontend && npm install && npm run dev

# Both at once (Windows)
scripts\start-dev.bat

# Tests
cd backend && go test ./...
```

The SQLite database file (`omnillm-studio.db`) is created in the `backend/` working directory. Attachments are stored in `backend/attachments/`.

## Backend Conventions

- **Handler pattern:** Each feature gets a `*Handler` struct in `api/` with constructor `NewXxxHandler(deps...)`. Methods are `func (h *XxxHandler) Verb(w http.ResponseWriter, r *http.Request)`. Register all routes in `router.go`.
- **Repository pattern:** One repo per entity in `repository/`. Constructor: `NewXxxRepo(db *sql.DB)`. Methods return `(*model, error)` or `([]model, error)`. Use `github.com/google/uuid` for IDs.
- **Error responses:** Use `respondError(w, status, msg)` or `respondErrorWithCode(w, status, code, msg, details)` from `api/helpers.go`. Success: `respondJSON(w, status, data)`.
- **Request parsing:** `decodeJSON(r, &v)` for JSON bodies. `chi.URLParam(r, "paramName")` for path params. Query params via `r.URL.Query().Get("key")`.
- **Models:** All in `internal/models/models.go`. JSON tags use `snake_case`. Optional fields use pointers with `omitempty`. No ORM — raw SQL with `database/sql`.
- **Config:** Environment variables with `OMNILLM_` prefix (`OMNILLM_PORT`, `OMNILLM_DB_PATH`, `OMNILLM_ATTACHMENTS_DIR`, `OMNILLM_PLUGIN_DIR`). Defaults in `config/config.go`.
- **Feature flags:** Stored in `feature_flags` table. Check via `FeatureFlagRepo`. Gate new features behind flags (e.g. `agent_mode`, `branching`, `semantic_search`).

## Provider-aware Current Information

Retrieval is backend-owned: the model chooses what to look for, the server chooses whether and how, and the answer is audited afterwards.

- `internal/websearch/gate.go` classifies deterministically in three ordered rule classes — `hardSuppressPatterns` (veto), `decisivePatterns` (short-circuit on explicit recency), then `triggerPatterns` minus `negativePatterns`. Subject-matter negatives are **weights, never vetoes**: a question about software can still be about the present state of the world.
- `internal/api/chat_search_route.go` consults the semantic router only when the gate declines, and only under the `tools_only` / `all_preflight` router modes. A router decision must be passed to the orchestrator as `force`, or its own gate check vetoes it.
- `internal/websearch/planner.go` assigns intent, answer shape, query set, result cap, search context size, source class, and minimum source count. Freshness is per intent: `pricing`, `benchmark`, and `release` deliberately carry **no** window, because vendor pages and release notes are rarely re-published within a day.
- `normalizePlan` clamps `MaxIterations` to `len(Queries)`; raising the iteration budget without expanding the query set does nothing.
- Native grounding is preferred for supported OpenAI, Anthropic, Gemini, and OpenRouter models because it removes a separate search-plus-summarization call. Anthropic goes through a Messages API adapter (`internal/llm/anthropic_search.go`); OpenRouter support is an allowlist of vendor prefixes, not the whole provider.
- Ollama, Groq, Together, Mistral, generic OpenAI-compatible providers, Claude 3.x, and rejected native requests use the configured Brave/DuckDuckGo provider and optional Jina enrichment.
- `Orchestrator.Preflight` retrieves without generating so a compound request ("find the latest prices and calculate the total") can act on the evidence. `requiresPostRetrievalTools` selects preflight versus orchestrator-owned. Native grounding cannot serve a preflight.
- Model-facing search uses `PlannedSearch`, never `DirectSearch`. `DirectSearch` is reserved for the explicit `/v1/websearch` endpoint.
- `frontend/src/clientContextFetch.ts` adds `omnillm_timezone` and `omnillm_locale` only to Omni API URLs. `internal/turncontext` resolves and validates that context.
- Direct schedule lookups should return the exact event and localized start time, not a generic explanation or a large table. Preserve FIFA World Cup aliases and the deterministic ESPN route.
- Do not add a second OpenRouter web plugin, send GPT-5-only fields to older OpenAI models, leak internal marker plugins, or install provider adapters globally.
- Never let a failed retrieval look like a normal answer. Write `search_attempted` / `search_failed` / `search_failure_reason` and let the UI render the warning from metadata; a system-prompt request to "mention this may be outdated" is not enforcement.
- Update `docs/PROVIDER_AWARE_SEARCH.md`, `docs/Feature FAQ.md`, and `docs/TECHNICAL_REFERENCE.md` when changing this behavior, and re-run `go test ./internal/eval -run TestRetrievalEvalTracksMetrics -v`.

## Tool Enforcement

- `llm.ToolChoice` is provider-neutral and translated at serialization time. It is sent on an **allowlist** (`internal/llm/tool_choice.go`): a provider that rejects an unknown field returns 400 and breaks the whole turn.
- `toolEnforcement` forces `tool_choice` on the first round only — a provider held at `required` never emits a final answer — then verifies what actually ran. Set `providerEnforced` from `llm.SupportsToolChoice`, never unconditionally.
- Streamed content cannot be retracted, so a late violation is recorded in metadata (`tool_required`, `tool_enforced`, `tool_requirement_unfulfilled`) and rendered as unverified rather than rejected.
- `runSyncToolLoop` gives the non-streaming path tool support, deliberately without browser tools: `genericRuntimeEligible()` refuses `browser_*` because they need the streaming loop's navigation cap, URL cache, and result sanitization.
- `auditAnswerEvidence` runs after generation on every path. `claim_warning` is a warning, not a gate — claim support is not decidable by string matching.

## Frontend Conventions

- **TypeScript types mirror Go models** in `types.ts` with `snake_case` field names (matching JSON tags).
- **API functions** return typed promises. SSE streaming uses raw `fetch()` with `ReadableStream` parsing in `api.ts`.
- **Toast notifications** via `sonner` (`toast.success()`, `toast.error()`).
- **Icons** from `lucide-react`.
- **Animations** via `framer-motion` for modals and overlays.

## Adding a New Feature (end-to-end checklist)

1. **Migration:** Add versioned migration in `db/db.go` (`versionedMigrations()` slice + SQL constant).
2. **Model:** Add struct to `models/models.go` with JSON tags.
3. **Repository:** Create `repository/xxx.go` with `NewXxxRepo(db)` and CRUD methods.
4. **Service (if needed):** Create package under `internal/` (e.g. `internal/newfeature/`).
5. **Handler:** Create `api/xxx_handler.go` with `NewXxxHandler(deps)` and HTTP methods.
6. **Router:** Wire repo → service → handler → routes in `router.go` inside the auth group.
7. **Frontend types:** Add interfaces to `types.ts`.
8. **Frontend API:** Add typed functions to `api.ts`.
9. **Component:** Create `components/XxxPanel.tsx`, integrate in `App.tsx`.
10. **Feature flag:** If gated, add flag check in both backend handler and frontend.
11. **SSE events:** If the feature involves long-running operations (indexing, search), add SSE events so the frontend can show meaningful status.

## Testing

- Backend tests use `*_test.go` in the same or `_test` package. Repository tests use in-memory SQLite: `db.Open(":memory:")` + `db.Migrate(database)` (see `repository/repository_test.go`'s `newTestDB` helper).
- Internal package tests (like `rag/chunker_test.go`) use the same package for access to unexported functions.
- Frontend unit tests run with Vitest via `npm run test:unit`; Playwright smoke coverage runs from the repository root via `npm run test:smoke`.

## External Dependencies

- **LLM providers:** OpenAI-compatible API format. `llm/service.go` handles provider routing, streaming, embeddings, and image generation.
- **Web search:** `internal/websearch/` plans intent, answer shape, freshness, query set, and source policy. Supported OpenAI, Anthropic, Gemini, and OpenRouter models use provider-native grounding; local and unsupported models fall back to Brave Search or DuckDuckGo, with selective Jina Reader extraction. The native adapters in `internal/llm/native_search.go` and `internal/llm/anthropic_search.go` are scoped to `llm.Service` HTTP clients and must never modify the global transport. Provider transports must not set `Accept-Encoding` by hand — `net/http` only auto-decompresses when it owns that header, and doing so silently broke every Brave response until `readResponseBody` was added.
- **RAG vector store:** [`chromem-go`](https://github.com/philippgille/chromem-go) v0.7.0 — embedded, persistent, zero-deps Go vector DB. Collections per conversation, workspace, and global scope, persisted under `<OMNILLM_CHROMEM_DIR>/<scope_id>/`. The wrapper lives at `internal/rag/store.go` (`VectorStore`); call sites never import chromem directly. Chunk text + metadata still live in the SQLite `document_chunks` table (chromem stores vectors only). Legacy `document_embeddings` rows lazy-migrate into chromem on first retrieve via `ChromemRetriever.tryLazyMigrate`.
- **File Library:** Durable file storage with conversation, workspace, and global scopes. Package at `internal/filelibrary/`. Hybrid vector + keyword search with citation formatting. SSE events (`file_search`, `file_search_results`, `rag_indexing`) stream status to the frontend. API routes under `/v1/file-library/`. Frontend panel at `frontend/src/components/FileLibraryPanel.tsx`.
- **Plugins:** JSON-RPC subprocess model. Plugin directory: `~/.omnillm-studio/plugins/` (override with `OMNILLM_PLUGIN_DIR`).
- **Encryption:** AES-256-GCM for API keys at rest (`internal/crypto/`). Derived from a machine-specific key.
