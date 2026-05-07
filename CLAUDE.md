# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Backend (Go 1.24+, requires GCC for cgo/SQLite)

```bash
cd backend
go run ./cmd/server                 # Run dev server on :8080
go test ./...                       # Run all tests
go test ./internal/rag -run TestX   # Run a single test
go build ./cmd/server               # Build headless server binary
go build ./cmd/desktop              # Build Wails desktop binary
```

### Frontend (Node 18+)

```bash
cd frontend
npm install
npm run dev          # Vite dev server on :5173, proxies /v1/* to :8080
npm run build        # tsc -b && vite build
npm run lint         # eslint .
```

### Both at once

```bash
scripts/start-dev.bat        # Windows
scripts/start-dev.sh         # Linux/macOS
scripts/start-wails-dev.bat  # Wails hot-reload desktop dev
```

### Playwright smoke tests (root)

```bash
npm run test:smoke           # Headless Chromium against the image-editor smoke spec
npm run test:smoke:headed    # Same with browser visible
```

`playwright.config.ts` boots its own backend on port 8090 with an isolated SQLite DB at `backend/test-results/playwright-smoke/smoke.db` and a Vite preview on 4173 — do not point smoke runs at the dev DB.

### Production / release builds

`scripts/build-all.sh` orchestrates Wails desktop (`build-wails-{windows,linux,macos}.{bat,sh}`) and headless web server (`build-web-*`) builds. CGO cross-compilation is not practical — use platform-native runners. See `.github/workflows/release.yml`.

## Architecture

### Big picture

Local-first LLM chat app: **Go backend** (Chi + SQLite) + **React/TypeScript frontend** (Vite + Tailwind v4 + Zustand). All HTTP routes live under `/v1/`. Same Go binary can run headless (`cmd/server`) or as a Wails desktop app (`cmd/desktop`) — desktop mode embeds the built frontend via `//go:embed` and starts the HTTP server on a random loopback port for SSE compatibility.

### Composition root

[backend/internal/api/router.go](backend/internal/api/router.go) is **the** place to start when tracing any feature. `NewRouter()` wires every repo → service → handler → chi route. There is no DI framework — read this file top-to-bottom to understand dependencies.

### Layered backend

`api/` (HTTP handlers) → domain services (`llm/`, `agent/`, `search/`, `analytics/`, `bundle/`, `rag/`, `tools/`, `templates/`, `plugins/`, `eval/`, `websearch/`, `auth/`) → `repository/` (raw `database/sql`) → `models/` → `db/`. No ORM. IDs are UUIDs from `github.com/google/uuid`. Models live in a single [backend/internal/models/models.go](backend/internal/models/models.go) with `snake_case` JSON tags; optional fields are pointers with `omitempty`.

### Database

Single SQLite file (default `omnillm-studio.db`) in WAL mode with tuned PRAGMAs (cache_size 64MB, mmap 256MB, synchronous=NORMAL). Versioned migrations live in [backend/internal/db/db.go](backend/internal/db/db.go) — each migration is a SQL constant appended to the `versionedMigrations()` slice and tracked in a `schema_versions` table. New tables always use `CREATE TABLE IF NOT EXISTS`; new columns must have defaults so older rows still load.

### Auth modes

Solo-mode (no users registered) → auth middleware passes through. Multi-user mode activates the moment the first user registers; subsequent registration requires `OMNILLM_ALLOW_PUBLIC_REGISTRATION=true` or admin invite. Bearer-token sessions, cleaned up by a 15-minute ticker in [cmd/server/main.go](backend/cmd/server/main.go).

### Streaming

SSE (`event:` + `data:` frames) for token-by-token chat, agent steps, web search progress, tool calls. Server uses `WriteTimeout: 5m` on the headless HTTP server. The frontend parses streams via raw `fetch()` + `ReadableStream` in [frontend/src/api.ts](frontend/src/api.ts). Event names include `token`, `done`, `web_search_*`, `tool_start`, `agent_*`.

### LLM provider routing

