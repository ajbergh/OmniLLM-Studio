# GitHub pull request merge design gate — August 2026

## Status

**Phase M1 read-only merge-requirements inspection is implemented. `github_merge_pull_request` remains intentionally not implemented. Phase M2 is the current design/evidence gate.**

The hosted coding workflow now reaches draft creation, exact-head PR/check/feedback/thread inspection, review-comment replies, review-thread resolution, guarded draft-to-ready transition, and bounded read-only merge-policy inspection. Merge is the first lifecycle step that irreversibly changes the configured default branch, so it requires a stricter policy boundary than the preceding collaboration mutations.

M1 confirms that OmniLLM-Studio can normalize a substantial portion of GitHub merge policy while failing closed on policy visibility it cannot prove. It also makes the remaining blocker explicit: active branch rules do not themselves prove whether the configured GitHub actor is constrained by ruleset bypass policy, and a non-visible classic-protection response cannot safely be equated with an unprotected branch. The project should therefore **not add a merge mutation yet**.

## Current evidence

Current implementation provides:

- exact hosted PR head/base/draft/merged/mergeability metadata through `github_get_pull_request`;
- exact-head check-run and commit-status inspection through `github_get_pull_request_checks`;
- bounded submitted-review/review-request inspection through `github_get_pull_request_feedback`;
- bounded review-thread resolution/outdated state through `github_get_pull_request_review_threads`;
- guarded ready-for-review mutation through `github_mark_pull_request_ready_for_review`;
- bounded read-only merge-policy inspection through `github_get_pull_request_merge_requirements`.

The M1 merge-requirements reader binds every result to a freshly fetched PR head/base and normalizes repository merge methods, active base-branch rules, and visible classic branch protection. It surfaces merge queue, required status/check contexts, strict checks, review counts, code-owner review, last-push approval, stale-review dismissal, conversation resolution, deployments, linear history, administrator visibility/enforcement, unknown material rules, and explicit `merge_policy_complete` state.

Those surfaces are necessary but are not yet sufficient for merge authorization when policy visibility is incomplete.

GitHub can impose merge requirements through classic branch protection and rulesets, including required status checks, approving reviews/code-owner review, last-push approval, conversation resolution, deployments, linear history, and merge queue. Rulesets can also restrict allowed merge methods. Classic branch-protection restrictions may be bypassable by administrators or actors with bypass permission unless administrator enforcement/no-bypass policy applies.

GitHub's REST merge endpoint supports an exact head precondition (`sha`) and returns conflict when that SHA no longer matches. That is a useful final compare-and-swap guard, but it does not by itself establish that OmniLLM's own pre-merge policy evaluation was complete or that the authenticated actor did not possess bypass authority.

Primary GitHub API references used for this design:

- Merge a pull request: `PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge`, including `sha` and `merge_method`.
- Get rules for a branch: `GET /repos/{owner}/{repo}/rules/branches/{branch}`.
- Get branch protection: `GET /repos/{owner}/{repo}/branches/{branch}/protection`.
- Get a repository ruleset: `GET /repos/{owner}/{repo}/rulesets/{ruleset_id}` for ruleset details, including bypass actors only when the caller has sufficient permission to view them.
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

## Implemented prerequisite: normalized merge-requirements inspection

M1 implements the bounded read-only capability:

```text
github_get_pull_request_merge_requirements
```

The capability is registered under the existing independent GitHub PR-read gate and does not itself mutate GitHub state or imply merge permission.

Detailed operator/runtime behavior is documented in `docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md`.

### Inputs

Model authority remains limited to:

- `remote` — configured GitHub remote ID;
- `number` — positive PR number.

The model cannot provide repository owner/name, API URL, base branch, head SHA, token, ruleset IDs, branch-protection IDs, required-check names, reviewer requirements, bypass actors, or merge method.

### Result binding

The result is bound to the freshly fetched PR and includes:

- configured remote/repository ID;
- PR number;
- exact current hosted head SHA;
- exact current base branch;
- open/draft/merged state;
- current `mergeable` and `mergeable_state`, preserving GitHub's nullable/unknown mergeability;
- policy-source completeness status;
- merge-queue requirement;
- allowed merge methods known from repository/applicable rule policy;
- required status checks and strict/up-to-date policy where visible;
- required approving-review count where visible;
- code-owner-review requirement where visible;
- last-push-approval requirement where visible;
- stale-review dismissal where visible;
- conversation-resolution requirement;
- required deployment environments where visible;
- linear-history requirement;
- configured-actor administrator visibility and classic administrator enforcement;
- normalized unknown material rule types;
- machine-readable `merge_policy_complete` fail-closed state.

