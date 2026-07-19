# Chat Studio Agent Runtime Implementation

**Branch:** `agent/chat-studio-agent-runtime-20260718`  
**Pull request:** #24

## Executive status

The branch contains a production-oriented baseline across the Chat Studio tools and Agent roadmap. Formatting, compilation, backend tests, race detection, frontend lint/unit/build, full Chromium Playwright specifications, CodeQL, vulnerability audits, container builds, and Helm checks passed on commit `1264499a53ccabf62bdde08fd7e67ce3ce6f7ada`.

The pull request remains a draft pending a focused composition-root audit. The audit must confirm that every service and handler described below is constructed, registered, routed, and shut down through `backend/internal/api/router.go`; source presence and green package tests alone do not prove end-to-end reachability.

## Phase 0 — Tool execution foundation

Implemented source modules include:

- Versioned tool contracts with schemas, risk, side-effect, timeout, output-size, and parallel-safety metadata
- Stable registry ordering and tool retrieval
- Structured tool lifecycle events and results
- SQLite-backed invocation audit records
- Approval broker APIs with editable arguments and explicit execution
- Global Chat Studio approval UI and continuation support

## Phase 1 — Agent and Research runtime

Implemented source modules include:

- Chat, Research, and Agent profiles
- Plan validation and repair
- Dependencies, retries, approval declarations, and parallel groups
- Checkpoint persistence, pause, cancel, and resume
- Interrupted-run recovery
- Adaptive recovery planning
- Safe parallel execution for read-only tools
- Per-run model, tool, duration, cost, and step budgets
- Append-only events and cursor replay
- Final-response persistence

## Phase 2 — First-party tools and asynchronous jobs

Implemented source modules include:

- Date/time and unit conversion
- Weather and ECB currency conversion
- Opt-in constrained Python analysis
- Durable job status and cancellation
- Image, Music, Video, and artifact-generation job adapters

The Python capability is a constrained subprocess and is not a hardened operating-system or container security boundary.

## Phase 3 — Research and connected apps

Implemented source modules include:

- Research-specific planning instructions
- Governed MCP-backed app catalog
- Per-user and per-workspace app mappings
- Declared read/write scopes
- Existing tool-policy enforcement for app actions

A generalized encrypted OAuth broker is not included.

## Phase 4 — Tasks and controlled memory

Implemented source modules include:

- One-time, recurring, and condition-watch task scheduling
- Restart recovery, pause, resume, and deletion
- Scoped memory records with expiration and source references
- Review, update, and deletion APIs
- Sensitive-token rejection and approval-required writes

External email, push, webhook, and operating-system notification adapters are not included.

## Phase 5 — Evaluation and safety

Implemented source modules include:

- Planner/tool-selection evaluation scenarios
- Expected and forbidden tool checks
- Argument validation and approval checks
- Durable runtime audit events
- Output and timeout enforcement
- Risk-aware default policies

## Validation fixes completed

- Removed the task-tool import cycle by moving task adapters from `internal/tools` to `internal/tasktools`.
- Corrected Agent Runtime TypeScript errors and upgraded the application TypeScript target/library to ES2021.
- Applied `gofmt` to all changed Go sources identified by CI.
- Enhanced CI formatting diagnostics.
- Verified all configured quality, security, browser, container, and Helm workflows on the validated code head.

## Remaining work before ready-for-review

1. Audit and complete `backend/internal/api/router.go` construction and route registration for:
   - Runtime schema initialization
   - Tool and Agent event recorders
   - Job manager and Studio job tools
   - Memory service, tools, and authenticated APIs
   - Task scheduler, task tool adapters, authenticated APIs, and shutdown hook
   - Connected-app service, tools, and authenticated APIs
   - Agent evaluation endpoint
   - Agent event replay endpoint
2. Verify default permissions for every newly registered side-effecting tool.
3. Add end-to-end tests proving the new authenticated routes and tools are reachable through the production router.
4. Convert additive schema creation into numbered migrations before a stable release.
5. Keep Gemini generic function calling disabled until thought-signature preservation is verified end to end.
