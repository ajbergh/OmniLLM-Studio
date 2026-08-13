# GitHub App authentication and binding authorization

OmniLLM-Studio supports a first-class, user-scoped GitHub App connection while preserving the operator-owned authorization boundaries of the existing Git/GitHub tool chain. This document records the implemented authentication, repository-binding, and binding-derived authorization model.

## Why a GitHub App

The Git/GitHub tool chain has separate operator gates for remote reads, fetch, push, remote branch creation, pull-request reads, draft pull-request creation, review replies, review-thread resolution, ready-for-review, and guarded merge. Authentication must not collapse those authorization boundaries.

A GitHub App user access token is therefore only a credential source. Connecting GitHub does **not** enable Git writes, push, branch publication, pull-request creation, review mutations, merge, clone, or default-branch push.

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

`backend/internal/api/router.go` constructs `NewGitHubAuthHandlerFromEnvironment(database)` once and mounts `MountGitHubAuthRoutes` only inside the existing authenticated `/v1` route group, immediately after the current-user route. The composition change is deliberately limited to the authenticated application boundary and does not move or rewrite unrelated routes.

Missing `OMNILLM_GITHUB_APP_CLIENT_ID` remains a supported runtime state: `GET /v1/github/auth` reports `configured=false`, start/poll return bounded service-unavailable responses, and disconnect remains idempotent. Solo mode continues to use the stable `local` owner through the same authenticated-route-group middleware boundary.

No Git/GitHub mutation gate is changed by the authentication route composition.

## G3a: request-scoped Git credential adapter and registry seam — implemented

`backend/internal/gitrepo.UserScopedRemoteService` decorates the existing `RemoteService` without changing its Git transport, GitHub API implementations, local repository service, mutation gates, state-binding checks, or per-remote policy.

Credential precedence is explicit:

1. if the invoking user has a GitHub App connection, the connected user credential is substituted for exact `github.com` remotes;
2. if that connection requires reauthorization or token refresh/resolution fails, execution fails closed and does **not** fall back to a shared operator token;
3. if the invoking user has no GitHub App connection, existing `TokenEnv` and public-remote behavior remain the backward-compatible operator/headless fallback;
4. a connected GitHub App credential is never applied to a non-GitHub remote.

The adapter intentionally separates credential **status** from credential **resolution**. `git_remotes` uses only the local/non-network connection-status callback, so the existing low-risk/no-network inventory tool cannot trigger a provider token refresh. Actual token resolution and refresh are deferred until a credentialed Git/GitHub network operation executes.

For app-only GitHub remotes, the adapter adds a backend-only synthetic credential reference to its per-request cloned remote configuration. This lets the existing capability predicates and credential-loading paths remain unchanged while making them credential-source agnostic. The base operator configuration is never mutated, and no token or credential reference is exposed in tool arguments or results.

`tools.NewRegistry()` remains the backward-compatible zero-dependency constructor. `tools.NewRegistryWithOptions(...)` adds injected request-scoped GitHub credential/binding dependencies and derives the stable owner from the existing invocation scope (`local` in solo mode). G3a initially used that seam only to rebind already-registered tool service interfaces; G6A and G6C later extended the same seam so binding-backed repositories can bootstrap operator-authorized tool shells without consulting user connection, token, or binding state during registry construction.

The following controls remain independent and authoritative:

