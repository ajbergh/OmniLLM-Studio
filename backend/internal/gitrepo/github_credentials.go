package gitrepo

import (
	"context"
	"fmt"
	"strings"
)

const githubAppSyntheticTokenEnv = "__OMNILLM_GITHUB_APP_TOKEN__"

// GitHubCredentialResolver provides request-scoped GitHub App credential state.
// GitHubCredentialConnected must be a local/non-network status check suitable for
// git_remotes. ResolveGitHubCredential may refresh an expiring credential and is
// called only by operations that already permit network access.
//
// connected=false with nil error means no user connection exists and preserves
// the operator TokenEnv fallback. Any resolver error fails closed and must not
// fall back to an operator credential for that request.
type GitHubCredentialResolver interface {
	GitHubCredentialConnected(ctx context.Context) (connected bool, err error)
	ResolveGitHubCredential(ctx context.Context) (token string, connected bool, err error)
}

// GitHubCredentialResolverFuncs adapts local-status and token-resolution
// functions to GitHubCredentialResolver.
type GitHubCredentialResolverFuncs struct {
	ConnectedFunc func(context.Context) (bool, error)
	ResolveFunc   func(context.Context) (string, bool, error)
}

// GitHubCredentialConnected implements GitHubCredentialResolver without
// performing token resolution or refresh.
func (f GitHubCredentialResolverFuncs) GitHubCredentialConnected(ctx context.Context) (bool, error) {
	if f.ConnectedFunc == nil {
		return false, nil
	}
	return f.ConnectedFunc(ctx)
}

// ResolveGitHubCredential implements GitHubCredentialResolver.
func (f GitHubCredentialResolverFuncs) ResolveGitHubCredential(ctx context.Context) (string, bool, error) {
	if f.ResolveFunc == nil {
		return "", false, nil
	}
	return f.ResolveFunc(ctx)
}

// UserScopedRemoteService decorates the existing RemoteService with a
// request-scoped GitHub credential. It intentionally preserves every existing
// network, mutation, repository, state-binding, and approval gate: only the
// credential source changes.
//
// A connected GitHub App credential takes precedence over TokenEnv for exact
// github.com remotes. If no user connection exists, the existing TokenEnv/public
// remote behavior is retained. Resolver/refresh failures fail closed and never
// fall back to TokenEnv. Non-GitHub remotes never receive the GitHub credential.
type UserScopedRemoteService struct {
	*RemoteService
	githubCredentials GitHubCredentialResolver
}

// NewUserScopedRemoteService wraps a configured remote service. A nil resolver
// is valid and preserves the existing operator-only credential behavior.
func NewUserScopedRemoteService(base *RemoteService, resolver GitHubCredentialResolver) *UserScopedRemoteService {
	return &UserScopedRemoteService{RemoteService: base, githubCredentials: resolver}
}

func (s *UserScopedRemoteService) scoped(ctx context.Context, remoteID string) (*RemoteService, error) {
	if s == nil || s.RemoteService == nil || s.githubCredentials == nil {
		if s == nil {
			return nil, nil
		}
		return s.RemoteService, nil
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return s.RemoteService, nil
	}
	if _, _, githubRemote := githubRepositoryFromRemote(remote); !githubRemote {
		return s.RemoteService, nil
	}

	token, connected, err := s.githubCredentials.ResolveGitHubCredential(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q GitHub credentials are unavailable", remoteID)
	}
	if !connected {
		return s.RemoteService, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("remote %q GitHub credentials are unavailable", remoteID)
	}

	clone := *s.RemoteService
	clone.remotes = cloneRemoteConfigs(s.remotes)
	target := clone.remotes[remoteID]
	credentialEnv := target.TokenEnv
	if credentialEnv == "" {
		credentialEnv = githubAppSyntheticTokenEnv
		target.TokenEnv = credentialEnv
	}
	if strings.TrimSpace(target.Username) == "" {
		target.Username = "x-access-token"
	}
	clone.remotes[remoteID] = target
	fallback := s.lookupEnv
	clone.lookupEnv = func(name string) (string, bool) {
		if name == credentialEnv {
			return token, true
		}
		if fallback == nil {
			return "", false
		}
		return fallback(name)
	}
	return &clone, nil
}

func cloneRemoteConfigs(remotes map[string]RemoteConfig) map[string]RemoteConfig {
	cloned := make(map[string]RemoteConfig, len(remotes))
	for id, remote := range remotes {
		cloned[id] = remote
	}
	return cloned
}

func (s *UserScopedRemoteService) summariesWithoutGitHubCredentials(ctx context.Context) []RemoteSummary {
	clone := *s.RemoteService
	clone.remotes = cloneRemoteConfigs(s.remotes)
	for id, remote := range clone.remotes {
		if _, _, githubRemote := githubRepositoryFromRemote(remote); githubRemote {
			remote.TokenEnv = ""
			clone.remotes[id] = remote
		}
	}
	return clone.Remotes(ctx)
}

