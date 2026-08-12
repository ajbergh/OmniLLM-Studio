# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and security constraints live in:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`

This file is the durable implementation tracker. It must be updated as phases move, PRs merge, or an implementation constraint is discovered.

## Non-negotiable invariants

1. Arbitrary model-generated processes never execute in the OmniLLM backend process.
2. Sandboxed/local extension processes do not inherit ambient backend secrets or the backend environment by default.
3. Models never supply physical host paths for sandbox mounts; application-owned workspace IDs are used instead.
4. Sandbox IDs are application-issued references and every operation revalidates user/workspace/conversation/run ownership.
5. Filesystem access is explicit: `read_only`, `read_write_no_delete`, or `read_write`.
6. Network is denied by default and may be widened only within operator policy.
7. Descendants inherit sandbox restrictions and cancellation destroys the execution process tree.
8. Runtimes report controls they actually enforce; required-but-unavailable controls fail closed.
9. Raw provider/GitHub/master/session/browser/SSH/cloud credentials are not injected into arbitrary sandboxes.
10. Existing reviewed Git state/digest protections remain authoritative for stage/commit/remote publication.
11. Local plugins and stdio MCP ultimately use the same sandbox boundary as arbitrary code/terminal execution.
12. Multi-user deployments never run arbitrary tenant code in the primary API process/container.

## Roadmap status

Status values: `NOT STARTED`, `IN PROGRESS`, `COMPLETE`, `BLOCKED`.

| Phase | Scope | Priority | Status | Branch / PR | Current evidence / next exit criterion |
|---|---|---:|---|---|---|
| 0 | Architecture, threat model, durable roadmap | P0 | **COMPLETE** | PR #98 — `agent/sandbox-foundation-roadmap-20260812` | Architecture + threat model + tracker implemented. PR #98 Quality Gate and Security Scan passed. |
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | **IN PROGRESS** | PR #101 — `agent/sandbox-protocol-v2-20260812` | Broker issues `sbx_` IDs, revalidates owner/TTL, negotiates runtime capabilities, authenticated HTTP runtime contract added. Merge after CI. |
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | `agent/sandboxd-linux-runtime-20260812` | Linux Bubblewrap runtime and authenticated `sandboxd` worker implemented; focused tests being completed before PR. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **IN PROGRESS** | PR #99 — `agent/sandbox-subprocess-boundary-20260812` | `os.Environ()` inheritance removed, common sanitized runner seam added, ambient-secret regression tests added. Merge after CI. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | NOT STARTED | TBD | Existing public tool contracts should remain stable while execution becomes ownership-bound and OS-sandboxed. |
| 5 | Workspace registry + RO/RW-no-delete/RW mounts + change journal | P1 | NOT STARTED | TBD | Opaque workspace IDs, canonical containment, durable before/after hashes, bounded revert. |
| 6 | Workspace tools: list/search/read/write/apply-patch/delete | P1 | NOT STARTED | TBD | Generic workspace operations use the existing Executor/policy/approval/audit boundary. |
| 7 | `terminal_exec` + cancellation + resource controls | P1 | NOT STARTED | TBD | Arbitrary terminal/build/test commands run only inside sandbox; wall/output/process/resource limits enforced and reported. |
| 8 | Network broker + allowlist/Ask approvals | P1 | NOT STARTED | TBD | Default-deny egress, private/loopback/metadata protections, destination-scoped approvals. |
| 9 | Credential broker + guarded Git/service integration | P1 | NOT STARTED | TBD | Authenticated operations work without raw host credentials entering arbitrary shell environments. |
| 10 | Full local plugin + stdio MCP sandbox migration | P1 | NOT STARTED | TBD | Extension subprocesses cannot exceed declared/effective filesystem/network/env capabilities. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | NOT STARTED | TBD | UI shows grants, network state, running executions, approvals, resource state, and reversible changes. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted identity/token, Job Object/process-tree confinement, ACL-scoped workspace, platform CI evidence. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and platform CI evidence. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Existing AgentRun/job model supports pause/resume/cancel/checkpoint/recovery/scheduling with sandbox state. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Arbitrary tenant execution isolated from API pod/container with worker identity, quotas, network policy, hardened security context. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable agent workspaces with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | begins in #99/#101/Linux runtime | Expand continuously across filesystem, process, resource, network, credential, artifact, tenant and trust-boundary attacks. |

## Active PR stack and merge order