The result normalizes policy facts rather than copying arbitrary hosted descriptions, parameter payloads, provider error bodies, or URLs into model context.

### Policy sources

The implementation inspects:

1. **Active branch rules/rulesets** for the exact PR base through GitHub's fixed branch-rules endpoint, bounded to one 100-rule page. The endpoint returns active rules that apply to the branch, including applicable higher-level rulesets.
2. **Classic branch protection** for the exact PR base where positively visible. A non-200 result is represented as ambiguous/unavailable rather than automatically interpreted as no protection.
3. **Repository merge-method configuration** to determine repository-level merge/squash/rebase availability and the configured actor's reported administrator permission.

The reader normalizes merge-relevant rules it understands and explicitly fails policy completeness for unknown material rule types or parameters. If the active-rule page reaches the bound, policy is also incomplete.

### Current fail-closed limitation: ruleset bypass visibility

The active-rules endpoint exposes which rules apply to the branch but does not itself prove whether the configured actor is constrained by ruleset bypass policy. GitHub's ruleset detail surface can expose `bypass_actors`, but visibility of those actors requires stronger permission than the ordinary metadata read used for the active-rules endpoint.

Accordingly, when active rulesets are present, M1 currently reports:

```text
ruleset_bypass_visibility = incomplete
potential_bypass = true
merge_policy_complete = false
```

This is not an error and is not evidence that a bypass actually exists. It means OmniLLM cannot yet prove the absence or applicability of bypass authority for a future merge decision.

### Classic-protection ambiguity

A classic branch-protection `404` or other non-visible result is not treated as proof that the base branch is unprotected. M1 reports `classic_protection_status=unavailable_or_unprotected` and leaves merge policy incomplete.

This conservative ambiguity is part of the M2 evidence review.

## Proposed merge mutation boundary

Only after M2 validates the M1 result against representative GitHub repository configurations and closes or accepts the remaining policy-visibility gaps should a separate high-risk mutation be considered:

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

A stale prior call to `github_get_pull_request_checks`, feedback, review threads, or merge requirements must never substitute for this fresh pre-mutation evaluation.

## Bypass safety

The most important merge-specific hazard is authenticated bypass authority.

GitHub documents that classic branch-protection restrictions do not necessarily apply to administrators or custom roles with bypass permission unless configured to do so. Rulesets can also define bypass actors. Therefore a successful GitHub merge API response cannot be treated as proof that normal repository requirements were satisfied.

The merge tool must enforce its normalized policy **before** the API request and refuse when the policy surface is incomplete. It must never intentionally invoke or model-select a bypass mechanism.

M2 must specifically determine whether the configured actor's effective ruleset bypass status can be proven through a fixed, bounded read surface with acceptable operator credential requirements. If it cannot, direct merge should remain unsupported.

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

If the active policy requires a merge queue, direct `PUT .../merge` is not the correct workflow. The M1 reader surfaces `merge_queue_required: true`; any future direct merge tool must fail closed.

Queue enrollment, queue state inspection, dequeueing, and queue-specific approval semantics are separate capabilities with different asynchronous/race behavior and are out of scope for the first direct-merge slice.

## Implementation sequence

### Phase M1 — read-only merge requirements — IMPLEMENTED

`github_get_pull_request_merge_requirements` now provides:

- fixed GitHub REST reads only;
- exact PR head/base binding;
- bounded normalized policy representation;
- active ruleset + classic branch-protection + repository merge-method inspection;
- explicit policy-completeness result;
- fail-closed handling for unknown material rule types and ambiguous policy visibility;
- merge-queue detection;
- focused tests for classic-protection ambiguity, ruleset/classic overlap, strict status checks, required reviews, conversation resolution, deployments, linear history, merge queue, administrator/bypass visibility, strict tool arguments, and independent read gating.

### Phase M2 — merge mutation review — CURRENT GATE

Validate the M1 normalized result against representative real repository configurations and threat-model the remaining gaps. In particular:

- verify ruleset bypass-actor visibility and configured-actor applicability;
- determine the minimum acceptable credential permission needed to prove bypass status;
- verify truly unprotected vs permission-obscured classic protection behavior;
- exercise repository + organization ruleset overlap;
- verify required-check integration binding and strict policy;
- verify merge-method intersection and merge-queue behavior;
- inventory any material rule types not yet normalized.

M2 may add narrowly scoped read-only evidence if needed. Do not proceed merely because GitHub itself would currently accept a merge.

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

**Phase M1 is implemented. Proceed next with Phase M2 only: validate policy completeness and configured-actor bypass visibility. Do not implement merge mutation unless M2 demonstrates that the normalized policy surface is sufficient and fail-closed for the configured actor.**
