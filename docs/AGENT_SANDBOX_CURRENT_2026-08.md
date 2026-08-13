# Agent Sandbox — Current Documentation Index (August 2026)

Use these files for the current sandbox implementation state after Windows Phase 12:

- `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` — active phase status, open enforcement gaps, and execution order.
- `SANDBOX_RUNTIME_CURRENT_2026-08.md` — current Linux/Windows runtime and persistent-extension operator behavior.
- `AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md` — implemented Broker/runtime/managed-process architecture and platform boundaries.
- `AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md` — completed Windows Phase 12 implementation and validation record.
- `AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md` — direct adversarial Windows assurance evidence.
- `AGENT_SANDBOX_THREAT_MODEL.md` — threat model and adversarial acceptance principles.

The older aggregate files `AGENT_SANDBOX_ROADMAP_2026-08.md`, `SANDBOX_RUNTIME.md`, and `AGENT_SANDBOX_ARCHITECTURE.md` remain useful historical design snapshots. During Phase 12E, the GitHub connector safety layer refused their large in-place replacements, so this index and the versioned current-state documents are the authoritative references for current status.

## Current checkpoint

Windows native confinement is complete through PR #149, merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

Both first-party protocol-v2 execution and persistent stdio MCP/plugin execution have native Windows AppContainer/Job confinement with behavior-level Windows assurance.

Still open outside Phase 12 completion:

- Issue #151 explicit execution-ID cancellation addressability;
- memory/CPU/PID/disk quotas;
- destination-enforced egress;
- broader workspace-registry/path-component TOCTOU assurance;
- service-specific credential broker consumers;
- macOS native confinement;
- dedicated server/Kubernetes workers.
