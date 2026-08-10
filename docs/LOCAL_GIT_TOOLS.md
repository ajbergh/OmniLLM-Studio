# Local Git repository tools

OmniLLM-Studio can expose explicitly configured local Git worktrees to chat and agent mode as read-only tools. This capability is separate from `github_repo_inspect`: GitHub repository URLs continue to use the GitHub API path, while local Git tools read the repository already present on the machine.

## Configure repository IDs

Set `OMNILLM_GIT_REPOSITORIES` before starting the backend. The value is a semicolon-separated list of `id=path` entries:

```text
OMNILLM_GIT_REPOSITORIES=omni=C:\src\OmniLLM-Studio;twynn=C:\src\Twynn
```

Repository IDs may contain letters, numbers, `.`, `_`, and `-`, must start with a letter or number, and are limited to 64 characters. The model receives only these stable IDs; configured filesystem paths are canonicalized at startup and are never returned by the tools.

If no repositories are configured, the local Git tool family is not registered.

## Available tools

| Tool | Purpose |
|---|---|
| `git_repositories` | List configured repository IDs with branch, HEAD, and clean/dirty state. |
| `git_status` | Read staged, unstaged, and untracked file status. |
| `git_diff` | Read the combined worktree diff against HEAD, or compare two committed revisions. |
| `git_log` | Read bounded commit history from a revision. |
| `git_show` | Read one commit's metadata, parent SHAs, and full message. |
| `git_branches` | List local branches and the current/detached HEAD state. |
| `git_blame` | Read bounded line attribution for a committed repository-relative file. |

All tools are registered as read-only, low-risk, non-network operations in the existing tool-policy framework. `git_diff` output is bounded, worktree binary files are not rendered, and oversized worktree files are omitted with warnings. `git_blame` rejects absolute paths, parent traversal, and `.git` paths.

## Implementation

The Git engine lives under `backend/internal/gitrepo/` and uses `github.com/go-git/go-git/v5` version `v5.19.2`. The LLM-facing adapters live in `backend/internal/tools/git_repo_tools.go` and depend on the read-only `gitrepo.Reader` contract rather than directly on go-git.

The initial phase deliberately does **not** clone repositories, contact remotes, stage files, create branches, commit, reset, clean, pull, or push. GitHub-hosted PRs, issues, reviews, and Actions remain provider/API capabilities rather than go-git responsibilities.

## Validation

Backend coverage includes repository configuration parsing, status/diff/log/show/branch/blame behavior, tool metadata and argument validation, conditional registry wiring, traversal rejection, and checks that configured local paths do not appear in model-visible results or errors.

Run the backend checks with:

```bash
cd backend
gofmt -w internal/gitrepo internal/tools/git_repo_tools.go internal/tools/git_repo_tools_test.go internal/tools/registry.go
go vet ./...
go test ./...
go test -race ./...
```

On Ubuntu 24.04, repository-wide commands that include the Wails desktop package require the existing `webkit2_41` build tag as documented in `CLAUDE.md`.

## Future phases

A later write-capability phase can add branch creation, checkout, staging, and commit behind explicit side-effecting tool definitions and approval policy. Clone/fetch/push should follow only after repository-workspace quotas, outbound transport policy, and credential handling are defined. Unsupported or compatibility-sensitive operations such as complex merges, rebase, LFS, reset, or clean should not be exposed merely because a generic Git command exists.
