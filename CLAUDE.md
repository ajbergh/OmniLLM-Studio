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

`api/` handlers call domain packages such as `llm/`, `agent/`, `search/`, `analytics/`, `bundle/`, `rag/`, `tools/`, `templates/`, `plugins/`, `eval/`, `websearch/`, `turncontext/`, `sports/`, `auth/`, `browser/`, `music/`, and `video/`. Repositories use raw `database/sql`; models use snake_case JSON tags and pointer fields for optional values.

### Database

SQLite runs in WAL mode with a busy timeout and tuned cache/mmap settings. Versioned migrations live in `backend/internal/db/db.go` and are tracked in `schema_versions`.

Foreign-key enforcement remains intentionally staged. Do not enable it without first auditing and repairing orphaned records and validating every delete path against existing user databases.

### Authentication and secrets

Solo mode bypasses user sessions only when no users exist and the server binds to loopback. Multi-user mode accepts a bearer token or the first-party HttpOnly session cookie. Session tokens are SHA-256 hashed at rest and expired rows are cleaned periodically.

Provider credentials use AES-256-GCM. Persistent container and Kubernetes deployments must provide a stable `OMNILLM_MASTER_KEY`; the runtime sets `OMNILLM_REQUIRE_MASTER_KEY=true`. Local desktop/server mode may use the machine-scoped seed file.

### Network and browser security

URL fetches must use the repository SSRF-safe transports. Validation and dialing must use the same resolved IP; never validate a hostname and then dial it through a second DNS lookup or an uncontrolled proxy.

Headless-browser sessions use isolated incognito contexts, serialized page operations, per-user quotas, destination validation, and Chromium sandboxing by default. `OMNILLM_BROWSER_NO_SANDBOX=true` is an explicit compatibility override, not a normal setting. New browser capabilities must preserve user/session storage isolation and reject private, loopback, metadata, reserved, non-HTTP, and credential-bearing destinations.

Remote MCP Streamable HTTP is dual-era: prefer the stateless `2026-07-28` contract and preserve the `2025-06-18` handshake/session fallback for legacy servers. OAuth-protected MCP resource URLs must remain HTTPS; do not weaken Bearer-token transport because `allow_private_network` is enabled. Preregistered and DCR OAuth credentials are issuer-bound, CIMD remains issuer-portable, and a DCR issuer migration must register a new client instead of reusing the previous client ID.

### Streaming

SSE carries chat tokens, agent steps, tool progress, file search, RAG indexing, web search, generation progress, and URL context events. The frontend parses streams with `fetch()` and `ReadableStream` in `frontend/src/api.ts`. Browser timezone and locale are attached to Omni API requests by `frontend/src/clientContextFetch.ts` and resolved through `internal/turncontext`; preserve that context through preflight, sports, native-grounding, and fallback paths. Preserve cancellation and terminal error/done events when modifying a stream.

The `done` payload and the saved message metadata must stay in sync. Anything the UI renders about how an answer was produced has to appear in both, or a reloaded conversation will disagree with the live stream. That currently covers `web_search`, `sources`, `search_attempted`, `search_failed`, `search_failure_reason`, `search_route`, `tool_required`, `tool_enforced`, `tool_requirement_unfulfilled`, `freshness_verified`, `answer_freshness`, `citation_count`, `native_citations`, and `claim_warning`.

A `web_search` event with `status: "failed"` also carries a client-safe `reason`. Never put a raw provider error on the wire; map it through `classifySearchFailure`.

### LLM provider routing

`backend/internal/llm/service.go` is the primary entry point for chat, embeddings, and image generation. Provider discovery and connectivity checks are privileged network operations. Do not accept provider API keys in URLs or query strings.

Provider-native search is implemented by the LLM-scoped transport in `backend/internal/llm/native_search.go`. It must remain scoped to `llm.Service` clients; never install it as `http.DefaultTransport` or wrap unrelated backend HTTP calls. The internal native-search marker must be removed before the request leaves the process.

### Current-information orchestration

Retrieval is backend-owned. The model decides *what* to look for; the server decides *whether*, *how*, and *whether the answer may be trusted*. Detailed design: `docs/PROVIDER_AWARE_SEARCH.md`.

**Classification.** `gate.go` is the cheap deterministic first pass and is authoritative when it fires. `chat_search_route.go` consults the semantic router only when the gate declines, and only under the `tools_only` / `all_preflight` router modes so the default configuration pays for no extra LLM call. A router decision must be passed through as `force` — the orchestrator entry points re-run the gate and would otherwise veto it.

