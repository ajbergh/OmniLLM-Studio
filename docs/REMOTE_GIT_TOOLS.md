# Remote Git tools

OmniLLM-Studio exposes remote Git as a separate security boundary from local repository access. Remote operations combine outbound network access, credentials, local repository mutation, remote ref mutation, and—in the clone path—filesystem creation. Every remote endpoint, repository binding, credential reference, and destructive-capability opt-in therefore comes from operator configuration rather than model arguments.

Local repository inspection and mutation are documented in `docs/LOCAL_GIT_TOOLS.md`.

## Operator configuration

Remote definitions live in `OMNILLM_GIT_REMOTES_JSON`, keyed by stable remote ID:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true,
    "allow_branch_create": false,
    "allow_default_branch_push": false,
    "allow_clone": false
  }
}
```

`repository` is an ID from `OMNILLM_GIT_REPOSITORIES`. For status, fetch, push, and branch publication it identifies an existing configured worktree. For clone it may identify an absent operator-preconfigured destination whose real parent directory already exists. The model never receives or supplies the configured filesystem path.

`token_env` names an operator environment variable. The token value is loaded only when authentication is needed and is never returned in tool results or accepted in tool arguments.

The following process-wide gates are independent:

- `OMNILLM_GIT_REMOTE_ENABLED=true` enables outbound remote Git inspection.
- `OMNILLM_GIT_WRITE_ENABLED=true` is additionally required by fetch, push, branch publication, and clone because each mutates local or remote state.
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED=true` is additionally required by `git_push` and is also a prerequisite for branch publication.
- `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED=true` is additionally required by `git_publish_branch`. Ordinary push enablement does not enable remote ref creation.
- `OMNILLM_GIT_REMOTE_CLONE_ENABLED=true` is additionally required by `git_clone`.
- `OMNILLM_GIT_CLONE_MAX_BYTES=<bytes>` and `OMNILLM_GIT_CLONE_MAX_ENTRIES=<count>` are mandatory clone budgets. Accepted ranges are 1 MiB–1 GiB and 128–100,000 entries.

Per-remote gates are also independent:

- `allow_push: true` permits guarded updates of existing branches when the process push gate is enabled.
- `allow_branch_create: true` permits guarded publication of a new same-named branch when the separate process branch-create gate is enabled. This setting is invalid unless `allow_push` is also true.
- `allow_default_branch_push: true` permits guarded updates to the existing default branch. This setting is invalid unless `allow_push` is also true.
- `allow_clone: true` permits guarded clone into the remote's configured repository destination.

`git_publish_branch` never creates `main`, `master`, or the advertised default branch, even when default-branch push is enabled. Creating a new remote ref and updating an existing protected ref intentionally remain separate capabilities.

Tool policy is another independent gate. `git_remote_status` and `git_fetch` are network-sensitive operations that default to `ask`. `git_push`, `git_publish_branch`, and `git_clone` are critical-risk, networked, side-effecting, and non-parallel and therefore default to `ask` after operator enablement.

## Egress boundary

Remote definitions accept only exact configured HTTPS endpoints on the normal HTTPS port. Configuration rejects embedded URL credentials, query strings, fragments, HTTP, SSH, `git://`, `file://`, alternate ports, environment proxies, redirects, and private/loopback/link-local/reserved/metadata destinations.

The dedicated remote Git transport resolves DNS, validates every returned address, and dials a validated address directly. It does not replace `http.DefaultTransport` or go-git's global protocol registry. Advertised-reference HTTP responses are bounded.

Private-network Git servers are intentionally unsupported by this boundary. Supporting them requires an explicit operator destination allowlist rather than a blanket private-network bypass.

## Available tools

| Tool | Purpose | Risk / side effect |
|---|---|---|
| `git_remotes` | List safe summaries for configured remotes without URLs, credential references, secrets, or filesystem paths. | Low-risk local inventory. |
| `git_remote_status` | Inspect one configured remote and return a bounded branch list plus a digest of the complete branch namespace. | High-risk network read; defaults to `ask`. |
| `git_fetch` | Fetch the exact reviewed head of the current local branch into bounded local object storage and an isolated tracking ref. | High-risk network + local mutation; defaults to `ask`. |
| `git_push` | Fast-forward the exact reviewed local HEAD to the same-named **existing** remote branch after reviewed fetch. | Critical-risk remote mutation; defaults to `ask`. |
| `git_publish_branch` | Create the same-named **new** remote branch at the exact reviewed local HEAD after reviewing remote branch state. | Critical-risk remote ref creation; defaults to `ask`. |
| `git_clone` | Clone one exact reviewed branch into the remote's absent operator-preconfigured destination under transfer/object/storage/entry limits. | Critical-risk network + repository creation; defaults to `ask`. |

