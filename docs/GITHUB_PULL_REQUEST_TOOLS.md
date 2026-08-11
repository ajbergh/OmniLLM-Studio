# GitHub pull request tools

OmniLLM-Studio provides a guarded GitHub collaboration boundary after local Git work has been committed and published. Read-only pull-request/check/feedback/thread-state inspection, draft pull-request creation, and inline review-comment replies are deliberately independent operator permissions. None implies Git push, branch creation, or another hosted mutation.

Local Git and remote publication are documented in `docs/LOCAL_GIT_TOOLS.md` and `docs/REMOTE_GIT_TOOLS.md`.

## Operator configuration

GitHub collaboration reuses an operator-configured remote from `OMNILLM_GIT_REMOTES_JSON`:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true,
    "allow_branch_create": true,
    "allow_pull_request_read": true,
    "allow_pull_request_create": true,
    "allow_pull_request_reply": true
  }
}
```

The selected remote supplies repository identity and credential configuration. The model never receives or submits the GitHub API URL, owner/repository name, token value, token environment-variable name, or local filesystem path.

Read-only PR/check/feedback/thread-state inspection has its own process-wide gate:

```text
OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED=true
```

Draft PR creation has a separate process-wide gate:

```text
OMNILLM_GITHUB_PULL_REQUEST_ENABLED=true
```

Replies to existing top-level inline review comments have another independent process-wide gate:

```text
OMNILLM_GITHUB_PULL_REQUEST_REPLY_ENABLED=true
```

All GitHub capabilities also require `OMNILLM_GIT_REMOTE_ENABLED=true`, a non-empty `token_env`, and an exact `https://github.com/<owner>/<repository>[.git]` remote URL. Read operations additionally require `allow_pull_request_read: true`; creation requires `allow_pull_request_create: true`; review replies require `allow_pull_request_reply: true` and are exposed as a high-risk side-effecting tool subject to normal OmniLLM tool policy and approval handling.

These gates are independent from one another and from `OMNILLM_GIT_WRITE_ENABLED`, `OMNILLM_GIT_REMOTE_PUSH_ENABLED`, and `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`. GitHub Enterprise hosts and arbitrary API base URLs remain unsupported; future Enterprise support should use an explicit operator API-endpoint binding rather than a model-supplied URL.

## Read-only tools

All read tools are low-risk, read-only, networked, credentialed, parallel-safe, bounded to a 64 KiB model result, and registered only when the global read gate is enabled. They do not accept repository names, API URLs, tokens, credential references, mutation controls, or arbitrary commit SHAs.

### `github_get_pull_request`

Reads bounded metadata for one PR. Inputs:

- `remote` — configured remote ID from `git_remotes`;
- `number` — positive pull request number.

The result includes number, URL, title, state/draft/merged state, mergeability when GitHub has computed it, source branch/head SHA, base branch, author, and update time. The PR body is intentionally omitted from this metadata view; hosted review/comment prose is exposed only through the separately bounded feedback tool below.

### `github_list_pull_requests`

Lists one bounded first page, sorted by most recently updated. Inputs:

- `remote` — configured remote ID;
- `state` — optional `open`, `closed`, or `all` (default `open`);
- `head_branch` — optional same-repository branch filter; the service constructs GitHub's owner-qualified filter internally;
- `limit` — optional 1–20 (default 10).

The API request asks for `limit + 1` records solely to report a `truncated` flag without exposing unbounded pagination to the model.

### `github_get_pull_request_checks`

Reads check runs and legacy/combined commit-status contexts for one PR. Inputs:

- `remote` — configured remote ID;
- `number` — positive pull request number.

The service first fetches the PR, validates its returned Git head SHA, and then queries both GitHub check runs and combined commit status for **that exact SHA**. The model cannot choose or substitute a commit reference. Up to 50 check runs and 50 status contexts are returned with truncation flags.

To reduce untrusted hosted content in model context, this view returns execution metadata only: check name/status/conclusion/app and commit-status context/state. Provider-supplied check output, annotations, descriptions, and arbitrary target/details URLs are not copied into the result.

### `github_get_pull_request_feedback`