- `OMNILLM_GIT_WRITE_ENABLED`
- `OMNILLM_GIT_REMOTE_ENABLED`
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED`
- `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`
- GitHub PR read/create/reply/thread-resolution/ready/merge gates
- static per-remote `allow_*` policy
- binding-derived operator policy from `OMNILLM_GITHUB_BINDING_CAPABILITIES_JSON`
- tool-level/scoped approval policy
- exact branch/head/remote-state preconditions

Authentication is not authorization.

## G3b: application credential composition — implemented

`NewRouterWithShutdown` constructs one shared GitHub auth runtime service and handler through `NewGitHubAuthRuntimeFromEnvironment(database)`. That exact service continues to back the authenticated `/v1/github/auth` routes and is also bound to the already-created tool registry through `ConfigureGitHubCredentials`.

The registry adapter preserves the intended credential semantics:

- the `git_remotes` status callback calls `githubauth.Service.Status`, which is local/non-network and cannot refresh a token;
- credentialed network operations call `AccessToken(ctx, owner)`, so expiring access tokens refresh only while an operation already permits network access;
- `githubauth.ErrNotConnected` maps to `connected=false`, preserving existing operator `TokenEnv` or public-remote fallback when the user has no GitHub connection;
- a persisted GitHub identity retains user-credential precedence even when its token is stale, so reauthorization is required rather than silently falling back to a shared operator token;
- reauthorization, refresh, persistence, or provider failures map to fail-closed execution and never fall back to operator credentials;
- solo mode uses the existing stable `local` invocation owner, while multi-user mode uses the authenticated invocation user ID;
- the same underlying `RemoteService` remains in use, so Git mutation gates, operator policy, approval policy, and exact reviewed-state preconditions are unchanged.

`ConfigureGitHubCredentials` supports post-construction binding and resolver replacement without nesting `UserScopedRemoteService` adapters. This keeps the registry initialization order intact.

No model-facing credential value or provider token is introduced by this composition.

## G4a: repository discovery and explicit binding — implemented

The authenticated GitHub surface supports bounded repository discovery and explicit owner-scoped association with an already configured local Git repository ID.

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

G4a does not make a binding an authorization grant.

## G4b: binding-backed request-scoped remotes — implemented

Active G4a bindings participate in the existing `UserScopedRemoteService` as request-scoped GitHub remotes without mutating global operator configuration.

A binding is eligible only when:

- its stored GitHub user ID matches the currently persisted GitHub identity for the invocation owner;
- its local repository ID is still present in `OMNILLM_GIT_REPOSITORIES`;
- it is not marked disabled;
- its stored GitHub `owner/repository` name passes the existing bounded GitHub owner/repository validation.

Eligible bindings receive a deterministic model-facing remote ID derived from the local repository ID (`github-<localRepositoryId>`, with bounded hashing for unusually long IDs). The exact `https://github.com/{owner}/{repository}.git` URL is constructed internally from validated stored identity. Tool arguments continue to accept only stable remote IDs; raw URLs, hostnames, worktree paths, and credentials never become model-facing arguments.

Static `OMNILLM_GIT_REMOTES_JSON` entries remain authoritative. If a synthesized binding would collide with a static remote ID, the static operator remote wins and the binding does not override it.

Binding lookup and GitHub connection-status lookup are local-only. `git_remotes` can inventory active bindings and report authentication presence without resolving or refreshing a token. Credential resolution remains deferred until a credentialed network operation executes, where the connected user's token is injected only into a per-request cloned `RemoteService`. The base remote map is never modified.

Bindings whose stored GitHub user ID belongs to a previously connected account are excluded. Disconnect removes the persisted GitHub identity, so bindings disappear from runtime inventory. If the same persisted identity instead requires reauthorization, the binding may remain visible for local inventory while actual network execution fails closed through `AccessToken` rather than falling back to operator credentials.

The initial G4b implementation synthesized bindings with no per-remote authorization flags. This established the fail-closed baseline: connecting/binding GitHub alone cannot enable push, branch publication, hosted PR operations, default-branch push, or clone. G6B preserves that baseline for repositories with no explicit operator binding policy and adds narrowly scoped opt-in authorization for selected operations.

`git_fetch` remains governed by its existing global remote gate, local Git write gate, approval policy, and reviewed local/remote-state preconditions. It is intentionally not a capability in the G6 binding policy because fetch mutates only the bounded local object database/tracking reference and does not publish a hosted mutation.

The API-layer runtime carries the GitHub auth service and repository handler together, allowing the router composition to supply both credential and binding state to the registry.

Model-facing descriptions for `git_remotes`, `git_remote_status`, and `git_fetch` refer to remote IDs available to the invocation rather than incorrectly implying that every readable/fetchable remote is a static operator entry.

Authentication remains distinct from authorization.

## G5: Settings UI — implemented

The existing **Tools** settings surface includes a first-class GitHub connection and repository-binding section immediately adjacent to scoped tool authorization.

The frontend adds a dedicated `githubSettingsApi` client and `GitHubSettingsSection` component with the following behavior:

- status distinguishes GitHub App configuration, connected identity, token expiry metadata, and reauthorization state without exposing token material;
- connect/reconnect uses the backend device-flow start/poll endpoints and displays only the GitHub user code, fixed verification URI, and expiry;
- the client schedules exactly one poll request at a time, honors backend/provider `retry_after_seconds`, and cancels timers/requests on unmount, restart, or disconnect;
- disconnect is explicitly described as removing OmniLLM-Studio's local GitHub connection rather than claiming GitHub-side revocation;
- repository discovery loads bounded pages and supports incremental pagination;
- users bind immutable GitHub numeric repository IDs to administrator-configured local repository IDs, replace an existing mapping, or remove it;
- stale bindings are surfaced when the connected account no longer matches or the local repository ID is no longer configured;
- the UI never asks for or displays filesystem paths, remote URLs, provider device codes, access tokens, or refresh tokens.

The section is placed immediately above `Scoped Tool Restrictions` to reinforce that GitHub identity/repository selection and tool authorization are separate controls. The UI explicitly states that connecting or binding GitHub does **not** enable push, branch publication, pull-request mutations, clone, merge, or any other write capability. Operator policy, process-wide gates, scoped restrictions, tool approvals, and reviewed-state preconditions remain authoritative.

Focused frontend tests cover device-flow poll scheduling, provider-directed delay changes, and terminal polling states.

## G6A: binding-only remote inspection bootstrap — implemented

Merged in PR #135 (`81ff3dbcb2ac7b02ae89e1d394344664ca308ba0`).

A valid request-scoped GitHub repository binding can now make `git_remotes` and `git_remote_status` available even when there is no matching static `OMNILLM_GIT_REMOTES_JSON` entry. The bootstrap is deliberately limited to the read/inventory shell needed to discover and inspect the binding-backed remote.

The bootstrap requires:

- at least one startup-configured local repository ID;
- the process-wide remote access gate;
- complete GitHub credential and binding callbacks supplied by application composition.

Registry construction does not call the binding resolver or token resolver. A user's current binding is looked up only for the invocation. This makes binding-backed repositories discoverable without converting authentication or repository association into write authority.

## G6B1: operator-owned binding capability schema — implemented

Merged in PR #144 (`e1af0a343e939354690b49054fe15388a3675f78`).

`OMNILLM_GITHUB_BINDING_CAPABILITIES_JSON` is an operator-owned map keyed by startup-allowlisted local repository IDs. Each entry may opt a binding-derived GitHub remote into selected capabilities:

- `allow_push`
- `allow_branch_create`
- `allow_pull_request_read`
- `allow_pull_request_create`
- `allow_pull_request_reply`
- `allow_pull_request_thread_resolution`
- `allow_pull_request_ready`
- `allow_pull_request_merge`
- `pull_request_merge_method` when merge is enabled

The parser fails closed per entry:

- invalid repository IDs are ignored;
- unknown fields reject only the affected entry;
- branch creation is invalid without push;
- merge is invalid without PR read and a valid merge method;
- a merge method is invalid when merge is disabled.

The schema intentionally cannot grant `allow_default_branch_push` or `allow_clone`. Those operations require separate static-remote/operator configuration and are not available to binding-derived policy.

User connection state, repository discovery, and repository bindings cannot populate or modify this environment-owned policy.

## G6B2: runtime application of binding capability policy — implemented

Merged in PR #145 (`9905ca0b62bdfcbea20578121bdaa971397354ce`).

`RemoteService` snapshots the binding capability map once during startup construction and retains only entries whose local repository IDs remain in the startup repository allowlist. Later environment changes do not mutate the live policy snapshot.

When an invocation supplies an eligible binding, `UserScopedRemoteService` synthesizes the remote, checks the normal static-remote collision rule, and then applies the matching snapshotted operator policy. A binding with no policy remains at the original G4b read-only authorization baseline.

Per-binding flags do not replace process-wide gates. Effective capability remains the intersection of:

1. startup local-repository allowlisting;
2. active owner-scoped binding identity;
3. operator binding policy where required;
4. the corresponding process-wide gate;
5. request-scoped credential availability for credentialed operations;
6. tool/scoped approval policy;
7. the operation's exact reviewed-state and branch/head preconditions.

Tests prove that binding policy cannot bypass disabled global gates and cannot grant clone or default-branch push even if those unrelated process-wide gates are enabled.

## G6C: binding-authorized tool-shell registration — implemented

Merged in PR #146 (`1e20c6c941f3944ea637d7e2a6c3b57097c0d49f`).

