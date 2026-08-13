# GitHub pull request merge requirements

OmniLLM-Studio exposes two bounded, read-only GitHub merge-policy inspection capabilities for a configured pull request:

- `github_get_pull_request_merge_requirements` — Phase M1 policy normalization;
- `github_get_pull_request_merge_policy_evidence` — Phase M2 policy/actor corroboration.

Neither capability merges a pull request, grants merge permission, changes repository policy, or authorizes a later merge by itself. Phase M2 deliberately returns `direct_merge_supported=false` even when its evidence is complete.

## Operator configuration

Both tools use the existing GitHub pull-request read boundary. The selected remote must be an operator-configured `https://github.com/<owner>/<repository>.git` remote with a token reference and:

```text
OMNILLM_GIT_REMOTE_ENABLED=true
OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED=true
```

and:

```json
{
  "allow_pull_request_read": true
}
```

No merge-specific process flag, per-remote merge permission, merge method, or side-effecting GitHub capability is introduced by M1 or M2.

## M1 — normalized merge requirements

### `github_get_pull_request_merge_requirements`

Model inputs are intentionally limited to:

- `remote` — configured GitHub remote ID;
- `number` — positive pull request number.

Repository owner/name, API URL, token, head SHA, base branch, ruleset/rule IDs, required-check names, reviewer requirements, bypass actors, and merge method are application/provider-derived.

The service fetches the PR through the fixed GitHub boundary and binds the result to its current hosted head SHA and base branch. Nullable GitHub mergeability is preserved as unknown rather than converted to a positive result.

M1 reads and normalizes bounded facts from:

1. repository merge settings;
2. active rules applying to the exact PR base branch;
3. classic branch protection when that REST surface is visible.

Where visible, M1 normalizes:

- open/draft/merged and mergeability state;
- exact head SHA and base branch;
- allowed merge methods;
- required status/check contexts and integration IDs;
- strict/up-to-date status-check policy;
- approving-review count;
- code-owner review;
- last-push approval;
- stale-review dismissal;
- conversation-resolution requirement;
- required deployment environments;
- linear history;
- required commit signatures;
- locked/read-only base state;
- merge queue;
- configured-actor administrator visibility and classic administrator enforcement;
- classic restrictions / pull-request bypass-allowance presence;
- unsupported material policy-rule types.

Unknown material rules are returned only by normalized rule type. Arbitrary provider descriptions, policy prose, hosted URLs, restriction identities, bypass identities, and provider error bodies are not copied into model context.

### M1 fail-closed state

`merge_policy_complete=false` means the normalized view is not proven sufficient for a future merge decision. It does **not** mean GitHub would reject a merge, and it must never be interpreted permissively.

M1 remains incomplete when, among other cases:

- repository merge settings are unavailable;
- active rules are unavailable or reach the bounded page limit;
- classic branch protection cannot be positively characterized;
- classic REST coverage has not been corroborated with merge-relevant GraphQL fields;
- active rulesets exist but actor bypass visibility is not proven;
- classic restrictions or bypass allowances are not actor-resolved;
- an unsupported material rule exists;
- administrator enforcement is insufficient for the configured actor.

A classic protection `404` or `403` is not, by itself, proof that a branch is unprotected.

## M2 — merge policy evidence

### `github_get_pull_request_merge_policy_evidence`

M2 starts from a fresh M1 result for the same configured `remote` + PR `number`, then adds bounded corroboration needed to reason about policy completeness and authenticated-actor bypass risk.

The model still supplies only:

- configured `remote` ID;
- positive PR `number`.

The service derives repository identity, current head/base, authenticated viewer identity, ruleset IDs, policy selectors, API paths, and credentials internally.

M2 performs the following read-only checks:

1. **Classic REST + GraphQL corroboration**
   - queries the exact base ref through a fixed GraphQL `BranchProtectionRule` selection;
   - re-reads the classic REST branch-protection object;
   - reconciles review settings, admin enforcement, restrictions, signatures, linear history, conversation resolution, lock state, strict status checks, and app-bound required checks;
   - supplements required deployment environments from GraphQL;
   - reports inconsistent/incomplete evidence instead of choosing one source when the sources disagree.

2. **Ruleset bypass visibility**
   - re-reads the bounded active rules for the exact base branch;
   - deduplicates active ruleset IDs;
   - inspects at most 20 ruleset details;
   - requires each active ruleset to expose a valid `bypass_actors` field before bypass evidence can be complete;
   - never assumes omitted or permission-hidden bypass actors mean "none".

