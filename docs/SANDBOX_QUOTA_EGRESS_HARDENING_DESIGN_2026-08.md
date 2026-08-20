# Sandbox quota and egress hardening design — August 2026

> **Status:** APPROVED IMPLEMENTATION CONTRACT / PARTIALLY IMPLEMENTED
>
> This document is the enforcement contract for `MASTER_PLAN.md` and `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`. Resource and arbitrary-process network capability bits remain false until platform-native enforcement and adversarial evidence are merged and green on the platform that advertises them. As of 2026-08-19, trusted **host-mediated brokered HTTP** destination enforcement is already implemented in `backend/internal/sandbox/brokered_http.go`; the remaining Phase 8 gap is forced destination-scoped egress for arbitrary sandbox process sockets.

## Goals

1. Add resource controls one independently verifiable primitive at a time.
2. Keep `RuntimeCapabilities` truthful per runtime and platform.
3. Preserve default-deny arbitrary-process networking until destination enforcement exists below untrusted code.
4. Reuse the existing host-mediated brokered HTTP boundary rather than duplicating it inside arbitrary runtimes.
5. Reject requested controls at session creation when a runtime cannot enforce them.
6. Require native negative evidence before a capability bit changes from false to true.

## Non-goals

- No broad arbitrary-process network enablement based only on Broker grants.
- No DNS-only allowlist checks followed by an independent connect-time lookup.
- No universal capability claim based on one operating system.
- No reliance on wall-time/output bounds as substitutes for memory, CPU, PID, or physical-disk quotas.
- No tenant arbitrary-code execution in the primary API process/container.
- No replacement of the existing trusted host-mediated `BrokeredHTTPClient` with a weaker direct-socket path.

## Resource capability contract

A runtime may advertise a resource capability only when a non-zero request value is translated into an operating-system/runtime enforcement primitive before untrusted code can exceed the requested authority.

| Capability | Request field | Required semantics |
|---|---|---|
| `memory_limit` | `resources.memory_bytes` | Bound aggregate sandbox execution memory at or below the configured byte ceiling, including preventing separately governed swap authority from defeating that ceiling; child processes must not evade the same bound. |
| `cpu_limit` | `resources.cpu_time_ms` | Bound aggregate CPU consumption for the sandbox execution/process tree; wall time is not equivalent. |
| `pid_limit` | `resources.max_processes` | Bound the number of concurrently active processes in the sandbox execution tree. |
| `disk_limit` | `resources.disk_bytes` | Bound bytes attributable to sandbox-writable storage, including runtime-owned workspace/home/tmp and generated artifacts where applicable. |

Zero values mean no caller-requested quota for that resource; they do not weaken other default runtime controls.

## Platform plan

### Windows AppContainer runtime

**Primitive:** Job Objects already exist at process creation through `PROC_THREAD_ATTRIBUTE_JOB_LIST`.

Implementation order:

1. **PID limit — implemented** using `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` on the same Job Object that already has `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`.
2. **Memory — implemented** using an aggregate Job Object memory limit (`JOB_OBJECT_LIMIT_JOB_MEMORY`) so descendants share the ceiling.
3. **CPU — rebuilt for validation in PR #232** using aggregate Job Object CPU accounting and whole-Job termination primitives; public capability promotion remains separate and fail-closed.
4. **Disk later** — AppContainer filesystem confinement does not itself meter physical bytes; implement explicit runtime-owned writable-root accounting/preflight plus enforcement that cannot be bypassed by alternate writable locations.

Required native evidence for every promoted resource capability:

- capability false before implementation and true only after the exact native test is present;
- creation succeeds when the matching requirement is requested and the quota is non-zero;
- a process-tree attempt beyond the ceiling is constrained while confinement remains intact;
- cancellation/timeout still tears down the Job tree;
- an unrelated concurrent sandbox is unaffected;
- zero-valued quotas preserve existing behavior.

### Linux Bubblewrap runtime

**Primitive boundary:** Bubblewrap supplies namespace/filesystem/network confinement but is not a complete resource controller. Resource enforcement uses a delegated cgroup-v2 boundary owned by `sandboxd` or an equivalent separately privileged worker boundary, not shell `ulimit` as the authoritative capability.

Current state:

