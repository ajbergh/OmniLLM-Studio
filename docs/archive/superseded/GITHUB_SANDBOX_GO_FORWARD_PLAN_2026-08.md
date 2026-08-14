> **Archived — superseded by MASTER_PLAN.md.** This point-in-time execution note predates PR #164 merging. Its useful rationale is preserved below; [MASTER_PLAN.md](../../MASTER_PLAN.md) is now the single active planning authority.

# GitHub Integration and Sandbox — Go-Forward Plan (August 2026)

> **Prepared from:** `main` at `a8cc3b6` and the open PR state on 2026-08-14.
>
> **Purpose:** Reconcile delivered work and active pull requests with the durable GitHub and agent-sandbox plans. This is an execution order, not an authorization expansion.

## Reconciled position

### GitHub integration

- GitHub App authentication and repository-binding authorization (G1–G6) are complete.
- Merge-policy evidence and guarded direct merge (M1, M2, M3A, and M3B) are implemented and remain fail closed.
- G7A bounded failing-check diagnostics merged in #161 (`3495658`).
- G7B bounded workflow/job/step status metadata merged in #163 (`7e3516c`). It is no longer merely “under final validation.”
- G7C is complete as a threat-review decision in #165 (`a8cc3b6`): raw CI log access is deferred and is not authorized by the existing PR-read policy.
- There are no open GitHub-integration PRs. The current open work is entirely macOS sandbox work.

### Sandbox capabilities

- Windows Phase 12 and cross-platform, caller-addressable execution cancellation are complete.
- macOS 13A Seatbelt primitives and 13B first-party local runtime are merged in #159 (`ce7d880`) and #162 (`840b00b`).
- **#164 / Phase 13C** adds native Seatbelt confinement for persistent macOS extensions. It is a clean, mergeable draft with all reported required checks green, but has no reviews yet.
- **#166 / Phase 13D** adds adversarial macOS assurance. It is a stacked draft, currently conflicting/dirty, because its base is the 13C branch rather than normalized `main`. Its previously run checks are green, but must be rerun after rebasing and conflict resolution.
- Neither the merged 13B runtime nor the open 13C/13D work claims Darwin process-tree isolation, destination allowlisting, or memory/CPU/PID/disk quotas. Those capability flags must remain false until separately enforced and proven.

## Ordered execution plan

### 1. Complete and merge Phase 13C (#164)

Owner: sandbox implementation/review

1. Keep #164 draft until a focused code and security review is complete; its native extension assurance, runtime assurance, Quality Gate, Security Scan, containers, and browser smoke checks are currently green.
2. Confirm the exact reviewed head remains `ca8cc84948c8be5fa6c27fff371da912498c9a4d`; re-run the normal final PR inspection if it changes.
3. Validate the Phase 13C non-claims in review: no hostile detached-process containment, destination allowlists, or resource quotas are implied by the extension implementation.
4. Merge #164 only after review approval and a final exact-head gate pass.

Exit: macOS persistent extensions are confined under the documented `auto|required|off` policy, and the Phase 13 and runtime documents on `main` accurately say that 13C is merged.

### 2. Normalize, validate, and merge Phase 13D (#166)

Owner: sandbox implementation/review

1. Rebase/retarget #166 onto `main` after #164 merges; resolve the current conflicts, particularly in the phase, runtime, and current-state documents.
2. Preserve the adversarial test scope: staged-workspace identity/source-swap, symlink aliases, cross-runtime reads, loopback denial, detached runtime and extension descendants, and explicit cleanup.
3. Preserve the demonstrated limitation: Seatbelt confinement survives detachment, while ordinary process-group teardown does not prove that all intentionally detached descendants are reaped. Keep `process_tree_isolation=false`.
4. Run the native adversarial macOS workflow and all repository gates again on the rebased head. Do not use results from the conflicting pre-rebase head as merge evidence.
5. Merge only after the rebased PR is clean, reviewed, and its exact final head passes the complete gate set.

Exit: Phase 13 is complete as a truthful macOS confinement and adversarial-assurance milestone; the post-merge documents clearly separate completed confinement from the remaining platform-wide enforcement gaps.

### 3. Reconcile the authoritative current-state documents immediately after the sandbox merges

Owner: documentation maintainer; may be included in the Phase 13D normalization PR if it remains narrowly reviewable.

1. Update `AGENT_SANDBOX_CURRENT_2026-08.md`, `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`, `AGENT_SANDBOX_PHASE13_MACOS_2026-08.md`, and `SANDBOX_RUNTIME_CURRENT_2026-08.md` to replace pre-merge 13B/13C language with final commit references and phase outcomes.
2. Retain the explicit open-gap list: resource quotas, destination-enforced egress, broader TOCTOU assurance, service-specific credential consumers, and the Darwin detached-process teardown limitation.
3. Make capability reporting the source of truth: neither operator configuration nor passing a happy-path test may promote an unimplemented control to enforced.

Exit: a reader of `main` does not need PR history to understand the macOS implementation status or its limitations.

### 4. Close the GitHub G7 documentation loop, then choose a narrow G7D charter

Owner: GitHub integration maintainer

1. Make a documentation-only correction in `GITHUB_INTEGRATION_PHASE7_2026-08.md`: mark G7B as merged in #163 and update the program status to reflect that G7A/G7B have landed and G7C’s no-raw-logs decision is complete.
2. Observe whether G7A annotations plus G7B job/step status metadata resolve actual CI diagnosis needs. Do not begin G7C implementation merely because log text is convenient.
3. If a next GitHub slice is warranted, write and approve a bounded G7D design before code. Prefer one independent family—such as issues/discussions read metadata, remote-branch lifecycle, or release/tag workflow—rather than combining several hosted mutations.
4. Give G7D its own process gate, per-remote authorization, model-input schema, exact object binding, bounded untrusted output model, provider permission/error handling, and adversarial tests. It must not inherit authority from PR-read, merge, or raw Git permissions.

Exit: GitHub planning accurately reflects landed G7 work, and any new collaboration capability has a separately approved security boundary.

### 5. Start the cross-platform hardening backlog only after the two macOS PRs are resolved

Priority order:

1. Enforced memory, CPU, PID/process-count, and physical-disk quotas (current Phase 7 resource-control gap).
2. Destination-aware egress policy that prevents DNS/proxy/redirect/private-address bypasses (Phase 8).
3. Broader workspace TOCTOU and mount/reparse assurance (Phase 5/9 continuation).
4. Service-specific credential-broker consumers and server/Kubernetes worker isolation (Phase 9, then Phase 15).

Each item needs platform-specific enforcement evidence, capability reporting, negative tests, and documentation before it is considered complete.

## Decision rules

- A merged commit records delivery; an open draft PR, even with green checks, is not complete.
- A stacked PR must be rebased and fully revalidated after its dependency merges.
- Passing checks do not justify a stronger sandbox capability claim than the tested enforcement mechanism supports.
- Raw GitHub workflow logs remain deferred unless a separately gated and threat-modeled proposal satisfies every G7C prerequisite.

