# Agent Sandbox Phase 13D — macOS Adversarial Assurance — August 2026

> **Status:** IN PROGRESS — first native adversarial suite green; stacked behind Phase 13C
>
> Phase 13D validates escape attempts and the truthful boundary of macOS process teardown. It does not upgrade capability claims merely because ordinary descendants are easy to terminate.

## Acceptance model

The Darwin runtime and persistent-extension backends must continue to enforce Seatbelt filesystem/network policy even when paths are manipulated or descendants deliberately detach from the root process group.

A deliberately detached descendant is also used to validate the current teardown limitation. If it outlives process-group cancellation/root kill while remaining Seatbelt-confined, the correct result is:

- `os_isolation = true`;
- `filesystem_isolation = true`;
- `network_isolation = true`;
- **`process_tree_isolation = false`**.

The capability must not be promoted to true unless a stronger native teardown mechanism can authoritatively reap detached sessions and that mechanism is proven adversarially.

## Native tests added

### Deterministic workspace source-swap rejection

The read-only Darwin staging copy is refactored so the post-open inode identity check can be exercised deterministically.

The test:

1. observes an approved regular source file;
2. replaces the path with a symlink to an outside file before the copy helper opens it;
3. calls the copy helper with the stale observation;
4. requires `os.SameFile` validation to fail before destination bytes are created.

This proves the copy-time identity defense rather than relying on a probabilistic concurrent race.

### Writable symlink alias attempts

Inside a writable runtime-owned root, an adversarial helper:

- creates a symlink to an unrelated host file and attempts to read through it;
- creates a symlink to an unrelated host destination and attempts to write through it.

Seatbelt resolves the target authority, so both operations must be denied even though creating the symlink itself is allowed beneath the runtime-owned writable root.

### Cross-runtime authority reuse

Two Darwin runtime sessions are created. A file placed in runtime A's private home root is passed by absolute path to runtime B's adversarial helper.

Runtime B knows the path string but does not receive runtime A's read authority. Reading the content must fail and the sentinel content must not appear in model-visible output.

This validates the core invariant that sandbox IDs/path knowledge are references, not ambient cross-runtime authorization.

### Detached runtime descendant

A runtime helper launches another copy of itself with `setsid`, creating an independent session/process group while inheriting the Seatbelt profile.

The root execution is then cancelled through the caller-known execution ID.

The detached child must:

- outlive the root process-group cancellation, demonstrating why `process_tree_isolation` remains false;
- remain unable to read an unrelated host file;
- remain unable to write an unrelated host path;
- remain unable to connect to a loopback listener;
- write its diagnostic report only inside the runtime-owned temp root.

The host test harness then explicitly kills the detached child so CI does not leak a process.

### Detached persistent-extension descendant

The same pattern is exercised through the Phase 13C persistent `CommandProcess` path.

A Seatbelt-confined extension launches a `setsid` descendant. Before root teardown, the detached child reports that host read/write and loopback remain denied. The extension root is then killed and waited.

The test requires:

- the detached child to remain alive after ordinary root process-group kill, preserving the documented limitation;
- the extension scratch root to be removed on root wait;
- the detached child to remain unable to write outside confinement;
- host-side test cleanup to terminate the detached process.

## CI

`.github/workflows/sandbox-macos-adversarial.yml` runs on `macos-latest` and executes:

```text
go test ./internal/sandbox -run '^TestDarwinAdversarial' -count=1 -v
```

The first native run for the Phase 13D branch passed all adversarial tests.

## What this evidence does and does not prove

It proves that the Seatbelt policy is inherited across ordinary and deliberately detached descendants for the tested filesystem/network operations, and that staged-copy identity checks fail closed on a deterministic path swap.

It also proves that process-group cancellation is not equivalent to authoritative process-tree teardown on macOS when an adversarial child calls `setsid`.

Therefore Phase 13D must **not** change `ProcessTreeIsolation` to true.

## Remaining completion gates

Before Phase 13 can be closed:

- Phase 13B must merge after its exact-head Quality/Security/container/native gates;
- Phase 13C must merge after its exact-head native extension and repository gates;
- this Phase 13D branch must be normalized onto the merged dependencies;
- authoritative current roadmap/runtime docs must be reconciled to the final merged state;
- exact final-head Quality Gate, Security Scan, macOS runtime/extension/adversarial workflows, applicable container validation, race/Playwright/Helm checks, and final review/diff checks must pass.

The known detached-process teardown limitation may remain documented after Phase 13 if capability reporting stays false and no operator-facing contract claims stronger containment.