1. writable/delegated cgroup-v2 support is detected at runtime startup;
2. one cgroup is created per sandbox execution before untrusted code starts;
3. `max_processes` maps to `pids.max`;
4. positive `memory_bytes` maps to `memory.max` with `memory.swap.max=0`, and native assurance observes `memory.events` for OOM enforcement evidence;
5. cumulative CPU primitives are rebuilt in PR #232: aggregate `cpu.stat usage_usec`, a bounded `cpu.max` sampling ceiling, final accounting, and whole-cgroup `cgroup.kill`; capability promotion remains gated;
6. writable runtime roots still need a separate physical-disk accounting design because cgroups do not provide a portable physical-disk-byte quota.

PID, memory, CPU, and disk capabilities remain independent. If the worker lacks a required delegated controller or strict interface, the corresponding capability remains false and a non-zero request for that resource fails closed.

### macOS Seatbelt runtime

Seatbelt provides filesystem/network policy, not the required aggregate resource metering. Do not advertise memory/CPU/PID/disk controls merely because process-group teardown exists.

Implementation candidates must be proven individually. In particular:

- `setrlimit`/launch-time limits may be useful for process-local controls but do not automatically satisfy aggregate descendant semantics;
- the known detached-descendant limitation means PID/CPU/memory claims require evidence that the chosen control follows or independently constrains detached descendants;
- `process_tree_isolation` remains false until the existing detached-process limitation is actually removed.

No macOS resource capability bit changes under this design alone.

## Destination-scoped egress contract

### Implemented trusted host-mediated path

`backend/internal/sandbox/brokered_http.go` already provides a destination-enforced HTTP/HTTPS egress path **outside** arbitrary sandbox process networking. It is intentionally narrower than `network_allowlist` and must remain available without implying that untrusted code has direct socket authority.

The current `BrokeredHTTPClient`:

- resolves every `NetworkGrant` against the exact `OwnerScope`;
- limits methods and request/response sizes;
- validates the URL against the approved grant before request construction;
- rejects dangerous caller-controlled headers;
- disables ambient HTTP proxy use;
- performs its own DNS resolution and rejects non-public addresses;
- pins each dial to an IP returned by the immediately preceding resolution;
- revalidates every redirect and limits redirect depth;
- prevents credential-bearing redirects from changing origin and requires HTTPS for those requests;
- redeems credential handles only through trusted exact-domain `HTTPCredentialConsumer` bindings;
- strips credential/cookie-bearing response state before returning bounded output.

`NetworkGrant` itself remains authorization metadata, not connectivity. Host-mediated HTTP is therefore **implemented**, while arbitrary-process destination-scoped socket egress remains **not implemented**.

### Remaining arbitrary-process capability

A runtime may advertise `network_allowlist=true` only when **every outbound connection from arbitrary sandbox code** is forced through an enforceable egress boundary. The boundary must provide all of the following:

1. default deny when no destination grant exists;
2. HTTP, HTTPS/CONNECT, WebSocket, raw TCP where supported, and DNS behavior are explicitly covered rather than assumed;
3. hostname validation and connect use the same resolved address or an equivalent pinned-address mechanism;
4. every redirect/new connection is revalidated where application protocols expose redirects;
5. private, loopback, link-local, metadata, multicast, unspecified, reserved, and otherwise disallowed addresses fail closed;
6. proxy environment variables supplied by untrusted code cannot bypass the boundary;
7. direct socket egress cannot bypass an HTTP proxy;
8. destination grants remain owner/session scoped and expire with their authorization;
9. denial and bounded destination metadata are auditable without logging credentials or sensitive payloads;
10. connection reuse cannot continue after grant expiry or change the approved destination set;
11. IPv4/IPv6 resolution and dual-stack fallback cannot widen authority beyond the approved grant.

### Recommended deployment boundary

For first-party server/Kubernetes workers, prefer a dedicated sandbox worker network namespace/pod plus an egress enforcement component whose infrastructure policy prevents direct alternate egress. A local application-layer proxy may reuse the same grant/destination validation concepts as `BrokeredHTTPClient`, but the worker network policy must ensure arbitrary code cannot route around that proxy.

A practical implementation should separate responsibilities:

- **trusted authorization plane:** `NetworkGrantStore` resolves owner/session-scoped hostname + port authority;
- **trusted resolution/policy plane:** resolve and classify destinations, pin approved addresses, and deny private/reserved ranges;
- **forced transport plane:** network namespace/pod/firewall routing ensures arbitrary process sockets can reach only the trusted egress component (and any strictly necessary DNS mechanism under the same policy);
- **audit plane:** bounded connection metadata records grant/session, approved hostname/port, resolved destination class, decision, and denial reason without request payloads or secrets.

For local desktop runtimes, keep the current **no-network** posture until an OS-specific forced-egress primitive is proven. Do not weaken AppContainer, Bubblewrap, or Seatbelt network denial simply because host-mediated brokered HTTP or browser egress exists.

