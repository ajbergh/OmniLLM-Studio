# GitHub draft-to-ready pull request tool

OmniLLM-Studio can advance one existing GitHub draft pull request to **ready for review** through a separately gated hosted mutation. This capability is intentionally independent from Git push, remote branch creation, draft PR creation, PR reads, review-comment replies, and review-thread resolution.

The implementation is deliberately narrow: it changes only GitHub's draft/ready state for one exact reviewed pull request. It does not request reviewers, submit or dismiss reviews, rerun workflows, change labels/assignees/milestones, retarget the base branch, merge or close the pull request, or delete the source branch.

See `docs/GITHUB_PULL_REQUEST_TOOLS.md` for the broader GitHub collaboration boundary and `docs/REMOTE_GIT_TOOLS.md` for remote Git publication.

## Operator configuration

The configured GitHub remote must opt in to the hosted mutation:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_pull_request_ready": true
  }
}
```

The independent process-wide gate must also be enabled:

```text
OMNILLM_GIT_REMOTE_ENABLED=true
OMNILLM_GITHUB_PULL_REQUEST_READY_ENABLED=true
```

A non-empty token must be available through the configured `token_env`. The remote must be an exact `https://github.com/<owner>/<repository>[.git]` URL. Repository identity, GitHub API host, token value, token environment-variable name, and pull-request node ID are not model inputs.

Enabling this gate does **not** enable any of the following:

- `OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED`;
- `OMNILLM_GITHUB_PULL_REQUEST_ENABLED` for draft creation;
- `OMNILLM_GITHUB_PULL_REQUEST_REPLY_ENABLED`;
- `OMNILLM_GITHUB_PULL_REQUEST_THREAD_RESOLUTION_ENABLED`;
- `OMNILLM_GIT_WRITE_ENABLED`;
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED`;
- `OMNILLM_GIT_REMOTE_BRANCH_CREATE_ENABLED`.

Likewise, none of those permissions imply ready-for-review permission.

## Tool

### `github_mark_pull_request_ready_for_review`

This is a high-risk, networked, credentialed, side-effecting, non-parallel tool subject to normal OmniLLM Allow / Ask / Off policy, scoped permissions, and approval handling.

The model may provide only:

- `remote` — configured GitHub remote ID from `git_remotes`;
- `number` — positive pull request number;
- `expected_head` — exact 40-character current PR head from reviewed GitHub PR output.

The tool does **not** accept:

- repository owner/name;
- API URL or GraphQL endpoint;
- token or credential reference;
- pull-request node ID;
- base branch;
- reviewer/team identities;
- arbitrary GraphQL text;
- workflow controls;
- merge method or merge/close controls;
- labels, assignees, milestone, or other PR metadata;
- source-branch deletion.

## State binding

Immediately before mutation, OmniLLM:

1. resolves the exact operator-configured `github.com` remote and credentials;
2. fetches the requested pull request through the existing GitHub REST boundary;
3. requires the pull request to remain open, unmerged, and draft;
4. requires its current hosted head SHA to equal `expected_head`;
5. advertises the configured Git remote and reads the default branch only from its advertised `HEAD` symref;
6. requires the pull request's current base branch to equal that advertised default branch;
7. runs one fixed application-owned GraphQL preflight query to resolve the opaque pull-request node ID and revalidate repository identity, PR number, draft/open/unmerged state, head SHA, and base branch;
8. sends only the internally resolved `pullRequestId` variable to GitHub's fixed `markPullRequestReadyForReview` mutation;
9. validates the mutation response back to the same node/repository/PR/head/base and requires the returned state to be open, unmerged, and `isDraft=false` before reporting success.

If the pull request is already ready, has closed or merged, has moved to another head, no longer targets the configured default branch, or changes between REST and GraphQL validation, the operation fails closed and requires fresh inspection.

The model never chooses a different repository, base branch, node ID, or GraphQL operation.

## Ambiguous outcome handling

Once GitHub's ready-for-review mutation begins, a transport failure, provider failure, GraphQL error, or malformed/contradictory success response can make the true hosted outcome uncertain.

OmniLLM does not blindly retry. It returns a sanitized error instructing the caller to inspect the pull request again before another attempt. Provider error bodies and GraphQL error messages are not copied into the model-visible error.

This is important because a mutation may have succeeded at GitHub even when the response was lost or could not be validated.

## Recommended lifecycle

After a draft PR has been created and its exact hosted state has been reviewed:

```text
github_get_pull_request
  → github_get_pull_request_checks
  → github_get_pull_request_feedback
  → github_get_pull_request_review_threads
  → address feedback with local Git changes as needed
  → git_push
  → repeat exact-head checks / feedback / thread inspection
  → resolve addressed review threads when appropriate
  → github_get_pull_request
  → approval → github_mark_pull_request_ready_for_review
  → github_get_pull_request
  → github_get_pull_request_checks
```

Marking a draft ready is **not** merge authorization. Merge remains intentionally unsupported until a separate threat model defines exact head/base/check/review-state requirements, operator authorization, merge-method policy, and ambiguous-outcome handling.

## Validation expectations

Changes to this hosted mutation boundary should pass the exact final repository head through:

```bash
cd backend
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm run lint
npm run test:unit
npm run build

cd ..
npx playwright test --project=chromium
```

Also require the repository Security Scan and applicable container/release workflows. Focused tests should cover independent gate behavior, GitHub.com-only repository derivation, exact PR-head/default-base binding, fixed GraphQL query/mutation variables, stale/already-ready rejection, post-mutation response validation, ambiguous-outcome sanitization, strict tool arguments, and conditional registry wiring.
