# Agent Sandbox — Current Documentation Index (August 2026)

Use these files for the current sandbox implementation state after Windows Phase 12 and during macOS Phase 13:

- `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` — active phase status, open enforcement gaps, and execution order.
- `SANDBOX_RUNTIME_CURRENT_2026-08.md` — current runtime and persistent-extension operator behavior; macOS runtime details are additionally tracked in the Phase 13 record until persistent-extension confinement lands.
- `AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md` — implemented Broker/runtime/managed-process architecture and platform boundaries.
- `AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md` — completed Windows Phase 12 implementation and validation record.
- `AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md` — direct adversarial Windows assurance evidence.
- `AGENT_SANDBOX_PHASE13_MACOS_2026-08.md` — active macOS native-confinement implementation and validation record.
- `AGENT_SANDBOX_THREAT_MODEL.md` — threat model and adversarial acceptance principles.

The older aggregate files `AGENT_SANDBOX_ROADMAP_2026-08.md`, `SANDBOX_RUNTIME.md`, and `AGENT_SANDBOX_ARCHITECTURE.md` remain useful historical design snapshots. The versioned current-state documents and this index are the authoritative references for current status.

## Current checkpoint

Windows native confinement is complete through PR #149, merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

Explicit execution cancellation addressability is complete in PR #155, merged as `35b5533d0556532a762aaa27522a9be4029f1fee`. Callers can supply a canonical execution reference before dispatch, the broker/runtime preserves it end-to-end, active duplicates fail closed, and cancellation targets the exact active execution.

macOS Phase 13A is complete in PR #159, merged as `ce7d880ab39402671a6f39407ea9319418089de4`. The fixed `/usr/bin/sandbox-exec` Seatbelt primitive and native denial behavior are proven on `macos-latest`.

macOS Phase 13B is implemented in PR #162 and is in final exact-head validation. The first-party Darwin local runtime now uses per-session Seatbelt confinement, detached read-only workspace staging, narrow filesystem roots, default network denial, bounded execution output/wall time, sanitized environment construction, and the caller-known cancellation contract. It deliberately does **not** advertise process-tree isolation, destination allowlists, or resource quotas that are not yet proven/enforced.

Still open after 13B:

- Phase 13C native confinement for persistent stdio MCP/plugin processes on macOS;
- Phase 13D adversarial macOS assurance, including detached-process/session escape attempts and path-race pressure;
- memory/CPU/PID/disk quotas where platform enforcement is not yet implemented;
- destination-enforced egress;
- broader workspace-registry/path-component TOCTOU assurance;
- service-specific credential broker consumers;
- dedicated server/Kubernetes workers.