## Credential consumers

The credential broker already provides opaque owner/TTL-scoped handles, and `BrokeredHTTPClient` already defines a trusted exact-domain `HTTPCredentialConsumer` registration seam. Remaining Phase 9 work is to register narrowly scoped service-specific consumers where product workflows require authenticated host-mediated requests.

Every consumer must:

- be registered by trusted application composition, never by a model or sandbox process;
- bind one normalized credential service to exact DNS hostnames, not wildcard credential destinations;
- use only explicitly supported injection forms (`Authorization` or `X-Api-Key` under the current contract);
- keep raw secret material inside trusted host code;
- refuse cross-origin credential redirects;
- remain independently revocable through the credential-handle lifecycle;
- have service-specific tests proving destination mismatch and owner mismatch fail closed.

Arbitrary sandbox environments must not receive raw provider credentials as a shortcut for Phase 9 completion.

## Capability reporting and admission

`RuntimeCapabilities` remains runtime-specific. Broker admission rejects `RuntimeRequirements` that the chosen runtime does not advertise.

Additional invariant for resource fields:

- when a caller supplies a non-zero quota for a resource that the runtime cannot enforce, runtime creation must fail closed even if the caller omitted the matching `RuntimeRequirements` bit. This prevents a request from silently degrading into an unenforced limit.

For networking:

- availability of `BrokeredHTTPClient` does **not** set `network_allowlist=true`;
- `network_allowlist` is reserved for forced arbitrary-process egress enforcement;
- a process sandbox requesting network authority must continue to fail closed while its runtime reports the capability false.

## Adversarial test matrix

Every new control needs both positive and negative tests.

- **Admission:** unsupported non-zero limit fails closed; supported limit is accepted.
- **Boundary:** exceed the configured quota and observe enforcement.
- **Descendants:** repeat the attempt from a child/grandchild process.
- **Isolation:** one sandbox cannot consume or mutate another sandbox's quota controller.
- **Lifecycle:** cancellation, timeout, TTL cleanup, and destroy remove the controller and descendants.
- **Race:** rapid create/exec/cancel/destroy does not leave an unbounded process/controller behind.
- **Capability truth:** status/capability output exactly matches the native control proven on that platform.
- **Memory evidence:** verify the configured hard memory limit and any required swap restriction, then require kernel accounting (`memory.events`) to show the over-limit descendant was constrained; an allocation merely failing for an unrelated reason is insufficient.
- **Host-mediated egress:** DNS rebinding shape, redirect, proxy env, private-address destination, alternate port, credential-origin switch, and response-header secret/cookie leakage fail closed.
- **Arbitrary-process egress:** direct-socket bypass, alternate protocol/port, CONNECT, WebSocket, DNS bypass, IPv4/IPv6 fallback, private/link-local/metadata destinations, grant expiry, and proxy bypass all fail unless explicitly authorized and enforced.

## Implementation sequence

Completed/current resource lineage:

1. Generic admission rejects unsupported non-zero memory/CPU/PID/disk requests.
2. Windows PID and aggregate memory controls are implemented with native evidence.
3. Linux delegated cgroup-v2 PID and strict aggregate memory enforcement are implemented.
4. Cumulative CPU enforcement primitives are rebuilt for exact-head validation in #232; capability promotion remains separate.

Next active sequence:

5. Finish Windows governed-workspace closeout (#231) and CPU validation (#232) before changing runtime capability claims.
6. Design physical-disk accounting/enforcement for runtime-owned writable roots as a separate resource-control slice.
7. Preserve existing host-mediated brokered HTTP as the trusted destination-enforced path.
8. Implement forced arbitrary-process egress first for server/Kubernetes workers only when both application-layer and infrastructure bypass evidence can be proven; keep local desktop runtimes no-network until their OS-specific boundary is proven.
9. Add service-specific credential consumers only for concrete authenticated brokered workflows, with exact-domain bindings and negative tests.
10. Re-evaluate macOS resource primitives independently; do not inherit claims from Windows/Linux.

## Exit criteria

The sandbox hardening program can treat a production worker profile as ready for durable arbitrary sandbox tasks only when the quotas required by that deployment are natively enforced, destination-scoped arbitrary-process egress is either enforced or explicitly unavailable with no-network fail-closed behavior, and the Phase 17 adversarial suite continuously verifies every advertised claim. Host-mediated brokered HTTP may be used independently because it does not grant arbitrary sandbox processes direct network access.
