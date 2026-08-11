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

## G3: request-scoped Git credential resolver — next

The next slice should let existing Git/GitHub tools consume the connected user's GitHub App credential without adding user IDs or credentials to model-facing tool arguments.

Tool invocation context already carries the OmniLLM owner. The Git remote layer should therefore resolve credentials internally from `context.Context`, using this precedence:

1. if the invoking user has a GitHub App connection, use that user credential;
2. if that connection requires reauthorization or token refresh fails, fail closed and do **not** fall back to a shared operator token;
3. if the invoking user has no GitHub App connection, retain `TokenEnv` as a backward-compatible operator/headless fallback;
4. never apply a GitHub App credential to a non-GitHub remote.

The `gitrepo` GitHub capability predicates must also become credential-source agnostic: repository host plus per-remote `allow_*` policy determines whether a capability is allowed, while the actual credential is resolved at execution time.

`tools.NewRegistry()` should remain the backward-compatible default, with an options/dependency path added for an injected GitHub credential provider. This preserves current deployments while allowing `NewRouterWithShutdown` to inject the user-scoped provider backed by `GitHubAppConnectionRepo` and `githubauth.Service`.

Crucially, the following existing controls remain independent and authoritative:

- `OMNILLM_GIT_WRITE_ENABLED`
- `OMNILLM_GIT_REMOTE_ENABLED`
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED`
- `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`
- GitHub PR read/create/reply/thread-resolution/ready gates
- per-remote `allow_*` policy
- tool-level/scoped approval policy
- exact branch/head/remote-state preconditions

Authentication is not authorization.

## Subsequent slices

1. **G3 — request-scoped GitHub credential resolver for existing Git/GitHub tools.**
2. **G4 — repository discovery and explicit repository-to-local-worktree binding.**
3. **G5 — Settings UI: Connect GitHub, choose repositories, status, reconnect, disconnect.**
4. **M2 — continue merge-policy completeness work independently.**
5. **M3 — consider guarded direct merge only after M2 can prove policy completeness.**
