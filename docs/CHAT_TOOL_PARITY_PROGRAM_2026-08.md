# Chat Studio Tool and Agent Parity Program — August 2026

This document records the implementation program that brought OmniLLM-Studio's Chat Studio tool, agent, and extensibility experience toward functional parity with mature tool-calling chat runtimes while preserving OmniLLM-Studio's local-first architecture and creative-studio differentiation.

> **Status:** Historical Phases 3–8 are implemented on `main`. This file is retained as the program record and was refreshed on 2026-08-13 so its current-state anchors match the live runtime. New work should be driven by verified gaps, not by assuming an old phase description is still current.

## Invariants

1. A tool that is **Off** is neither advertised nor executed by Chat Studio or Agent Mode.
2. A tool that is **Ask** remains discoverable but cannot execute without approval.
3. Per-chat and per-turn controls may narrow access but never widen a hard Settings deny.
4. Deterministic routing may select a capability, but it must use the same policy and execution boundary as model-selected tools.
5. Capability availability, policy, provider support, credentials, and runtime health are separate states.
6. Large tool catalogs should be discovered progressively instead of injecting every schema into every turn.
7. Authentication is not authorization: a connected GitHub identity, plugin, MCP server, or external credential source never widens operator/tool policy by itself.
8. Arbitrary model-directed process execution must use the sandbox/confinement architecture rather than running in the primary backend process.

## Completed phases

### Phase 3 — Unified capability gateway — COMPLETE

Implemented outcomes:

- Chat Studio and Agent Mode resolve tool availability through the same registry/executor policy boundary.
- Settings `Off`, `Ask`, and `On` behavior is enforced at execution time, not only in the UI.
- Deterministic routing uses the same tool policy as model-selected calls.
- Tool result/error events are surfaced instead of silently disappearing.
- Capability, policy, credentials, and runtime availability remain distinct concepts.

Current anchors include:

- `backend/internal/tools/registry.go`
- `backend/internal/tools/executor.go`
- `backend/internal/api/message_handler.go`
- Chat Studio tool Settings and scoped authorization UI.

### Phase 4 — Tool catalog and progressive discovery — COMPLETE

Implemented outcomes:

- Tool metadata is normalized through the registry.
- Large catalogs can be discovered progressively instead of forcing every schema into every model turn.
- Built-in, plugin, MCP, Git/GitHub, browser, sandbox, sports, and creative capability families use the same discoverability/policy concepts.
- Tool descriptions remain model-facing contracts rather than permission grants.

### Phase 5 — Agent loop and approval lifecycle — COMPLETE

Implemented outcomes:

- Multi-step agent execution shares the normal tool executor.
- Side-effecting/high-risk tools can stop for approval and resume after a decision.
- Tool lifecycle/result/error events remain visible to the conversation rather than being swallowed.
- Cancellation and bounded execution are part of the runtime contract.

Durable long-running task scheduling/recovery remains a separate sandbox/agent roadmap item; completion of this historical phase did not imply a persistent Codex/Cowork-style task scheduler.

### Phase 6 — Extensibility and MCP/plugin governance — COMPLETE

Implemented outcomes:

- Local plugins and stdio MCP servers are first-class extension surfaces.
- Extension tools are still filtered through the normal registry/executor policy.
- Ambient backend secret inheritance was removed from local extension subprocesses.
- `OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off` defines the confinement policy boundary.
- Linux can use Bubblewrap/rootfs confinement; Windows now uses native AppContainer/Job confinement when available; macOS native confinement remains Phase 13 work.

Current sandbox/runtime details live in:

- `docs/SANDBOX_RUNTIME.md`
- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_ROADMAP_2026-08.md`

### Phase 7 — Governed local compute and developer workflows — COMPLETE as a tool-surface milestone

The original Phase 7 goal was to add code execution, terminal/file/Git workflows, and safer local developer tooling without giving the model unrestricted host access.

#### Current code-execution architecture

`code_execute` no longer depends on the retired unauthenticated one-shot `/v1/execute` design.

Current flow:

```text
Chat / Agent tool call
        |
        v
Tool Executor policy / approval
        |
        v
sandbox.Broker
owner-bound sbx_ session + capability checks
        |
        v
protocol-v2 Runtime
local Linux/Windows or authenticated HTTP worker
        |
        v
