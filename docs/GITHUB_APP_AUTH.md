# GitHub App authentication foundation

OmniLLM-Studio is moving from operator-supplied static GitHub tokens toward a first-class, user-scoped GitHub App connection. This document defines the boundary for that work.

## Why a GitHub App

The existing Git/GitHub tool chain already has separate operator gates for remote reads, fetch, push, remote branch creation, pull-request reads, draft pull-request creation, review replies, review-thread resolution, and ready-for-review. Authentication must not collapse those authorization boundaries.

A GitHub App user access token is therefore treated only as a credential source. Connecting GitHub does **not** enable Git writes, push, branch publication, pull-request creation, review mutations, or future merge capability.

## G1: device-flow service — implemented

`backend/internal/githubauth` provides the backend-only authentication service:

- GitHub App client ID is operator-owned configuration via `OMNILLM_GITHUB_APP_CLIENT_ID`.
- Device authorization uses GitHub's fixed `https://github.com/login/device/code` and `https://github.com/login/oauth/access_token` endpoints.
- The provider `device_code` remains backend-only; user-facing results contain only the user code, verification URI, expiry, and minimum poll interval.
- Polling is bounded to one provider request per service call and never starts a background polling loop.
- Provider polling intervals are enforced in the backend, including `slow_down` handling.
- A successful token exchange is immediately bound to the authenticated GitHub identity through `GET https://api.github.com/user` before credentials are accepted.
- Expiring user access tokens can be refreshed with the device-flow refresh token without a GitHub App client secret.
- Refresh-token rotation is serialized to avoid concurrent refresh races.
- Token response bodies and provider error bodies are never copied into public errors.
- Authentication responses are size-bounded, redirects are disabled, and provider endpoints are fixed.

## G2a: encrypted persistence and secret-free HTTP boundary — implemented

GitHub App credentials do **not** use the generic `settings` table. The general settings API enumerates that table, which is the wrong visibility boundary even for encrypted credential blobs.

G2a adds a dedicated `github_app_connections` table with:

- OmniLLM owner/user ID as the primary ownership key;
- GitHub numeric user ID and login;
- AES-256-GCM encrypted access token;
- AES-256-GCM encrypted refresh token;
- access-token expiry;
- refresh-token expiry;
- token type and non-secret scope metadata;
- no plaintext token columns.

`repository.GitHubAppConnectionRepo` implements the `githubauth.CredentialStore` contract and ensures this isolated schema idempotently. Reads decrypt credentials only for backend execution. Tests inspect raw SQLite columns and verify that access and refresh token plaintext never persists.

The HTTP boundary is implemented through `GitHubAuthHandler` and exposes only secret-free status/device-flow structures. All handlers derive ownership from `auth.ScopeUserIDFromContext`, which preserves authenticated-user isolation and the stable `local` owner in solo mode. Authentication responses are marked `Cache-Control: no-store`.

The route mount helper defines:

```text
GET    /v1/github/auth
POST   /v1/github/auth/device/start
POST   /v1/github/auth/device/poll
DELETE /v1/github/auth
```

Device poll performs at most one provider request; clients retry according to `retry_after_seconds`.

## G2b: application composition — implemented

`backend/internal/api/router.go` now constructs `NewGitHubAuthHandlerFromEnvironment(database)` once and mounts `MountGitHubAuthRoutes` only inside the existing authenticated `/v1` route group, immediately after the current-user route. The composition change is deliberately limited to four added router lines and does not move or rewrite any existing route.

Missing `OMNILLM_GITHUB_APP_CLIENT_ID` remains a supported runtime state: `GET /v1/github/auth` reports `configured=false`, start/poll return bounded service-unavailable responses, and disconnect remains idempotent. Solo mode continues to use the stable `local` owner through the same authenticated-route-group middleware boundary.

No frontend behavior is added in G2b and no Git/GitHub mutation gate is changed.

## G3a: request-scoped Git credential adapter and registry seam — implemented

`backend/internal/gitrepo.UserScopedRemoteService` now decorates the existing `RemoteService` without changing its Git transport, GitHub API implementations, local repository service, mutation gates, state-binding checks, or per-remote policy.

Credential precedence is explicit:

1. if the invoking user has a GitHub App connection, the connected user credential is substituted for exact `github.com` remotes;
2. if that connection requires reauthorization or token refresh/resolution fails, execution fails closed and does **not** fall back to a shared operator token;
3. if the invoking user has no GitHub App connection, existing `TokenEnv` and public-remote behavior remain the backward-compatible operator/headless fallback;
4. a connected GitHub App credential is never applied to a non-GitHub remote.

The adapter intentionally separates credential **status** from credential **resolution**. `git_remotes` uses only the local/non-network connection-status callback, so the existing low-risk/no-network inventory tool cannot trigger a provider token refresh. Actual token resolution and refresh are deferred until a credentialed Git/GitHub network operation executes.

For app-only GitHub remotes, the adapter adds a backend-only synthetic credential reference to its per-request cloned remote configuration. This lets the existing capability predicates and credential-loading paths remain unchanged while making them credential-source agnostic. The base operator configuration is never mutated, and no token or credential reference is exposed in tool arguments or results.

`tools.NewRegistry()` remains the backward-compatible zero-dependency constructor. `tools.NewRegistryWithOptions(...)` adds an injected GitHub credential status/resolution path, derives the stable owner from the existing invocation scope (`local` in solo mode), recovers the exact already-constructed `RemoteService`, and rebinds the currently registered Git remote/GitHub collaboration tool service interfaces to the request-scoped adapter. It does not add tools or bypass registration gates.

The following existing controls remain independent and authoritative:

- `OMNILLM_GIT_WRITE_ENABLED`
- `OMNILLM_GIT_REMOTE_ENABLED`
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED`
- `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`
- GitHub PR read/create/reply/thread-resolution/ready gates
- per-remote `allow_*` policy
- tool-level/scoped approval policy
- exact branch/head/remote-state preconditions

Authentication is not authorization.

## G3b: application credential composition — next

`NewRouterWithShutdown` should next share the existing user-scoped `githubauth.Service` with the registry options path:

- the non-network registry status callback should use the service's secret-free/local connection status for the invocation owner;
- the execution callback should use `AccessToken(ctx, owner)` so expiring tokens refresh only during an already-networked operation;
- `githubauth.ErrNotConnected` should map to `connected=false` so existing `TokenEnv` fallback remains available;
- reauthorization, refresh, persistence, or provider failures should map to `connected=true` plus an error so `gitrepo` fails closed rather than falling back;
- the same service should continue backing the authenticated `/v1/github/auth` routes.

This should remain a small composition-only change. No model-facing tool schema or permission gate needs to change.

## Subsequent slices

1. **G3b — wire the existing GitHub auth service into `NewRegistryWithOptions`.**
2. **G4 — repository discovery and explicit repository-to-local-worktree binding.**
3. **G5 — Settings UI: Connect GitHub, choose repositories, status, reconnect, disconnect.**
4. **M2 — continue merge-policy completeness work independently.**
5. **M3 — consider guarded direct merge only after M2 can prove policy completeness.**