No remote tool accepts a raw URL, token, credential reference, arbitrary refspec, force flag, remote deletion request, model-selected clone path, or caller-selected quota.

## Reviewed remote state

`git_remote_status` returns advertised branch heads, capped at 200 entries for display, and a `branch_state_digest`. The digest is SHA-256 over the complete advertised `refs/heads/*` namespace sorted by ref name and head hash. It intentionally excludes tags and other refs.

The digest serves a different purpose from a branch head hash:

- existing-branch fetch/push binds to the reviewed 40-character `expected_remote_head` for that branch;
- new-branch publication binds to the reviewed `branch_state_digest`, because branch absence has no object hash to carry forward.

A branch-state digest still covers branches omitted from the bounded display. Any branch creation, deletion, or head movement changes the digest and makes a pending publication stale. Tag-only changes do not invalidate branch publication approval.

## Guarded fetch

`git_fetch` requires:

- `remote` from `git_remotes`;
- `expected_branch` and `expected_head` from `git_status`;
- `expected_remote_head` for that same branch from reviewed `git_remote_status`.

The service serializes with OmniLLM local Git mutations, rechecks local branch/HEAD, re-advertises the configured remote, and requires the remote branch to still equal `expected_remote_head`.

Fetch uses the dedicated egress-guarded upload-pack transport. The request wants only the reviewed branch head, disables sideband and thin-pack modes, never changes HEAD/index/worktree/config, and never accepts an arbitrary refspec.

Compressed received pack data is capped at 64 MiB. Successful fetch records the reviewed remote state under:

```text
refs/remotes/omnillm/<remote-id-digest>/<branch>
```

The raw remote ID is not embedded in the tracking namespace. `git_push` requires this tracking ref to still equal the reviewed remote head.

## Guarded push of an existing branch

`git_push` updates only the current local branch's same-named **existing** remote branch. It requires the remote/write/push gates, per-remote `allow_push`, critical-risk approval, exact local branch/HEAD preconditions, the preceding fetched tracking state, and a verified fast-forward relationship.

Immediately before push the service re-advertises the remote and requires the branch to still equal `expected_remote_head`. Direct default-branch updates are rejected unless `allow_default_branch_push` is enabled; `main` and `master` remain conservative fallbacks when HEAD symref information is unavailable.

The receive-pack request contains exactly one command:

```text
refs/heads/<current-branch>: expected-remote-head -> expected-local-head
```

The reviewed remote hash is sent as the command's old object ID. A concurrent remote branch change therefore causes the server to reject the update instead of overwriting newer state.

Only fast-forward updates are supported. `git_push` does not create/delete refs, force, force-with-lease, mirror, prune, follow tags, push tags, send arbitrary push options, or accept arbitrary refspecs.

Objects are enumerated from the exact local HEAD while advertised remote objects are treated as haves. The object list is capped at 100,000 and the generated pack at 64 MiB. Pack generation streams directly into the receive-pack session.

After success the isolated OmniLLM tracking ref advances to the pushed HEAD. If local tracking bookkeeping fails after the remote accepted the command, the tool reports that the remote mutation succeeded and requires a fresh `git_remote_status` before another mutation.

## Guarded publication of a new branch

`git_publish_branch` fills the safe workflow gap between `git_create_branch`/`git_commit` and later `git_push` calls. It creates a remote ref; it is therefore deliberately not folded into ordinary push.

It accepts exactly four model-facing arguments:

- `remote` — configured remote ID;
- `expected_branch` — exact current local branch from `git_status`;
- `expected_head` — exact local HEAD from `git_status`;
- `expected_remote_state_digest` — exact `branch_state_digest` from reviewed `git_remote_status`.

Publication requires:

- remote network, local write, remote push, and separate remote branch-create process gates;
- per-remote `allow_push: true` and `allow_branch_create: true`;
- critical-risk tool approval;
- a valid current local branch/HEAD;
- a reviewed remote branch-state digest;
- the same branch name to be absent from the remote;
- the target not to be `main`, `master`, or the advertised default branch.

The service re-advertises the remote immediately before publication and recomputes the complete branch-state digest. If any remote branch was created, deleted, or moved after review, publication fails and requires a fresh `git_remote_status`.

The receive-pack request contains exactly one branch-creation command:

```text
refs/heads/<current-branch>: 0000000000000000000000000000000000000000 -> expected-local-head
```

The zero old object ID is the server-side compare-and-swap precondition for ref creation. If another actor creates the target branch after the digest check but before the command is applied, the server rejects the command rather than allowing OmniLLM-Studio to convert the operation into an update.

Publication reuses the guarded push pack path: at most 100,000 objects and 64 MiB of generated pack data. It never creates a differently named remote branch, creates a caller-selected ref, updates an existing remote branch, creates tags, deletes refs, or forces an update.

After success the isolated OmniLLM tracking ref is set to the published HEAD, allowing later reviewed `git_remote_status` → `git_fetch` → `git_push` operations to follow the normal existing-branch path.

## Guarded clone

`git_clone` requires `remote`, `expected_branch`, and the reviewed 40-character `expected_remote_head`. The caller cannot choose the URL, destination, credentials, refspec, transfer limit, storage quota, or entry limit.

The remote's configured repository ID identifies an absent destination. The destination parent must already exist and may not be or traverse symlinked directories. Clone creates a private `0700` sibling staging directory, performs all object ingestion and checkout there, validates a clean repository at the exact reviewed branch/HEAD, rechecks destination absence, and atomically renames staging into the configured destination. Any failure before promotion removes staging.

Clone uses the same dedicated SSRF-guarded upload-pack transport. The selected branch is re-advertised and must still equal `expected_remote_head`. Thin-pack and sideband are disabled.

Resource controls are independent:

- compressed pack bytes are limited to the smaller of the configured clone byte budget or 128 MiB;
- the pack header must use the version supported by pinned go-git v5.19.2 (version 2) and may declare at most 200,000 objects;
- `.git` storage and worktree checkout share one logical-byte quota;
- files, directories, symlinks, and temporary files share one cumulative entry quota.

Quota accounting covers sparse writes, truncate expansion, symlink target bytes, implicit parent creation through Billy filesystem operations, and clone-local temp files. Entry reservations are conservative and not refunded after removal. Ambiguous filesystem failures keep reserved expansion rather than manufacturing quota credit.

Staging uses Billy `BoundOS`; `.git` is created through the shared quota wrapper before chroot. Empty-directory `TempFile` calls are forced into the clone root instead of the host temp directory.

Application byte/entry limits are hard logical-content admission ceilings, not exact physical-block accounting. Deployments that require a physical disk-consumption ceiling should additionally use an OS/filesystem-enforced quota around configured clone storage.

Clone does not initialize submodules, invoke Git LFS downloads, overwrite an existing directory, dynamically register a model-selected path, or continue after a changed remote head or quota violation.

## Recommended workflows

### Update an existing remote branch

```text
git_status
  → git_diff / local review
  → local stage/commit workflow
  → git_status
  → git_remote_status
  → approval → git_fetch
  → fresh git_status if local state may have changed
  → approval → git_push
```

### Publish a newly created feature branch

```text
git_status
  → git_create_branch / git_checkout
  → git_diff / git_stage / git_status / git_commit
  → git_status
  → git_remote_status
  → verify the same-named branch is absent and retain branch_state_digest
  → approval → git_publish_branch
  → fresh git_remote_status before any later remote mutation
```

If the target branch already exists, do not publish it. Use the reviewed `git_fetch` → `git_push` path instead.

### Clone into an operator-preconfigured destination

```text
git_remotes
  → git_remote_status
  → choose reviewed branch/head
  → approval → git_clone
  → git_status / git_log / git_show using the configured repository ID
```

Stale local state, stale remote branch state, an existing publication target, non-fast-forward history, protected branch names, disabled gates, denied tool policy, invalid packs, excessive object counts, or transfer/storage/entry limit violations all fail closed. The remedy is refreshed inspection or operator configuration—not force or retry-through behavior.

## Validation expectations

Focused backend coverage should include remote configuration rejection and credential indirection; global and per-remote gates; status branch-state digest behavior; strict tool arguments; fetch and push transfer limits; tracking refs; fast-forward/default-branch policy; branch publication absence/digest checks and zero-hash creation CAS; clone destination confinement and transfer/object/storage/entry quotas; and conditional registry wiring.

Before merging remote Git changes, validate the exact final head with repository formatting, `go vet`, unit/integration tests, race detection, Windows desktop compilation, frontend checks, Playwright smoke coverage, dependency audits, Go and JavaScript/TypeScript CodeQL, Helm validation, and container builds. Review Advanced Security and PR review threads before readiness.
