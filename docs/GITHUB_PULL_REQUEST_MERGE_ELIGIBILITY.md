# GitHub pull request merge eligibility

Phase M3A adds `github_get_pull_request_merge_eligibility`, a bounded **read-only** evidence tool for deciding whether the current hosted state of one configured GitHub pull request satisfies the merge policy that OmniLLM-Studio can prove.

M3A does not merge a pull request, choose a merge method, delete a branch, change repository policy, or grant merge authority. Every result returns:

```text
direct_merge_supported = false
```

The future side-effecting merge slice is Phase M3B. Any older generic `M3` wording in the merge design refers to that M3B mutation boundary; M3A remains read-only.

## Operator and model boundary

M3A uses the existing GitHub pull-request read gates:

```text
OMNILLM_GIT_REMOTE_ENABLED=true
OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED=true
```

with an operator-configured GitHub remote whose JSON policy includes:

```json
{
  "allow_pull_request_read": true
}
```

Model-facing arguments are intentionally limited to:

- `remote` — configured remote ID;
- `number` — positive pull-request number.

The model cannot supply repository owner/name, API host, token, head SHA, base branch, required checks, GitHub App IDs, reviewer identities, deployment environments, rulesets, policy selectors, merge method, or branch-deletion options.

## Evidence composition

M3A starts by running a fresh M2 `GetPullRequestMergePolicyEvidence` pass. Therefore an M3A result cannot become complete when policy, bypass, actor-role, or exact head/base evidence is already incomplete in M2.

After M2, M3A binds its current-state inspection to the exact M2 head/base and evaluates the following evidence.

### Pull-request state and mergeability

The PR must remain:

- open;
- non-draft / ready for review;
- unmerged;
- on the exact M2 head/base;
- known mergeable when GitHub has finished computing mergeability.

`mergeable=null`, head/base movement, or an unavailable final re-read is incomplete evidence rather than an implicit success.

### Default-base and strict status policy

M3A reads the repository default branch and requires the PR base to match it.

When strict/up-to-date status checks are required, M3A reads the exact current base ref and compares it with the PR head. A head that is `ahead` of, or `identical` to, the current base is current; `behind` or `diverged` is known-unsatisfied. Unknown comparison state is incomplete.

### Required checks and statuses

M3A reads exact-head check runs and commit statuses with fixed bounded pages.

For each normalized required context it evaluates:

- context name;
- completion/conclusion;
- the numeric GitHub App ID when the policy binds the requirement to a specific integration;
- legacy commit status only where applicable.

Truncated check/status evidence is incomplete. A visible required context that has not satisfied its required source is complete-but-ineligible.

### Review evidence

M3A uses a fixed GraphQL selection for:

- aggregate `reviewDecision`;
- bounded review requests with `asCodeOwner`;
- bounded `latestOpinionatedReviews(..., writersOnly: true)`.

It counts qualifying write-eligible approvals instead of assuming `reviewDecision=APPROVED` proves the required approval count.

When stale approvals are dismissed on push, an approving review counts only when its review commit is the exact current PR head.

When code-owner review is required, provider review state is corroborated with outstanding code-owner review requests.

Unknown review states or review pages exceeding the fixed bound are incomplete.

#### Last-push approval

`require_last_push_approval` is deliberately **unsupported for direct-merge eligibility** in M3A.

GitHub's aggregate review state and qualifying approval count do not prove the actor relationship required by the last-push rule: an approving reviewer must be someone other than the actor responsible for the most recent reviewable push.

Until OmniLLM-Studio has a bounded actor-aware source that can prove that relationship, M3A returns incomplete evidence with the blocking reason:

```text
last_push_approval_evidence_unavailable
```

This is intentional fail-closed behavior.

### Review-thread resolution

When conversation resolution is required, M3A reuses the bounded review-thread reader for the exact PR head.

The first page may contain at most 20 threads. Any next page, total count beyond that bound, read failure, head mismatch, or unvalidated thread identity makes the evidence incomplete. A visible unresolved thread is complete-but-ineligible.

