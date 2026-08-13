# Agent Sandbox Phase 12D — Windows Adversarial Assurance

> **Status:** COMPLETE
>
> **PR:** #149
>
> **Final validated head:** `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb`
>
> **Squash merge:** `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`

Phase 12D adds direct Windows-native adversarial evidence for the confinement surfaces delivered in Phase 12B and Phase 12C. It does not widen runtime capability claims or weaken the existing AppContainer, Job Object, staging, environment, or network policies.

## Prior merged checkpoints

- **12A / PR #127:** Windows security primitives; squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.
- **12B / PR #128:** first-party protocol-v2 Windows AppContainer runtime; final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed Quality, Security, native Windows, Chromium/race, Helm, and applicable container gates; squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.
- **12C / PR #139:** persistent stdio MCP/plugin AppContainer confinement; final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed Quality, Security, applicable container validation, Windows lifecycle/native jobs, and unresolved-review inspection; squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.

Phase 12E subsequently reconciles the aggregate trackers and runtime/operator documentation using the final #149 evidence.

## 12D adversarial coverage

`backend/internal/sandbox/extension_process_windows_test.go` and `extension_process_windows_policy_test.go` directly test the persistent Windows extension boundary.

### Identity and filesystem authority

- Starts independently confined persistent extensions with separate AppContainer profiles.
- Verifies the tested child token reports `TokenIsAppContainer`.
- Verifies a confined extension cannot read or write another extension's sandbox-owned profile data.
- Verifies unrelated host-file reads and writes are denied.
- Verifies the staged extension command bundle is read-only.
- Verifies sandbox-owned `HOME` remains writable.

### Network and secret isolation

- Sets an ambient backend `OMNILLM_MASTER_KEY` and verifies it is absent in the confined child.
- Attempts a connection to a parent loopback listener and requires default-deny network behavior.
- Exercises `required` native mode with an explicit `GITHUB_TOKEN` and requires the credential-sensitive environment policy to reject it when `OMNILLM_EXTENSION_ALLOW_SECRET_ENV` is not enabled.

### Process-tree confinement

- A persistent extension root spawns a descendant and exits normally; the test retains a process handle and proves the descendant is terminated by Job teardown.
- A persistent extension root spawns a descendant and remains active; cancelling the extension context must return `context.Canceled` from `Wait` and terminate the descendant process through the Job Object.

### Staging and path-shape defenses

- A multiply-linked source file created with a Windows hard link is rejected by `stageWindowsReadOnlyWorkspace`.
- A directory junction created with `mklink /J` is rejected as a reparse point.
- `remapWindowsExtensionArgs` rejects an unrelated absolute host argument outside the staged command/workspace roots.

These tests complement the existing Phase 12B staging checks, including post-open `GetFinalPathNameByHandle` containment verification, rather than replacing them.

### Extension policy modes

The final branch also proves:

- `auto` selects the native Windows extension backend when available;
- `required` selects native confinement and fails closed when it cannot be provided;
- `off` explicitly selects the sanitized host compatibility boundary.

## Validation history

### Diagnostic PR #148

Draft PR #148 first introduced the assertions. On `windows-latest`, every new adversarial test passed together with the existing Windows runtime/security-primitives suite and `cmd/sandboxd`. The Linux backend gate stopped only because the new test file needed canonical `gofmt` formatting.

A temporary branch-only formatter produced the canonical Go source. PR #148 was closed without merge so the formatter workflow and diagnostic history would not enter the final review surface.

### Clean PR #149

The canonical tests were replayed directly onto current `main`. The branch later added the policy-mode test and this evidence document, ending at exact final head:

`8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb`

That exact head passed:

- backend formatting, vet, unit/integration tests, and race detector;
- dedicated Windows sandbox/adversarial tests and `cmd/sandboxd`;
- Windows plugin lifecycle;
- Windows desktop compatibility;
- frontend lint/unit/build;
- Chromium Playwright smoke;
- Helm lint/render;
- dependency vulnerability audit;
- Go and JavaScript/TypeScript CodeQL;
- frontend and backend multi-architecture container builds;
- final unresolved-review-thread inspection.

PR #149 then squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

## Additional finding: explicit cancellation contract

Phase 12D also exposed a separate protocol-v2 lifecycle issue now tracked as #151. Context cancellation and Windows Job process-tree teardown are implemented and proved above. However, the explicit `Cancel(executionID)` API cannot currently be addressed by a synchronous caller while `Exec` is active because the runtime-generated execution ID is returned only after `Exec` completes.

Issue #151 is a protocol/runtime lifecycle defect, not a failure of Windows confinement. It remains open after Phase 12 completion and should be repaired as a small Phase 7 lifecycle slice.

## Phase 12E handoff

After #149 merged, Phase 12E:

1. reconciles `AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md` with #139/#149 evidence;
2. reconciles `AGENT_SANDBOX_ROADMAP_2026-08.md` and marks Windows Phase 12 complete;
3. updates `SANDBOX_RUNTIME.md` with the final Windows runtime/extension behavior and limitations;
4. keeps capability claims limited to enforced controls;
5. leaves cross-program gaps explicit: resource quotas, destination-enforced egress, broader workspace TOCTOU assurance, service-specific credential consumers, interpreter availability, worker packaging/deployment, and issue #151.