Three rule classes in `gate.go`, evaluated in this order. Do not collapse them:

1. `hardSuppressPatterns` veto outright — fenced code, "fix/refactor this", authoring requests, conceptual "in programming" questions.
2. `decisivePatterns` short-circuit scoring for explicit recency signals.
3. `triggerPatterns` minus `negativePatterns` must reach the threshold.

Subject-matter negatives are **weights, never vetoes**. A question about software can still be a question about the present state of the world; the earlier veto behavior suppressed exactly the questions most likely to need retrieval.

**Freshness is chosen per intent, not globally.** `inferTimeRange` defaults to *no window*. The `pricing`, `benchmark`, and `release` intents deliberately carry none, because vendor pricing pages, model cards, and release notes are rarely re-published within a day and a blanket filter removes the authoritative source. News, market data, weather, and scores keep tight windows.

**Plans must be internally consistent.** `normalizePlan` clamps `MaxIterations` to `len(Queries)` because `searchWithPlan` clamps its loop to the query count; raising the iteration budget without emitting more queries changes nothing. Use `queryVariants` to expand a query set.

**Sufficiency, not result count.** `ResultsLikelyAnswerable` compares distinct **hosts** against `MinSources` and requires an authoritative host when the plan names `PreferredDomains`. Prefer `PreferredDomains` (ranking, via `rankByPreferredDomains`) over `AllowedDomains` (hard filter) for pricing and benchmark work: a missing entry in a hard filter silently drops the only good source.

**Execution paths, and the rule that must not be relaxed.** `Process` / `ProcessStream` let the orchestrator own the turn, which is cheapest when native grounding can fold retrieval and generation into one call. `Preflight` retrieves without generating so a follow-up tool can act on the evidence.

**Turn ownership and tool calling are mutually exclusive.** The orchestrator paths generate an answer and return, so the tool loop never runs — no MCP, plugin, or app tool can be invoked. `retrievalMayOwnTurn` therefore permits ownership only when there is plausibly nothing else to run:

- never when the prompt has a follow-up action (`requiresPostRetrievalTools`);
- never when it names a private or account-scoped source (`referencesPrivateSource`);
- with a connected integration (`integrationToolsConnected`), only `Direct` may — keyword matching cannot tell "the latest from Alice on the launch" from "the latest news";
- with no integration connected, **native grounding wins any shape**, because a preflight can only use the local provider, and routing away from working native grounding onto an unreliable scraper makes every answer worse;
- local-only providers may own `Direct` and `Brief`, and no more: owning the turn buys nothing over a preflight when both run one local search.

This decision is two-sided, and both sides have been observed failing in production:

- Widening the gate to trigger on `latest`, `current <noun>`, `search for`, and `find` captured the phrasings people use to ask for tool-backed data, and those turns silently answered from the public web instead of calling the tool.
- Then over-correcting sent Gemini research questions to a preflight, which can only use DuckDuckGo, which rate-limits mid-turn — producing an answer that recommended models from over a year earlier while the model's own working grounding went unused.

Neither "always own the turn" nor "never own the turn" is right. Keep both guards.

**Do not force a tool retry after a failed retrieval.** It runs the same plan against the provider that just failed, and consumes the one round where the model could reach the tool that can actually answer.

**Providers declare their own limits; the planner must respect them.** `Provider.Capabilities()` reports `MaxQueriesPerTurn`, `SupportsFreshnessFilter`, and `ProvidesPublicationDates`, and `searchWithPlan` clamps its iteration count to the first. This is not decoration: Brave is an API with a quota, while DuckDuckGo is a scraped HTML endpoint that serves an anti-bot challenge after roughly one request per source address. Issuing an expanded three-query plan against DuckDuckGo guarantees that queries two and three come back empty, turning a deliberate breadth increase into a reliability loss. Report a new provider's limits honestly, including when that means reporting it as weak.

`ErrSearchProviderRateLimited` is a distinct sentinel because the correct response differs from every other failure: retrying within the turn cannot succeed. It stops the query loop, surfaces as `provider_rate_limited` in metadata, and makes the `web_search` tool return a non-retryable result telling the model to stop. Do not fold it back into a generic provider error — an earlier version reported it as "markup may have changed", which blamed the parser and sent readers to the wrong file.

Preserve provider-specific contracts:

