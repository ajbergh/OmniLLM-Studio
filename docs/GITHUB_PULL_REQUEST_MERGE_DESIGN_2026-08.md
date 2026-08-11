# GitHub pull request merge design gate — August 2026

## Status

**Design only. `github_merge_pull_request` is intentionally not implemented.**

The hosted coding workflow now reaches draft creation, exact-head PR/check/feedback/thread inspection, review-comment replies, review-thread resolution, and guarded draft-to-ready transition. Merge is the first lifecycle step that irreversibly changes the configured default branch, so it requires a stricter policy boundary than the preceding collaboration mutations.

This review concludes that OmniLLM-Studio should **not add a merge mutation yet**. The current read surface can prove the PR's current head and report observed checks/reviews/threads, but it cannot yet produce a normalized, fail-closed answer to: **which merge requirements are actually active for this base branch, and are they satisfied without relying on the configured GitHub actor's bypass privileges?**

## Current evidence

Current `main` already provides:

- exact hosted PR head/base/draft/merged/mergeability metadata through `github_get_pull_request`;
- exact-head check-run and commit-status inspection through `github_get_pull_request_checks`;
- bounded submitted-review/review-request inspection through `github_get_pull_request_feedback`;
- bounded review-thread resolution/outdated state through `github_get_pull_request_review_threads`;
- guarded ready-for-review mutation through `github_mark_pull_request_ready_for_review`.

Those surfaces are necessary but not sufficient for merge authorization.

GitHub can impose merge requirements through classic branch protection and rulesets, including required status checks, approving reviews/code-owner review, last-push approval, conversation resolution, deployments, linear history, and merge queue. Rulesets can also restrict allowed merge methods. Classic branch-protection restrictions may be bypassable by administrators or actors with bypass permission unless administrator enforcement/no-bypass policy applies.

GitHub's REST merge endpoint supports an exact head precondition (`sha`) and returns conflict when that SHA no longer matches. That is a useful final compare-and-swap guard, but it does not by itself establish that OmniLLM's own pre-merge policy evaluation was complete or that the authenticated actor did not possess bypass authority.

Primary GitHub API references used for this design:

- Merge a pull request: `PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge`, including `sha` and `merge_method`.
- Get rules for a branch: `GET /repos/{owner}/{repo}/rules/branches/{branch}`.
- Get branch protection: `GET /repos/{owner}/{repo}/branches/{branch}/protection`.
- GitHub GraphQL `BranchProtectionRule` fields for required reviews, checks, deployments, conversation resolution, linear history, strict status checks, and bypass allowances.

## Threat model

A merge tool must assume:

1. the model may attempt to merge the wrong PR, stale head, wrong repository, or wrong base;
2. hosted comments/reviews/check names are untrusted external data and never authorization;
3. repository merge policy can change between earlier inspection and mutation;
4. the configured token may have administrator or bypass privileges stronger than ordinary contributors;
5. GitHub can report `mergeable=null` while computing mergeability;
6. observed green checks are not equivalent to knowing which checks are required;
7. review requirements may include code-owner approval, last-push approval, stale-review dismissal, or a minimum approval count;
8. a branch may require conversation resolution, deployments, strict up-to-date state, linear history, or merge queue;
9. rulesets and classic branch protection can coexist, and organization-level rulesets can apply to a repository;
10. a network/provider failure after a merge request may leave the actual outcome unknown.

The design must therefore prefer false negatives over a policy-bypassing false positive.

## Required prerequisite: normalized merge-requirements inspection

Before implementing any merge mutation, add a bounded read-only capability tentatively named:

```text
github_get_pull_request_merge_requirements
```

The exact public name may change during implementation review, but the capability must be independent from merge permission and must not itself mutate GitHub state.

### Inputs

Model authority should remain limited to:

- `remote` — configured GitHub remote ID;
- `number` — positive PR number.

