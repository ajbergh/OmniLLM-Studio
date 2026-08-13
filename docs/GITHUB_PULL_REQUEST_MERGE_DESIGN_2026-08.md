# GitHub pull request merge design gate — August 2026

## Status

**Phase M1 merge-requirements inspection, Phase M2 policy/actor evidence, and Phase M3A read-only current-state merge eligibility are implemented. `github_merge_pull_request` remains intentionally not implemented. Phase M3B guarded direct merge is the next candidate slice and must consume a fresh complete M3A result for the exact PR/head/base immediately before mutation.**

The hosted coding workflow now covers draft PR creation, exact-head PR/check/feedback/thread inspection, review-comment replies, thread resolution, guarded draft-to-ready transition, normalized merge-requirements inspection, and actor/policy evidence corroboration. Merge remains the first lifecycle action that irreversibly changes the configured default branch, so its mutation boundary must be stricter than all preceding collaboration tools.

## Current implemented evidence

The runtime provides:

- `github_get_pull_request` — exact hosted PR metadata;
- `github_get_pull_request_checks` — exact-head check runs and commit statuses;
- `github_get_pull_request_feedback` — bounded submitted reviews and review requests;
- `github_get_pull_request_review_threads` — bounded review-thread state/location;
- `github_mark_pull_request_ready_for_review` — separately gated draft-to-ready mutation;
- `github_get_pull_request_merge_requirements` — M1 normalized merge policy;
- `github_get_pull_request_merge_policy_evidence` — M2 classic/ruleset/actor corroboration;
- `github_get_pull_request_merge_eligibility` — M3A exact-head current-state eligibility evidence.

M1, M2, and M3A are all read-only under the existing GitHub PR-read gate. None registers, enables, or calls a merge mutation.

## Phase M1 — normalized merge requirements — IMPLEMENTED

M1 binds policy to a freshly fetched PR head/base and normalizes repository merge settings, active rules for the exact base, and visible classic branch protection.

It surfaces, where visible:

- allowed merge methods;
- merge queue requirements;
- required status checks and integration IDs;
- strict/up-to-date status policy;
- approving-review count;
- code-owner review;
- last-push approval;
- stale-review dismissal;
- conversation resolution;
- required deployments;
- linear history;
- required signatures;
- branch lock state;
- administrator visibility/enforcement;
- classic restriction/bypass presence;
- unknown material policy rules;
- fail-closed `merge_policy_complete`.

M1 deliberately remains incomplete when a policy source cannot be proven sufficient.

## Phase M2 — policy/actor evidence — IMPLEMENTED

M2 adds the read-only capability:

```text
github_get_pull_request_merge_policy_evidence
```

Model authority remains limited to configured `remote` ID plus positive PR `number`.

M2 starts from a fresh M1 result and then:

1. corroborates classic protection using the exact-ref GraphQL `BranchProtectionRule` plus the REST branch-protection object;
2. supplements classic required deployments and app-bound required checks from GraphQL;
3. re-reads bounded active branch rules, deduplicates active ruleset IDs, and inspects at most 20 ruleset details;
4. requires each active ruleset detail to expose `bypass_actors` before bypass evidence can be complete;
5. obtains the configured viewer identity through the fixed GraphQL read and verifies its repository role through the collaborator-permission endpoint;
6. accepts only standard GitHub repository roles as role-complete; custom roles remain unverified;
7. re-fetches the PR after evidence collection and fails closed if head/base changed.

M2 never copies arbitrary policy prose, provider error bodies, or bypass identities into model context.

### M2 completion semantics

`evidence_complete=true` requires:

- complete/consistent classic REST + GraphQL evidence;
- complete or not-applicable ruleset detail evidence;
- visible bypass lists for every active ruleset;
- a standard, consistent configured-actor repository role;
- configured actor proven constrained by visible policy;
- complete normalized merge policy;
- no remaining M2 blocking reason;
- stable PR head/base throughout inspection.

Even a complete result always returns:

```text
direct_merge_supported = false
```

M2 is an evidence primitive, not merge authorization.

