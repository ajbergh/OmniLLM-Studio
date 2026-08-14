# Agent Sandbox Phase 13 — macOS Native Confinement — August 2026

> **Status:** IN PROGRESS
>
> Phase 13 is intentionally split into native-evidence slices. Phase 13A proves a usable macOS Seatbelt primitive but does **not** yet register a macOS first-party runtime or persistent extension backend.

## Objective

Bring macOS to the same truthful confinement standard used for Linux and Windows: controls are advertised only after the first-party runtime actually enforces them and native macOS CI proves the behavior. Missing primitives or unsupported requested controls fail closed.

## Phase 13A — Seatbelt foundation

Phase 13A adds a Darwin-only foundation around the fixed system launcher:

```text
/usr/bin/sandbox-exec
```

The foundation:

- never accepts an alternate launcher path from model or request input;
- fails closed when the system launcher is absent or not executable;
- builds a default-deny Seatbelt profile;
- permits process/system startup operations needed by the test child;
- denies network access unless a future profile explicitly grants it;
- canonicalizes and resolves explicit writable roots before granting `file-write*` beneath them;
- rejects non-directory writable roots;
- uses the existing sanitized subprocess environment boundary.

### Native evidence

A dedicated `macos-latest` Quality Gate job runs the Darwin-only assurance suite and requires `/usr/bin/sandbox-exec` to exist.

Diagnostic native evidence on PR #159 head `60cc8801748809b47a3bc25ae993087b440e4db7` used GitHub's `macos-26-arm64` image (macOS 26.5.2, arm64) with Go 1.25.13 and passed:

- write below one explicitly canonicalized writable root;
- denial of a write to a separate ungranted host root;
- denial of loopback TCP access under the default-deny profile;
- fail-closed rejection of a non-directory write root.

That run proves the primitive is usable on the current native runner. It is diagnostic evidence only; the exact final PR head must still pass native macOS plus the full applicable repository gates.

## Explicit Phase 13A non-claims

The Phase 13A profile intentionally allows host file reads for process-startup compatibility while the minimum Darwin read set is investigated. Therefore this slice does **not** claim or advertise first-party macOS filesystem isolation.

Phase 13A also does not:

- implement `NewLocalRuntime` on Darwin;
- change `RuntimeCapabilities` on macOS;
- enable Python/JavaScript/shell sandbox execution on macOS;
- migrate stdio MCP or local plugin processes to Seatbelt;
- provide destination-scoped egress;
- provide memory, CPU, PID, or physical-disk quotas;
- claim durable task/runtime recovery.

Until the later runtime slice lands, `local_runtime_other.go` continues to fail closed for Darwin. Until the later extension slice lands, `extension_process_other.go` keeps `required` fail-closed and `auto` on the sanitized compatibility boundary.

## Planned Phase 13 slices

### Phase 13B — first-party local runtime

Required exit criteria:

- Darwin-specific `NewLocalRuntime` implementation;
- per-runtime scratch/workspace lifecycle;
- narrowly enumerated executable/system read roots rather than Phase 13A's host-read compatibility grant;
- explicit read-only workspace handling and writable scratch semantics;
- default-deny network behavior;
- bounded wall/output behavior consistent with the Broker contract;
- caller-known execution ID/cancellation integration;
- descendant/process-tree termination evidence;
- truthful capabilities with unsupported quotas/egress remaining false;
- native negative tests for host reads/writes, network, secret inheritance, cancellation, and workspace containment.

### Phase 13C — persistent extension confinement

Required exit criteria:

- Darwin-specific `platformExtensionCommandContext` implementation;
- `required` uses Seatbelt or fails closed;
- `auto` uses the native backend when its prerequisites are satisfied;
- no pre-confinement child execution window;
- sanitized/minimal environment and credential-sensitive explicit environment rejection;
- stdio lifecycle compatibility for MCP and local plugins;
- descendant termination and host-file/network denial evidence on `macos-latest`.

### Phase 13D — adversarial assurance and completion review

Required evidence includes:

- symlink/rename/path-component escape attempts;
- writable-root aliasing and canonicalization attacks;
- cross-runtime authority reuse;
- ungranted host reads/writes;
- loopback and external-network denial where network is `none`;
- inherited secret/environment denial;
- cancellation/timeout/forced teardown of descendants;
- persistent extension equivalents;
- exact final-head Quality, Security, native macOS, Playwright/race, Helm, and applicable container validation.

Phase 13 may be marked complete only after both arbitrary sandbox execution and persistent extension processes have native macOS confinement evidence. Phase 13A alone is a foundation, not completion.

## Known platform constraint

`/usr/bin/sandbox-exec` is treated as a platform prerequisite, not as a capability that OmniLLM can emulate safely. The implementation must continue to fail closed if a future macOS release or deployment image removes or disables it. Native CI is therefore part of the ongoing compatibility contract rather than a one-time discovery check.
