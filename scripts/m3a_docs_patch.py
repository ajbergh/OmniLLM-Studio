from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"missing expected text in {path}: {old[:80]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")

# Design gate: advance status and split M3 into M3A evidence + M3B mutation.
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "**Phase M1 merge-requirements inspection and Phase M2 read-only merge-policy evidence are implemented. `github_merge_pull_request` remains intentionally not implemented. Phase M3 guarded direct merge is the next candidate slice, but it must fail closed unless a fresh M2 evidence pass is complete for the exact PR/head/base and configured actor.**",
    "**Phase M1 merge-requirements inspection, Phase M2 policy/actor evidence, and Phase M3A read-only current-state merge eligibility are implemented. `github_merge_pull_request` remains intentionally not implemented. Phase M3B guarded direct merge is the next candidate slice and must consume a fresh complete M3A result for the exact PR/head/base immediately before mutation.**",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "- `github_get_pull_request_merge_policy_evidence` — M2 classic/ruleset/actor corroboration.\n\nM1 and M2 are both read-only under the existing GitHub PR-read gate. Neither registers, enables, or calls a merge mutation.",
    "- `github_get_pull_request_merge_policy_evidence` — M2 classic/ruleset/actor corroboration;\n- `github_get_pull_request_merge_eligibility` — M3A exact-head current-state eligibility evidence.\n\nM1, M2, and M3A are all read-only under the existing GitHub PR-read gate. None registers, enables, or calls a merge mutation.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "## Threat model for direct merge",
    "## Phase M3A — current-state merge eligibility — IMPLEMENTED\n\nM3A adds `github_get_pull_request_merge_eligibility` as a bounded read-only eligibility reader. It always starts from a fresh M2 pass and remains bound to the exact hosted PR head/base.\n\nM3A proves, where bounded evidence is complete:\n\n- open, ready-for-review, unmerged PR state and known positive mergeability;\n- repository default-base binding and strict base currency when required;\n- exact-head required checks, including numeric GitHub App identity;\n- qualifying write-eligible approval count using bounded latest opinionated reviews;\n- code-owner state corroboration through provider review state plus outstanding code-owner requests;\n- bounded required review-thread resolution;\n- exact-head required deployment success selected by validated timestamps rather than undocumented list ordering;\n- required commit signature evidence within the bounded first page;\n- final PR head/base stability after inspection.\n\nKnown-unsatisfied requirements produce `eligibility_complete=true` with `eligible=false`. Hidden, truncated, stale, ambiguous, or otherwise unverifiable evidence produces `eligibility_complete=false`. M3A explicitly leaves last-push approval unsupported because aggregate review state does not prove that the approving reviewer differs from the actor responsible for the most recent reviewable push.\n\nEvery M3A result keeps `direct_merge_supported=false`; this phase is evidence, not mutation authority.\n\n## Threat model for direct merge",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "## Phase M3 — guarded direct merge — NEXT CANDIDATE\n\nM2 closes the architecture gap by providing a fail-closed evidence predicate. It does **not** guarantee that every repository/token can satisfy it. M3 may therefore be implemented only as a mutation that refuses whenever the configured credential cannot produce complete evidence.",
    "## Phase M3B — guarded direct merge — NEXT CANDIDATE\n\nM3A closes the current-state evidence gap but deliberately does not authorize mutation. M3B may therefore be implemented only as a separately gated high-risk mutation that runs a fresh M3A pass and refuses whenever eligibility evidence is incomplete or unsatisfied.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "5. run a **fresh M2 evidence pass** for the exact PR/head/base;\n6. require `evidence_complete=true` and `merge_policy_complete=true`;",
    "5. run a **fresh M3A eligibility pass** for the exact PR/head/base; M3A itself reruns M2;\n6. require `eligibility_complete=true`, `eligible=true`, `evidence_complete=true`, and `merge_policy_complete=true`;",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "A stale result from any prior read tool must never substitute for this immediate preflight.",
    "A stale result from any prior read tool must never substitute for this immediate preflight. Repositories whose active policy requires last-push approval remain unsupported until an actor-aware bounded proof is implemented.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    "**M1 and M2 are implemented as fail-closed read-only prerequisites. The next engineering slice may implement M3 guarded direct merge, but M3 must refuse unless a fresh M2 result proves `evidence_complete=true` for the exact configured repository, actor, PR head, and base. Permission-obscured or otherwise incomplete policy remains unsupported rather than bypassed.**",
    "**M1, M2, and M3A are implemented as fail-closed read-only prerequisites. The next engineering slice may implement M3B guarded direct merge, but M3B must refuse unless a fresh M3A result proves `eligibility_complete=true` and `eligible=true` for the exact configured repository, actor, PR head, and base. Permission-obscured, truncated, last-push-dependent, or otherwise incomplete evidence remains unsupported rather than bypassed.**",
)

