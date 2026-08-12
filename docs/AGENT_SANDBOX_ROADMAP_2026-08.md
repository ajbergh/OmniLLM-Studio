# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products. The program preserves the existing scoped tool policy, approval framework, guarded Git workflow, browser isolation, local-first architecture, and multi-user ownership model.

## Why this program exists

OmniLLM-Studio already has strong control-plane governance: Allow / Ask / Deny policy, global-to-conversation scoped permission tightening, request-scoped tool restrictions, approval events, timeouts, result limits, and guarded Git mutations bound to reviewed repository state.

The remaining parity gap is the execution boundary. Today:

- `code_execute` delegates isolation to an operator-configured external HTTP service;
- `python_analysis` executes a restricted language subset in a temporary host process and explicitly is not an OS sandbox;
- local plugins execute as ordinary backend child processes;
- stdio MCP servers execute as ordinary backend child processes and currently inherit the host environment before configured overrides are appended;
- there is no first-class sandboxed coding workspace with generic read/search/write/patch/terminal capabilities, change journaling, or workspace access modes.

The target architecture makes the shared tool Executor the authorization/control plane and a new sandbox Broker/runtime the execution/data plane.

## Non-negotiable invariants

1. **No arbitrary model-generated process executes in the OmniLLM backend process.**
2. **No sandboxed process receives ambient backend credentials or the backend environment by default.**
3. **The model never supplies host filesystem paths for sandbox mounts.** It receives opaque workspace IDs created by the application/operator.
4. **Sandbox session IDs are backend-issued and ownership-bound** to user/workspace/conversation/agent-run context.
5. **Filesystem access is explicit**: read-only, read/write-no-delete, or read/write.
6. **Network is denied by default** and may be opened only through application policy with bounded destination rules.
7. **Descendant processes inherit the sandbox restrictions.** Killing/cancelling a sandbox execution terminates its process tree.
8. **Resource use is bounded** by wall time, CPU, memory, PID/process, disk, and output limits where the platform/runtime supports them.
9. **Sandbox artifacts are promoted through application-owned IDs and validation**, not trusted arbitrary URLs returned by a worker.
10. **Local plugin and stdio MCP subprocesses ultimately use the same sandbox execution boundary** as model-generated commands.
11. **Credentials are brokered host-side**; provider secrets, GitHub credentials, `OMNILLM_MASTER_KEY`, browser cookies, and host SSH/cloud-agent credentials are not injected into arbitrary sandboxes.
12. **Workspace mutations are journaled and reviewable** with before/after hashes and task ownership.
13. **Existing guarded Git tools remain authoritative for reviewed stage/commit/remote publication workflows.** A sandbox terminal must not silently replace stale-approval/state-binding protections.
14. **Multi-user server deployments isolate tenant execution from the primary API process and from other tenants.**
15. **Sandbox policy and tool approval are separate layers.** A tool may be approved yet still be technically constrained by the sandbox.

## Target architecture

```text
Chat / Agent / durable task
          |
      Tool Executor
 Allow / Ask / Deny / scope
          |
     Sandbox Broker
   /        |         \
workspace  network  credentials
  policy    policy     broker
      \       |        /
       Sandbox Runtime
      (first-party sandboxd)
        /      |      \
     Linux   Windows  macOS
        \      |      /
     untrusted descendants
 code / shell / build / MCP / plugin
```

The Broker belongs in the normal backend composition root and never directly executes arbitrary commands. The runtime is an independently constrained execution component with a versioned protocol.

## Roadmap and progress

Status values: `NOT STARTED`, `IN PROGRESS`, `COMPLETE`, `BLOCKED`.