The model must not provide repository owner/name, API URL, base branch, head SHA, token, ruleset IDs, branch-protection IDs, required-check names, reviewer requirements, bypass actors, or merge method.

### Required result binding

The result must be bound to the freshly fetched PR and include at minimum:

- configured remote/repository ID;
- PR number;
- exact current hosted head SHA;
- exact current base branch;
- open/draft/merged state;
- current `mergeable` and `mergeable_state`, preserving an explicit unknown/computing state;
- policy-source completeness status;
- whether merge queue is required;
- allowed merge methods known from applicable policy;
- required status checks and whether strict/up-to-date status is required;
- required approving-review count when knowable;
- code-owner-review requirement when knowable;
- last-push-approval requirement when knowable;
- conversation-resolution requirement;
- required deployment environments when knowable;
- linear-history requirement;
- whether the policy surface indicates bypass capability/administrator non-enforcement relevant to the configured actor;
- a machine-readable `merge_policy_complete` / fail-closed equivalent.

The result should normalize policy facts, not copy arbitrary hosted descriptions or URLs into model context.

### Policy sources

The implementation should inspect all applicable sources that can affect merge behavior, rather than treating one API as authoritative for every repository configuration:

1. **Active branch rules/rulesets** for the exact PR base. GitHub's branch-rules endpoint returns active rules that apply to that branch, including higher-level rulesets.
2. **Classic branch protection** for the exact PR base where visible. The classic protection API exposes required checks, required reviews, last-push approval, conversation resolution, linear history, and administrator enforcement, but requires repository Administration read permission.
3. **Repository merge-method configuration** needed to determine whether merge/squash/rebase are permitted at the repository level.
4. **Current PR/check/review/thread/deployment state** for the exact head after policy requirements are known.
5. **Merge-queue state/policy**. If merge queue is required, direct merge must be considered ineligible; a future queue-specific capability would require its own design.

If any policy source that could materially change merge authorization is inaccessible or ambiguous, the normalized result must report policy as incomplete and a later merge tool must refuse to act.

A `404` or permission failure from a classic branch-protection endpoint must not automatically be interpreted as "no protection" when the configured credential cannot prove that interpretation.

## Proposed merge mutation boundary

Only after the read-only requirements surface is implemented and validated should a separate high-risk mutation be considered:

```text
github_merge_pull_request
```

### Proposed operator gates

Use independent configuration, for example:

```text
OMNILLM_GITHUB_PULL_REQUEST_MERGE_ENABLED=true
```

and per remote:

```json
{
  "allow_pull_request_merge": true,
  "pull_request_merge_method": "squash"
}
```

Names remain proposed until implementation. Merge permission must not be implied by PR read/create/reply/thread-resolution/ready permissions, local Git write access, remote push permission, or provider `viewerCan*` flags.

The merge method should be operator-configured or derived from an unambiguous repository policy. The model must not choose an arbitrary merge strategy. The first implementation should support only one explicitly configured method per remote.

### Proposed model inputs

Limit model inputs to:

- configured `remote` ID;
- positive PR `number`;
- exact reviewed `expected_head`.

Do not accept repository, API URL, token, base branch, alternate head/ref, merge method, commit title/message, reviewer overrides, bypass flags, workflow controls, or source-branch deletion.

### Mandatory pre-mutation checks

Immediately before merge, the service should:

1. resolve the operator-configured `github.com` repository/token;
2. fetch the PR and require open, non-draft, unmerged state;
3. require exact hosted head == `expected_head`;
4. advertise the configured Git remote and require the PR base == its advertised default branch;
5. obtain a **fresh normalized merge-requirements result** for that exact PR/head/base;
6. require policy visibility to be complete;
7. reject merge queue requirements rather than bypassing or approximating them;
8. require current mergeability to be known and positive;
9. require every normalized required status/check condition for the exact relevant commit to be satisfied;
10. require normalized review/code-owner/last-push requirements to be satisfied;
11. require conversation-resolution and deployment requirements to be satisfied;
12. require the operator-configured merge method to be allowed by both repository configuration and applicable rules;
13. refuse when policy indicates a condition the implementation does not understand;
14. perform one fixed application-owned merge request with GitHub's exact-head `sha` precondition.

