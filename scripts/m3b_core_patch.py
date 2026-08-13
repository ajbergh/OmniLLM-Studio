from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"missing anchor in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")

# remote_config.go
path = "backend/internal/gitrepo/remote_config.go"
replace_once(path,
'''\t// GitHubPullRequestReadyEnabledEnv independently enables advancing an exact
\t// reviewed draft pull request to ready-for-review state.
\tGitHubPullRequestReadyEnabledEnv = "OMNILLM_GITHUB_PULL_REQUEST_READY_ENABLED"
)''',
'''\t// GitHubPullRequestReadyEnabledEnv independently enables advancing an exact
\t// reviewed draft pull request to ready-for-review state.
\tGitHubPullRequestReadyEnabledEnv = "OMNILLM_GITHUB_PULL_REQUEST_READY_ENABLED"
\t// GitHubPullRequestMergeEnabledEnv independently enables the guarded direct
\t// merge mutation. Read/create/reply/ready/push access does not imply merge.
\tGitHubPullRequestMergeEnabledEnv = "OMNILLM_GITHUB_PULL_REQUEST_MERGE_ENABLED"
)''')
replace_once(path,
'''\tAllowPullRequestReady            bool   `json:"allow_pull_request_ready,omitempty"`
\tAllowDefaultBranchPush           bool   `json:"allow_default_branch_push,omitempty"`''',
'''\tAllowPullRequestReady            bool   `json:"allow_pull_request_ready,omitempty"`
\tAllowPullRequestMerge            bool   `json:"allow_pull_request_merge,omitempty"`
\tPullRequestMergeMethod           string `json:"pull_request_merge_method,omitempty"`
\tAllowDefaultBranchPush           bool   `json:"allow_default_branch_push,omitempty"`''')
replace_once(path,
'''\tPullRequestReadyAllowed            bool   `json:"pull_request_ready_allowed"`
\tDefaultBranchPushAllowed           bool   `json:"default_branch_push_allowed"`''',
'''\tPullRequestReadyAllowed            bool   `json:"pull_request_ready_allowed"`
\tPullRequestMergeAllowed            bool   `json:"pull_request_merge_allowed"`
\tPullRequestMergeMethod             string `json:"pull_request_merge_method,omitempty"`
\tDefaultBranchPushAllowed           bool   `json:"default_branch_push_allowed"`''')
replace_once(path,
'''\tcandidate.Username = strings.TrimSpace(candidate.Username)
\tcandidate.TokenEnv = strings.TrimSpace(candidate.TokenEnv)''',
'''\tcandidate.Username = strings.TrimSpace(candidate.Username)
\tcandidate.TokenEnv = strings.TrimSpace(candidate.TokenEnv)
\tcandidate.PullRequestMergeMethod = strings.ToLower(strings.TrimSpace(candidate.PullRequestMergeMethod))''')
replace_once(path,
'''\tif candidate.AllowDefaultBranchPush && !candidate.AllowPush {
\t\treturn RemoteConfig{}, false
\t}
\treturn candidate, true''',
'''\tif candidate.AllowDefaultBranchPush && !candidate.AllowPush {
\t\treturn RemoteConfig{}, false
\t}
\tif candidate.AllowPullRequestMerge {
\t\tif !candidate.AllowPullRequestRead || !validGitHubMergeMethod(candidate.PullRequestMergeMethod) {
\t\t\treturn RemoteConfig{}, false
\t\t}
\t} else if candidate.PullRequestMergeMethod != "" {
\t\treturn RemoteConfig{}, false
\t}
\treturn candidate, true''')