- OpenAI uses `web_search_options`; only GPT-5 models receive the optional `verbosity` field.
- Anthropic uses the Messages API `web_search` server tool, which the OpenAI-compatibility endpoint does not accept. `internal/llm/anthropic_search.go` rewrites the request to `/v1/messages` and converts the response and SSE stream back. The tool type is version-selected per model; Claude 3.x predates server tools and must stay on the local fallback.
- Gemini uses native `generateContent` / `streamGenerateContent` with `google_search`, then converts responses back to the existing OpenAI-compatible internal shape.
- OpenRouter uses `openrouter:web_search` and is an **allowlist of vendor prefixes, not the whole provider**. A route that ignores the server tool returns HTTP 200 with an ungrounded answer, which is indistinguishable from success. Remove the deprecated `web` plugin when the server tool is present so the request contains only one web mechanism.
- Streaming native-search failures may retry locally only before answer content has been emitted.
- Adapters must synthesize OpenAI-style `url_citation` annotations so one parser in `service.go` handles every grounded provider.

**Never let a retrieval failure look like success.** A failed search writes `search_attempted` / `search_failed` / `search_failure_reason` to message metadata and the SSE `done` payload on both handler paths, and the frontend renders the warning from that metadata. Prompt-level mitigations are not enforcement: the reviewed regression was a model ignoring "mention that the information may not be current".

Sports lookup remains the cheaper deterministic path when ESPN can answer the question. `internal/router/deterministic.go` and `internal/sports/` must preserve browser-timezone conversion and concise one-event schedule answers.

### Tool enforcement and the evidence contract

`ToolChoice` on `llm.ChatRequest` is provider-neutral and translated at serialization time. It is sent on an **allowlist** (`internal/llm/tool_choice.go`) rather than a denylist: a provider that rejects an unknown field returns 400 and breaks the whole turn, which is worse than degrading to advisory behavior.

`toolEnforcement` forces `tool_choice` on the **first round only** — a provider held at `required` never emits a final answer — then verifies which tools actually ran. Content already streamed cannot be retracted, so a late violation is recorded in metadata (`tool_required`, `tool_enforced`, `tool_requirement_unfulfilled`) and rendered as unverified. Set `providerEnforced` from `llm.SupportsToolChoice`, never unconditionally, or the metadata claims an enforcement that never left the process.

Model-facing search must use `Orchestrator.PlannedSearch`, not `DirectSearch`. `DirectSearch` is reserved for the explicit `/v1/websearch` endpoint where the caller supplies every parameter. A model that omits `time_range`, region, and locale must get server-chosen values, not none.

After generation, `auditAnswerEvidence` runs on every path — streaming, non-streaming, and orchestrator-owned. It records `freshness_verified`, `answer_freshness`, `citation_count`, `native_citations`, and `claim_warning`. Three rules hold in the freshness logic:

- An undated result is a third state, neither fresh nor stale.
- `freshness_verified` requires *every* dated result inside the window, not a majority.
- No requested window means there is nothing to verify against.

`claim_warning` is a **warning, not a gate**. Claim support is not decidable by string matching, and an over-eager validator that rejects correct answers is worse than a permissive one. Do not promote it to a rejection without first measuring its false-positive rate against `internal/eval/retrieval_eval.go`.

### Chat tool loops

The streaming loop in `message_handler.go` is the full implementation: SSE progress, a per-turn browser-navigation cap, a visited-URL cache, and browser result sanitization. `runSyncToolLoop` gives the non-streaming path tool support with a lower round cap and **no browser tools** — `genericRuntimeEligible()` refuses `browser_*` calls precisely because they need that surrounding machinery, so they stay on the streaming path rather than getting a weaker second implementation.

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

Two local-environment notes that cost real time if you don't know them:

- `gofmt -l .` reports every file when `core.autocrlf=true`, because the working tree is CRLF while CI checks out LF. Normalize with `tr -d '\r'` into a temp tree before comparing, or trust CI.
- `npx tsc --noEmit -p tsconfig.json` does **not** typecheck `frontend/src` — that config only holds project references. Use `npm run build` (`tsc -b && vite build`), which is what CI runs. A missing type import passed `--noEmit` and failed the build.

Retrieval classification has a tracked corpus in `internal/eval/retrieval_eval.go`. It is deterministic — no network, no model — and reports trigger recall, false negatives and positives, intent accuracy, freshness-policy accuracy, and query-expansion rate. Run it when changing the gate, the planner, or any freshness policy:

```bash
cd backend
go test ./internal/eval -run TestRetrievalEvalTracksMetrics -v
```

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
- `OMNILLM_MCP_OAUTH_REDIRECT_URI`

Defaults and parsing live in `backend/internal/config/config.go`.