A stale prior call to `github_get_pull_request_checks`, feedback, or review threads must never substitute for this fresh pre-mutation evaluation.

## Bypass safety

The most important merge-specific hazard is authenticated bypass authority.

GitHub documents that classic branch-protection restrictions do not necessarily apply to administrators or custom roles with bypass permission unless configured to do so. Therefore a successful GitHub merge API response cannot be treated as proof that normal repository requirements were satisfied.

The merge tool must enforce its normalized policy **before** the API request and refuse when the policy surface is incomplete. It must never intentionally invoke or model-select a bypass mechanism.

## Exact-head compare-and-swap

For a direct merge implementation, prefer GitHub's REST merge endpoint with the `sha=expected_head` precondition. GitHub documents a conflict response when the supplied head SHA no longer matches the pull request.

This server-side guard should supplement, not replace, the fresh preflight reads. It prevents a last-moment head change from turning a reviewed approval into a merge of different code.

No retry should alter or omit the expected SHA.

## Ambiguous outcome handling

Once the merge request begins, network/provider ambiguity must be treated differently from a normal validation failure.

If the API response cannot be trusted or received:

1. do not retry automatically;
2. re-fetch the exact PR;
3. if it is merged, verify the hosted result corresponds to the expected PR/head/base and return inspected state rather than issuing another merge request;
4. if it is still open, require a complete fresh merge-requirements evaluation and a new approval before any later mutation;
5. sanitize provider bodies and error text before returning model-visible errors.

The first merge implementation must never delete the source branch automatically.

## Merge queue

If the active policy requires a merge queue, direct `PUT .../merge` is not the correct workflow. The merge-requirements reader should surface `merge_queue_required: true` and a direct merge tool should fail closed.

Queue enrollment, queue state inspection, dequeueing, and queue-specific approval semantics are separate capabilities with different asynchronous/race behavior and are out of scope for the first merge slice.

## Implementation sequence

### Phase M1 — read-only merge requirements

Implement and validate `github_get_pull_request_merge_requirements` (name tentative):

- fixed GitHub REST/GraphQL reads only;
- exact PR head/base binding;
- bounded normalized policy representation;
- active ruleset + classic branch-protection + repository merge-method inspection;
- explicit policy-completeness result;
- fail closed on unknown rule types that can affect merge eligibility;
- merge-queue detection;
- focused tests for permission-denied/incomplete policy, ruleset/classic-protection overlap, strict status checks, required reviews, unresolved conversations, deployments, linear history, merge queue, and bypass visibility.

### Phase M2 — merge mutation review

After M1 lands, review the normalized result against real repository configurations and threat-model the remaining gaps. Do not proceed merely because GitHub itself would currently accept a merge.

### Phase M3 — guarded direct merge

Only if M2 concludes policy visibility is sufficient:

- add independent process/per-remote gates;
- add fixed operator merge method;
- add exact-head merge mutation;
- re-run M1 internally immediately before mutation;
- add ambiguous-outcome re-inspection;
- keep branch deletion separate;
- validate through Quality, Security, container/release checks, and manual PR-policy fixtures where practical.

## Explicit non-goals

This design does not authorize or combine:

- reviewer/team requests or removal;
- review submission/dismissal;
- workflow rerun/cancel;
- base retargeting;
- labels/assignees/milestones;
- arbitrary close/reopen;
- source-branch deletion;
- merge-queue enrollment;
- policy/ruleset/branch-protection modification;
- bypassing GitHub protections.

## Decision

**Proceed next with Phase M1 only: bounded read-only merge-requirements inspection. Do not implement merge mutation until that capability is complete, validated, and reviewed.**
