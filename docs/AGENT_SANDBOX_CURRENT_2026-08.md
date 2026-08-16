# Agent Sandbox — Current Documentation Index (August 2026)

Use these files for the current sandbox implementation state after Windows Phase 12, completed macOS Phase 13, and the active quota-hardening program:

- `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` — active phase status, open enforcement gaps, and execution order.
- `SANDBOX_RUNTIME_CURRENT_2026-08.md` — current runtime and persistent-extension operator behavior.
- `SANDBOX_QUOTA_EGRESS_HARDENING_DESIGN_2026-08.md` — approved resource-quota and forced-egress implementation contract.
- `AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md` — implemented Broker/runtime/managed-process architecture and platform boundaries.
- `AGENT_SANDBOX_PHASE13_MACOS_2026-08.md` — macOS native-confinement implementation and validation record.
- `AGENT_SANDBOX_PHASE13D_MACOS_ASSURANCE_2026-08.md` — macOS adversarial-assurance evidence and explicit teardown limitation.
- `AGENT_SANDBOX_THREAT_MODEL.md` — threat model and adversarial acceptance principles.

Completed Windows Phase 12 records and older aggregate snapshots are retained under `archive/`. The versioned current-state documents, this index, and `MASTER_PLAN.md` are authoritative for current status and outstanding work.

## Current checkpoint

Windows native confinement is complete through PR #149, merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

Explicit execution cancellation addressability is complete in PR #155, merged as `35b5533d0556532a762aaa27522a9be4029f1fee`. Callers can supply a canonical execution reference before dispatch, the broker/runtime preserves it end-to-end, active duplicates fail closed, and cancellation targets the exact active execution.

macOS Phase 13A is complete in PR #159, merged as `ce7d880ab39402671a6f39407ea9319418089de4`. The fixed `/usr/bin/sandbox-exec` Seatbelt primitive and native denial behavior are proven on `macos-latest`.

macOS Phase 13B merged in PR #162 as `840b00bb6d2b74d1a88eb1fd910d06dab64118a2`. The first-party Darwin local runtime uses per-session Seatbelt confinement, detached read-only workspace staging, narrow filesystem roots, default network denial, bounded execution output/wall time, sanitized environment construction, and the caller-known cancellation contract.

macOS Phase 13C merged in PR #164 as `44f410793a70444963ec1eecb989b15df159b5f1`, adding native Seatbelt confinement for persistent stdio MCP/plugin processes. Phase 13D adversarial assurance merged in PR #166 as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17`, completing Phase 13. It proves confinement survives deliberately detached descendants while preserving the truthful `process_tree_isolation=false` capability report.

Quota hardening is now active. Broker resource admission fails closed after PR #170 (`9a2db5bf`); Windows process-count enforcement merged in PR #171 (`11dfab99`); Windows aggregate memory enforcement merged in PR #172 (`bc9eb6f2`). PR #173 has native Ubuntu evidence that a correctly delegated Linux cgroup-v2 worker can atomically place executions into per-execution `pids.max` cgroups and deny an over-limit fork. Linux advertises this capability only when the delegated boundary is usable; the documentation-inclusive PR head still requires full validation before merge.

Still open after the current Linux PID slice:

- Linux aggregate memory enforcement with `memory.max` and `memory.events` evidence;
- CPU semantics and physical-disk quota design;
- macOS resource quotas where independently enforceable primitives have not been proven;
- destination-enforced egress;
- broader workspace-registry/path-component TOCTOU assurance;
- service-specific credential broker consumers;
- dedicated server/Kubernetes workers, including production cgroup delegation and worker placement.