[backend/internal/llm/service.go](backend/internal/llm/service.go) is the single entry point for chat, embeddings, and image generation across OpenAI / Anthropic / Gemini / Ollama / OpenRouter / Groq / Together / Mistral / any OpenAI-compatible API. Provider profiles (with AES-256-GCM-encrypted keys via `internal/crypto/`) are stored per-user in `provider_profiles`. Reasoning/thinking effort maps per-provider — for Anthropic, `low/medium/high` maps to extended-thinking `budget_tokens` of 2k / 8k / 16k.

### RAG

Two-store design:

- **Chunk text + metadata** stay in SQLite (`document_chunks`).
- **Vectors** live in [`chromem-go`](https://github.com/philippgille/chromem-go), one persistent collection per conversation under `<OMNILLM_CHROMEM_DIR>/<conversation_id>/`.

The wrapper is `internal/rag/store.go` (`VectorStore`); **never import `chromem` directly from call sites**. The retriever is `ChromemRetriever`. Legacy `document_embeddings` SQL rows lazy-migrate into chromem on first retrieve (`tryLazyMigrate`) — `POST /v1/rag/reindex-all` (admin) drops collections to force a re-migration.

### Image Studio

Distinct from quick image generation. A "session" is a tree of generate/edit/variation nodes with masks and reference assets. Sessions, nodes, assets, masks, and references are relational. Provider adapters under `internal/llm/` route per-model to OpenAI / Gemini / Stable Diffusion / Together / OpenRouter. Frontend lives in [frontend/src/components/image/](frontend/src/components/image/) with a Zustand store at [frontend/src/stores/imageEditor.ts](frontend/src/stores/imageEditor.ts).

### Frontend state

Zustand stores in [frontend/src/stores/index.ts](frontend/src/stores/index.ts) — separate slices for conversations, messages, providers, settings, feature flags. **No router** — navigation is conditional rendering in [frontend/src/App.tsx](frontend/src/App.tsx) with framer-motion `AnimatePresence` modal overlays. TypeScript types in [frontend/src/types.ts](frontend/src/types.ts) mirror Go model JSON tags exactly (`snake_case`).

## Conventions

### Adding a new backend feature (end-to-end)

1. Migration appended to `versionedMigrations()` in `db/db.go` (new tables = `CREATE TABLE IF NOT EXISTS`; new columns must have defaults).
2. Struct in `models/models.go` with `snake_case` JSON tags.
3. Repository in `repository/xxx.go` with `NewXxxRepo(db *sql.DB)` and CRUD methods returning `(*model, error)` or `([]model, error)`.
4. Optional service package under `internal/`.
5. Handler in `api/xxx_handler.go` with `NewXxxHandler(deps...)` constructor.
6. Wire in `router.go` (composition root) inside the auth group.
7. Frontend: add types to `types.ts`, typed API functions to `api.ts`, component in `components/`, integrate in `App.tsx`.
8. Gate behind a feature flag when appropriate (`feature_flags` table; check via `FeatureFlagRepo`).

### Handler / repository idioms

- Responses: `respondJSON(w, status, data)`, `respondError(w, status, msg)`, `respondErrorWithCode(w, status, code, msg, details)` — all from [backend/internal/api/helpers.go](backend/internal/api/helpers.go).
- Request bodies: `decodeJSON(r, &v)`. Path params: `chi.URLParam(r, "name")`. Query: `r.URL.Query().Get("key")`.

### Tests

- Repository tests use in-memory SQLite via the `newTestDB` helper (open `":memory:"` then `db.Migrate`).
- Internal-package tests (e.g. `rag/chunker_test.go`) use the same package to access unexported helpers.
- No frontend unit-test framework is configured — Playwright smoke tests at the repo root cover the image editor.

## Environment variables

`OMNILLM_PORT` (8080), `OMNILLM_BIND_ADDRESS` (127.0.0.1), `OMNILLM_DB_PATH`, `OMNILLM_ATTACHMENTS_DIR`, `OMNILLM_CORS_ORIGINS`, `OMNILLM_ALLOW_PUBLIC_REGISTRATION`, `OMNILLM_PLUGIN_DIR` (`~/.omnillm-studio/plugins`), `OMNILLM_CHROMEM_DIR` (sibling of DB), `OMNILLM_CHROMEM_COMPRESS`. Defaults are in [backend/internal/config/config.go](backend/internal/config/config.go).
