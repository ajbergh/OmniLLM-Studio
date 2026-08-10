# Remote Git tools — Phase 3

Phase 3 extends the guarded local Git capabilities in `docs/LOCAL_GIT_TOOLS.md` to remote repositories. Remote Git is treated as a separate security boundary because it combines outbound network access, credentials, local repository mutation, and potentially consequential remote side effects.

## Security model

Remote endpoints are operator configuration, not model arguments. This is mandatory because tool arguments are persisted in the tool-invocation audit trail. Model-facing calls use only stable remote IDs and state preconditions; they never receive or submit a raw remote URL, credential value, or credential environment-variable name.

Remote configuration uses `OMNILLM_GIT_REMOTES_JSON`, a JSON object keyed by stable remote ID:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true,
    "allow_default_branch_push": false
  }
}
```

The token value lives only in the named operator environment variable. It is loaded immediately before authentication and is not returned by remote inventory/status results or included in tool arguments.

Three independent operator gates apply:

- `OMNILLM_GIT_REMOTE_ENABLED=true` — permits outbound remote Git inspection.
- `OMNILLM_GIT_WRITE_ENABLED=true` — additionally required by `git_fetch` because fetch mutates the local object database and an isolated remote-tracking ref.
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED=true` — additionally required by `git_push`; the selected remote must also set `allow_push: true`.

Direct pushes to the remote's default branch are denied unless that remote also sets `allow_default_branch_push: true`. `allow_default_branch_push` is invalid unless `allow_push` is also enabled.

The existing tool approval policy remains another independent gate. `git_remote_status` and `git_fetch` are high-risk network operations that default to `ask`. `git_push` is critical-risk, networked, side-effecting, and non-parallel, so it also defaults to `ask` even after all operator gates are enabled.

## Egress boundary

Remote definitions accept only exact configured HTTPS endpoints on the normal HTTPS port. Configuration rejects URL credentials, query strings, fragments, HTTP, SSH, `git://`, `file://`, alternate ports, private/loopback/link-local/reserved/metadata IP ranges, environment proxies, and redirects.

DNS is resolved once and every returned address is validated before dialing one validated IP directly. The Git HTTP transport is dedicated to the remote Git service and does not replace `http.DefaultTransport` or go-git's global protocol registry. Remote HTTP response bodies are bounded to 4 MiB.

Private-network Git servers are deliberately unsupported in this phase. Supporting them later requires an explicit destination allowlist rather than a blanket private-network bypass.

## Available tools

| Tool | Purpose | Risk / side effect |
|---|---|---|
| `git_remotes` | List safe summaries for operator-configured remotes. URLs, credential references, and secret values are omitted. | Low-risk read-only local inventory. |
| `git_remote_status` | Contact one configured remote and return a bounded list of advertised branch heads. | High-risk network read; defaults to `ask`. |
| `git_fetch` | Fetch the exact reviewed head of the current local branch into bounded local object storage and an isolated tracking ref. | High-risk local mutation + network; defaults to `ask`. |
| `git_push` | Push the exact reviewed local HEAD to the same-named existing remote branch after a successful reviewed fetch. | Critical-risk remote mutation + network; defaults to `ask`. |

No remote tool accepts a raw URL, token, credential reference, arbitrary refspec, force flag, or destination branch.

## Guarded fetch

`git_fetch` requires:

- `remote` — configured remote ID from `git_remotes`;
- `expected_branch` — current local branch from `git_status`;
- `expected_head` — current local HEAD from `git_status`;
- `expected_remote_head` — hash for that same branch from a reviewed `git_remote_status` result.

The service rechecks local branch/HEAD after acquiring the same mutation mutex used by local Git writes, opens only the configured repository bound to the remote, re-advertises the configured remote branch, and rejects the operation if the remote head changed.

Fetch deliberately does not use go-git's process-global high-level remote transport path. It creates an upload-pack session through OmniLLM-Studio's dedicated egress-guarded transport.

The fetch request is restricted to the current branch's exact remote head. It does not force, prune, fetch tags explicitly, recurse submodules, update Git config, move HEAD, modify the current local branch, change the index/worktree, or accept arbitrary refspecs.

