from pathlib import Path

cred = Path("backend/internal/gitrepo/github_credentials.go")
text = cred.read_text(encoding="utf-8")
old = '''// GetPullRequestMergeRequirements delegates read-only merge-policy inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequestMergeRequirements(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeRequirementsResult, error) {
\tscoped, err := s.scoped(ctx, remoteID)
\tif err != nil || scoped == nil {
\t\treturn nil, err
\t}
\treturn scoped.GetPullRequestMergeRequirements(ctx, remoteID, number)
}
'''
new = old + '''
// GetPullRequestMergePolicyEvidence delegates M2 policy/actor evidence through
// the request-scoped credential so configured-actor semantics match the actual
// credential used for this invocation.
func (s *UserScopedRemoteService) GetPullRequestMergePolicyEvidence(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergePolicyEvidenceResult, error) {
\tscoped, err := s.scoped(ctx, remoteID)
\tif err != nil || scoped == nil {
\t\treturn nil, err
\t}
\treturn scoped.GetPullRequestMergePolicyEvidence(ctx, remoteID, number)
}

// GetPullRequestMergeEligibility delegates M3A exact-head eligibility inspection
// through the request-scoped credential. The scoped service then performs the
// complete M2 -> M3A read sequence with that same credential.
func (s *UserScopedRemoteService) GetPullRequestMergeEligibility(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeEligibilityResult, error) {
\tscoped, err := s.scoped(ctx, remoteID)
\tif err != nil || scoped == nil {
\t\treturn nil, err
\t}
\treturn scoped.GetPullRequestMergeEligibility(ctx, remoteID, number)
}
'''
if old not in text:
    raise SystemExit("github_credentials.go anchor not found")
cred.write_text(text.replace(old, new, 1), encoding="utf-8")

registry = Path("backend/internal/tools/registry_github_credentials.go")
text = registry.read_text(encoding="utf-8")
old = '''\t\tcase *githubPullRequestMergeRequirementsTool:
\t\t\ttyped.service = service
'''
new = old + '''\t\tcase *githubPullRequestMergePolicyEvidenceTool:
\t\t\ttyped.service = service
\t\tcase *githubPullRequestMergeEligibilityTool:
\t\t\ttyped.service = service
'''
if old not in text:
    raise SystemExit("registry_github_credentials.go anchor not found")
registry.write_text(text.replace(old, new, 1), encoding="utf-8")