1. **#98** `docs(sandbox): establish agent sandbox parity program` → `main`
2. **#99** `security(sandbox): harden local subprocess environment boundary` → currently stacked on #98; retarget to `main` after #98 merges
3. **#101** `feat(sandbox): add ownership-bound protocol v2 control plane` → currently stacked on #99; retarget after #99 merges
4. Linux first-party runtime PR → stack on #101, then retarget after #101 merges
5. `code_execute` / `python_analysis` Broker convergence
6. workspace registry/journal
7. workspace tools
8. terminal + resource controls
9. network + credential brokers
10. extension migration and desktop UX
11. platform runtimes, durable/remote workers, multi-agent isolation

## Completed implementation evidence

### Phase 0

- durable program tracker;
- target Broker/runtime architecture;
- threat model covering filesystem, process, network, credentials, resources, tenant isolation, artifacts, Git and prompt/tool-result trust;
- clarification that historical Chat Tool Parity Phase 7 completed an external sandbox integration contract, not first-party cross-platform workspace confinement.

### Phase 3 — work implemented in PR #99, pending merge

- `backend/internal/sandbox/process.go` introduces a shared process-construction seam;
- stdio MCP no longer starts with `os.Environ()`;
- plugins use the same sanitized runner seam;
- allowlisted compatibility environment preserves only platform execution essentials plus explicit configured values;
- regression tests prove ambient `OMNILLM_MASTER_KEY`, `GITHUB_TOKEN` and `SSH_AUTH_SOCK` do not leak to generic child environments, and explicit MCP-configured values still work;
- compatibility `HostCommandRunner` is explicitly **not** represented as an OS sandbox.

### Phase 1 — work implemented in PR #101, pending merge

- protocol-v2 owner/mount/network/resource/runtime/session/execution/artifact types;
- `sandbox.Broker` with backend-issued opaque session IDs;
- exact owner-scope checks on exec/cancel/status/destroy;
- session TTL enforcement;
- runtime capability requirements fail closed;
- authenticated HTTP runtime transport requires a bearer token and HTTPS for non-loopback endpoints;
- redirects and oversized runtime responses are rejected;
- arbitrary worker artifact URLs are not part of the v2 artifact trust contract;
- tests cover cross-owner access, expiry, missing runtime controls, service authentication and code/terminal request validation.

### Phase 2 — current Linux branch

Implemented so far:

- `Runtime` abstraction consumed by Broker;
- Linux `LocalRuntime` using Bubblewrap with an operator-configured immutable rootfs;
- network namespace isolation/default no-network execution;
- isolated writable scratch workspace;
- cleared child environment with controlled PATH/HOME/TMPDIR;
- bounded stdout/stderr;
- timeout/context cancellation and tracked execution cancellation;
- runtime capabilities intentionally report memory/CPU/PID/disk limits as **false** until those controls are actually enforced;
- non-Linux local runtime currently fails closed rather than silently using unrestricted host execution;
- authenticated `backend/cmd/sandboxd` protocol-v2 worker;
- focused tests for command/code modes, workspace-relative directory containment, output truncation, capability non-overclaiming, worker authentication and strict JSON request handling.

Still required before Phase 2 can be marked complete:

- CI/build confirmation;
- Linux integration fixture proving Bubblewrap/rootfs isolation on a capable runner;
- application routing/configuration that selects the first-party worker/runtime in normal supported deployments;
- explicit packaging/deployment instructions for the runtime rootfs and Bubblewrap dependency.

## Security acceptance categories

Every relevant phase expands negative tests for:

- traversal, absolute paths, symlink/junction/reparse/hard-link and rename escape;
- orphan/daemon descendants, fork/process abuse and cancellation escape;
- CPU/memory/disk/file-count/output exhaustion;
- localhost/private/link-local/metadata/DNS-rebinding/proxy/network bypass;
- backend/provider/GitHub/master/session/browser/SSH/cloud credential access;
- cross-user/workspace/conversation/run/sandbox/artifact references;
- artifact path/MIME/size/hash attacks;
- Git publication bypass around reviewed state preconditions;
- prompt/tool-result instructions attempting to alter policy.

## Validation policy

Applicable PRs must pass the repository gates before merge:

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

Security-sensitive runtime/deployment work additionally requires Security Scan and applicable container/deployment checks. Platform confinement is not marked complete from cross-compilation alone; platform-native tests are required.

## Progress-update rule

Update this file whenever a phase changes status, a PR/branch is opened or merged, an exit criterion is satisfied, or an implementation limitation changes. A phase is `COMPLETE` only when its stated enforcement properties are implemented and validated; partial or compatibility behavior remains `IN PROGRESS` or `BLOCKED` rather than being overstated.
