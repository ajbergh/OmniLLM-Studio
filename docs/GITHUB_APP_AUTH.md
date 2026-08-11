# GitHub App authentication foundation

OmniLLM-Studio is moving from operator-supplied static GitHub tokens toward a first-class, user-scoped GitHub App connection. This document defines the boundary for that work.

## Why a GitHub App

The existing Git/GitHub tool chain already has separate operator gates for remote reads, fetch, push, remote branch creation, pull-request reads, draft pull-request creation, review replies, review-thread resolution, and ready-for-review. Authentication must not collapse those authorization boundaries.

A GitHub App user access token is therefore treated only as a credential source. Connecting GitHub does **not** enable Git writes, push, branch publication, pull-request creation, review mutations, or future merge capability.

## G1: device-flow service

`backend/internal/githubauth` provides the first backend-only authentication slice:

- GitHub App client ID is operator-owned configuration via `OMNILLM_GITHUB_APP_CLIENT_ID`.
- Device authorization uses GitHub's fixed `https://github.com/login/device/code` and `https://github.com/login/oauth/access_token` endpoints.
- The provider `device_code` remains backend-only; API-facing results contain only the user code, verification URI, expiry, and minimum poll interval.
- Polling is bounded to one provider request per service call and never starts a background polling loop.
- Provider polling intervals are enforced in the backend, including `slow_down` handling.
- A successful token exchange is immediately bound to the authenticated GitHub identity through `GET https://api.github.com/user` before credentials are accepted.
- Expiring user access tokens can be refreshed with the device-flow refresh token without a GitHub App client secret.
- Token response bodies and provider error bodies are never copied into public errors.
- Authentication responses are size-bounded and use fixed provider endpoints.

The package defines a `CredentialStore` contract but intentionally does not select a persistence implementation in G1. Implementations must encrypt access and refresh tokens at rest and must never expose them through API responses or model/tool context.

## Persistence boundary

Do **not** store GitHub App credentials in the generic `settings` table. The general settings API enumerates that table, which is the wrong visibility boundary even for encrypted credential blobs.

The next slice should add a dedicated user-scoped persistence surface (planned schema V51) with:

- OmniLLM owner/user ID as the primary ownership key;
- encrypted access token;
- encrypted refresh token;
- access-token expiry;
- refresh-token expiry;
- GitHub numeric user ID and login;
- token type and non-secret metadata;
- no plaintext token columns.

The repository should follow the existing `repository.MCPOAuthRepo` pattern and reuse `internal/crypto` AES-256-GCM encryption.

## Planned authenticated API

After dedicated persistence is in place, wire user-authenticated routes such as:

```text
GET    /v1/github/auth
POST   /v1/github/auth/device/start
POST   /v1/github/auth/device/poll
DELETE /v1/github/auth
```

These routes are user-scoped through `auth.ScopeUserIDFromContext`. Device poll performs one provider request at most; clients retry according to `retry_after_seconds`.

A later web authorization flow may be added for browser/server deployments. Desktop/headless operation should continue to support device flow.

## G3: Git credential resolver

Only after persistence and authenticated API wiring are proven should `backend/internal/gitrepo` consume these credentials.

The intended integration is a backend-only credential provider that can resolve an access token for `(userID, remoteID)` and refresh it as needed. `TokenEnv` remains a backward-compatible operator/headless fallback.

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

1. **G2 — encrypted user-scoped persistence and authenticated API routes.**
2. **G3 — request-scoped GitHub credential resolver for existing Git/GitHub tools.**
3. **G4 — repository discovery and explicit repository-to-local-worktree binding.**
4. **G5 — Settings UI: Connect GitHub, choose repositories, status, reconnect, disconnect.**
5. **M2 — continue merge-policy completeness work independently.**
6. **M3 — consider guarded direct merge only after M2 can prove policy completeness.**