### Confirmed permission-obscured behavior

M2 explicitly distinguishes a confirmed unprotected branch from permission-obscured classic protection.

A branch is only classified as classically unprotected when:

- exact-ref GraphQL reports no `BranchProtectionRule`; and
- the classic REST protection endpoint returns `404`.

A REST `403` remains incomplete even when GraphQL reports no classic rule.

During M2 validation against OmniLLM-Studio, the active-rules read for `main` was visible and empty while the classic protection REST endpoint was permission-obscured. That real response pattern validates the fail-closed rule: zero visible active rules plus a `403` must not be converted into "unprotected".

## Phase M3A — current-state merge eligibility — IMPLEMENTED

M3A adds `github_get_pull_request_merge_eligibility` as a bounded read-only eligibility reader. It always starts from a fresh M2 pass and remains bound to the exact hosted PR head/base.

M3A proves, where bounded evidence is complete:

- open, ready-for-review, unmerged PR state and known positive mergeability;
- repository default-base binding and strict base currency when required;
- exact-head required checks, including numeric GitHub App identity;
- qualifying write-eligible approval count using bounded latest opinionated reviews;
- code-owner state corroboration through provider review state plus outstanding code-owner requests;
- bounded required review-thread resolution;
- exact-head required deployment success selected by validated timestamps rather than undocumented list ordering;
- required commit signature evidence within the bounded first page;
- final PR head/base stability after inspection.

Known-unsatisfied requirements produce `eligibility_complete=true` with `eligible=false`. Hidden, truncated, stale, ambiguous, or otherwise unverifiable evidence produces `eligibility_complete=false`. M3A explicitly leaves last-push approval unsupported because aggregate review state does not prove that the approving reviewer differs from the actor responsible for the most recent reviewable push.

Every M3A result keeps `direct_merge_supported=false`; this phase is evidence, not mutation authority.

## Threat model for direct merge

A future merge tool must assume:

1. the model may identify the wrong PR, stale head, wrong repository, or wrong base;
2. hosted comments/reviews/check names are untrusted reference data and never authorization;
3. repository policy can change between an earlier inspection and mutation;
4. the configured token may have administrator/custom/bypass authority stronger than ordinary contributors;
5. GitHub can report `mergeable=null` while computing mergeability;
6. green observed checks are not equivalent to knowing which checks are required;
7. reviews can require code owners, last-push approval, stale-review dismissal, or a minimum approval count;
8. policy may require conversation resolution, deployments, strict up-to-date state, linear history, signatures, or merge queue;
9. classic protection and repository/organization rulesets can coexist;
10. a provider/network failure after a merge request can make the mutation outcome ambiguous.

The design prefers false negatives over a policy-bypassing false positive.

## Phase M3B — guarded direct merge — NEXT CANDIDATE

M3A closes the current-state evidence gap but deliberately does not authorize mutation. M3B may therefore be implemented only as a separately gated high-risk mutation that runs a fresh M3A pass and refuses whenever eligibility evidence is incomplete or unsatisfied.

### Independent operator gates

M3 should add a separate process gate and per-remote permission, for example:

```text
OMNILLM_GITHUB_PULL_REQUEST_MERGE_ENABLED=true
```

and:

```json
{
  "allow_pull_request_merge": true,
  "pull_request_merge_method": "squash"
}
```

Final names should follow existing remote/config conventions when implemented.

Merge authority must not be implied by:

- GitHub PR read/create/reply/resolve/ready permissions;
- local Git mutation access;
- remote Git push access;
- provider `viewerCan*` fields;
- repository administrator status;
- a previous M1/M2 result;
- a successful prior CI observation.

### Model inputs

Limit M3 model inputs to:

- configured `remote` ID;
- positive PR `number`;
- exact reviewed `expected_head`.

Do not accept repository owner/name, API URL, token, base branch, alternate ref, merge method, commit title/message, bypass flag, workflow control, or branch-deletion option from the model.

The merge method should be operator-configured and intersected with current repository/ruleset policy.

### Mandatory fresh preflight

