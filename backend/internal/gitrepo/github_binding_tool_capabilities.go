package gitrepo

// GitHubBindingToolCapabilities summarizes which binding-backed tool families
// may be registered from startup operator policy. It deliberately describes
// only potential authority: request-scoped binding existence, credentials,
// exact remote policy, reviewed Git state, and approval remain invocation-time
// requirements.
type GitHubBindingToolCapabilities struct {
	Fetch                       bool
	Push                        bool
	BranchCreate                bool
	PullRequestRead             bool
	PullRequestCreate           bool
	PullRequestReply            bool
	PullRequestThreadResolution bool
	PullRequestReady            bool
	PullRequestMerge            bool
}

// GitHubBindingToolCapabilities returns the binding-backed tool families that
// startup configuration can potentially authorize. User-owned connection or
// binding state is intentionally absent from this decision. Every capability is
// intersected with its existing process-wide gate, and clone/default-branch push
// remain outside the binding-derived policy model.
func (s *RemoteService) GitHubBindingToolCapabilities() GitHubBindingToolCapabilities {
	if s == nil {
		return GitHubBindingToolCapabilities{}
	}

	capabilities := GitHubBindingToolCapabilities{Fetch: s.FetchEnabled()}
	for _, policy := range s.githubBindingCapabilities {
		capabilities.Push = capabilities.Push || policy.AllowPush
		capabilities.BranchCreate = capabilities.BranchCreate || policy.AllowBranchCreate
		capabilities.PullRequestRead = capabilities.PullRequestRead || policy.AllowPullRequestRead
		capabilities.PullRequestCreate = capabilities.PullRequestCreate || policy.AllowPullRequestCreate
		capabilities.PullRequestReply = capabilities.PullRequestReply || policy.AllowPullRequestReply
		capabilities.PullRequestThreadResolution = capabilities.PullRequestThreadResolution || policy.AllowPullRequestThreadResolution
		capabilities.PullRequestReady = capabilities.PullRequestReady || policy.AllowPullRequestReady
		capabilities.PullRequestMerge = capabilities.PullRequestMerge || policy.AllowPullRequestMerge
	}

	capabilities.Push = capabilities.Push && s.PushMutationEnabled()
	capabilities.BranchCreate = capabilities.BranchCreate && s.BranchCreateMutationEnabled()
	capabilities.PullRequestRead = capabilities.PullRequestRead && s.GitHubPullRequestReadAccessEnabled()
	capabilities.PullRequestCreate = capabilities.PullRequestCreate && s.GitHubPullRequestMutationEnabled()
	capabilities.PullRequestReply = capabilities.PullRequestReply && s.GitHubPullRequestReplyMutationEnabled()
	capabilities.PullRequestThreadResolution = capabilities.PullRequestThreadResolution && s.GitHubPullRequestThreadResolutionMutationEnabled()
	capabilities.PullRequestReady = capabilities.PullRequestReady && s.GitHubPullRequestReadyMutationEnabled()
	capabilities.PullRequestMerge = capabilities.PullRequestMerge && s.GitHubPullRequestMergeMutationEnabled()
	return capabilities
}
