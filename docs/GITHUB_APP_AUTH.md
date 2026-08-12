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

## G3b: application credential composition — implemented

`NewRouterWithShutdown` now constructs one shared GitHub auth runtime service and handler through `NewGitHubAuthRuntimeFromEnvironment(database)`. That exact service continues to back the authenticated `/v1/github/auth` routes and is also bound to the already-created tool registry through `ConfigureGitHubCredentials`.

The registry adapter preserves the intended credential semantics:

- the `git_remotes` status callback calls `githubauth.Service.Status`, which is local/non-network and cannot refresh a token;
- credentialed network operations call `AccessToken(ctx, owner)`, so expiring access tokens refresh only while an operation already permits network access;
- `githubauth.ErrNotConnected` maps to `connected=false`, preserving existing operator `TokenEnv` or public-remote fallback when the user has no GitHub connection;
- a persisted GitHub identity retains user-credential precedence even when its token is stale, so reauthorization is required rather than silently falling back to a shared operator token;
- reauthorization, refresh, persistence, or provider failures map to fail-closed execution and never fall back to operator credentials;
- solo mode uses the existing stable `local` invocation owner, while multi-user mode uses the authenticated invocation user ID;
- the same underlying `RemoteService` remains in use, so Git mutation gates, per-remote `allow_*` policy, approval policy, and exact reviewed-state preconditions are unchanged.

`ConfigureGitHubCredentials` also supports post-construction binding and resolver replacement without nesting `UserScopedRemoteService` adapters. This keeps the existing registry initialization order intact and makes the router change limited to replacing the handler-only GitHub auth constructor with the shared runtime constructor plus one registry-binding call.

No model-facing tool schema or permission gate changes in G3b.

## G4a: repository discovery and explicit binding — implemented

The authenticated GitHub surface now supports bounded repository discovery and explicit owner-scoped association with an already configured local Git repository ID.

Repository discovery is implemented by `backend/internal/githubrepo` and uses only fixed GitHub API endpoints:

- `GET /user/repos` lists repositories visible to the connected user with bounded page size and response size;
- `GET /repositories/{id}` validates a selected repository using its immutable numeric GitHub repository ID;
- redirects are disabled and provider response bodies/errors are not copied into public errors;
- access tokens remain backend-only and are obtained from the existing user-scoped `githubauth.Service` only when a discovery request performs network access.

The authenticated API adds:

```text
GET    /v1/github/repositories?page=&per_page=
GET    /v1/github/repository-bindings
PUT    /v1/github/repository-bindings/{localRepositoryId}
DELETE /v1/github/repository-bindings/{localRepositoryId}
```

Binding requests accept only a numeric `github_repository_id`. The local side accepts only a stable repository ID already present in the startup `OMNILLM_GIT_REPOSITORIES` allowlist. Filesystem paths, GitHub remote URLs, hostnames, and credentials are neither accepted nor returned by this surface.

`github_repository_bindings` persists only the OmniLLM owner ID, stable local repository ID, connected GitHub numeric user ID, immutable GitHub repository ID, full repository name, default branch, and non-secret repository state flags. It stores no local worktree path and no token material.

Bindings deliberately retain the GitHub numeric user ID that created them. After reconnecting as a different GitHub account, an old binding is reported with `account_matches=false` instead of silently becoming active for the new identity. A binding whose local repository ID is no longer configured is similarly reported with `local_configured=false`.

G4a does not add model-facing tools and does not make a binding an authorization grant.

## G4b: binding-backed request-scoped remotes — implemented

Active G4a bindings now participate in the existing `UserScopedRemoteService` as request-scoped GitHub remotes without mutating global operator configuration.

A binding is eligible only when:

- its stored GitHub user ID matches the currently persisted GitHub identity for the invocation owner;
- its local repository ID is still present in `OMNILLM_GIT_REPOSITORIES`;
- it is not marked disabled;
- its stored GitHub `owner/repository` name passes the existing bounded GitHub owner/repository validation.

Eligible bindings receive a deterministic model-facing remote ID derived from the local repository ID (`github-<localRepositoryId>`, with bounded hashing for unusually long IDs). The exact `https://github.com/{owner}/{repository}.git` URL is constructed internally from validated stored identity. Tool arguments continue to accept only stable remote IDs; raw URLs, hostnames, worktree paths, and credentials never become model-facing arguments.

Static `OMNILLM_GIT_REMOTES_JSON` entries remain authoritative. If a synthesized binding would collide with a static remote ID, the static operator remote wins and the binding does not override it.

Binding lookup and GitHub connection-status lookup are local-only. `git_remotes` can inventory active bindings and report authentication presence without resolving or refreshing a token. Credential resolution remains deferred until a credentialed network operation executes, where the connected user's token is injected only into a per-request cloned `RemoteService`. The base remote map is never modified.

Bindings whose stored GitHub user ID belongs to a previously connected account are excluded. Disconnect removes the persisted GitHub identity, so bindings disappear from runtime inventory. If the same persisted identity instead requires reauthorization, the binding may remain visible for local inventory while actual network execution fails closed through `AccessToken` rather than falling back to operator credentials.

Synthesized binding remotes deliberately set **none** of the per-remote authorization flags. They do not grant:

- `allow_push`
- `allow_branch_create`
- `allow_pull_request_read`
- `allow_pull_request_create`
- `allow_pull_request_reply`
- `allow_pull_request_thread_resolution`
- `allow_pull_request_ready`
- `allow_default_branch_push`
- `allow_clone`

Consequently, connecting/binding GitHub does not enable push, branch publication, hosted PR operations, default-branch push, or clone. Existing process-wide gates, per-remote policy, scoped/tool approvals, and exact reviewed-state preconditions remain authoritative. `git_fetch` remains governed by its existing global remote gate, local Git write gate, approval policy, and reviewed local/remote state preconditions.

The API-layer runtime now carries the GitHub auth service and repository handler together, allowing the existing router composition call to supply both credential and binding state to the registry without changing `router.go` for G4b.

Model-facing descriptions for `git_remotes`, `git_remote_status`, and `git_fetch` now refer to remote IDs available to the invocation rather than incorrectly implying that every readable/fetchable remote is a static operator entry.

Authentication remains distinct from authorization.

## Subsequent slices

1. **G5 — Settings UI: Connect GitHub, choose repositories, status, reconnect, disconnect.**
2. **M2 — continue merge-policy completeness work independently.**
3. **M3 — consider guarded direct merge only after M2 can prove policy completeness.**