Immediately before any merge request, M3 must:

1. resolve the operator-configured GitHub repository and credential;
2. fetch the PR and require open, non-draft, unmerged state;
3. require current hosted head == `expected_head`;
4. advertise the configured Git remote and require PR base == advertised default branch;
5. run a **fresh M3A eligibility pass** for the exact PR/head/base; M3A itself reruns M2;
6. require `eligibility_complete=true`, `eligible=true`, `evidence_complete=true`, and `merge_policy_complete=true`;
7. reject merge-queue policy rather than approximating queue semantics;
8. require current mergeability to be known and positive;
9. inspect exact-head checks/statuses and require every normalized required check to be satisfied with its integration binding where applicable;
10. freshly inspect reviews/review requests and enforce approval, code-owner, last-push, and stale-review semantics that the implementation can prove;
11. freshly inspect review threads when conversation resolution is required;
12. verify required deployment environments are satisfied through a bounded application-owned read before merge;
13. enforce signatures/linear-history/lock/read-only requirements as appropriate;
14. require the operator merge method to be allowed by current repository/ruleset policy;
15. refuse on any unknown material rule or unverifiable prerequisite;
16. issue exactly one fixed GitHub merge request with the exact-head SHA precondition.

A stale result from any prior read tool must never substitute for this immediate preflight. Repositories whose active policy requires last-push approval remain unsupported until an actor-aware bounded proof is implemented.

## Exact-head compare-and-swap

Use GitHub's REST merge endpoint with the reviewed head SHA supplied as the server-side precondition. The expected SHA must never be omitted or changed during a retry.

The server-side precondition supplements—not replaces—the fresh M2/current-state preflight.

## Ambiguous mutation outcome

If a merge request is sent but the response cannot be trusted or received:

1. do not retry automatically;
2. re-fetch the exact PR;
3. if merged, verify the hosted result corresponds to the expected PR/head/base and return inspected state;
4. if still open, require a new complete preflight and a new approval before any later mutation;
5. sanitize provider bodies/error text before exposing errors to the model.

Source-branch deletion must remain a separate capability and must never happen implicitly.

## Merge queue

If policy requires a merge queue, direct `PUT .../merge` is not the correct workflow. M3 must fail closed.

Queue enrollment, queue state, dequeue, and queue-specific approval semantics are separate capabilities and remain out of scope for the first direct-merge slice.

## Validation plan for M3

Before any M3 merge:

- focused unit tests for every process/per-remote gate;
- exact `remote + number + expected_head` schema tests;
- stale-head/default-base tests;
- M2-incomplete refusal tests for classic `403`, hidden ruleset bypass actors, custom roles, source disagreement, and unknown rules;
- required-check integration binding and strict-check tests;
- review/code-owner/last-push/stale-review tests;
- conversation-resolution tests;
- required-deployment tests;
- merge-method intersection tests;
- merge-queue refusal tests;
- ambiguous-outcome reinspection tests;
- mutation-at-most-once tests;
- independence from other Git/GitHub mutation gates;
- audit/approval classification as a high-risk side effect;
- full formatting, vet, backend unit/integration/race, frontend lint/unit/build, Windows/Helm/Playwright, Security Scan, and backend/frontend container validation on the exact final head.

## Explicit non-goals

M1–M3 do not combine or implicitly authorize:

- reviewer/team request/removal;
- review submission/dismissal;
- workflow rerun/cancel;
- base retargeting;
- labels/assignees/milestones;
- arbitrary close/reopen;
- source-branch deletion;
- merge-queue enrollment/dequeue;
- ruleset/branch-protection changes;
- any intentional bypass of GitHub protections.

## Decision

**M1, M2, and M3A are implemented as fail-closed read-only prerequisites. The next engineering slice may implement M3B guarded direct merge, but M3B must refuse unless a fresh M3A result proves `eligibility_complete=true` and `eligible=true` for the exact configured repository, actor, PR head, and base. Permission-obscured, truncated, last-push-dependent, or otherwise incomplete evidence remains unsupported rather than bypassed.**