| Phase | Scope | Priority | Status | Branch / PR | Exit criteria |
|---|---|---:|---|---|---|
| 0 | Architecture, threat model, durable roadmap | P0 | IN PROGRESS | `agent/sandbox-foundation-roadmap-20260812` | Security boundary and acceptance criteria are documented and merged. |
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | NOT STARTED | TBD | Versioned create/exec/status/cancel/artifact lifecycle; service auth; no model-chosen reusable session IDs. |
| 2 | First-party sandbox runtime abstraction + Linux implementation seam | P1 | NOT STARTED | TBD | Backend can use a first-party runtime implementation instead of requiring an unspecified external service; runtime exposes enforced policy/capability status. |
| 3 | Immediate subprocess hardening: sanitized env + broker-ready process abstraction for stdio MCP/plugins | P0 | NOT STARTED | TBD | No `os.Environ()` inheritance for stdio MCP; plugin/MCP launch paths use a constrained runner abstraction and have regression tests. |
| 4 | Sandbox `python_analysis` and generic code execution through Broker | P1 | NOT STARTED | TBD | Restricted Python remains defense-in-depth but no longer launches directly outside the shared sandbox execution boundary. |
| 5 | Workspace model + RO/RW-no-delete/RW mounts + change journal | P1 | NOT STARTED | TBD | Opaque workspace IDs, ownership checks, containment, before/after hashes, revertable task changes. |
| 6 | Workspace tools: list/search/read/write/apply-patch/delete | P1 | NOT STARTED | TBD | Generic coding/document workspace operations are bounded, scoped, auditable, and policy governed. |
| 7 | `terminal_exec` + process-tree cancellation + resource controls | P1 | NOT STARTED | TBD | Arbitrary terminal/build/test commands execute only inside sandbox; limits are reported/enforced. |
| 8 | Network broker + destination allowlist/Ask approvals | P1 | NOT STARTED | TBD | Default-deny network, SSRF/private/metadata protections, exact destination approval semantics. |
| 9 | Credential broker + Git integration without secret injection | P1 | NOT STARTED | TBD | Sandbox can perform workflows requiring authenticated host services without receiving raw host secrets. |
| 10 | Plugin + stdio MCP full sandbox migration | P1 | NOT STARTED | TBD | Local extensions cannot access host filesystem/network/environment outside manifest/policy grants. |
| 11 | Desktop workspace/sandbox UX + visible permissions/change review | P1 | NOT STARTED | TBD | User can see workspace mode, network policy, running commands, resource status, approvals, and change journal. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted token/job/process-tree/ACL confinement implemented and exercised on Windows CI. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced macOS runtime implemented and exercised on macOS CI. |
| 14 | Durable sandbox-backed agent tasks: pause/resume/checkpoint/schedule | P2 | NOT STARTED | TBD | Persisted AgentRun/task maps cleanly to recoverable sandbox state and artifacts. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Arbitrary tenant code never runs in primary API pod/container; worker security context/network/resource policies documented and validated. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Parallel agents have independent writable workspaces and controlled promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | NOT STARTED | TBD | Filesystem/process/resource/network/credential/tenant escape cases execute on supported platform CI. |

## Phase 0 — Architecture and threat model

Deliverables:

- `docs/AGENT_SANDBOX_ROADMAP_2026-08.md`
- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`

This phase also corrects terminology: the historical Chat Tool Parity Phase 7 completed the **external code-sandbox integration contract**, not a first-party cross-platform workspace sandbox implementation.

## Phase 1 — Sandbox Protocol v2

Proposed backend package direction:

```text
backend/internal/sandbox/
```

The protocol must represent application-derived:

- owner scope;
- workspace mounts and access modes;
- network mode and allowed destinations;
- runtime/image/profile;
- environment allowlist;
- resource limits;
- TTL/lifetime;
- artifact IDs and hashes.

Recommended lifecycle:

```text
POST   /v2/sandboxes
POST   /v2/sandboxes/{id}/exec
POST   /v2/sandboxes/{id}/cancel
GET    /v2/sandboxes/{id}/status
GET    /v2/sandboxes/{id}/changes
GET    /v2/sandboxes/{id}/artifacts
DELETE /v2/sandboxes/{id}
```

Desktop-local transport should prefer a Unix socket or Windows named pipe where practical. Remote/server workers require authenticated transport; sandbox authentication credentials are application-owned and never model-visible.

## Phase 2 — First-party sandbox runtime

Proposed executable:

```text
backend/cmd/sandboxd/
```

The runtime is selected behind an interface so platform implementations can differ without changing tool contracts.

Linux target controls:

- user/mount/PID/network namespaces where supported;
- no-new-privileges;
- read-only runtime root;
- explicit bind mounts;
- cgroup v2 resource limits where available;
- seccomp or an established confinement helper;
- process-tree teardown;
- no ambient host environment or credentials.

Higher-assurance server deployments may use gVisor/Kata/microVM-backed workers behind the same Broker contract.

## Phase 3 — Immediate subprocess hardening

This phase is intentionally early because it addresses a concrete current risk independent of the complete sandbox runtime.

Required changes:

- stdio MCP no longer starts with `os.Environ()`;
- process environments are explicit allowlists;
- plugin and MCP launch code depends on a shared process/sandbox runner abstraction;
- runtime status makes it clear when a local extension is sandboxed versus compatibility/legacy execution;
- default behavior fails closed for configurations that explicitly require sandboxed extensions.

## Phase 4 — Python and code execution convergence

`python_analysis` retains AST/builtin restrictions but runs inside the common sandbox. `code_execute` moves from a thin external-service adapter to the shared Broker/session model.

The existing public tool name may remain stable while the internal runtime changes.

## Phase 5 — Workspace model

A workspace is an application-owned, opaque mapping to one or more physical roots. Models receive workspace IDs, never raw host roots.

Access modes:

```text
read_only
read_write_no_delete
read_write
```

Each write records:

- workspace ID;
- sandbox/task/agent-run ID;
- relative path;
- operation;
- before hash/presence;
- after hash/presence;
- timestamp;
- optional patch summary.

The journal supports review and bounded revert semantics.

## Phase 6 — Workspace tools

Proposed capabilities:

```text
workspace_list
workspace_search
workspace_read
workspace_write
workspace_apply_patch
workspace_delete
```

Deletion remains independently higher risk. Tools must use the existing Executor, scoped policy, approval, idempotency, timeout, event, and request restriction behavior.

## Phase 7 — Sandboxed terminal

Proposed capability:

```text
terminal_exec
```

It accepts a command/argv and an opaque sandbox/workspace context. It never accepts an arbitrary host working directory.

Execution reports bounded stdout/stderr, exit status, duration, resource/cancellation metadata, and produced artifact IDs.

## Phase 8 — Network broker

Modes:

```text
none
allowlist
approval_required
```

All modes preserve private/loopback/link-local/cloud-metadata/reserved-address protections unless an explicit operator-only deployment policy says otherwise. Model/user approval must not silently override hard deployment restrictions.

Approval cards should include destination hostname/IP/port, requested action, and scope (`once` versus task/session where supported).

## Phase 9 — Credential broker

Raw credentials remain host-side. Sandboxes call application-owned capability brokers for operations such as guarded Git/GitHub publication or configured external services.

Do not inject:

- provider API keys;
- GitHub tokens;
- `OMNILLM_MASTER_KEY`;
- session secrets;
- browser cookies;
- SSH agent handles;
- cloud metadata credentials.

Existing request-scoped GitHub credential work and guarded Git tools are preferred integration points.

## Phase 10 — Plugin and MCP migration

Plugins and stdio MCP become sandbox workloads with declared capability requests. Plugin/MCP manifests/configuration may request filesystem/network/env capabilities, but application/operator policy remains authoritative.

A plugin signature/provenance mechanism is complementary; it does not replace runtime confinement.

## Phase 11 — Desktop UX

The desktop UI must answer at all times:

1. What can this agent/process read?
2. What can it write/delete?
3. Can it reach the network, and where?
4. What is currently running?
5. What changed?
6. What approval is being requested and why?

Likely integration areas include Chat Studio, Agent Mode, Settings, Tool Picker, and a dedicated workspace/sandbox status/change-review surface.

## Phase 12/13 — Windows and macOS enforcement

Windows must use platform confinement primitives rather than treating a temporary directory as a sandbox. The implementation should cover restricted identity/token behavior, Job Object/process-tree management, controlled handles/environment, ACL-scoped workspace access, and network restrictions where feasible.

macOS must use OS-enforced sandboxing/confinement suitable for shipped desktop builds, with explicit workspace/network capabilities and process-tree inheritance.

## Phase 14 — Durable Cowork-style tasks

Extend existing AgentRun/job persistence rather than creating a second orchestration engine. Persist the association between durable task/run and sandbox/workspace/change/artifact state.

Capabilities:

- pause;
- resume;
- cancel;
- checkpoint;
- recover after process restart where runtime allows;
- scheduled/recurring tasks;
- durable artifacts and task history.

A local desktop cannot continue while the host is powered off; that parity requires Phase 15 remote workers.

## Phase 15 — Server/Kubernetes workers

The primary API process/pod must never execute arbitrary tenant commands. Use dedicated workers/pods/jobs with:

- non-root execution;
- read-only root filesystem where possible;
- no privilege escalation;
- dropped Linux capabilities;
- seccomp profile;
- ephemeral storage;
- CPU/memory/PID quotas;
- network policy;
- tenant ownership binding;
- authenticated server/worker control channel.

## Phase 16 — Multi-agent isolation

Each agent gets its own writable workspace/worktree. Promotion back to the canonical repository passes through reviewed change inspection and existing Git state-binding/precondition logic.

## Phase 17 — Assurance suite

Required adversarial categories:

### Filesystem

- traversal and absolute paths;
- symlink/junction/reparse escapes;
- hard links;
- rename races;
- device/proc/special-file access;
- case/normalization edge cases;
- delete/replace outside mounted roots.

### Process

- fork/process bombs;
- orphan/daemon children;
- nested shells;
- inherited handles/descriptors;
- process-tree escape after timeout/cancel.

### Resources

- CPU exhaustion;
- memory exhaustion;
- disk fill;
- huge file counts;
- huge stdout/stderr.

### Network

- localhost/private/link-local/metadata;
- DNS rebinding;
- IPv6 bypass;
- direct-IP/proxy bypass;
- Unix/named socket escape.

### Credentials

Attempt access to backend environment, provider secrets, master key, GitHub credentials, browser state, SSH agents, and cloud metadata credentials.

### Tenant isolation

Cross-user/workspace/conversation/agent-run/sandbox access attempts must fail.

### Prompt/tool-result trust

Sandbox stdout, plugin/MCP results, downloaded files, and network content remain untrusted data and never gain instruction authority merely because they came from a tool.

## PR sequencing

The recommended merge order is intentionally dependency-aware:

1. Architecture + threat model + roadmap.
2. Immediate subprocess environment hardening and runner seam.
3. Protocol v2 + backend ownership/session model.
4. First-party runtime abstraction/Linux implementation.
5. Python/code execution convergence.
6. Workspace model and journal.
7. Workspace tools.
8. Terminal/resource controls.
9. Network broker.
10. Credential broker/Git integration.
11. Plugin/MCP complete migration.
12. Desktop UX.
13. Windows runtime.
14. macOS runtime.
15. Durable tasks.
16. Server/Kubernetes workers.
17. Multi-agent worktrees.
18. Continuous adversarial suite expansion.

## Validation policy

Every sandbox PR should run the applicable subset of the repository's canonical gates:

```bash
cd backend
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm run lint
npm run test:unit
npm run build

