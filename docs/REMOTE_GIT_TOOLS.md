# Remote Git tools — Phase 3

Phase 3 extends the guarded local Git capabilities in `docs/LOCAL_GIT_TOOLS.md` to remote repositories. Remote Git is treated as a separate security boundary because it combines outbound network access, credentials, local repository mutation, repository creation, and potentially consequential remote side effects.

## Security model

Remote endpoints are operator configuration, not model arguments. This is mandatory because tool arguments are persisted in the tool-invocation audit trail. Model-facing calls use only stable remote IDs and state preconditions; they never receive or submit a raw remote URL, credential value, credential environment-variable name, clone destination path, or clone quota.

Remote configuration uses `OMNILLM_GIT_REMOTES_JSON`, a JSON object keyed by stable remote ID:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true,
    "allow_default_branch_push": false,
    "allow_clone": false
  }
}
```

The `repository` field is an ID from `OMNILLM_GIT_REPOSITORIES`. For an existing repository it identifies the local worktree used by fetch/push. For clone it identifies the operator-preconfigured destination path; that path must not exist yet and its parent directory must already exist.

The token value lives only in the named operator environment variable. It is loaded immediately before authentication and is not returned by remote inventory/status results or included in tool arguments.

Independent operator gates apply:

- `OMNILLM_GIT_REMOTE_ENABLED=true` — permits outbound remote Git inspection.
- `OMNILLM_GIT_WRITE_ENABLED=true` — additionally required by fetch, push, and clone because they mutate local Git state.
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED=true` — additionally required by `git_push`; the selected remote must also set `allow_push: true`.
- `OMNILLM_GIT_REMOTE_CLONE_ENABLED=true` — additionally required by `git_clone`; the selected remote must also set `allow_clone: true`.
- `OMNILLM_GIT_CLONE_MAX_BYTES=<bytes>` — mandatory logical-content budget for one clone. Accepted range is 1 MiB through 1 GiB.
- `OMNILLM_GIT_CLONE_MAX_ENTRIES=<count>` — mandatory cumulative file/directory/symlink creation budget. Accepted range is 128 through 100,000 entries.

Clone has no implicit quota defaults. Setting the clone enable flag without both valid budgets leaves `git_clone` unregistered.

Direct pushes to the remote's default branch are denied unless that remote also sets `allow_default_branch_push: true`. `allow_default_branch_push` is invalid unless `allow_push` is also enabled.

The existing tool approval policy remains another independent gate. `git_remote_status` and `git_fetch` are high-risk network operations that default to `ask`. `git_push` and `git_clone` are critical-risk, networked, side-effecting, and non-parallel, so they default to `ask` even after all operator gates are enabled.

## Egress boundary

Remote definitions accept only exact configured HTTPS endpoints on the normal HTTPS port. Configuration rejects URL credentials, query strings, fragments, HTTP, SSH, `git://`, `file://`, alternate ports, private/loopback/link-local/reserved/metadata IP ranges, environment proxies, and redirects.

DNS is resolved once and every returned address is validated before dialing one validated IP directly. The Git HTTP transport is dedicated to the remote Git service and does not replace `http.DefaultTransport` or go-git's global protocol registry. Remote HTTP response bodies used for advertised-reference negotiation are bounded to 4 MiB.

Private-network Git servers are deliberately unsupported in this phase. Supporting them later requires an explicit destination allowlist rather than a blanket private-network bypass.

## Available tools

| Tool | Purpose | Risk / side effect |
|---|---|---|
| `git_remotes` | List safe summaries for operator-configured remotes. URLs, credential references, secret values, and filesystem paths are omitted. | Low-risk read-only local inventory. |
| `git_remote_status` | Contact one configured remote and return a bounded list of advertised branch heads. | High-risk network read; defaults to `ask`. |
| `git_fetch` | Fetch the exact reviewed head of the current local branch into bounded local object storage and an isolated tracking ref. | High-risk local mutation + network; defaults to `ask`. |
| `git_push` | Push the exact reviewed local HEAD to the same-named existing remote branch after a successful reviewed fetch. | Critical-risk remote mutation + network; defaults to `ask`. |
| `git_clone` | Clone one exact reviewed remote branch into that remote's absent, operator-preconfigured repository destination under transfer/object/storage/entry limits. | Critical-risk repository creation + network; defaults to `ask`. |

No remote tool accepts a raw URL, token, credential reference, arbitrary refspec, force flag, model-selected filesystem path, or caller-selected quota.

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

## Guarded clone

`git_clone` requires exactly three model-facing state arguments:

- `remote` — configured remote ID from `git_remotes`;
- `expected_branch` — exact branch name reviewed in `git_remote_status`;
- `expected_remote_head` — exact 40-character branch hash reviewed in `git_remote_status`.

The caller cannot choose a URL, local path, remote refspec, credentials, transfer limit, storage quota, or entry limit. Those values remain operator configuration.

### Destination and promotion

The selected remote is already bound to a repository ID in `OMNILLM_GIT_REPOSITORIES`. For clone:

1. the configured destination path must be absent;
2. its parent directory must already exist, and the parent path may not itself be or traverse symlinked directories;
3. OmniLLM-Studio creates a private `0700` sibling temporary directory;
4. all Git object ingestion and worktree checkout happen inside that temporary directory;
5. the repository must validate as clean with the exact reviewed branch and HEAD;
6. the configured destination is checked again for absence;
7. the completed sibling directory is atomically renamed into the configured destination;
8. any failure before promotion removes the temporary clone directory.

The repository ID becomes usable by the existing local Git tools after promotion without dynamically registering a model-selected path.

### Transfer and object limits

Clone uses the same dedicated SSRF-guarded upload-pack transport as fetch rather than go-git's process-global high-level clone transport.

Immediately before transfer, the remote is re-advertised and the selected branch must still equal `expected_remote_head`. The request wants exactly that reviewed commit and disables thin-pack and sideband modes so the received Git pack can be bounded and validated directly.

The clone pack is limited to the smaller of the configured clone byte budget or **128 MiB compressed**. Before object ingestion, the Git pack header must use the pack version supported by the pinned go-git implementation (**version 2** in go-git v5.19.2) and may declare at most **200,000 objects**. These limits bound compressed network/storage input and many-small-object CPU/memory amplification independently of checkout size.

### Shared clone storage quota

The temporary worktree filesystem and its `.git` chroot share one quota object. The configured byte budget is charged before logical file expansion is allowed, including:

- Git pack/object/index/config writes under `.git`;
- checked-out regular-file writes;
- sparse writes that seek beyond EOF before writing;
- file expansion through truncate;
- symlink target bytes.

The cumulative entry budget counts files, directories, symlinks, and temporary files created through the guarded filesystem. Missing parent directories that Billy would create implicitly through `Create`, `OpenFile`, `Rename`, or `TempFile` are admitted and charged before filesystem mutation. Empty-directory `TempFile` calls are forced into the clone-local root rather than the process-wide host temp directory. Entry reservations are intentionally conservative and are not refunded on removal, so retries or partial failures cannot manufacture quota credit.

The byte counter is also intentionally conservative on ambiguous filesystem failures. A failed truncate expansion keeps its reservation because a filesystem may have partially mutated before returning an error.

The staging filesystem uses Billy's bounded OS implementation so filesystem operations are securely joined beneath the private clone root even after symlinks are created during checkout. The `.git` directory is explicitly created through the quota wrapper before the storage filesystem is chrooted into it, so the repository metadata root itself consumes the shared entry budget.

These are **hard application-level logical-content and entry ceilings** on clone writes made through the guarded Billy filesystem. They are not a promise that physical disk-block or filesystem-metadata allocation equals the logical byte count; block size, journals, inode tables, copy-on-write behavior, and filesystem implementation add overhead. Deployments requiring an exact physical disk-consumption boundary should additionally place configured clone storage inside an OS/filesystem-enforced quota. The application entry limit prevents an attacker from using an unlimited tree of zero-byte files to bypass the logical byte budget.

### Deliberately unsupported clone behavior

Guarded clone does not:

- accept a model-selected destination;
- overwrite or merge into an existing directory;
- recurse or initialize Git submodules;
- invoke Git LFS downloads;
- fetch additional tags or branches beyond the object closure needed for the reviewed commit;
- write an operator remote URL/token into tool results;
- use SSH, `git://`, `file://`, redirects, environment proxies, or private-network destinations;
- continue after a changed remote branch head or quota violation.

## Recommended model workflows

For existing repositories, the safe mutation sequence is:

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

For an operator-preconfigured clone destination:

```text
git_remotes
  → git_remote_status
  → choose the reviewed branch/head
  → approval → git_clone
  → git_status / git_log / git_show using the configured repository ID
```

A stale local HEAD, changed local branch, changed remote head, missing fetched tracking state, non-fast-forward push relationship, protected default branch, existing clone destination, disabled operator gate, missing clone budget, denied tool policy, invalid pack, excessive object count, or exceeded transfer/storage/entry limit fails closed and requires refreshed inspection or operator configuration rather than force or retry-through behavior.

## Validation

Focused backend tests cover remote configuration rejection, credential indirection, global write/network/push/clone gates, required clone budgets, remote tool metadata and strict argument contracts, fetch transfer limiting, isolated tracking refs, push default-branch policy, push transfer limiting, clone destination admission, clone pack-header/object-count validation, shared clone byte/entry quota, sparse-write and truncate expansion, symlink accounting, implicit parent-directory accounting, clone-local temporary-file confinement, registry gating, and an end-to-end fake-transport clone that generates a real Git pack, checks out the reviewed commit, validates a clean repository, atomically promotes it, and verifies no staging directory remains.

Existing repository-wide checks exercise the new code through formatting, vet, unit/integration tests, race detection, Windows desktop compilation, frontend validation, dependency audits, CodeQL, Playwright smoke coverage, Helm validation, and container image builds.

Before merging a Phase 3 change, validate the exact final head with the repository's Quality Gate, Security Scan, and container workflows and review any Advanced Security threads.