func (s *UserScopedRemoteService) summariesWithGitHubCredential(ctx context.Context) []RemoteSummary {
	if s == nil || s.RemoteService == nil || s.githubCredentials == nil {
		if s == nil || s.RemoteService == nil {
			return nil
		}
		return s.RemoteService.Remotes(ctx)
	}
	connected, err := s.githubCredentials.GitHubCredentialConnected(ctx)
	if err != nil {
		return s.summariesWithoutGitHubCredentials(ctx)
	}
	if !connected {
		return s.RemoteService.Remotes(ctx)
	}

	clone := *s.RemoteService
	clone.remotes = cloneRemoteConfigs(s.remotes)
	for id, remote := range clone.remotes {
		if _, _, githubRemote := githubRepositoryFromRemote(remote); !githubRemote {
			continue
		}
		if remote.TokenEnv == "" {
			remote.TokenEnv = githubAppSyntheticTokenEnv
		}
		if strings.TrimSpace(remote.Username) == "" {
			remote.Username = "x-access-token"
		}
		clone.remotes[id] = remote
	}
	return clone.Remotes(ctx)
}

// Remotes returns request-scoped safe summaries using only the resolver's local
// connection-status check. It never resolves or refreshes a GitHub token.
func (s *UserScopedRemoteService) Remotes(ctx context.Context) []RemoteSummary {
	return s.summariesWithGitHubCredential(ctx)
}

// RemoteStatus delegates remote inspection through the request-scoped credential.
func (s *UserScopedRemoteService) RemoteStatus(ctx context.Context, remoteID string) (*RemoteStatusResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.RemoteStatus(ctx, remoteID)
}

// Fetch delegates guarded fetch through the request-scoped credential.
func (s *UserScopedRemoteService) Fetch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemoteFetchResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.Fetch(ctx, remoteID, expectedBranch, expectedHead, expectedRemoteHead)
}

// Push delegates guarded push through the request-scoped credential.
func (s *UserScopedRemoteService) Push(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemotePushResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.Push(ctx, remoteID, expectedBranch, expectedHead, expectedRemoteHead)
}

// PublishBranch delegates guarded branch publication through the request-scoped credential.
func (s *UserScopedRemoteService) PublishBranch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest string) (*RemoteBranchPublishResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.PublishBranch(ctx, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest)
}

// Clone delegates guarded clone through the request-scoped credential.
func (s *UserScopedRemoteService) Clone(ctx context.Context, remoteID, expectedBranch, expectedRemoteHead string) (*RemoteCloneResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.Clone(ctx, remoteID, expectedBranch, expectedRemoteHead)
}

// CreateDraftPullRequest delegates draft PR creation through the request-scoped credential.
func (s *UserScopedRemoteService) CreateDraftPullRequest(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest, title, body string) (*GitHubPullRequestResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.CreateDraftPullRequest(ctx, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest, title, body)
}

// GetPullRequest delegates hosted PR inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequest(ctx context.Context, remoteID string, number int) (*GitHubPullRequestReadResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.GetPullRequest(ctx, remoteID, number)
}

// ListPullRequests delegates hosted PR inventory through the request-scoped credential.
func (s *UserScopedRemoteService) ListPullRequests(ctx context.Context, remoteID, state, headBranch string, limit int) (*GitHubPullRequestListResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.ListPullRequests(ctx, remoteID, state, headBranch, limit)
}

// GetPullRequestChecks delegates hosted CI/status inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequestChecks(ctx context.Context, remoteID string, number int) (*GitHubPullRequestChecksResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.GetPullRequestChecks(ctx, remoteID, number)
}

// GetPullRequestFeedback delegates hosted feedback inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequestFeedback(ctx context.Context, remoteID string, number int, kind string, page, limit int) (*GitHubPullRequestFeedbackResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.GetPullRequestFeedback(ctx, remoteID, number, kind, page, limit)
}

// GetPullRequestReviewThreads delegates hosted review-thread inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequestReviewThreads(ctx context.Context, remoteID string, number int, after string, limit int) (*GitHubPullRequestReviewThreadsResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.GetPullRequestReviewThreads(ctx, remoteID, number, after, limit)
}

// ReplyToPullRequestReviewComment delegates the guarded hosted reply mutation through the request-scoped credential.
func (s *UserScopedRemoteService) ReplyToPullRequestReviewComment(ctx context.Context, remoteID string, number int, expectedHead string, commentID, expectedReviewID int64, expectedUpdatedAt, body string) (*GitHubPullRequestReviewReplyResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.ReplyToPullRequestReviewComment(ctx, remoteID, number, expectedHead, commentID, expectedReviewID, expectedUpdatedAt, body)
}

// SetPullRequestReviewThreadResolved delegates the guarded thread-state mutation through the request-scoped credential.
func (s *UserScopedRemoteService) SetPullRequestReviewThreadResolved(ctx context.Context, remoteID string, number int, expectedHead, threadID string, expectedResolved, expectedOutdated, resolved bool) (*GitHubPullRequestReviewThreadResolutionResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.SetPullRequestReviewThreadResolved(ctx, remoteID, number, expectedHead, threadID, expectedResolved, expectedOutdated, resolved)
}

// MarkPullRequestReadyForReview delegates the guarded draft-to-ready mutation through the request-scoped credential.
func (s *UserScopedRemoteService) MarkPullRequestReadyForReview(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestReadyResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.MarkPullRequestReadyForReview(ctx, remoteID, number, expectedHead)
}

// GetPullRequestMergeRequirements delegates read-only merge-policy inspection through the request-scoped credential.
func (s *UserScopedRemoteService) GetPullRequestMergeRequirements(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeRequirementsResult, error) {
	scoped, err := s.scoped(ctx, remoteID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return scoped.GetPullRequestMergeRequirements(ctx, remoteID, number)
}
