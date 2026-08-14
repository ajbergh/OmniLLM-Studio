# GitHub pull request merge design gate — August 2026

## Status

**Phases M1, M2, M3A, and M3B are implemented.** M1 normalizes merge requirements, M2 corroborates policy and configured-actor evidence, M3A produces fail-closed read-only current-state eligibility, and M3B exposes the independently gated critical-risk `github_merge_pull_request` mutation. M3B reruns M3A immediately before mutation and refuses unless the exact reviewed PR/head/base remains completely provable and eligible.

The hosted coding workflow now covers draft PR creation, exact-head PR/check/feedback/thread inspection, review-comment replies, thread resolution, guarded draft-to-ready transition, normalized merge-requirements inspection, actor/policy evidence corroboration, current-state merge eligibility, and guarded direct merge. Merge is the first lifecycle action in this sequence that irreversibly changes the configured default branch, so its mutation boundary remains stricter than all preceding collaboration tools.

## Current implemented evidence and mutation surface

The runtime provides:

- `github_get_pull_request` — exact hosted PR metadata;
- `github_get_pull_request_checks` — exact-head check runs and commit statuses;
- `github_get_pull_request_feedback` — bounded submitted reviews and review requests;
- `github_get_pull_request_review_threads` — bounded review-thread state/location;
- `github_mark_pull_request_ready_for_review` — separately gated draft-to-ready mutation;
- `github_get_pull_request_merge_requirements` — M1 normalized merge policy;
- `github_get_pull_request_merge_policy_evidence` — M2 classic/ruleset/actor corroboration;
- `github_get_pull_request_merge_eligibility` — M3A exact-head current-state eligibility evidence;
- `github_merge_pull_request` — M3B exact-head guarded merge mutation.

M1, M2, and M3A remain read-only under the existing GitHub PR-read gate. M3B has independent process-wide and per-remote mutation authorization and is registered as a critical-risk, side-effecting, non-parallel capability.

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

Even a complete M2 result returns `direct_merge_supported=false`: M2 is an evidence primitive, not mutation authority.

### Confirmed permission-obscured behavior

M2 explicitly distinguishes a confirmed unprotected branch from permission-obscured classic protection. A branch is only classified as classically unprotected when exact-ref GraphQL reports no `BranchProtectionRule` and the classic REST protection endpoint returns `404`. A REST `403` remains incomplete even when GraphQL reports no classic rule.

During M2 validation against OmniLLM-Studio, the active-rules read for `main` was visible and empty while the classic protection REST endpoint was permission-obscured. That response pattern validates the fail-closed rule: zero visible active rules plus a `403` must not be converted into "unprotected".

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

Every M3A result keeps `direct_merge_supported=false`; M3A is evidence, not mutation authority. M3B consumes M3A internally but does not change that M3A result contract.

## Threat model for direct merge

The implemented merge boundary assumes:

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

The implementation prefers false negatives over a policy-bypassing false positive.

## Phase M3B — guarded direct merge — IMPLEMENTED

M3B is a separately gated critical-risk mutation. It never treats an earlier displayed eligibility result as durable authorization. Each merge call runs a fresh M3A pass and refuses whenever eligibility evidence is incomplete or unsatisfied.

### Independent operator gates

The process-wide gate is:

```text
OMNILLM_GITHUB_PULL_REQUEST_MERGE_ENABLED=true
```

The configured remote must also opt in and select a merge method:

```json
{
  "allow_pull_request_read": true,
  "allow_pull_request_merge": true,
  "pull_request_merge_method": "squash"
}
```

`allow_pull_request_merge` is invalid without pull-request read access or a supported merge method. Supported configured methods are `merge`, `squash`, and `rebase`.

Merge authority is not implied by:

- GitHub PR read/create/reply/resolve/ready permissions;
- local Git mutation access;
- remote Git push access;
- provider `viewerCan*` fields;
- repository administrator status;
- a previous M1/M2/M3A result;
- a successful prior CI observation.

### Model inputs

The tool accepts exactly:

- configured `remote` ID;
- positive PR `number`;
- exact 40-character reviewed `expected_head`.

It does not accept repository owner/name, API URL, token, base branch, alternate ref, merge method, commit title/message, bypass flag, workflow control, or branch-deletion option from the model. Repository identity, credential, base/default branch, API endpoint, and merge method remain operator/application-controlled.

### Mandatory fresh preflight

Immediately before any merge request, M3B:

1. resolves the operator-configured GitHub repository, credential, and merge method;
2. runs a fresh `GetPullRequestMergeEligibility` pass for the configured PR; M3A itself reruns M2/M1 evidence;
3. requires `eligibility_complete=true` and `eligible=true`;
4. requires the fresh eligibility head to equal the exact user-reviewed `expected_head`;
5. requires the fresh eligibility result to verify the repository default base;
6. intersects the operator-configured merge method with the fresh policy's allowed methods;
7. therefore refuses merge-queue, last-push-dependent, hidden, truncated, ambiguous, stale, or otherwise unsupported evidence through the M3A/M2 fail-closed path;
8. issues exactly one fixed GitHub merge request with the exact-head SHA server-side precondition.

A stale result from a prior read tool never substitutes for this immediate preflight.

## Exact-head compare-and-swap

M3B uses GitHub's REST merge endpoint with the reviewed head SHA supplied as the server-side precondition. The expected SHA is never omitted or changed during an automatic retry because the implementation does not blindly retry merge mutations.

The server-side precondition supplements—not replaces—the fresh M3A/M2 policy and state proof.

## Ambiguous mutation outcome

If the merge request is sent but the transport or response cannot establish a trustworthy result, the implementation performs one bounded PR reinspection. It returns a successful confirmed result only when the hosted PR is merged and still matches the expected head/base with a valid merge commit SHA.

Otherwise it returns an explicit unknown/invalid outcome and requires hosted state to be reinspected before any later merge attempt. It never automatically submits a second merge request. Source-branch deletion is never implicit.

## Merge queue

When policy requires a merge queue, M3A cannot produce the complete direct-merge predicate needed by M3B. Direct `PUT .../merge` is not used to approximate queue enrollment. Queue enrollment, queue state, dequeue, and queue-specific approval semantics remain separate unsupported capabilities.

## Validation coverage

M3B has focused coverage for its independent process/per-remote gates, strict `{remote, number, expected_head}` schema, exact-head/default-base binding, fresh eligibility refusal, merge-method intersection, policy-incomplete/unsatisfied refusal, merge-queue and unsupported-evidence refusal through M3A, one-shot mutation behavior, ambiguous-outcome reinspection, and independence from unrelated Git/GitHub mutation permissions.

Changes to this security-sensitive surface must still pass the full repository formatting, vet, backend unit/integration/race, frontend lint/unit/build, Windows/Helm/Playwright, Security Scan, and applicable backend/frontend container validation on the exact final head.

## Explicit non-goals

M1–M3B do not combine or implicitly authorize:

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

**M1, M2, M3A, and M3B are implemented. Direct merge remains intentionally fail-closed: `github_merge_pull_request` may issue its one exact-head merge request only after a fresh M3A pass proves `eligibility_complete=true` and `eligible=true` for the exact configured repository, PR head, and default base, and the operator-configured merge method remains allowed. Permission-obscured, truncated, last-push-dependent, merge-queue, stale, or otherwise incomplete evidence remains unsupported rather than bypassed.**