### Required deployments

Required environments are normalized and bounded to at most 20.

For each environment M3A reads exact-head deployments and selects the newest deployment using validated `created_at` timestamps rather than relying on undocumented API list ordering. It then selects the newest deployment status by validated timestamp.

Missing, malformed, over-limit, or timestamp-ambiguous evidence is incomplete. A newest validated state other than `success` is complete-but-ineligible.

### Required signatures

When commit signatures are required, M3A reads the PR commit set with a fixed page bound of 100.

An invalid commit SHA or unverifiable page is incomplete. An unsigned/unverified visible commit is complete-but-ineligible. Exactly reaching the page limit is treated as pagination ambiguity and therefore incomplete rather than assuming no next page exists.

### Final exact-state revalidation

Before returning, M3A fetches the PR again and requires the exact M2 head/base to remain unchanged. This prevents a later caller from mistaking evidence gathered across two different hosted states for one coherent result.

## Result semantics

M3A separates **known-unsatisfied** from **unproven** evidence:

- `eligibility_complete=true, eligible=true` — every prerequisite represented by the supported policy is both visible and satisfied for the exact head/base;
- `eligibility_complete=true, eligible=false` — evidence is complete enough to decide, but at least one visible prerequisite is unsatisfied;
- `eligibility_complete=false, eligible=false` — at least one material prerequisite cannot be proven safely.

`blocking_reasons` is normalized and sorted. Provider prose, arbitrary URLs, review bodies, deployment descriptions, bypass identities, and provider error bodies are not copied into model-visible authorization logic.

## Fixed bounds

Current M3A safety bounds include:

- up to 50 check runs;
- up to 50 commit statuses;
- up to 20 review requests / latest opinionated reviews;
- up to 20 review threads for a complete thread-resolution decision;
- up to 20 required deployment environments;
- up to 20 exact-head deployments per required environment before evidence is considered ambiguous;
- up to 100 deployment statuses per inspected deployment;
- fewer than 100 PR commits for complete signature evidence.

The implementation intentionally prefers a false negative or incomplete result over silently widening one of these bounds into an authorization decision.

## M3B handoff

A future `github_merge_pull_request` mutation must be separately gated and high-risk. It must not treat a previously displayed M3A result as a durable authorization token.

Immediately before its one-shot merge request, M3B must:

1. bind the configured repository/credential and exact user-reviewed `expected_head`;
2. run M3A again for the PR;
3. require `eligibility_complete=true` and `eligible=true`;
4. require the returned head to equal `expected_head` and the returned base to remain the configured default branch;
5. require the operator-configured merge method to remain allowed by the fresh normalized policy;
6. refuse merge-queue, last-push-dependent, hidden, truncated, ambiguous, or otherwise unsupported evidence;
7. issue exactly one fixed GitHub merge request with the exact-head server-side SHA precondition;
8. never delete the source branch implicitly;
9. never automatically retry an ambiguous network/provider outcome without first reinspecting hosted state and obtaining a new complete preflight/approval.

M3B merge authority must be independent from PR read/create/reply/thread-resolution/ready gates, local Git write access, remote Git push access, administrator status, and provider `viewerCan*` fields.

## Validation coverage

M3A tests cover the positive composed M1 → M2 → M3A path and fail-closed cases including:

- app-bound required-check mismatch;
- truncated check/status evidence;
- strict-base staleness;
- insufficient qualifying approval count despite aggregate `APPROVED` state;
- outstanding code-owner review state;
- unsupported last-push approval;
- failed required deployment;
- deployment bounds and timestamp selection;
- signature pagination ambiguity;
- strict `{remote, number}` tool arguments;
- read-only low-risk tool metadata;
- registration only under the independent PR-read gate;
- `direct_merge_supported=false` for every result.

The exact final PR head must still pass the repository-wide formatting, vet, backend unit/integration/race, frontend lint/unit/build, Windows/Helm/Playwright, Security Scan, and backend/frontend container gates before M3A is merged.