Reads one bounded page of hosted collaboration evidence for a PR. Inputs:

- `remote` — configured remote ID;
- `number` — positive pull request number;
- `kind` — one of `reviews`, `review_comments`, `comments`, or `review_requests`;
- `page` — optional 1–100 (default 1); `review_requests` supports only page 1;
- `limit` — optional 1–20 (default 10).

The service validates `kind`, `page`, and `limit` **before** contacting GitHub, then fetches the PR and validates its returned head SHA before reading the requested collaboration surface. Every result therefore includes the exact current PR `head`; submitted reviews and inline comments with valid commit IDs additionally report whether their commit equals that current head.

The four kinds map to distinct hosted evidence:

- `reviews` — submitted review summary/state/body, author, submit time, review commit, and author association. GitHub returns reviews chronologically.
- `review_comments` — inline review comments, newest-updated first, including file path, line/side/range, review ID, reply relationship, current/original commits, author, and timestamps.
- `comments` — general PR timeline comments from GitHub's issue-comment surface, preserving GitHub's default ordering.
- `review_requests` — outstanding requested users followed by teams. This endpoint is not paginated by GitHub, so OmniLLM applies the local `limit` and reports `may_have_more` when additional identities were omitted.

For REST-paginated feedback, OmniLLM requests exactly `limit` items. `may_have_more: true` means the page was full and another page **may** exist; this deliberately avoids requesting `limit + 1` and then creating pagination gaps. Request the next `page` when additional evidence is needed.

Hosted review/comment bodies are preserved as evidence, not rewritten or interpreted by the GitHub service. Each body is capped at 1,536 UTF-8-safe bytes; inline file paths are capped at 1,024 UTF-8-safe bytes. Truncation is explicit in `body_truncated` / `path_truncated`. With at most 20 items per page, this keeps the model-facing result below the shared 64 KiB tool budget in normal metadata bounds.

Reviewer prose is untrusted external content. Before any tool result is sent to a model, the LLM provider boundary documented in `docs/TOOL_RESULT_TRUST_BOUNDARY.md` inserts a runtime-owned system directive telling the model to treat tool output as reference data rather than instruction authority. A review saying “ignore prior instructions,” requesting secrets, or asking the agent to run another tool is therefore evidence to evaluate—not authorization to act. Allow / Ask / Off policy, scoped permissions, and side-effect approval remain authoritative.

### `github_get_pull_request_review_threads`

Reads one bounded cursor page of GitHub review-thread **state and location metadata** without copying reviewer prose. Inputs:

- `remote` — configured remote ID;
- `number` — positive pull request number;
- `after` — optional opaque `next_cursor` returned by the previous call to this same tool, capped at 512 bytes;
- `limit` — optional 1–20 (default 10).

OmniLLM first fetches the PR through the existing REST boundary and validates its current head SHA. It then sends a **fixed application-owned GraphQL query** to `https://api.github.com/graphql`; owner, repository, PR number, page size, and opaque cursor are supplied only as GraphQL variables. The model cannot provide GraphQL query text, fields, API endpoints, or mutations.

The GraphQL response must report a valid `headRefOid` equal to the REST-fetched PR head. If the PR head changes between the REST and GraphQL reads, the operation fails rather than combining thread state from one revision with PR metadata from another.

Each returned thread includes only bounded state/location metadata: opaque thread node ID, `is_resolved`, `is_outdated`, `is_collapsed`, bounded file path, line/range sides when present, subject type, resolver login when present, and GitHub's `viewer_can_reply`, `viewer_can_resolve`, and `viewer_can_unresolve` capability flags. Thread node IDs are required and bounded to 256 bytes; paths reuse the 1,024-byte UTF-8-safe path limit. Review bodies remain available only through `github_get_pull_request_feedback`.

Pagination is cursor-based. `has_next_page: true` is returned only with a non-empty bounded `next_cursor`; callers pass that cursor back as `after` to retrieve the next page. Cursors are opaque hosted data and must not be interpreted, modified, or reused as another tool's identifier.