cd ..
npx playwright test --project=chromium
```

Security-sensitive runtime/deployment changes also require the repository security and container/deployment workflows where applicable. Platform sandbox implementations require platform-native CI coverage; they must not be declared complete solely from compilation on another OS.

## Completion definition

OmniLLM-Studio reaches the intended sandbox parity milestone when:

- arbitrary agent processes cannot read/write outside explicit workspace grants;
- descendants cannot escape sandbox policy;
- network is default-deny and approval/allowlist governed;
- backend secrets are not ambient in sandboxes;
- CPU/memory/disk/PID/time/output are bounded and observable;
- sandbox IDs are unguessable/application-issued and ownership checked;
- plugins and stdio MCP use the same execution boundary;
- workspace changes are visible/revertable;
- users can select RO/RW-no-delete/RW access;
- Windows/macOS/Linux desktop modes have OS-level enforcement;
- server/Kubernetes execution is isolated from the API process and other tenants;
- durable tasks can pause/resume/checkpoint and remote workers can continue without the desktop;
- the adversarial suite continuously exercises escape classes.

## Progress update protocol

Every PR in this program must update the roadmap table in this file before merge. A phase may be marked `COMPLETE` only when its exit criteria are implemented and validated. Partial work remains `IN PROGRESS`; intentionally deferred platform behavior must be stated explicitly rather than treated as complete.
