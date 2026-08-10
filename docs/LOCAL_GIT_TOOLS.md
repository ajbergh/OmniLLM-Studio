# Local Git repository tools

OmniLLM-Studio can expose explicitly configured local Git worktrees to chat and agent mode. Read-only inspection is enabled by repository configuration; local mutations require a separate operator opt-in and remain subject to the existing tool approval policy. This capability is separate from `github_repo_inspect`: GitHub repository URLs continue to use the GitHub API path, while local Git tools operate on repositories already present on the machine.

## Configure repository IDs

Set `OMNILLM_GIT_REPOSITORIES` before starting the backend. The value is a semicolon-separated list of `id=path` entries:

```text
OMNILLM_GIT_REPOSITORIES=omni=C:\src\OmniLLM-Studio;twynn=C:\src\Twynn
```

Repository IDs may contain letters, numbers, `.`, `_`, and `-`, must start with a letter or number, and are limited to 64 characters. The model receives only these stable IDs; configured filesystem paths are canonicalized at startup and are never returned by the tools.

For normal local inspection/mutation, the configured path must contain a Git repository when the tool executes. The guarded remote clone flow documented in `docs/REMOTE_GIT_TOOLS.md` may instead bind a repository ID to an **absent** destination path whose real parent directory already exists. After a successful atomic clone promotion, the same repository ID becomes available to the normal local Git tools; the model never supplies or dynamically registers the destination path.

If no repositories are configured, the local Git tool family is not registered.

## Read-only tools

| Tool | Purpose |
|---|---|
| `git_repositories` | List configured repository IDs with branch, HEAD, and clean/dirty state. |
| `git_status` | Read branch, HEAD, staged/unstaged/untracked status, and a deterministic `index_digest`. |
| `git_diff` | Read the combined worktree diff against HEAD, or compare two committed revisions. Worktree mode also returns a `worktree_digest` over the full changed worktree state. |
| `git_log` | Read bounded commit history from a revision. |
| `git_show` | Read one commit's metadata, parent SHAs, and full message. |
| `git_branches` | List local branches and identify the current/detached HEAD state. |
| `git_blame` | Read bounded line attribution for a committed repository-relative file. |

These tools are registered as read-only, low-risk, non-network operations in the existing tool-policy framework. `git_diff` output is bounded; worktree binary files, symlinks, directories, and oversized files may be omitted from the rendered patch with warnings. The worktree digest is separate from the rendered patch and fingerprints all changed paths, including regular-file bytes, Git-relevant mode, final symlink targets, status codes, and deletions. It streams file content rather than buffering it, so binary and oversized files remain covered even when they are not rendered.

Worktree file access remains constrained to the configured canonical repository root. Symlinked parent directories are rejected before filesystem-sensitive staging/fingerprinting operations, and a final symlink is fingerprinted/staged as a link object rather than dereferenced. `git_blame` rejects Unix and Windows absolute paths, parent traversal, and `.git` paths.

## Enable local mutations

Write tools are **not registered by default**, even when repositories are configured. An operator must explicitly enable them before backend startup:

```text
OMNILLM_GIT_WRITE_ENABLED=true
```

This is deliberately separate from `OMNILLM_GIT_REPOSITORIES`. A deployment can expose local Git inspection without granting the model any repository mutation capability.

When enabled, the following tools are registered:

| Tool | Purpose | Additional safety boundary |
|---|---|---|
| `git_create_branch` | Create a local branch without switching HEAD. | Requires the `expected_head` observed from `git_status`. |
| `git_checkout` | Switch to an existing local branch. | Requires matching `expected_head` and a completely clean worktree/index. |
| `git_stage` | Stage exact changed repository-relative files. | Requires `expected_branch`, `expected_head`, and `expected_index_digest` from `git_status` plus `expected_worktree_digest` from a reviewed worktree `git_diff`; rejects globs, directories, stage-all, traversal, and symlinked parent paths. |
| `git_commit` | Commit the already-staged index only. | Requires matching `expected_branch`, `expected_head`, and `expected_index_digest` from a fresh `git_status` after staging; no auto-stage, amend, empty commit, or detached-HEAD commit. |

All four mutation definitions are high-risk, side-effecting, non-network, and non-parallel. OmniLLM-Studio's `EffectivePolicy` therefore defaults them to `ask` even when no explicit persisted tool-policy row exists. The operator environment flag and user approval are independent gates: enabling writes makes the tools available, but does not silently auto-approve their execution.

### Stale-approval protection

`git_status` returns the current local branch, HEAD hash, and an `index_digest` derived from the staged index. A worktree-mode `git_diff` returns a `worktree_digest` derived from the complete changed worktree state. Mutation tools carry the observed state back as execution preconditions:

- every write requires `expected_head`, so an approval created against one commit cannot silently execute after HEAD changes;
- `git_stage` requires the local branch, HEAD, and index digest from the status snapshot **and** the worktree digest from the diff the caller reviewed;
- if a file remains `modified` but its bytes, mode, symlink target, presence, or broader changed-path set differs from the reviewed diff, staging is rejected;
- the service recomputes the complete worktree digest immediately before staging and also fingerprints each requested path immediately before and after its `Worktree.Add` call; a path changing during staging triggers rollback when the service can prove it still owns the index state;
- `git_commit` requires the local branch, HEAD, and index digest from a **fresh status after staging**, so it cannot silently commit a different branch or staged set than the one observed before approval;
- the service rechecks branch/HEAD/index preconditions immediately before committing and serializes mutations performed through OmniLLM-Studio;
- preconditions detect conflicting changes made by other Git clients before execution. The service does not force past mismatches.

`git_stage` intentionally does **not** return a new index digest. Callers must run `git_status` again before requesting `git_commit`; this makes commit approval depend on an explicit post-stage snapshot rather than simply forwarding state returned by the staging mutation.

If a precondition no longer matches, the tool fails and instructs the caller to refresh `git_status` and/or `git_diff` rather than guessing or forcing the operation.

### Staging and checkout boundaries

`git_stage` accepts only explicit changed file paths, with a bounded file count. It does not expose `git add .`, glob expansion, directory recursion, or stage-all semantics. Requested paths are validated before the index is changed. Parent directories that resolve through symlinks are rejected using `filepath-securejoin` before the repository-relative path is passed to go-git. A final path may itself be a symlink because Git stages the link object rather than dereferencing it. Deleted files are supported and are staged as index deletions.

For a multi-file stage operation, the service tracks the index digest it most recently produced. If a later stage step fails, it restores the original index only when the current index still matches that service-owned digest; this avoids blindly overwriting a concurrent external index change. The caller is told to refresh status/diff after a staging error to verify final state.

`git_checkout` only targets existing local branches and refuses to run if the repository has any staged, unstaged, or untracked changes. Force checkout, branch creation during checkout, detached checkout, reset, and clean are not exposed.

`git_commit` commits the existing staged index with go-git's normal repository-configured author identity. It does not include unrelated unstaged/untracked files.

## Implementation

The Git engine lives under `backend/internal/gitrepo/` and uses `github.com/go-git/go-git/v5` version `v5.19.2`. LLM-facing read adapters live in `backend/internal/tools/git_repo_tools.go`; mutation adapters live in `backend/internal/tools/git_repo_mutation_tools.go`. The service satisfies separate `gitrepo.Reader` and `gitrepo.Writer` contracts so read and write surfaces remain explicit. `github.com/cyphar/filepath-securejoin` is used directly for mutation-path containment and remains pinned at the version already present in the go-git dependency graph.

The tools documented on this page never contact a remote. Remote Git is a separate explicitly configured tool family with independent network, credential, fetch, push, and quota-enforced clone gates; see `docs/REMOTE_GIT_TOOLS.md`. The clone implementation uses Billy's bounded OS filesystem plus a shared application quota across `.git` storage and worktree checkout. GitHub-hosted PRs, issues, reviews, Actions, and other provider operations remain API capabilities rather than local-go-git responsibilities.

On a shared server, `OMNILLM_GIT_REPOSITORIES` and `OMNILLM_GIT_WRITE_ENABLED` are process-wide operator settings. Enable local writes only when every configured repository is intentionally writable by that OmniLLM-Studio deployment and the surrounding OS account has appropriately limited filesystem permissions.

## Validation

Backend coverage includes repository configuration parsing, write-gate behavior, status/index-digest generation, reviewed-worktree digest generation, oversized omitted-content fingerprinting, branch creation, clean-only checkout, exact-path staging, stale-branch/HEAD/index/worktree rejection, deletion staging, final-symlink staging without dereference, staged-only commits, tool metadata and argument validation, conditional registry wiring, Unix/Windows path-containment rejection, worktree symlink isolation, configured-path non-disclosure, guarded clone destination admission, clone transfer/object limits, shared clone byte/entry accounting, sparse/truncate expansion, implicit parent-directory accounting, clone-local temp-file confinement, and atomic clone promotion.

Run the backend checks with:

```bash
cd backend
gofmt -w internal/gitrepo internal/tools/git_repo_tools.go internal/tools/git_repo_mutation_tools.go internal/tools/git_remote*.go internal/tools/registry.go
go vet ./...
go test ./...
go test -race ./...
```

On Ubuntu 24.04, repository-wide commands that include the Wails desktop package require the existing `webkit2_41` build tag as documented in `CLAUDE.md`.

## Future work

Remote inspection, guarded fetch, guarded push, and quota-enforced guarded clone are documented separately in `docs/REMOTE_GIT_TOOLS.md`. Deployments that require an exact physical block-allocation ceiling for cloned repositories should add an OS/filesystem-enforced quota around configured clone storage in addition to the application's logical-byte and entry ceilings.

Unsupported or compatibility-sensitive operations such as merges, rebase, LFS, reset, clean, submodule updates, force push, remote ref deletion, or destructive history rewriting should not be exposed merely because a generic Git command exists.