OS-confined process
```

`backend/internal/tools/code_sandbox_tool.go` creates/reuses application-issued `sbx_...` sessions through `backend/internal/sandbox.Broker`. Ownership is revalidated on reuse; network is off by default; runtime capability requirements fail closed.

`python_analysis` keeps its restricted Python policy as defense in depth but also executes through the Broker/runtime path. It does not silently fall back to unrestricted host Python.

Runtime configuration is documented in `docs/SANDBOX_RUNTIME.md`. The primary protocol-v2 HTTP settings are:

```text
OMNILLM_SANDBOX_URL=<authenticated-protocol-v2-worker>
OMNILLM_SANDBOX_TOKEN=<service-token>
```

`OMNILLM_CODE_SANDBOX_URL` is retained only as a URL compatibility alias for the protocol-v2 worker; it is not the old `/v1/execute` contract.

Linux and Windows have first-party local confinement implementations. Windows native confinement Phase 12 is complete. macOS native confinement is not yet implemented.

#### Workspace and terminal controls

- Workspaces are identified by opaque application-owned IDs; model tool arguments do not receive physical host roots.
- Workspace mutations use governed list/search/read/write/apply-patch/delete/revert tools plus state-bound journaling.
- `terminal_exec` uses explicit argv execution and the sandbox Broker/runtime boundary.
- Current terminal project access is deliberately read-only; source mutation remains in journaled workspace tools.
- Resource/capability claims are fail-closed and reflect controls actually enforced by the selected runtime.

#### Guarded Git and GitHub workflow

The local/host-side Git workflow now extends well beyond the original Phase 7 baseline:

```text
workspace edit/build/test
        |
        v
git_status / git_diff
        |
        v
reviewed local-state digest
        |
        v
git_stage / git_commit
        |
        v
git_remote_status / git_fetch
        |
        v
git_push / git_publish_branch
        |
        v
