# Chat Studio Agent Runtime Implementation

**Branch:** `agent/chat-studio-agent-runtime-20260718`  
**Pull request:** #24

## Executive status

The Chat Studio tools and Agent roadmap are implemented and connected through the production backend composition root. Backend completion was committed in `f2dea6b8445ec8a28fad6f90592a050feef64d88` and includes production route registration, lifecycle ownership, persistent schema migration, authenticated scope propagation, safe default policies, and end-to-end router coverage.

The branch retains the documented product boundaries below, but no backend composition work remains outstanding for this pull request.

## Phase 0 — Tool execution foundation

Implemented:

- Versioned tool contracts with schemas, risk, side-effect, timeout, output-size, and parallel-safety metadata
- Stable registry ordering and retrieval
- Structured tool lifecycle events and SQLite-backed invocation audit records
- Approval broker APIs with editable arguments and explicit continuation
- Authenticated approval list and resolution routes
- Global Chat Studio approval UI integration

## Phase 1 — Agent and Research runtime

Implemented:

- Chat, Research, and Agent profiles
- Plan validation, dependencies, retries, repair, approvals, and parallel read groups
- Checkpoint persistence, pause, cancel, resume, and interrupted-run recovery
- Per-run model, tool, duration, cost, and step budgets
- Append-only durable Agent events and cursor replay
- Final-response persistence
- Authenticated user, workspace, conversation, message, and run scope propagation into tools
- Durable event publication for both interactive SSE runs and background scheduled runs

## Phase 2 — First-party tools and asynchronous jobs

Implemented and registered in `backend/internal/api/router.go`:

- Date/time, unit conversion, weather, and ECB currency utilities
- Opt-in constrained Python analysis
- Durable job status and cancellation
- Image, Music, Video, and artifact-generation job adapters
- Authenticated job list, detail, and cancellation routes
- Graceful job-manager shutdown

The Python capability is a constrained subprocess and is not a hardened operating-system or container security boundary.

## Phase 3 — Research and connected apps

Implemented and registered:

- Research-specific planning instructions
- Governed MCP-backed app catalog
- Per-user and per-workspace app mappings
- Declared read/write scopes
- MCP server existence validation for app connections
- Authenticated catalog, connection-list, connect, and disconnect routes
- Existing tool-policy enforcement for app actions

A generalized encrypted OAuth broker is not included.

## Phase 4 — Tasks and controlled memory

Implemented and registered:

- One-time, recurring, and condition-watch task scheduling
- Restart recovery, pause, resume, and deletion
- Scheduler ownership and graceful shutdown
- Scoped memory records with expiration and source references
- Authenticated memory review, create, update, and deletion APIs
- Sensitive-token rejection and approval-required writes
- Stable local owner identity for solo mode while retaining authenticated user isolation in multi-user mode

External email, push, webhook, and operating-system notification adapters are not included.

## Phase 5 — Evaluation and safety

Implemented and registered:

- Planner and tool-selection evaluation scenarios
- Expected and forbidden tool checks
- Argument validation and approval checks
- Durable runtime audit events
- Output and timeout enforcement
- Risk-aware default policies
- Admin-only Agent evaluation scenario and execution routes

Side-effecting runtime tools default to `ask`, including memory writes/deletes, task creation/updates, job cancellation, media and artifact generation, Python analysis, and connected-app mutations.

## Persistence and lifecycle

- Runtime tables are included in numbered database migration V42.
- Runtime schema initialization remains idempotent for compatibility.
- Tool and Agent event recorders are installed as application-wide sinks.
- Scheduled tasks, jobs, event recorders, browser sessions, and MCP processes participate in API shutdown.
- Server and desktop entry points stop HTTP intake before shutting down API-owned runtime services.

## Backend validation coverage

Added focused tests for:

- V42 runtime migration and table creation
- Invocation-scope inheritance and overlay behavior
- Durable Agent event publication without a live SSE callback
- Production-router construction
- Runtime route reachability
- Runtime tool registration
- Approval-required default policies
- Agent event replay route mounting
- Graceful runtime shutdown

The repository Quality Gate remains the authoritative full-suite validation and runs formatting, vet, backend unit/integration tests, race detection, frontend checks, Playwright, Windows plugin tests, and Helm checks.

## Retained boundaries

- Gemini generic function calling remains disabled until mandatory thought-signature preservation is verified end to end.
- Python analysis remains a constrained subprocess, not a hardened sandbox.
- Connected apps remain MCP-backed; a generalized OAuth broker is not part of this release.
- Scheduled results are written to conversations; external notification delivery is not part of this release.

## Provider-aware search follow-on — 2026-07-19

**Branch:** `agent/provider-aware-search-orchestration-20260719`

**Pull request:** #26

The Chat Studio runtime now includes a provider-aware current-information layer that complements the Agent/tool foundation documented above.

Implemented:

- Adaptive direct, brief, standard, and research answer plans
- OpenAI `web_search_options` for supported OpenAI models
- Native Gemini Google Search grounding for supported Gemini models
- OpenRouter `openrouter:web_search` server-side grounding
- Brave/DuckDuckGo plus selective Jina fallback for local, unsupported, or rejected native requests
- Pre-content streaming retry from native grounding to the local search stack
- Answerability validation for direct factual lookups
- Browser timezone and locale propagation through `internal/turncontext`
- Deterministic FIFA World Cup routing, ESPN timezone conversion, and concise single-event schedule rendering
- Provider citation normalization into the existing SSE response path
- LLM-scoped transport isolation so native grounding cannot rewrite unrelated backend HTTP traffic

The canonical design and operational reference is [Provider-aware search](PROVIDER_AWARE_SEARCH.md). User-facing behavior is documented in [Feature FAQ](Feature%20FAQ.md), and the component map is documented in [Technical Reference](TECHNICAL_REFERENCE.md#provider-aware-current-information-routing).
