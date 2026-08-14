> **Archived — completed.** This native Windows assurance checkpoint merged in PR #149. See [MASTER_PLAN.md](../../MASTER_PLAN.md) for the remaining sandbox work.

# Agent Sandbox Phase 12D — Windows Adversarial Assurance

> **Status:** IN PROGRESS
>
> **Active branch:** `agent/sandbox-windows-assurance-12d-clean-20260813`
>
> **Active PR:** #149
>
> **Base:** current `main` at PR creation, `d994fa08875238c12f25519c258cdaa1efb7459d`

Phase 12D adds direct Windows-native adversarial evidence for the Windows confinement surfaces delivered in Phase 12B and Phase 12C. It does not widen runtime capability claims or weaken the existing AppContainer, Job Object, staging, environment, or network policies.

## Prior merged checkpoints

- **12A / PR #127:** Windows security primitives; squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.
- **12B / PR #128:** first-party protocol-v2 Windows AppContainer runtime; final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed the required Quality, Security, native Windows, Chromium/race, Helm, and applicable container gates; squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.
- **12C / PR #139:** persistent stdio MCP/plugin AppContainer confinement; final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed Quality Gate, Security Scan, applicable container validation, Windows lifecycle/native jobs, and had no unresolved review threads; squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.

The older aggregate Phase 12 trackers still contain stale pre-#139 wording. Their reconciliation is intentionally deferred to Phase 12E so this assurance PR remains test-focused and reviewable.

## 12D adversarial coverage

`backend/internal/sandbox/extension_process_windows_test.go` directly tests the persistent Windows extension boundary.

### Identity and filesystem authority

- Starts two concurrently confined persistent extensions with separate AppContainer profiles.
- Verifies the tested child token reports `TokenIsAppContainer`.
- Verifies a confined extension cannot read or write another extension's sandbox-owned profile data.
- Verifies unrelated host-file reads and writes are denied.
- Verifies the staged extension command bundle is read-only.
- Verifies sandbox-owned `HOME` remains writable.

### Network and secret isolation

- Sets an ambient backend `OMNILLM_MASTER_KEY` and verifies it is absent in the confined child.
- Attempts a connection to a parent loopback listener and requires default-deny network behavior.
- Exercises `required` native mode with an explicit `GITHUB_TOKEN` and requires the existing credential-sensitive environment policy to reject it when `OMNILLM_EXTENSION_ALLOW_SECRET_ENV` is not enabled.

### Process-tree confinement

- A persistent extension root spawns a descendant and exits normally; the host retains a process handle and proves the descendant is terminated by Job teardown.
- A persistent extension root spawns a descendant and remains active; cancelling the extension context must return `context.Canceled` from `Wait` and terminate the descendant process through the Job Object.

### Staging and path-shape defenses

- A multiply-linked source file created with a Windows hard link must be rejected by `stageWindowsReadOnlyWorkspace`.
- A directory junction created with `mklink /J` must be rejected as a reparse point.
- `remapWindowsExtensionArgs` must reject an unrelated absolute host argument outside the staged command/workspace roots.

These tests complement the existing Phase 12B staging checks, including post-open `GetFinalPathNameByHandle` containment verification, rather than replacing them.

## Validation evidence

### Diagnostic PR #148

Draft PR #148 first introduced the assertions. On `windows-latest`, every new adversarial test passed together with the existing Windows runtime/security-primitives suite and `cmd/sandboxd`. The Linux backend gate stopped only because the newly added test file was not canonical `gofmt` output.

A temporary branch-only formatter produced the canonical Go source. PR #148 was then closed without merge so the formatter workflow and diagnostic history would not become part of the final review surface.

### Clean PR #149

The canonical formatter-produced test file was replayed onto current `main` as a one-commit, one-file branch with no CI workflow modifications. Initial clean head:

`db82e44eeb4aa3a275303b82dee0437cca59de7b`

On that clean head:

- backend `Verify formatting` passed;
- Helm lint/render passed;
- `Sandbox confinement primitives (Windows)` passed, including all new 12D adversarial tests, the existing Windows runtime/primitives suite, and `cmd/sandboxd`.

This documentation commit creates a newer PR head, so the earlier pass is diagnostic evidence only. The exact final head must pass the complete required repository gate set before merge.

## Merge exit criteria

PR #149 remains draft until the exact final head passes:

- backend formatting, vet, unit/integration tests, and race detector;
- dedicated Windows sandbox/adversarial tests and `cmd/sandboxd`;
- Windows plugin lifecycle;
- Windows desktop compatibility;
- frontend lint/unit/build;
- Chromium Playwright smoke;
- Helm lint/render;
- dependency vulnerability audit;
- Go and JavaScript/TypeScript CodeQL;
- applicable frontend/backend container builds;
- unresolved-review-thread review.

Any native failure that exposes a confinement defect must be fixed in the implementation. The tests must not be weakened to preserve a prior capability claim.

## Remaining Phase 12 work

After #149 merges, Phase 12E will:

1. reconcile `docs/AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md` with the merged #139 and #149 evidence;
2. reconcile `docs/AGENT_SANDBOX_ROADMAP_2026-08.md`, removing stale references to #128/#12C as active work;
3. update `docs/SANDBOX_RUNTIME.md` and relevant operator documentation with the final Windows runtime/extension behavior and limitations;
4. confirm capability claims remain limited to enforced controls;
5. mark Phase 12 complete only after those documentation changes pass repository validation.

Open cross-program items remain outside Phase 12 completion unless explicitly reassigned: resource quotas, destination-enforced egress, broader workspace-registry/path-component TOCTOU assurance, service-specific credential consumers, and automated `sandboxd` packaging/deployment.