# Requirements: document the third read-only layer and the M3B handoff.
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "OmniLLM-Studio exposes two bounded, read-only GitHub merge-policy inspection capabilities for a configured pull request:\n\n- `github_get_pull_request_merge_requirements` — Phase M1 policy normalization;\n- `github_get_pull_request_merge_policy_evidence` — Phase M2 policy/actor corroboration.\n\nNeither capability merges a pull request, grants merge permission, changes repository policy, or authorizes a later merge by itself. Phase M2 deliberately returns `direct_merge_supported=false` even when its evidence is complete.",
    "OmniLLM-Studio exposes three bounded, read-only GitHub merge-evidence capabilities for a configured pull request:\n\n- `github_get_pull_request_merge_requirements` — Phase M1 policy normalization;\n- `github_get_pull_request_merge_policy_evidence` — Phase M2 policy/actor corroboration;\n- `github_get_pull_request_merge_eligibility` — Phase M3A exact-head current-state eligibility.\n\nNone merges a pull request, grants merge permission, changes repository policy, or authorizes a later merge by itself. M2 and M3A deliberately return `direct_merge_supported=false` even when their evidence is complete.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "No merge-specific process flag, per-remote merge permission, merge method, or side-effecting GitHub capability is introduced by M1 or M2.",
    "No merge-specific process flag, per-remote merge permission, merge method, or side-effecting GitHub capability is introduced by M1, M2, or M3A.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "## Security boundary",
    "## M3A — current-state merge eligibility\n\n### `github_get_pull_request_merge_eligibility`\n\nM3A starts from a fresh M2 policy/actor evidence pass and then evaluates whether the exact current PR state satisfies the visible policy. Model inputs remain only configured `remote` plus positive PR `number`; repository identity, head/base, required checks, reviewers, deployments, rules, and API endpoints remain application/provider-derived.\n\nM3A checks:\n\n- open, non-draft, unmerged state and known positive mergeability;\n- configured repository default-base binding;\n- strict base currency when required;\n- exact-head required checks/statuses, including numeric GitHub App binding;\n- bounded qualifying approval count from write-eligible latest opinionated reviews;\n- stale-review head association when stale approvals are dismissed;\n- provider review decision and outstanding code-owner requests;\n- bounded review-thread resolution when required;\n- exact-head deployment evidence for each required environment, with newest deployment/status selected by validated timestamps;\n- bounded commit-signature evidence when required;\n- final exact head/base revalidation.\n\n`eligibility_complete=true` means the evidence set was complete enough to decide. `eligible=true` additionally means no known prerequisite is unsatisfied. A missing or truncated page, unknown mergeability, hidden policy, unstable head/base, ambiguous deployment ordering, or other unverifiable prerequisite clears completeness rather than being treated as satisfied.\n\nM3A intentionally fails closed for `require_last_push_approval`: GitHub's aggregate review state and approval count do not prove that an approving reviewer differs from the actor responsible for the most recent reviewable push. Until a bounded actor-aware proof exists, such repositories remain unsupported for direct merge.\n\nEvery M3A result keeps `direct_merge_supported=false`.\n\n## Security boundary",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "Both merge-policy tools are low-risk, read-only, networked, credentialed, and parallel-safe.",
    "All three merge-evidence tools are low-risk, read-only, networked, credentialed, and parallel-safe.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "## Implication for Phase M3\n\nM2 establishes a fail-closed evidence primitive that a future direct-merge implementation may call immediately before mutation. It does **not** establish that every configured token or repository will produce complete evidence.\n\nA future `github_merge_pull_request` implementation must therefore refuse before mutation unless a fresh M2 pass for the exact PR/head/base returns `evidence_complete=true`. Repositories or credentials that hide classic protection, ruleset bypass actors, actor role semantics, or other material policy must remain unsupported for direct merge.\n\nM3 must also independently validate current check/review/thread/deployment satisfaction and mergeability; M2 proves policy visibility, not fulfillment of every current-state prerequisite.",
    "## Implication for Phase M3B\n\nM3A establishes the read-only current-state predicate needed by a future direct-merge implementation. A future `github_merge_pull_request` must rerun M3A immediately before mutation and refuse unless `eligibility_complete=true` and `eligible=true` for the exact expected head/base. M3A itself reruns M2, so hidden policy/bypass/actor evidence remains fail-closed.\n\nRepositories whose policy requires unsupported last-push approval, or whose required evidence is hidden/truncated/ambiguous, remain unsupported for direct merge.",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "Focused M1/M2 tests cover or should continue to cover:",
    "Focused M1/M2/M3A tests cover or should continue to cover:",
)
replace_once(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    "- `direct_merge_supported=false` for all M2 results.",
    "- `direct_merge_supported=false` for all M2/M3A results;\n- exact-head app-bound checks, strict-base state, bounded qualifying approval counts, code-owner state, deployment timestamp selection, signature bounds, last-push fail-closed behavior, and final head/base revalidation.",
)