Binding-only configurations can now register the Git/GitHub tool shells that startup operator policy can potentially authorize. Registration is based only on the snapshotted operator policy plus process-wide gates; it does not query the invoking user's connection, token, or binding.

`GitHubBindingToolCapabilities` summarizes potential startup authority for:

- fetch;
- push;
- remote branch publication;
- PR reads and review-thread reads;
- draft PR creation;
- review replies;
- review-thread resolution;
- ready-for-review;
- guarded merge.

Important boundaries:

- `git_fetch` is registered independently when remote access and local Git write are enabled, matching the pre-existing G4b fetch model;
- `git_push` requires a binding policy that allows push plus the global push prerequisites;
- `git_publish_branch` is filtered independently and requires `allow_branch_create`; `allow_push` alone does not surface branch publication;
- each hosted GitHub family requires its matching binding-policy flag and process-wide gate;
- `git_clone` is never registered from binding-derived policy;
- registration is additive/idempotent when static remotes already caused some tool shells to exist;
- every bootstrapped shell is rebound to `UserScopedRemoteService`, so actual execution still resolves the invocation owner and rechecks the exact binding remote rather than using startup policy as an executable credential or remote target.

G6C regression tests additionally corrected an older negative test that referenced the nonexistent `github_create_pull_request` name instead of the canonical `github_create_draft_pull_request`, so the no-policy bootstrap test now genuinely protects against accidental draft-PR registration.

## Effective authorization matrix

A check mark means the factor is required for the binding-backed operation. Additional operation-specific validation may still apply.

| Operation | Active owner binding | Binding policy | Process-wide gate(s) | User credential when needed | Approval / reviewed state | Binding policy can grant clone/default-branch push? |
|---|---:|---:|---:|---:|---:|---:|
| `git_remotes` | ✓ | — | remote enabled | status only; no token refresh | normal tool policy | No |
| `git_remote_status` | ✓ | — | remote enabled | ✓ for private/credentialed remote | ✓ | No |
| `git_fetch` | ✓ | — | remote enabled + local Git write | ✓ for private/credentialed remote | ✓ exact local/remote state | No |
| `git_push` | ✓ | `allow_push` | remote + push + local Git write | ✓ | ✓ exact branch/head/remote state | No |
| `git_publish_branch` | ✓ | `allow_push` + `allow_branch_create` | remote + push + branch-create + local Git write | ✓ | ✓ exact reviewed local state | No |
| PR read / review threads | ✓ | `allow_pull_request_read` | remote + PR-read | ✓ | normal tool policy | No |
| Draft PR create | ✓ | `allow_pull_request_create` | remote + PR-create | ✓ | ✓ exact published/reviewed state | No |
| Review reply | ✓ | `allow_pull_request_reply` | remote + reply | ✓ | ✓ | No |
| Thread resolve/unresolve | ✓ | `allow_pull_request_thread_resolution` | remote + thread-resolution | ✓ | ✓ | No |
| Mark ready | ✓ | `allow_pull_request_ready` | remote + ready | ✓ | ✓ | No |
| Guarded PR merge | ✓ | `allow_pull_request_read` + `allow_pull_request_merge` + merge method | remote + PR-read + merge | ✓ | ✓ merge-policy/preflight evidence | No |
| Clone | N/A for binding-derived authority | unsupported | static clone controls only | as configured statically | static clone checks | **No** |
| Default-branch push | N/A for binding-derived authority | unsupported | static push/default-branch controls only | as configured statically | exact push checks | **No** |

The matrix is intentionally conjunctive. A GitHub connection, repository binding, operator binding policy, or process-wide flag by itself is never sufficient for a side-effecting operation.

## Current follow-up work

The G1-G6 authentication/binding authorization path is implemented. Follow-up work should remain separate from the credential/authorization boundary:

1. keep operator deployment examples and Settings copy synchronized with new binding-policy fields as configuration surfaces evolve;
2. preserve negative authorization tests whenever new Git/GitHub tool families are added;
3. keep merge-policy/eligibility evidence and guarded direct merge tests aligned with GitHub API behavior without making authentication imply merge authority;
4. treat clone and default-branch publication as separate high-risk operator-controlled features unless an explicit future design re-evaluates them.

Any future capability added to binding-derived remotes must start fail-closed, require an explicit operator-owned policy field, remain underneath its process-wide gate, and preserve request-scoped identity plus operation-specific reviewed-state checks.