GitHub's viewer capability flags are **descriptive hosted state, not OmniLLM authorization**. For example, `viewer_can_resolve: true` means the configured GitHub identity may be able to resolve that thread at GitHub; it does not authorize OmniLLM to mutate thread state. Resolve/unresolve remains absent from this read-only slice and would require its own operator gate, high-risk tool policy/approval, and exact-state revalidation before any future mutation.

## Review reply tool

### `github_reply_to_pull_request_review_comment`

Posts one reply to an **existing top-level inline review comment**. It is a high-risk, networked, credentialed, side-effecting, non-parallel hosted communication mutation and therefore remains subject to normal OmniLLM policy and approval controls.

The model may provide only:

- `remote` — configured remote ID;
- `number` — positive PR number;
- `expected_head` — exact 40-character current PR head from reviewed GitHub PR/feedback output;
- `comment_id` — exact top-level inline comment ID from `github_get_pull_request_feedback(kind="review_comments")`;
- `expected_review_id` — exact `review_id` returned with that comment;
- `expected_updated_at` — exact reviewed comment `updated_at` timestamp;
- `body` — non-empty valid UTF-8 reply text, capped at 8 KiB.

The tool does **not** accept repository owner/name, GitHub API URL, token, token environment variable, alternate commit ref, review state, thread ID/resolution state, reviewer list, ready/draft state, workflow controls, merge/close controls, or arbitrary comment/review creation.

Immediately before posting, OmniLLM:

1. resolves the exact operator-configured `github.com` remote and credentials;
2. fetches the PR and requires it to still be open and unmerged;
3. requires the PR's current hosted head SHA to equal `expected_head`;
4. fetches the exact review comment by `comment_id`;
5. requires that comment to belong to the requested PR and `expected_review_id`;
6. requires `in_reply_to_id` to be empty/zero, preventing replies-to-replies;
7. requires the hosted comment `updated_at` to equal `expected_updated_at`;
8. sends only the bounded reply `body` to GitHub's review-comment reply endpoint;
9. validates the created reply back to the same PR, review, and parent comment before returning `posted: true`.

A changed PR head, closed/merged PR, edited/replaced comment, wrong PR/review identity, or nested reply target fails closed and requires fresh hosted inspection before another attempt.

GitHub's review-comment reply POST is not idempotent. Once the POST begins, a transport/provider failure or an invalid success response can leave the true hosted outcome uncertain. OmniLLM therefore reports that the reply outcome is unknown or could not be validated and explicitly requires `github_get_pull_request_feedback(kind="review_comments")` to be run again **before retrying**. Callers must not blindly retry an uncertain reply, because that could create a duplicate notification/comment.

The result intentionally contains only bounded confirmation metadata: remote/repository IDs, PR/head, parent comment ID, review ID, created reply ID/time, and `posted`. The posted body and provider/API response details are not copied back into result metadata.

## Draft creation tool

### `github_create_draft_pull_request`

Creates a draft PR from the current published feature branch to the configured remote's advertised default branch.

The model may provide only:

- `remote` — configured remote ID from `git_remotes`;
- `expected_branch` — current local branch from `git_status`;
- `expected_head` — exact 40-character local HEAD from `git_status`;
- `expected_remote_state_digest` — exact 64-character `branch_state_digest` from a reviewed `git_remote_status` after branch publication;
- `title` — 1–256 characters;
- `body` — optional, bounded to 32 KiB.

The tool does **not** accept repository owner/name, GitHub API URL, token, base branch, alternate head/fork owner, ready-for-review, merge method, labels, reviewers, teams, assignees, milestone, or project controls. It is critical-risk, networked, side-effecting, non-parallel, and defaults to `ask`.

## Draft creation state binding

Immediately before creation, OmniLLM-Studio:

1. serializes against local Git mutations made through OmniLLM;
2. rechecks the exact current local branch and `expected_head`;
3. re-advertises the operator-configured Git remote using the dedicated SSRF-guarded Git transport;
4. recomputes the complete `branch_state_digest` and requires it to equal `expected_remote_state_digest`;
5. requires the same-named remote source branch to exist at exactly `expected_head`;
6. reads the default branch only from the remote's advertised `HEAD` symref;
7. rejects creation when source equals the default branch;
8. rechecks local branch/HEAD once more before the GitHub API request.

