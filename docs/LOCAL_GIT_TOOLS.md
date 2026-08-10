# Local Git repository tools

OmniLLM-Studio can expose explicitly configured local Git worktrees to chat and agent mode. Read-only inspection is enabled by repository configuration; local mutations require a separate operator opt-in and remain subject to the existing tool approval policy. This capability is separate from `github_repo_inspect`: GitHub repository URLs continue to use the GitHub API path, while local Git tools operate on repositories already present on the machine.

## Configure repository IDs

Set `OMNILLM_GIT_REPOSITORIES` before starting the backend. The value is a semicolon-separated list of `id=path` entries:

```text
OMNILLM_GIT_REPOSITORIES=omni=C:\src\OmniLLM-Studio;twynn=C:\src\Twynn
```

Repository IDs may contain letters, numbers, `.`, `_`, and `-`, must start with a letter or number, and are limited to 64 characters. The model receives only these stable IDs; configured filesystem paths are canonicalized at startup and are never returned by the tools.

If no repositories are configured, the local Git tool family is not registered.

## Read-only tools

| Tool | Purpose |
|---|---|
| `git_repositories` | List configured repository IDs with branch, HEAD, and clean/dirty state. |
| `git_status` | Read staged, unstaged, and untracked file status, plus a deterministic `index_digest`. |
| `git_diff` | Read the combined worktree diff against HEAD, or compare two committed revisions. |
| `git_log` | Read bounded commit history from a revision. |
| `git_show` | Read one commit's metadata, parent SHAs, and full message. |
| `git_branches` | List local branches and the current/detached HEAD state. |
| `git_blame` | Read bounded line attribution for a committed repository-relative file. |

These tools are registered as read-only, low-risk, non-network operations in the existing tool-policy framework. `git_diff` output is bounded; worktree binary files, symlinks, directories, and oversized files are omitted with warnings. Worktree file reads are resolved against the configured canonical repository root before content is opened, so repository symlinks cannot be used to read outside files. `git_blame` rejects Unix and Windows absolute paths, parent traversal, and `.git` paths.

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
| `git_stage` | Stage exact changed repository-relative files. | Requires matching `expected_head`; rejects globs, directories, stage-all, traversal, and unsafe resolved paths. |
| `git_commit` | Commit the already-staged index only. | Requires matching `expected_head` and `expected_index_digest`; no auto-stage, amend, empty commit, or detached-HEAD commit. |

All four mutation definitions are high-risk, side-effecting, non-network, and non-parallel. OmniLLM-Studio's `EffectivePolicy` therefore defaults them to `ask` even when no explicit persisted tool-policy row exists. The operator environment flag and user approval are independent gates: enabling writes makes the tools available, but does not silently auto-approve their execution.

### Stale-approval protection

`git_status` returns both the current HEAD hash and an `index_digest` derived from the staged index. Mutating tools carry the observed state back as preconditions:

- every write requires `expected_head`, so an approval created against one commit cannot silently execute after HEAD changes;
- `git_commit` also requires `expected_index_digest`, so it cannot silently commit a different staged set than the one observed before approval;
- the service serializes mutations performed through OmniLLM-Studio, while the preconditions detect conflicting changes made by other Git clients before execution.

If a precondition no longer matches, the tool fails and instructs the caller to run `git_status` again rather than guessing or forcing the operation.

### Staging and checkout boundaries

`git_stage` accepts only explicit changed file paths, with a bounded file count. It does not expose `git add .`, glob expansion, directory recursion, or stage-all semantics. All requested paths are validated before the index is changed; if a staged operation fails, the service restores the original index state.

`git_checkout` only targets existing local branches and refuses to run if the repository has any staged, unstaged, or untracked changes. Force checkout, branch creation during checkout, detached checkout, reset, and clean are not exposed.

`git_commit` commits the existing staged index with go-git's normal repository-configured author identity. It does not include unrelated unstaged/untracked files.

## Implementation

The Git engine lives under `backend/internal/gitrepo/` and uses `github.com/go-git/go-git/v5` version `v5.19.2`. LLM-facing read adapters live in `backend/internal/tools/git_repo_tools.go`; mutation adapters live in `backend/internal/tools/git_repo_mutation_tools.go`. The service satisfies separate `gitrepo.Reader` and `gitrepo.Writer` contracts so read and write surfaces remain explicit.

No local Git tool contacts a remote. GitHub-hosted PRs, issues, reviews, Actions, and other provider operations remain API capabilities rather than go-git responsibilities.

On a shared server, `OMNILLM_GIT_REPOSITORIES` and `OMNILLM_GIT_WRITE_ENABLED` are process-wide operator settings. Enable local writes only when every configured repository is intentionally writable by that OmniLLM-Studio deployment and the surrounding OS account has appropriately limited filesystem permissions.

## Validation

Backend coverage includes repository configuration parsing, write-gate behavior, status/index-digest generation, branch creation, clean-only checkout, exact-path staging, stale-HEAD and stale-index rejection, staged-only commits, tool metadata and argument validation, conditional registry wiring, Unix/Windows path-containment rejection, worktree symlink isolation, and checks that configured local paths do not appear in model-visible results or errors.

Run the backend checks with:

```bash
cd backend
gofmt -w internal/gitrepo internal/tools/git_repo_tools.go internal/tools/git_repo_mutation_tools.go internal/tools/git_repo_tools_test.go internal/tools/registry.go
go vet ./...
go test ./...
go test -race ./...
```

On Ubuntu 24.04, repository-wide commands that include the Wails desktop package require the existing `webkit2_41` build tag as documented in `CLAUDE.md`.

## Future phases

Clone/fetch/push should follow only after repository-workspace quotas, outbound transport policy, and credential handling are defined. Unsupported or compatibility-sensitive operations such as merges, rebase, LFS, reset, clean, submodule updates, or destructive history rewriting should not be exposed merely because a generic Git command exists.