# remote.go
path = "backend/internal/gitrepo/remote.go"
replace_once(path,
'''\tgithubPullRequestReadyEnabled            bool
\tcloneEnabled                             bool''',
'''\tgithubPullRequestReadyEnabled            bool
\tgithubPullRequestMergeEnabled            bool
\tcloneEnabled                             bool''')
replace_once(path,
'''\tservice.githubPullRequestReadyEnabled = boolEnvironment(GitHubPullRequestReadyEnabledEnv)
\tif maxBytes, maxEntries, ok := cloneLimitsFromEnvironment(); ok {''',
'''\tservice.githubPullRequestReadyEnabled = boolEnvironment(GitHubPullRequestReadyEnabledEnv)
\tservice.githubPullRequestMergeEnabled = boolEnvironment(GitHubPullRequestMergeEnabledEnv)
\tif maxBytes, maxEntries, ok := cloneLimitsFromEnvironment(); ok {''')
replace_once(path,
'''// GitHubPullRequestReadyEnabled reports the independent process-wide gate for
// marking a reviewed draft pull request ready for review.
func (s *RemoteService) GitHubPullRequestReadyEnabled() bool {
\treturn s != nil && s.githubPullRequestReadyEnabled
}
''',
'''// GitHubPullRequestReadyEnabled reports the independent process-wide gate for
// marking a reviewed draft pull request ready for review.
func (s *RemoteService) GitHubPullRequestReadyEnabled() bool {
\treturn s != nil && s.githubPullRequestReadyEnabled
}

// GitHubPullRequestMergeEnabled reports the independent process-wide gate for
// guarded direct pull request merge.
func (s *RemoteService) GitHubPullRequestMergeEnabled() bool {
\treturn s != nil && s.githubPullRequestMergeEnabled
}
''')
replace_once(path,
'''// GitHubPullRequestReadyMutationEnabled reports whether the process permits the
// draft-to-ready hosted mutation. It is independent from PR reads, creation,
// replies, thread resolution, local Git writes, and Git push.
func (s *RemoteService) GitHubPullRequestReadyMutationEnabled() bool {
\treturn s != nil && s.Enabled() && s.GitHubPullRequestReadyEnabled()
}
''',
'''// GitHubPullRequestReadyMutationEnabled reports whether the process permits the
// draft-to-ready hosted mutation. It is independent from PR reads, creation,
// replies, thread resolution, local Git writes, and Git push.
func (s *RemoteService) GitHubPullRequestReadyMutationEnabled() bool {
\treturn s != nil && s.Enabled() && s.GitHubPullRequestReadyEnabled()
}

// GitHubPullRequestMergeMutationEnabled reports whether the process permits the
// guarded direct-merge mutation. M3B deliberately also requires the independent
// PR-read gate because every merge must run a fresh M3A read-only preflight.
func (s *RemoteService) GitHubPullRequestMergeMutationEnabled() bool {
\treturn s != nil && s.GitHubPullRequestReadAccessEnabled() && s.GitHubPullRequestMergeEnabled()
}
''')
replace_once(path,
'''\t\t\tPullRequestReadyAllowed:            s.GitHubPullRequestReadyMutationEnabled() && remoteSupportsGitHubPullRequestReady(remote),
\t\t\tDefaultBranchPushAllowed:           s.PushMutationEnabled() && remote.AllowPush && remote.AllowDefaultBranchPush,''',
'''\t\t\tPullRequestReadyAllowed:            s.GitHubPullRequestReadyMutationEnabled() && remoteSupportsGitHubPullRequestReady(remote),
\t\t\tPullRequestMergeAllowed:            s.GitHubPullRequestMergeMutationEnabled() && remoteSupportsGitHubPullRequestMerge(remote),
\t\t\tPullRequestMergeMethod:             remote.PullRequestMergeMethod,
\t\t\tDefaultBranchPushAllowed:           s.PushMutationEnabled() && remote.AllowPush && remote.AllowDefaultBranchPush,''')

# eligibility result carries fresh policy merge methods into M3B.
path = "backend/internal/gitrepo/github_pull_request_merge_eligibility.go"
replace_once(path,
'''\tBaseBranch                   string                                `json:"base_branch"`
\tPolicyEvidenceComplete       bool                                  `json:"policy_evidence_complete"`''',
'''\tBaseBranch                   string                                `json:"base_branch"`
\tAllowedMergeMethods          []string                              `json:"allowed_merge_methods"`
\tPolicyEvidenceComplete       bool                                  `json:"policy_evidence_complete"`''')
replace_once(path,
'''\t\tHead: policy.Head, BaseBranch: policy.BaseBranch, PolicyEvidenceComplete: policy.EvidenceComplete,
\t\tStrictBaseCurrent:''',
'''\t\tHead: policy.Head, BaseBranch: policy.BaseBranch,
\t\tAllowedMergeMethods: sortedUniqueStrings(policy.Requirements.AllowedMergeMethods),
\t\tPolicyEvidenceComplete: policy.EvidenceComplete,
\t\tStrictBaseCurrent:''')

# internal PR response gains merge_commit_sha solely for post-outcome confirmation.
path = "backend/internal/gitrepo/github_pull_request_read.go"
replace_once(path,
'''\tMerged         bool   `json:"merged"`
\tMergeable      *bool  `json:"mergeable"`''',
'''\tMerged         bool   `json:"merged"`
\tMergeCommitSHA string `json:"merge_commit_sha"`
\tMergeable      *bool  `json:"mergeable"`''')

# User-scoped merge delegate.
path = "backend/internal/gitrepo/github_credentials.go"
text = Path(path).read_text(encoding="utf-8")
anchor = '''// GetPullRequestMergeEligibility delegates M3A exact-head eligibility inspection
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
addition = anchor + '''
// MergePullRequest delegates M3B through the same request-scoped credential used
// for its fresh M2/M3A preflight and the one-shot merge request.
func (s *UserScopedRemoteService) MergePullRequest(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestMergeResult, error) {
\tscoped, err := s.scoped(ctx, remoteID)
\tif err != nil || scoped == nil {
\t\treturn nil, err
\t}
\treturn scoped.MergePullRequest(ctx, remoteID, number, expectedHead)
}
'''
if anchor not in text:
    raise SystemExit("github_credentials.go M3A anchor missing")
Path(path).write_text(text.replace(anchor, addition, 1), encoding="utf-8")