Received pack data has a hard 64 MiB compressed-transfer ceiling. The normal filesystem storer persists the bounded pack into the Git object database. Fetch never checks out the received tree, so it does not expand those objects into arbitrary worktree bytes.

Successful fetch records the reviewed remote head under an OmniLLM-owned tracking namespace:

```text
refs/remotes/omnillm/<remote-id-digest>/<branch>
```

The raw configured remote ID is not embedded in the ref name. `git_push` requires this tracking ref to still equal `expected_remote_head`, enforcing the intended status → fetch → push sequence.

## Guarded push

`git_push` accepts the same four state-binding arguments as fetch. It additionally requires:

- `OMNILLM_GIT_REMOTE_PUSH_ENABLED=true`;
- the existing local write gate;
- the selected remote's `allow_push: true`;
- critical-risk tool approval;
- an existing remote branch with the same name as the current local branch;
- the OmniLLM tracking ref from the preceding fetch to equal `expected_remote_head`;
- the reviewed remote commit to be available locally;
- the reviewed remote commit to be an ancestor of local HEAD, unless both hashes are already equal;
- the remote to still advertise exactly `expected_remote_head` immediately before push;
- default-branch opt-in when the destination is the advertised default branch (with conservative `main`/`master` fallback protection when HEAD symref is unavailable).

The push request contains exactly one receive-pack command:

```text
refs/heads/<current-branch>: expected-remote-head -> expected-local-head
```

The command's old object ID is the reviewed remote hash. The remote Git server therefore evaluates the update against that old hash; if the branch changes before the command is applied, the update fails rather than silently overwriting the newer remote state.

Only fast-forward updates are permitted. Guarded push does not create branches, delete refs, force, force-with-lease, mirror, prune, follow tags, push tags, send arbitrary push options, or accept arbitrary refspecs.

Objects sent to the remote are calculated from the exact local HEAD while treating advertised remote objects as haves. The generated pack has a hard 64 MiB ceiling and the precomputed object list is capped at 100,000 objects. Pack generation streams directly into the dedicated receive-pack session rather than writing an unbounded temporary pack file.

After a successful remote update, the isolated OmniLLM tracking ref is advanced to the pushed HEAD. If that local bookkeeping update fails after the remote has accepted the push, the tool reports that the remote push succeeded but requires a fresh `git_remote_status` before another remote mutation.

## Recommended model workflow

The safe mutation sequence is:

```text
git_status
  → git_diff / local review as needed
  → local branch/stage/commit workflow
  → git_status
  → git_remote_status
  → approval → git_fetch
  → fresh git_status when local state may have changed
  → approval → git_push
```

A stale local HEAD, changed local branch, changed remote head, missing fetched tracking state, non-fast-forward relationship, protected default branch, disabled operator gate, denied tool policy, or exceeded transfer/object limit fails closed and requires a refreshed inspection rather than force or retry-through behavior.

## Clone quota requirement

`git_clone` remains intentionally unregistered. Clone can receive attacker-controlled objects and then expand them into a new worktree before final repository size is known. A compressed pack ceiling alone is not enough because checkout expansion can be much larger than the pack.

Phase 3 will not expose clone until the implementation can enforce a hard storage ceiling across both object ingestion and worktree writes, or place the entire clone target inside an OS-enforced quota boundary. Temporary clone directories must be private, cleaned on failure, and atomically promoted only after repository/path validation.

A free-space preflight plus post-clone size check is not considered sufficient because it detects quota violations only after the potentially damaging writes occurred.

## Validation

Focused backend tests cover remote configuration rejection, credential indirection, global write/network/push gates, remote tool metadata and strict argument contracts, fetch transfer limiting, isolated tracking refs, push default-branch policy, push transfer limiting, and registry gating. Existing repository-wide checks exercise the new code through formatting, vet, unit/integration tests, race detection, Windows desktop compilation, frontend validation, dependency audits, and CodeQL.

Before merging a Phase 3 change, validate the exact final head with the repository's Quality Gate and Security Scan workflows and review any Advanced Security threads. Container build status should also be checked because remote Git remains part of the backend shipped in container deployments.
