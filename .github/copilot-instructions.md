# OmniLLM-Studio — Copilot Instructions

## Architecture Overview

Local-first LLM chat app: **Go backend** (API + SQLite) + **React/TypeScript frontend** (Vite + Tailwind v4 + Zustand). All API routes live under `/v1/`. The Vite dev server proxies `/v1/*` to the Go backend at `:8080`.

### Backend (`backend/`)

- **Entry point:** `cmd/server/main.go` — opens SQLite DB, runs migrations, builds router, starts HTTP server with graceful shutdown.
- **Router:** `internal/api/router.go` — single `NewRouter()` function wires ALL repos → services → handlers → chi routes. This is the composition root; understand it first when tracing any feature.
- **Layered architecture:** `api/` handlers → domain services (`llm/`, `agent/`, `search/`, `analytics/`, `bundle/`, `rag/`, `tools/`, `templates/`, `plugins/`, `eval/`, `websearch/`, `auth/`) → `repository/` → `models/` → `db/`.
- **Database:** SQLite with WAL mode, `MaxOpenConns(1)`. Versioned migrations in `internal/db/db.go` (V1–V27). New tables always use `CREATE TABLE IF NOT EXISTS` with defaults on new columns. Migration versions are tracked in a `schema_versions` table.
- **Auth:** Solo-mode by default (no users = middleware passthrough). Multi-user mode activates when first user registers. Auth middleware in `auth/auth.go` uses Bearer tokens (`Authorization` header).
- **Streaming:** SSE (Server-Sent Events) for LLM responses. `WriteTimeout: 0` on the HTTP server. SSE events use `event:` + `data:` format (e.g., `token`, `done`, `web_search_*`, `tool_start`, `agent_*`).

### Frontend (`frontend/src/`)

- **State:** Zustand stores in `stores/index.ts` — separate stores for conversations, messages, providers, settings, feature flags.
- **API client:** `api.ts` — typed `apiFetch<T>()` wrapper with auto-attached auth token. Namespaced API objects (`api.*`, `branchApi.*`, `agentApi.*`, `templateApi.*`, etc.).
- **Components:** Single-file components in `components/`. No routing library for navigation — relies on conditional rendering in `App.tsx` with modal overlays (framer-motion `AnimatePresence`).
- **Styling:** Tailwind CSS v4 via `@tailwindcss/vite` plugin. Dark theme with indigo/purple accents. Utility-first inline classes directly in JSX.

## Development Workflow

```bash
# Backend (requires Go 1.23+ and GCC for cgo/SQLite)
cd backend && go run ./cmd/server

# Frontend (requires Node 18+)
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

## Testing

- Backend tests use `*_test.go` in the same or `_test` package. Repository tests use in-memory SQLite: `db.Open(":memory:")` + `db.Migrate(database)` (see `repository/repository_test.go`'s `newTestDB` helper).
- Internal package tests (like `rag/chunker_test.go`) use the same package for access to unexported functions.
- No frontend test framework is currently configured.

## External Dependencies

- **LLM providers:** OpenAI-compatible API format. `llm/service.go` handles provider routing, streaming, embeddings, and image generation.
- **Web search:** Brave Search API (key in settings) or DuckDuckGo (zero-config fallback). Jina Reader for URL content extraction.
- **RAG vector store:** [`chromem-go`](https://github.com/philippgille/chromem-go) v0.7.0 — embedded, persistent, zero-deps Go vector DB. One collection per conversation, persisted under `<OMNILLM_CHROMEM_DIR>/<conversation_id>/`. The wrapper lives at `internal/rag/store.go` (`VectorStore`); call sites never import chromem directly. Chunk text + metadata still live in the SQLite `document_chunks` table (chromem stores vectors only). Legacy `document_embeddings` rows lazy-migrate into chromem on first retrieve via `ChromemRetriever.tryLazyMigrate`.
- **Plugins:** JSON-RPC subprocess model. Plugin directory: `~/.omnillm-studio/plugins/` (override with `OMNILLM_PLUGIN_DIR`).
- **Encryption:** AES-256-GCM for API keys at rest (`internal/crypto/`). Derived from a machine-specific key.
