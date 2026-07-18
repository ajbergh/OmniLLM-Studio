# OmniLLM-Studio remediation status

Branch: `agent/full-remediation-20260718`  
Review baseline: `main` at `0bc8c9f3a08e30738742806abaea4630b7f6a415`  
Date: 2026-07-18

## Summary

This branch implements the release-blocking and high-priority findings from the July 18, 2026 deep engineering review. The changes intentionally preserve the existing Go/React/SQLite/Wails architecture and are split into reviewable commits.

## Phase 0 — Release blockers

| Finding | Status | Implementation |
|---|---|---|
| Desktop loopback API trust | Complete | Wails now generates a cryptographically random per-launch URL prefix, never logs the secret, removes wildcard CORS, tightens local file permissions, and makes browser automation opt-in. |
| Plugin runtime deadlock | Complete | Plugin JSON-RPC uses separate state/write locks, asynchronous response dispatch, request cancellation, bounded initialization/shutdown, stderr capture, and lifecycle tests. |
| Ollama discovery SSRF | Complete | Discovery is admin-or-solo only, validates provider URLs, restricts redirects, supports stored provider profiles, and no longer accepts OpenRouter API keys in query strings. |
| Browser isolation | Substantially complete | Every session receives an incognito browser context; operations are serialized; quotas are per user; profiles are ephemeral; final navigation destinations are revalidated; sandbox disablement requires an explicit override. Full CDP interception of every subresource remains a follow-up hardening item. |

## Phase 1 — Authentication, data, and secrets

| Finding | Status | Implementation |
|---|---|---|
| Plaintext session tokens | Complete | Only SHA-256 token digests are persisted; plaintext bearer values are returned once. Existing sessions are intentionally invalidated on upgrade. |
| First-user registration race | Complete | First-user count and administrator creation are serialized. |
| Login throttling | Complete | Authentication attempts are limited by both source and normalized account name. |
| HttpOnly web sessions | Backend complete | Login/register now issue HttpOnly SameSite cookies and logout clears them. Bearer responses remain for API compatibility. Removing the frontend's legacy localStorage fallback is deferred to a dedicated UI compatibility patch. |
| Bundle restore fidelity | Complete for existing bundle entities | Conversation ownership/workspace and message branch/parent/user fields are restored. ZIP entry count, expansion size, path, and per-entry limits were added. Export/import coverage for every studio-specific entity remains a version-2 bundle project. |
| Encryption-key durability | Complete | Persistent container and Helm deployments require an externally supplied stable master key. Local desktop/server mode retains machine-scoped seed behavior. |
| Foreign-key enforcement | Staged | Not enabled blindly against existing user databases. A dedicated orphan-repair migration and compatibility audit remain required before changing SQLite enforcement globally. |

## Phase 2 — Uploads, media, and deployment

| Finding | Status | Implementation |
|---|---|---|
| Attachment MIME trust | Complete | Attachments are content-sniffed, declared/content mismatches are rejected, unknown binaries are rejected, archive-backed office formats are handled explicitly, and temporary multipart files are cleaned up. |
| Video upload mismatch | Complete | Server body-read behavior and Docker/Compose/Helm/Nginx limits now support the documented 500 MB ceiling. |
| Missing FFmpeg runtime | Complete | The backend runtime image includes FFmpeg and FFprobe. |
| Invalid Compose healthcheck | Complete | Compose and the backend image use real HTTP health probes. |
| Container key persistence | Complete | Compose fails fast without a 64-character external key; container and Helm runtime set `OMNILLM_REQUIRE_MASTER_KEY=true`. |
| Web security headers | Complete for container/Helm paths | Nginx configurations include CSP, frame, referrer, permissions, and content-type protections. |

## Phase 3 — CI and release governance

| Finding | Status | Implementation |
|---|---|---|
| Missing quality gate | Complete | New `ci.yml` runs Go formatting/vet/tests/race detection, frontend lint/unit/build, the complete Playwright suite, and Helm validation. |
| Missing security automation | Complete | Added CodeQL, Go vulnerability scanning, npm audits, and Dependabot configuration. |
| Toolchain drift | Complete | Go 1.25 is used by CI, release, and backend container builds. |
| Unpinned Wails release tool | Complete | Release workflow installs Wails v2.12.0 explicitly. |
| Manual release tag bug | Complete | Workflow-dispatch releases validate and checkout an existing semantic-version tag and use it consistently. |
| Helm-only PRs skipped | Complete | Container workflow pull-request paths include the Helm chart. |
| Metadata drift | Complete | Root and frontend package versions align at 0.2.0 and root licensing aligns with MIT. |

## Tests added

- Plugin lifecycle, concurrent request cancellation, and entrypoint containment.
- Session-token hashing and lookup/delete behavior.
- Bundle ownership, workspace, branch, parent-message, and user-field restoration.
- Attachment MIME mismatch and unknown-binary rejection.

## Validation state

GitHub Actions is the authoritative validation environment for this branch because the current execution environment cannot clone the repository or run its Go/Node toolchains locally. The draft pull request should not be merged until all new quality, security, container, and Helm checks are green.

## Explicit follow-up items

These are intentionally not represented as complete:

1. Add CDP request interception for browser subresources, WebSockets, frames, and service-worker traffic.
2. Remove the frontend bearer-token `localStorage` compatibility path after cookie-based login is exercised across Vite, Wails, reverse-proxy, and direct API modes.
3. Introduce bundle schema v2 covering File Library, Image Studio, Music Studio, Video Studio, timelines, render jobs, MCP, plugins, evaluations, and workspace membership.
4. Audit and repair existing orphaned rows before enabling SQLite foreign-key enforcement.
5. Replace the composition-root shutdown slice with an application runtime object that returns startup errors instead of calling `log.Fatalf` inside router construction.
6. Generate frontend contracts from an OpenAPI schema and return both version and commit from the build-information endpoint.