GitHub PR / review / merge tools
```

Guarded Git mutations remain bound to reviewed repository/worktree/head state. GitHub collaboration includes PR inspection, check/status inspection, feedback/review threads, draft PR creation, review replies, thread resolution, draft-to-ready transition, merge-policy/eligibility inspection, and guarded direct merge.

`github_merge_pull_request` is **implemented**. It is a critical-risk mutation that accepts only configured `remote`, positive PR `number`, and exact 40-character `expected_head`. The server performs a fresh fail-closed eligibility preflight, requires the current default base and operator-configured allowed merge method, sends one exact-head merge request, does not delete the source branch, and does not automatically retry an ambiguous mutation outcome.

Current anchors include:

- `backend/internal/gitrepo/github_pull_request_merge.go`
- `backend/internal/tools/github_pull_request_merge_tool.go`
- `backend/internal/gitrepo/github_pull_request_merge_eligibility.go`
- `backend/internal/tools/github_pull_request_merge_eligibility_tool.go`

The old statement that “M3B guarded direct merge is the next gap” is no longer current.

### Phase 8 — Reliability, observability, and user-facing trust — COMPLETE as the historical parity phase

Implemented outcomes include:

- bounded tool results and explicit error results;
- lifecycle/progress events for tool and generation workflows;
- approval-state visibility;
- cancellation/timeout propagation through major tool paths;
- safer handling of provider/tool failures rather than silent success;
- expanded smoke/unit/integration coverage across Chat Studio tools and Settings behavior.

The repository's current Quality Gate, Security Scan, native Windows confinement jobs, Playwright smoke suite, Helm checks, and applicable container builds provide the ongoing regression boundary. This does not mean every future capability is complete; it means the historical Phase 8 reliability foundation is on `main`.

## Post-program current-state refresh — 2026-08-13

### GitHub App identity and repository bindings

OmniLLM-Studio now has a first-class user-scoped GitHub App connection and repository-binding architecture documented in `docs/GITHUB_APP_AUTH.md`.

Important current semantics:

- user GitHub credentials are credential sources only;
- repository bindings are owner-scoped associations, not permission grants;
- static operator remotes remain authoritative on ID collisions;
- operator-owned binding capability policy is snapshotted at startup;
- binding-backed Git/GitHub tool shells are registered only when startup operator policy plus existing process-wide gates can potentially authorize them;
- actual invocation still revalidates owner binding/account identity, credentials, exact per-remote policy, reviewed Git state, approval, and global gates;
- binding-derived policy cannot grant clone or default-branch push.

This closes the earlier gap where a user could connect GitHub but binding-only repositories could not surface the appropriate governed tool families.

### Guarded direct merge

M1 merge requirements, M2 actor/policy evidence, M3A current-state eligibility, and M3B guarded merge are all implemented in code. Any documentation that still calls M3B future work should be treated as stale and updated against current `main`.

The merge implementation deliberately remains stricter than ordinary collaboration mutations because it changes the configured default branch.

### Sandbox maturity

The sandbox program has advanced independently beyond the historical Phase 7 tool milestone:

- protocol-v2 Broker sessions and ownership checks are on `main`;
- Linux first-party Bubblewrap confinement is available;
- Windows AppContainer/Job confinement and persistent extension confinement are implemented and natively adversarial-tested;
- workspace grants/journaling/change review are implemented;
- network destination authorization exists, but first-party destination-enforced egress is not yet implemented;
- memory/CPU/PID/physical-disk quotas are not yet advertised;
- macOS native confinement remains open;
- packaged dedicated server/Kubernetes workers and durable sandbox-backed tasks remain open.

## Verified remaining gaps

The following are current gaps after refreshing this program against live `main`; they replace older “next phase” assumptions.

### P0/P1 — sandbox/runtime completion work

Tracked in `docs/AGENT_SANDBOX_ROADMAP_2026-08.md`:

- macOS native confinement (Phase 13);
- broader workspace path-component TOCTOU/rename-swap assurance outside staged-copy flows;
- memory, CPU, PID/process-count, and physical-disk quotas;
- first-party destination-enforced egress/allowlisting;
- service-specific credential broker consumers;
- `sandboxd` packaging and dedicated server/Kubernetes worker deployment;
- durable sandbox-backed task scheduling/recovery;
- multi-agent isolated worktrees/workspaces.

Issue #151 separately tracks explicit execution-ID cancellation addressability for active synchronous sandbox executions. Context cancellation, timeout, session destruction, and process-tree teardown work; the explicit ID-addressed cancellation contract is a narrower lifecycle defect.

### P1/P2 — developer collaboration breadth

The repository now has guarded local Git, remote publication, extensive PR review workflows, and guarded merge. Remaining parity opportunities should be evaluated individually rather than treated as a missing generic “GitHub merge” phase. Candidate areas include:

- richer issue/project/discussion lifecycle tools where they support real Chat Studio workflows;
- explicit remote branch cleanup/deletion with separate high-risk gates if operator demand justifies it;
- managed multi-worktree lifecycle tied to future multi-agent isolation rather than unrestricted raw Git worktree commands;
- deeper CI/action log diagnostics surfaced through bounded, repository-scoped tools;
- release/tag workflows only with similarly strict reviewed-state and operator-policy boundaries.

### P2 — durable agent execution

For Codex/Cowork-style parity, the major remaining difference is not basic tool calling; it is durable execution orchestration:

- persisted task/sandbox association;
- pause/resume/cancel/recovery after process or application restart;
- queued scheduling/admission control;
- durable checkpoints and artifacts;
- isolated parallel agents/worktrees;
- dedicated worker execution in server/Kubernetes modes.

These belong to the sandbox/agent task roadmap rather than another expansion of the synchronous Chat tool catalog.

### P2 — external-service breadth

Plugin/MCP extensibility is in place, but built-in integration breadth can still grow where a first-party experience materially improves safety or UX. Any new service integration should preserve the same distinction between credential connection, operator authorization, scoped approval, and runtime/tool availability.

## Current architecture anchors

When reviewing or extending Chat Studio tools, start from these live areas rather than the historical implementation order:

### Tool policy and execution

- `backend/internal/tools/registry.go`
- `backend/internal/tools/executor.go`
- `backend/internal/api/message_handler.go`
- tool Settings/scoped authorization frontend components

### Sandbox/code/terminal/workspace

- `backend/internal/sandbox/`
- `backend/internal/tools/code_sandbox_tool.go`
- `backend/internal/tools/python_analysis_tool.go`
- `backend/internal/tools/terminal_exec_tool.go`
- workspace tool/journal implementation under `backend/internal/sandbox/` and `backend/internal/tools/`
- `docs/SANDBOX_RUNTIME.md`
- `docs/AGENT_SANDBOX_ROADMAP_2026-08.md`

### Git/GitHub

- `backend/internal/gitrepo/`
- `backend/internal/tools/git_*`
- `backend/internal/tools/github_*`
- `docs/GITHUB_APP_AUTH.md`
- current GitHub merge policy/eligibility implementation and tests

### Plugins/MCP/browser/creative tools

- plugin and MCP managers/runners under `backend/internal/`
- browser/headless-browser tools and SSRF/session controls
- image/music/video generation tool families and their provider-specific adapters

## Validation expectations for new parity work

Any future parity slice should be considered complete only when:

1. availability/registration is correct;
2. Settings/scoped policy is enforced by the Executor;
3. model-facing schema does not expose application-owned security inputs;
4. side effects have an appropriate risk/approval class;
5. errors are visible and bounded;
6. cancellation/timeout semantics are defined;
7. backend/frontend/API contracts are updated together when applicable;
8. focused negative tests prove the permission boundary;
9. the relevant repository Quality/Security/platform/container gates pass on the exact final head;
10. durable docs are updated to describe what is actually implemented, not the intended future state.

## Program conclusion

The August 2026 parity program successfully moved OmniLLM-Studio from a growing collection of tool integrations to a governed capability runtime with shared policy, extensibility, sandboxed local execution, workspace/Git workflows, user-scoped GitHub integration, and guarded hosted collaboration through merge.

The next maturity gains are predominantly **runtime isolation across remaining platforms, durable agent/task orchestration, resource/network enforcement, and carefully selected collaboration/service breadth**—not another wholesale rewrite of Chat Studio's core tool-calling architecture.
