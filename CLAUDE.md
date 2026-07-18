# CLAUDE.md

This file provides implementation guidance for contributors and coding agents working in this repository.

## Commands

### Backend

Go 1.25+ is required. Linux desktop builds also require GCC, GTK3, and WebKit2GTK. Ubuntu 24.04 uses WebKit2GTK 4.1 and the `webkit2_41` build tag.

```bash
cd backend
go run ./cmd/server
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/server
```

On Ubuntu 24.04 when a command includes the Wails desktop package:

```bash
GOFLAGS=-tags=webkit2_41 go test ./...
GOFLAGS=-tags=webkit2_41 go vet ./...
```

Use `scripts/build-wails-linux.sh` rather than invoking the Linux Wails build manually; the script detects WebKit2GTK 4.0 versus 4.1 and supplies the correct tag.

### Frontend

Node.js 24 is the CI and container build toolchain.

```bash
cd frontend
npm ci
npm run dev
npm run lint
npm run test:unit
npm run build
```

### Both at once

```bash
scripts/start-dev.bat        # Windows
scripts/start-dev.sh         # Linux/macOS
scripts/start-wails-dev.bat  # Wails hot-reload desktop dev
```

### Playwright

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:smoke
npm run test:smoke:headed
```

`npm run test:smoke` runs the complete Chromium Playwright suite. `playwright.config.ts` boots an isolated backend on port 8090 with its SQLite database under `backend/test-results/playwright-smoke/`; never point smoke tests at a development database.

### Production and release builds

`scripts/build-all.sh` orchestrates native Wails and headless web builds. CGO desktop builds must run on platform-native runners. Release behavior is defined by `.github/workflows/release.yml`; do not replace pinned Go, Node, or Wails versions with `latest`.

## Architecture

### Big picture

OmniLLM-Studio is a local-first Go and React application. The backend uses Chi, SQLite, Server-Sent Events, durable media storage, and chromem-go. The frontend uses React, TypeScript, Vite, Tailwind, and Zustand. The same backend runs headless or inside a Wails desktop application.

Desktop mode starts a loopback HTTP server because SSE cannot pass through the Wails asset handler. That server is protected by a cryptographically random per-launch URL prefix. Do not log, persist, weaken, or replace that prefix with wildcard CORS or localhost-only trust.

### Composition root

`backend/internal/api/router.go` is the composition root. Read it top-to-bottom when tracing a feature. It constructs repositories, services, handlers, tools, and routes without a dependency-injection framework.

### Backend layers

`api/` handlers call domain packages such as `llm/`, `agent/`, `search/`, `analytics/`, `bundle/`, `rag/`, `tools/`, `templates/`, `plugins/`, `eval/`, `websearch/`, `auth/`, `browser/`, `music/`, and `video/`. Repositories use raw `database/sql`; models use snake_case JSON tags and pointer fields for optional values.

### Database

SQLite runs in WAL mode with a busy timeout and tuned cache/mmap settings. Versioned migrations live in `backend/internal/db/db.go` and are tracked in `schema_versions`.

Foreign-key enforcement remains intentionally staged. Do not enable it without first auditing and repairing orphaned records and validating every delete path against existing user databases.

### Authentication and secrets

Solo mode bypasses user sessions only when no users exist and the server binds to loopback. Multi-user mode accepts a bearer token or the first-party HttpOnly session cookie. Session tokens are SHA-256 hashed at rest and expired rows are cleaned periodically.

Provider credentials use AES-256-GCM. Persistent container and Kubernetes deployments must provide a stable `OMNILLM_MASTER_KEY`; the runtime sets `OMNILLM_REQUIRE_MASTER_KEY=true`. Local desktop/server mode may use the machine-scoped seed file.

### Network and browser security

URL fetches must use the repository SSRF-safe transports. Validation and dialing must use the same resolved IP; never validate a hostname and then dial it through a second DNS lookup or an uncontrolled proxy.

Headless-browser sessions use isolated incognito contexts, serialized page operations, per-user quotas, destination validation, and Chromium sandboxing by default. `OMNILLM_BROWSER_NO_SANDBOX=true` is an explicit compatibility override, not a normal setting. New browser capabilities must preserve user/session storage isolation and reject private, loopback, metadata, reserved, non-HTTP, and credential-bearing destinations.

### Streaming

SSE carries chat tokens, agent steps, tool progress, file search, RAG indexing, web search, generation progress, and URL context events. The frontend parses streams with `fetch()` and `ReadableStream` in `frontend/src/api.ts`. Preserve cancellation and terminal error/done events when modifying a stream.

### LLM provider routing

`backend/internal/llm/service.go` is the primary entry point for chat, embeddings, and image generation. Provider discovery and connectivity checks are privileged network operations. Do not accept provider API keys in URLs or query strings.

### RAG and File Library

Chunk text and metadata live in SQLite. Vectors live in chromem-go collections scoped by conversation, workspace, or global identity. Only `internal/rag/store.go` should import chromem directly.

Attachments are content-sniffed before persistence and indexing. Unknown binary data and declared/content MIME mismatches must remain rejected. File Library indexing is synchronous where the next chat turn depends on immediate retrieval.

### Image Studio

Image sessions form a relational tree of generation, edit, mask, reference, and variant nodes. Backend provider adapters live under `internal/llm/`; frontend state is in `frontend/src/stores/imageEditor.ts`.

### Music and Video Studios

Music and video share provider-profile and durable-asset conventions. Video creation and timeline editing share `internal/video/` and `/v1/video` while exposing separate frontend workspaces.

Timeline JSON is validated by `ValidateTimelineDocument`. `renderer_capabilities.go` must match actual FFmpeg behavior because the frontend derives export-fidelity warnings from it. Frontend timeline types must mirror Go structs. Each mutation follows clone, mutate, one undo snapshot, then autosave; pointer drags commit once on pointer-up.

The backend container intentionally includes FFmpeg and FFprobe. Do not remove them while container deployments advertise probing and rendering.

### Frontend state

Zustand stores live under `frontend/src/stores/`. Navigation is primarily conditional rendering in `App.tsx`; do not introduce a second competing navigation model without an explicit redesign. TypeScript request and response types must mirror backend JSON contracts.

## Adding a backend feature

1. Add a versioned migration when persistence changes.
2. Add or update Go models with snake_case JSON tags.
3. Add repository methods using existing error and transaction conventions.
4. Add a domain service when logic should not live in a handler.
5. Add a handler using shared JSON/error helpers.
6. Wire it in `router.go` inside the correct auth/role scope.
7. Update frontend types, API client methods, state, and UI.
8. Add feature gating where appropriate.
9. Add cancellation/progress events for long-running operations.
10. Add backend, frontend, and Playwright coverage appropriate to the change.

## Handler and repository conventions

- Responses: `respondJSON`, `respondError`, `respondErrorWithCode`, and `respondInternalError`.
- Requests: `decodeJSON`; path parameters via `chi.URLParam`.
- User-owned resources must be loaded through ownership-aware repository/service methods.
- Do not expose raw provider errors, secrets, filesystem paths, or subprocess command lines to clients.
- Never interpolate untrusted input into shell commands; pass discrete argv values.

## Validation

The required pull-request gate is `.github/workflows/ci.yml`:

- canonical Go formatting
- `go vet`
- backend unit/integration tests
- Go race detector
- frontend lint, unit tests, and production build
- Windows plugin lifecycle and path-containment test
- complete Playwright Chromium suite
- Helm lint and template validation

`.github/workflows/security.yml` runs govulncheck, npm audits, and CodeQL. `.github/workflows/container.yml` builds both multi-architecture images and validates the Helm chart. Do not merge a dependency, security-sensitive, deployment, browser, plugin, auth, import/export, or persistence change while its applicable gate is red.

## Environment variables

Core variables include:

- `OMNILLM_PORT`
- `OMNILLM_BIND_ADDRESS`
- `OMNILLM_DB_PATH`
- `OMNILLM_ATTACHMENTS_DIR`
- `OMNILLM_CORS_ORIGINS`
- `OMNILLM_ALLOW_PUBLIC_REGISTRATION`
- `OMNILLM_MASTER_KEY`
- `OMNILLM_REQUIRE_MASTER_KEY`
- `OMNILLM_PLUGIN_DIR`
- `OMNILLM_CHROMEM_DIR`
- `OMNILLM_CHROMEM_COMPRESS`
- `OMNILLM_MAX_UPLOAD_BYTES`
- `OMNILLM_BROWSER_ENABLED`
- `OMNILLM_BROWSER_EXEC_PATH`
- `OMNILLM_BROWSER_CACHE_DIR`
- `OMNILLM_BROWSER_MAX_SESSIONS`
- `OMNILLM_BROWSER_SESSION_TTL`
- `OMNILLM_BROWSER_NO_SANDBOX`

Defaults and parsing live in `backend/internal/config/config.go`.