3. **Configured actor role**
   - obtains the connected viewer login from the fixed GraphQL read;
   - checks that actor through the fixed collaborator-permission endpoint;
   - treats only standard GitHub repository roles (`read`, `triage`, `write`, `maintain`, `admin`) as role-complete;
   - treats custom roles as unverified because their additional permissions are not proven by this metadata read;
   - fails closed when repository permission sources disagree.

4. **Exact-state revalidation**
   - re-fetches the pull request after evidence collection;
   - clears completeness if the PR head or base changed during inspection.

### `evidence_complete`

`evidence_complete=true` is intentionally narrow. It requires all of the following:

- classic policy evidence is complete and internally consistent;
- active-rules/ruleset-detail evidence is complete or not applicable;
- every active ruleset's bypass list was positively visible;
- the configured actor has a standard, consistent repository role;
- actor bypass is proven constrained for the visible policy;
- the resulting normalized merge policy is complete;
- no M2 blocking reason remains;
- the PR head/base remained stable for the evidence pass.

Even then:

```text
direct_merge_supported = false
```

M2 is an evidence gate, not merge authorization.

### Important classic-protection distinction

M2 only marks an unprotected classic branch as confirmed when the exact-ref GraphQL read reports no `BranchProtectionRule` **and** the REST protection endpoint returns `404`.

Permission-obscured REST results such as `403` remain incomplete even when GraphQL returns no classic rule. This prevents a credential-visibility failure from being misclassified as an unprotected branch.

This distinction is exercised by regression tests and was also observed against the OmniLLM-Studio repository during M2 validation: the active-rules endpoint for `main` was visible and empty while the classic protection endpoint was permission-obscured. The correct runtime behavior is therefore to remain fail-closed, not to infer "unprotected".

## Ruleset and classic bypass behavior

M2 does not copy bypass identities into model context. It only uses bounded provider responses to determine whether bypass lists are visible and whether any bypass actors are present.

Evidence remains incomplete or blocking when:

- an active ruleset detail cannot be read;
- `bypass_actors` is omitted or null;
- a ruleset detail's source/enforcement does not match the active rule;
- one or more bypass actors are present;
- classic pull-request bypass allowances are present;
- an administrator actor is not covered by classic administrator enforcement;
- actor role metadata is custom, absent, or inconsistent.

The implementation never intentionally invokes a bypass mechanism.

## Security boundary

Both merge-policy tools are low-risk, read-only, networked, credentialed, and parallel-safe. They use the existing dedicated GitHub transport:

- API host fixed to `https://api.github.com`;
- repository derived only from the operator-configured `github.com` Git remote;
- token loaded from the configured environment-variable reference immediately before use;
- redirects disabled;
- private/local/reserved target protections inherited from the dedicated GitHub client;
- bounded response bodies;
- fixed REST/GraphQL operations;
- provider error bodies suppressed from model-visible errors.

These tools do not add or imply permission to merge, mark ready, create/close/retarget a PR, reply, resolve a thread, push Git refs, rerun workflows, request reviewers, alter rulesets/branch protection, or delete a branch.

## Implication for Phase M3

M2 establishes a fail-closed evidence primitive that a future direct-merge implementation may call immediately before mutation. It does **not** establish that every configured token or repository will produce complete evidence.

A future `github_merge_pull_request` implementation must therefore refuse before mutation unless a fresh M2 pass for the exact PR/head/base returns `evidence_complete=true`. Repositories or credentials that hide classic protection, ruleset bypass actors, actor role semantics, or other material policy must remain unsupported for direct merge.

M3 must also independently validate current check/review/thread/deployment satisfaction and mergeability; M2 proves policy visibility, not fulfillment of every current-state prerequisite.

## Validation coverage

Focused M1/M2 tests cover or should continue to cover:

- operator-bound repository/token and exact PR head/base binding;
- nullable mergeability;
- active rules and classic protection overlap;
- REST + GraphQL classic corroboration;
- required deployments and app-bound status checks;
- strict status checks;
- approving/code-owner/last-push/stale-review requirements;
- conversation resolution, signatures, linear history, and lock state;
- merge queue detection;
- bounded active rules and ruleset-detail inspection;
- positively confirmed unprotected branches;
- permission-obscured classic protection, including `403` fail-closed behavior;
- hidden and visible ruleset bypass actors;
- standard vs custom configured-actor repository roles;
- PR head/base change during evidence collection;
- unknown material rules;
- strict `remote + number` model-facing arguments;
- read-only registration under the GitHub PR-read gate;
- independence from every GitHub mutation gate;
- `direct_merge_supported=false` for all M2 results.

Before merging M2 or any future M3 work, the exact final PR head should pass repository formatting, vet, backend tests/race, frontend lint/unit/build, Windows/Helm/Playwright coverage, Security Scan, and backend/frontend container validation.
