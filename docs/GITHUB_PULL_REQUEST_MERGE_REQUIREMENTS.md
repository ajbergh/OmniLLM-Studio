# GitHub pull request merge requirements

OmniLLM-Studio exposes a bounded read-only merge-policy inspection capability for a configured GitHub pull request. It exists to answer which merge requirements are visible for the pull request's **current GitHub head and base branch** without granting merge permission.

This capability is the Phase M1 prerequisite defined in `docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md`. It does **not** implement `github_merge_pull_request` and must not be treated as merge authorization by itself.

## Operator configuration

The tool uses the existing GitHub pull-request read boundary. The selected remote must be an operator-configured `https://github.com/<owner>/<repository>.git` remote with a token reference and:

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

No merge-specific process or per-remote permission is introduced by this read-only slice.

## Tool

### `github_get_pull_request_merge_requirements`

Inputs are intentionally limited to:

- `remote` — configured GitHub remote ID from `git_remotes`;
- `number` — positive pull request number.

The model cannot supply repository owner/name, API URL, token, head SHA, base branch, ruleset/rule IDs, required-check names, reviewer requirements, bypass actors, or merge method.

The service first fetches the pull request through the existing fixed GitHub REST boundary and binds every result to its current hosted head SHA and base branch.

## Normalized policy sources

The reader combines bounded facts from three fixed GitHub REST surfaces:

1. repository settings, for enabled merge methods and whether the configured actor is reported as an administrator;
2. active rules applying to the exact PR base branch, including applicable higher-level rules exposed by GitHub's branch-rules endpoint;
3. classic branch protection for the exact PR base when that surface is visible to the configured credential.

The result normalizes, where visible:

- current PR open/draft/merged and mergeability state;
- exact head SHA and base branch;
- repository/ruleset-allowed merge methods;
- required status/check contexts and integration IDs;
- strict/up-to-date status-check policy;
- required approving-review count;
- code-owner review;
- last-push approval;
- stale-review dismissal;
- review-thread/conversation resolution;
- required deployment environments;
- linear history;
- merge queue;
- configured-actor administrator visibility and classic administrator enforcement;
- unsupported material policy rule types.

Provider descriptions, arbitrary hosted URLs, policy prose, and error bodies are not copied into the model-facing result.

## `merge_policy_complete`

`merge_policy_complete` is deliberately fail-closed. `false` means the reader cannot prove that its normalized policy view is sufficient for a later merge decision. It does **not** mean GitHub would reject a merge, and it must never be converted into a permissive assumption.

The current implementation reports incomplete policy when any of the following applies:

- repository merge settings are unavailable;
- active rules are unavailable or reach the bounded 100-rule page limit;
- classic branch protection is not positively visible;
- an active ruleset exists but the active-rules surface does not prove whether the configured actor is constrained by ruleset bypass policy;
- a material rule type or rule parameter is not normalized by this implementation;
- the configured actor is an administrator while visible classic protection does not enforce administrators.

A `404` from the classic protection endpoint is therefore represented as `classic_protection_status: "unavailable_or_unprotected"`, not as proof that the base branch is unprotected.

## Ruleset bypass limitation

GitHub's active branch-rules endpoint tells OmniLLM which rules apply to the base branch, but it does not by itself establish the configured actor's effective ruleset bypass position. When active rules are present, this M1 implementation reports:

```text
ruleset_bypass_visibility = incomplete
merge_policy_complete = false
```

and `potential_bypass = true`.

That conservative result is intentional. Phase M2 must review whether a separately bounded actor/ruleset-detail read can close this gap safely before any merge mutation is considered.

## Unknown rules

Unknown or unsupported material rule types are returned only by normalized type name in `unknown_policy_rules`; their arbitrary parameter payload is not copied into model context. Any such rule keeps policy incomplete.

The first implementation recognizes the merge-relevant active rule types needed by M1, including pull-request requirements, required status checks, required deployments, required linear history, and merge queue. Ref-creation/deletion/non-fast-forward rules are not treated as direct prerequisites for merging an already-existing PR into its existing base ref.

## Security boundary

`github_get_pull_request_merge_requirements` is low-risk, read-only, networked, credentialed, and parallel-safe. It uses the same dedicated GitHub client as the other PR read tools:

- API host fixed to `https://api.github.com`;
- repository derived only from the selected operator-configured `github.com` Git remote;
- token read from the configured environment-variable reference immediately before use;
- redirects disabled;
- private/local/reserved target protection inherited from the dedicated GitHub transport;
- bounded response bodies;
- fixed REST paths and pinned GitHub REST API version;
- provider error bodies suppressed from model-visible errors.

This capability does not add or imply permission to merge, mark ready, create a PR, reply, resolve a thread, push Git refs, alter branch protection/rulesets, rerun workflows, request reviewers, close a PR, or delete a branch.

## Validation

Focused tests should continue to cover:

- operator-bound repository/token and exact PR head/base binding;
- active rules and classic branch-protection overlap;
- strict status checks and app-bound contexts;
- approving-review, code-owner, last-push, stale-review, and conversation-resolution requirements;
- deployment and linear-history requirements;
- merge queue detection;
- bounded active-rule handling;
- classic-protection `404` ambiguity;
- administrator/bypass visibility;
- unknown material rules;
- strict `remote + number` model-facing arguments;
- independence from every hosted mutation gate.

Before merge, the exact final PR head should pass repository formatting, vet, backend tests/race, frontend checks, Windows/Helm/Playwright coverage, Security Scan, and backend/frontend container validation.