# Parity roadmap: record M3A and narrow Priority 1 to M3B.
replace_once(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    "- bounded M2 merge-policy evidence that corroborates classic REST/GraphQL policy, active ruleset bypass visibility, configured-actor repository role, and exact PR state while remaining strictly read-only.",
    "- bounded M2 merge-policy evidence that corroborates classic REST/GraphQL policy, active ruleset bypass visibility, configured-actor repository role, and exact PR state while remaining strictly read-only;\n- bounded M3A current-state merge eligibility that composes fresh M2 evidence with exact-head checks, review, deployment, thread, signature, default-base, and mergeability evidence while remaining strictly read-only.",
)
replace_once(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    "- `backend/internal/gitrepo/github_pull_request_merge_policy_evidence.go`\n- `backend/internal/tools/github_pull_request_merge_policy_evidence_tool.go`",
    "- `backend/internal/gitrepo/github_pull_request_merge_policy_evidence.go`\n- `backend/internal/gitrepo/github_pull_request_merge_eligibility.go`\n- `backend/internal/tools/github_pull_request_merge_policy_evidence_tool.go`\n- `backend/internal/tools/github_pull_request_merge_eligibility_tool.go`",
)
replace_once(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    "### Priority 1 — Phase M3 guarded direct merge — CONDITIONAL NEXT SLICE\n\nM2 now provides a fail-closed evidence primitive, so guarded direct merge can be implemented as the next isolated engineering slice **only if the mutation refuses whenever a fresh M2 pass is incomplete**. M2 does not guarantee that every repository/token can satisfy its evidence requirements.",
    "### Completed — Phase M3A current-state merge eligibility\n\n`github_get_pull_request_merge_eligibility` is registered only under the existing GitHub PR-read gate. It composes a fresh M2 evidence pass with bounded current-state checks for exact-head CI/App identity, qualifying approval count, code-owner state, strict base currency, review-thread resolution, timestamp-selected deployments, signatures, default base, mergeability, and final head/base stability. Known-unsatisfied evidence returns complete-but-ineligible; hidden/truncated/ambiguous evidence fails closed. Last-push approval remains intentionally unsupported until actor-aware proof is available. `direct_merge_supported` always remains false.\n\nImplementation anchors:\n\n- `backend/internal/gitrepo/github_pull_request_merge_eligibility.go`\n- `backend/internal/gitrepo/github_pull_request_merge_eligibility_test.go`\n- `backend/internal/gitrepo/github_pull_request_merge_eligibility_bounds_test.go`\n- `backend/internal/tools/github_pull_request_merge_eligibility_tool.go`\n- `docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md`\n- `docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md`\n\n### Priority 1 — Phase M3B guarded direct merge — CONDITIONAL NEXT SLICE\n\nM3A now provides the fail-closed current-state predicate, so guarded direct merge can be implemented as the next isolated engineering slice **only if the mutation reruns M3A immediately before mutation and refuses unless eligibility is complete and satisfied**.",
)
replace_once(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    "- run a fresh M2 evidence pass immediately before mutation and require `evidence_complete=true` plus complete normalized policy;",
    "- run a fresh M3A eligibility pass immediately before mutation and require `eligibility_complete=true` plus `eligible=true`; M3A itself reruns M2;",
)
replace_once(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    "Repositories/credentials that hide classic protection, ruleset bypass actors, actor-role semantics, or other material policy remain unsupported for direct merge.",
    "Repositories/credentials that hide classic protection, ruleset bypass actors, actor-role semantics, last-push actor relationships, or other material evidence remain unsupported for direct merge.",
)
