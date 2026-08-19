# Sandbox CPU and Disk Enforcement Plan — August 2026

Status: ACTIVE IMPLEMENTATION CONTRACT

This document resolves the semantic ambiguity that previously blocked truthful `cpu_limit` and `disk_limit` capability reporting. It does **not** enable either capability by itself. Runtime capability bits remain false until the platform implementation and native negative evidence described here are present.

## CPU contract

`resources.cpu_time_ms` means **cumulative CPU time consumed by the complete sandbox execution process tree**, measured as user-mode plus kernel/system CPU time.

It is not:

- wall-clock time (`wall_time_ms` already exists),
- a CPU percentage,
- a CPU-rate throttle,
- a single-process `RLIMIT_CPU`, or
- a cgroup `cpu.max` quota-per-period value.

An execution that spends 400 ms of user CPU and 100 ms of kernel CPU has consumed 500 ms of the configured CPU budget regardless of whether that work occurred serially or across descendants.

### Linux enforcement

The execution is already atomically born inside one delegated cgroup-v2 child. CPU enforcement must therefore use that same execution cgroup as the accounting identity.

Implementation requirements:

1. Delegate/enable the `cpu` controller at the configured sandbox cgroup root and advertise CPU support only after startup probing succeeds.
2. Read `cpu.stat` `usage_usec` from the execution cgroup. This aggregates descendants in that cgroup and includes user + system usage.
3. Establish the baseline before untrusted execution and monitor cumulative delta while the execution is alive.
4. When delta reaches/exceeds `cpu_time_ms`, terminate the whole execution cgroup with `cgroup.kill` and return a stable quota-exceeded error/metadata value distinct from caller cancellation and wall timeout.
5. Bound enforcement overshoot. Polling alone with an unbounded runnable process count is not sufficient. The implementation must either combine accounting with an enforceable aggregate CPU-rate ceiling or require another mechanism that establishes a tested worst-case overshoot bound.
6. Native assurance must run parallel CPU burners/descendants and prove aggregate accounting cannot be bypassed by process or thread fan-out.

`cpu.max` may be used as an **overshoot bounding mechanism**, but never as the semantic implementation of `cpu_time_ms` itself.

### Windows enforcement

Windows Job Objects remain the process-tree accounting authority.

Implementation requirements:

1. Query Job accounting (`TotalUserTime + TotalKernelTime`) for the execution Job Object.
2. Monitor cumulative delta from the pre-execution baseline.
3. Terminate the entire Job Object when the configured budget is exhausted.
4. Do not substitute `JOB_OBJECT_CPU_RATE_CONTROL_*` because it is rate control.
5. Do not treat `PerJobUserTimeLimit` alone as the application contract because it excludes kernel/system CPU time.
6. Native tests must include child-process CPU pressure and prove a descendant cannot escape aggregate accounting.

### Darwin enforcement

Darwin must keep `cpu_limit=false` until a process-tree accounting primitive with a defensible detached-descendant model is selected and proven. Existing process-group teardown is not sufficient to claim aggregate CPU accounting for deliberately detached descendants, and Phase 13 deliberately reports `process_tree_isolation=false` for that reason.

## Disk contract

`resources.disk_bytes` means a **hard upper bound on writable filesystem storage attributable to the sandbox execution/session**, enforced before additional writes succeed. It is not a post-execution byte count.

The following do not satisfy `disk_limit`:

- counting files after the command exits,
- limiting stdout/stderr,
- limiting returned artifact bytes,
- a host free-space preflight alone, or
- deleting excess output after the limit has already been exceeded.

### Linux disk enforcement

Preferred boundary: a per-sandbox/session writable filesystem whose kernel-enforced size is configured before untrusted execution (for example a size-bounded tmpfs or a separately quota-enforced filesystem/subvolume). Bubblewrap should mount that bounded writable boundary at every writable sandbox path.

The implementation must prove:

- writes fail at/near the configured bound,
- descendants share the same bound,
- hard links/sparse files/rename patterns do not create an accounting escape,
- read-only workspace mounts are not charged as writable sandbox storage,
- cleanup releases the allocation.

### Windows disk enforcement

A plain AppContainer directory does not provide a per-directory hard byte quota. `disk_limit` must remain false until the runtime uses an enforceable storage boundary (for example an appropriately quota-capable virtual disk/filesystem or another Windows-native mechanism with per-sandbox hard enforcement). Post-hoc directory sizing is insufficient.

### Darwin disk enforcement

Seatbelt controls access, not storage consumption. `disk_limit` remains false until writable sandbox storage is backed by a hard quota-capable boundary and native evidence proves it.

## Capability/admission rules

- `CPULimit` is true only for runtimes that implement the cumulative tree contract above.
- `DiskLimit` is true only for runtimes that enforce the pre-write storage bound above.
- Broker admission continues to reject non-zero requested limits when the selected runtime reports the corresponding capability false.
- No implementation may flip a capability bit solely because the operating system exposes a similarly named but semantically different primitive.

## Validation exit criteria

A platform CPU/disk slice is merge-ready only when:

1. the exact contract is enforced from before untrusted execution,
2. descendants cannot bypass accounting,
3. quota exhaustion is distinguishable from wall timeout/cancellation,
4. cleanup is deterministic,
5. capability reporting matches the implementation,
6. native negative/adversarial tests exercise the bypass cases, and
7. the exact PR head passes repository Quality Gate, Security Scan, and applicable platform sandbox assurance workflows.
