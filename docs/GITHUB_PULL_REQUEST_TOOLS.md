# GitHub pull request tools

OmniLLM-Studio can create a **draft** pull request after a local feature branch has been committed and published through the guarded Git workflow. GitHub collaboration is intentionally a separate security boundary from Git transport: creating a pull request mutates a hosted collaboration system even though it does not modify the local worktree or Git object database.

The Git branch must already exist on the configured remote at the exact reviewed local HEAD. Local Git and remote publication are documented in `docs/LOCAL_GIT_TOOLS.md` and `docs/REMOTE_GIT_TOOLS.md`.

## Operator configuration

Draft pull request creation reuses an operator-configured remote from `OMNILLM_GIT_REMOTES_JSON`:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true,
    "allow_branch_create": true,
    "allow_pull_request_create": true
  }
}
```

The selected remote supplies all repository identity and credential configuration. The model never receives or submits the GitHub API URL, owner/repository name, token value, token environment-variable name, or local filesystem path.

The capability has its own process-wide gate:

```text
OMNILLM_GITHUB_PULL_REQUEST_ENABLED=true
```

It also requires:

- `OMNILLM_GIT_REMOTE_ENABLED=true`, because the service re-inspects the exact configured Git remote before creating the PR;
- the selected remote's `allow_pull_request_create: true`;
- a non-empty `token_env` on that remote;
- an exact `https://github.com/<owner>/<repository>[.git]` remote URL;
- critical-risk tool approval under the normal OmniLLM tool policy.

The GitHub PR gate is independent from `OMNILLM_GIT_WRITE_ENABLED`, `OMNILLM_GIT_REMOTE_PUSH_ENABLED`, and `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`. Those gates control the preceding local/publish operations but do not silently grant GitHub API mutation rights.

Private GitHub Enterprise hosts and arbitrary API base URLs are deliberately unsupported in this slice. Supporting GitHub Enterprise later should use an explicit operator API endpoint binding rather than deriving or accepting a model-supplied URL.

## Available tool

### `github_create_draft_pull_request`

Creates a draft pull request from the current published feature branch to the configured remote's advertised default branch.

The model may provide only:

- `remote` — configured remote ID from `git_remotes`;
- `expected_branch` — current local branch from `git_status`;
- `expected_head` — exact 40-character local HEAD from `git_status`;
- `expected_remote_state_digest` — exact 64-character `branch_state_digest` from a reviewed `git_remote_status` after branch publication;
- `title` — 1–256 characters;
- `body` — optional, bounded to 32 KiB.

The tool does **not** accept:

- repository owner/name;
- GitHub API URL;
- token or credential reference;
- base branch;
- alternate head branch or fork owner;
- draft=false / ready-for-review;
- merge method or merge command;
- labels, reviewers, teams, assignees, milestone, or project controls.

It is critical-risk, networked, side-effecting, non-parallel, and defaults to `ask`.

## State binding

Draft PR creation requires a fresh reviewed remote snapshot after the branch is published.

Immediately before the GitHub API mutation, OmniLLM-Studio:

1. serializes against local Git mutations made through OmniLLM;
2. rechecks the exact current local branch and `expected_head`;
3. re-advertises the operator-configured Git remote using the dedicated SSRF-guarded Git transport;
4. recomputes the complete `branch_state_digest` and requires it to equal `expected_remote_state_digest`;
5. requires the same-named remote source branch to exist at exactly `expected_head`;
6. reads the default branch only from the remote's advertised `HEAD` symref;
7. rejects the operation if the source branch is itself the default branch;
8. rechecks local branch/HEAD once more before the GitHub API request.

The model cannot choose another base branch. If the remote does not advertise a trustworthy `HEAD` symref, PR creation fails closed rather than guessing `main` or `master`.

## GitHub API boundary

The API client is dedicated to this capability:

- API host is fixed to `https://api.github.com`;
- repository owner/name are parsed only from an exact operator-configured `github.com` Git remote;
- token value is read from that remote's configured `token_env` immediately before use;
- environment proxies are not used;
- redirects are disabled so the Authorization header cannot follow a redirect;
- DNS is resolved through the same private/local/reserved-address rejection used by guarded remote Git;
- response bodies are capped at 1 MiB;
- requests use the pinned GitHub REST API version header.

API error bodies are not copied into model-visible errors.

## Duplicate and race handling

Before creating a PR, the service queries open pull requests for the exact repository, source branch, and advertised default base.

- If no matching PR exists, it issues one draft PR creation request.
- If a matching open PR already exists at the exact reviewed source SHA, no duplicate is created; the existing PR is returned with `already_exists: true`.
- If a matching open PR exists at a different source SHA, the operation fails and requires refreshed Git state.

GitHub's create-pull-request API accepts a branch name rather than an immutable source commit, so a remote branch can theoretically move after the Git advertisement and before GitHub applies the API request. OmniLLM-Studio therefore validates the returned PR after creation: it must be open, draft, use the expected head branch, report the exact `expected_head` SHA, and target the advertised default branch.

If GitHub returns a newly created PR whose source/base/draft state does not match those invariants, OmniLLM-Studio immediately attempts to close it and reports the validation failure. If cleanup cannot be confirmed, the error identifies the PR number so an operator can inspect it. This containment step does not claim an atomic GitHub-side compare-and-swap that the API does not provide.

## Recommended end-to-end coding workflow

```text
git_status
  → git_create_branch / git_checkout
  → git_diff / git_stage / git_status / git_commit
  → git_status
  → git_remote_status
  → approval → git_publish_branch
  → git_remote_status
  → verify published branch head and retain branch_state_digest
  → approval → github_create_draft_pull_request
```

After draft creation, later changes should return to the normal reviewed existing-branch workflow:

```text
local edit/stage/commit
  → git_status
  → git_remote_status
  → approval → git_fetch
  → approval → git_push
```

Draft PR creation does not imply permission to mark a PR ready, request reviewers, change metadata, merge, close arbitrary PRs, or delete the source branch. Those should be separate capabilities with their own approval and state-binding rules if added later.

## Validation expectations

Focused tests cover GitHub.com-only repository derivation, independent process gating, exact published source-head binding, advertised-default-base selection, duplicate open-PR reuse, stale remote-state rejection before any API call, strict model-facing arguments, post-create source/base/draft validation, cleanup of a mismatched newly created draft, and conditional registry wiring.

Before merging changes to this boundary, validate the exact final head with repository formatting, `go vet`, backend unit/integration tests, race detection, frontend checks, Windows desktop checks, Playwright smoke coverage, dependency audits, Go and JavaScript/TypeScript CodeQL, Helm validation, and backend/frontend container builds. Review PR and Advanced Security threads before readiness.