The model cannot choose another base branch. If the remote does not advertise a trustworthy `HEAD` symref, creation fails closed rather than guessing `main` or `master`.

## Shared GitHub API boundary

The API client is dedicated to GitHub collaboration:

- REST and GraphQL API hosts are fixed to `https://api.github.com`;
- repository owner/name are parsed only from an exact operator-configured `github.com` Git remote;
- token value is read from that remote's configured `token_env` immediately before use;
- environment proxies are not used;
- redirects are disabled so Authorization cannot follow a redirect;
- DNS uses the private/local/reserved-address rejection used by guarded remote Git;
- response bodies are capped at 1 MiB;
- REST requests use the repository's pinned GitHub REST API version header;
- the review-thread GraphQL query is application-owned and model inputs are passed only as variables;
- REST/GraphQL provider error bodies and GraphQL error messages are not copied into model-visible errors.

## Draft duplicate and race handling

Before creation, the service queries open PRs for the exact repository, source branch, and advertised default base. A matching PR at the exact reviewed SHA is reused; a matching PR at a different SHA fails and requires refreshed Git state.

Because GitHub's create-PR API accepts a branch name rather than an immutable commit, OmniLLM-Studio validates a newly created PR after creation. It must remain open/draft, use the expected head branch/SHA, and target the advertised default branch. An unexpected newly created PR is immediately closed when possible; if cleanup cannot be confirmed, the error identifies the PR number for operator inspection.

## Recommended end-to-end coding workflow

```text
git_status
  → git_create_branch / git_checkout
  → git_diff / git_stage / git_status / git_commit
  → git_remote_status
  → approval → git_publish_branch
  → git_remote_status
  → approval → github_create_draft_pull_request
  → github_get_pull_request
  → github_get_pull_request_checks
  → github_get_pull_request_feedback (reviews / review_comments / comments / review_requests as needed)
  → github_get_pull_request_review_threads (when unresolved/outdated thread state matters)
  → approval → github_reply_to_pull_request_review_comment (only when a response is actually needed)
  → github_get_pull_request_feedback(kind="review_comments")
  → github_get_pull_request_review_threads
```

Later commits return to the reviewed existing-branch path:

```text
local edit/stage/commit
  → git_status
  → git_remote_status
  → approval → git_fetch
  → approval → git_push
  → github_get_pull_request
  → github_get_pull_request_checks
  → github_get_pull_request_feedback
  → github_get_pull_request_review_threads
  → approval → github_reply_to_pull_request_review_comment (when needed)
  → github_get_pull_request_feedback(kind="review_comments")
  → github_get_pull_request_review_threads
```

Read access, draft creation, review-thread state inspection, and review replies do not imply permission to mark a PR ready, request/remove reviewers, change metadata, resolve/unresolve review threads, submit/dismiss reviews, rerun workflows, merge, close arbitrary PRs, or delete the source branch. Those remain separate future capabilities only if a later audit demonstrates the need.

## Validation expectations

Focused tests cover independent read/create/reply gates, GitHub.com-only repository derivation, operator-bound authentication, strict model-facing arguments, bounded REST/cursor pagination, fixed GraphQL query text, opaque cursor variables, same-repository head filters, exact PR-head binding for check/status/review/thread evidence and review replies, GraphQL head-race rejection, bounded thread IDs/paths, provider-error-body/GraphQL-error-message suppression, hosted viewer-capability non-authority, hostile hosted-text preservation under the LLM trust boundary, UTF-8-safe feedback/reply bounds, stale/closed PR rejection, edited/nested/wrong-identity comment rejection, ambiguous POST outcome handling, creation state binding, duplicate reuse, race containment, response validation, and conditional registry wiring.

Before merging changes to this boundary, validate the exact final head with repository formatting, `go vet`, backend unit/integration tests, race detection, frontend checks, Windows desktop checks, Playwright smoke coverage, dependency audits, Go and JavaScript/TypeScript CodeQL, Helm validation, and backend/frontend container builds. Review PR and Advanced Security threads before readiness.
