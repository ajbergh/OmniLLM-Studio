package gitrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const maxRemoteStatusRefs = 200

var errRemoteDisabled = fmt.Errorf("remote Git access is disabled")

// RemoteReader is the read-only network boundary for configured Git remotes.
// Implementations resolve stable remote IDs internally; callers never supply a
// URL or credential.
type RemoteReader interface {
	Remotes(ctx context.Context) []RemoteSummary
	RemoteStatus(ctx context.Context, remoteID string) (*RemoteStatusResult, error)
}

// RemoteReference is a bounded advertised branch reference.
type RemoteReference struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// RemoteStatusResult reports advertised branch heads without exposing the
// configured endpoint URL or credentials. BranchStateDigest fingerprints the
// complete branch namespace, including refs omitted from the bounded display.
type RemoteStatusResult struct {
	Remote            string            `json:"remote"`
	Repository        string            `json:"repository"`
	Host              string            `json:"host"`
	Authenticated     bool              `json:"authenticated"`
	References        []RemoteReference `json:"references"`
	BranchStateDigest string            `json:"branch_state_digest"`
	Truncated         bool              `json:"truncated,omitempty"`
}

// RemoteService owns operator-configured outbound Git endpoints. The Git
// transport and GitHub API client are dedicated to this service so remote
// operations do not alter process-wide HTTP or go-git behavior. local is the
// same configured Service used by local Git tools so reviewed local state can be
// bound to network mutations.
type RemoteService struct {
	remotes                                  map[string]RemoteConfig
	ids                                      []string
	enabled                                  bool
	pushEnabled                              bool
	branchCreateEnabled                      bool
	githubPullRequestEnabled                 bool
	githubPullRequestReadEnabled             bool
	githubPullRequestReplyEnabled            bool
	githubPullRequestThreadResolutionEnabled bool
	cloneEnabled                             bool
	cloneMaxBytes                            int64
	cloneMaxEntries                          int64
	transport                                transport.Transport
	githubClient                             *http.Client
	lookupEnv                                func(string) (string, bool)
	local                                    *Service
}

// NewRemoteServiceFromEnvironment constructs the remote service from operator
// configuration. Entries referring to an unconfigured local repository are
// discarded, preventing a remote definition from creating a filesystem target.
func NewRemoteServiceFromEnvironment(local *Service) *RemoteService {
	configured := ParseRemoteConfig(os.Getenv(RemotesEnv))
	filtered := make(map[string]RemoteConfig, len(configured))
	if local != nil {
		for id, remote := range configured {
			if _, ok := local.repositories[remote.Repository]; ok {
				filtered[id] = remote
			}
		}
	}
	service := newRemoteService(filtered, boolEnvironment(RemoteEnabledEnv), boolEnvironment(RemotePushEnabledEnv), newRemoteStatusTransport(), os.LookupEnv)
	service.local = local
	service.branchCreateEnabled = boolEnvironment(RemoteBranchCreateEnabledEnv)
	service.githubPullRequestEnabled = boolEnvironment(GitHubPullRequestEnabledEnv)
	service.githubPullRequestReadEnabled = boolEnvironment(GitHubPullRequestReadEnabledEnv)
	service.githubPullRequestReplyEnabled = boolEnvironment(GitHubPullRequestReplyEnabledEnv)
	service.githubPullRequestThreadResolutionEnabled = boolEnvironment(GitHubPullRequestThreadResolutionEnabledEnv)
	if maxBytes, maxEntries, ok := cloneLimitsFromEnvironment(); ok {
		service.cloneMaxBytes = maxBytes
		service.cloneMaxEntries = maxEntries
	}
	service.cloneEnabled = boolEnvironment(RemoteCloneEnabledEnv)
	return service
}

func newRemoteService(configured map[string]RemoteConfig, enabled, pushEnabled bool, remoteTransport transport.Transport, lookupEnv func(string) (string, bool)) *RemoteService {
	if configured == nil {
		configured = map[string]RemoteConfig{}
	}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	return &RemoteService{
		remotes: configured, ids: sortedRemoteIDs(configured), enabled: enabled,
		pushEnabled: pushEnabled, transport: remoteTransport, githubClient: newGitHubAPIClient(), lookupEnv: lookupEnv,
	}
}

// Configured reports whether at least one valid remote is bound to a configured
// local repository ID.
func (s *RemoteService) Configured() bool { return s != nil && len(s.ids) > 0 }

// Enabled reports whether the operator allowed outbound Git network access.
func (s *RemoteService) Enabled() bool { return s != nil && s.enabled }

// PushEnabled reports the independent process-wide remote push gate. Local Git
// write access, per-remote allow_push, default-branch policy, and tool approval
// are additional independent gates.
func (s *RemoteService) PushEnabled() bool { return s != nil && s.pushEnabled }

// BranchCreateEnabled reports the separate process-wide remote ref-creation gate.
func (s *RemoteService) BranchCreateEnabled() bool { return s != nil && s.branchCreateEnabled }

// GitHubPullRequestEnabled reports the separate process-wide gate for creating
// GitHub draft pull requests. It does not imply Git push or branch creation.
func (s *RemoteService) GitHubPullRequestEnabled() bool {
	return s != nil && s.githubPullRequestEnabled
}

// GitHubPullRequestReadEnabled reports the independent process-wide gate for
// read-only pull request, CI/check, hosted feedback, and review-thread inspection.
func (s *RemoteService) GitHubPullRequestReadEnabled() bool {
	return s != nil && s.githubPullRequestReadEnabled
}

// GitHubPullRequestReplyEnabled reports the independent process-wide gate for
// posting replies to existing top-level inline review comments.
func (s *RemoteService) GitHubPullRequestReplyEnabled() bool {
	return s != nil && s.githubPullRequestReplyEnabled
}

// GitHubPullRequestThreadResolutionEnabled reports the independent process-wide
// gate for changing an existing review thread's resolved state.
func (s *RemoteService) GitHubPullRequestThreadResolutionEnabled() bool {
	return s != nil && s.githubPullRequestThreadResolutionEnabled
}

// PushMutationEnabled reports whether the process has enabled all global gates
// required for a remote push mutation. Per-remote policy is checked by Push.
func (s *RemoteService) PushMutationEnabled() bool {
	return s != nil && s.Enabled() && s.PushEnabled() && s.local != nil && s.local.WriteEnabled()
}

// BranchCreateMutationEnabled reports whether all global prerequisites for
// creating a new remote branch are enabled. Per-remote allow_branch_create is
// checked by PublishBranch.
func (s *RemoteService) BranchCreateMutationEnabled() bool {
	return s != nil && s.PushMutationEnabled() && s.BranchCreateEnabled()
}

// GitHubPullRequestReadAccessEnabled reports whether hosted pull request reads
// are globally enabled. Per-remote policy, github.com identity, and credential
// presence are checked by each read operation.
func (s *RemoteService) GitHubPullRequestReadAccessEnabled() bool {
	return s != nil && s.Enabled() && s.GitHubPullRequestReadEnabled()
}

// GitHubPullRequestMutationEnabled reports whether the global prerequisites for
// a GitHub draft-PR mutation are enabled. Unlike Git writes it does not require
// local write access; the local service is used only to bind exact branch/HEAD.
func (s *RemoteService) GitHubPullRequestMutationEnabled() bool {
	return s != nil && s.Enabled() && s.GitHubPullRequestEnabled() && s.local != nil
}

// GitHubPullRequestReplyMutationEnabled reports whether the process permits the
// hosted communication mutation. It is intentionally independent from Git push,
// local Git writes, draft-PR creation, and read-only PR inspection.
func (s *RemoteService) GitHubPullRequestReplyMutationEnabled() bool {
	return s != nil && s.Enabled() && s.GitHubPullRequestReplyEnabled()
}

// GitHubPullRequestThreadResolutionMutationEnabled reports whether the process
// permits review-thread state mutation. It is independent from PR reads, reply
// permission, draft creation, local Git writes, and Git push.
func (s *RemoteService) GitHubPullRequestThreadResolutionMutationEnabled() bool {
	return s != nil && s.Enabled() && s.GitHubPullRequestThreadResolutionEnabled()
}

// CloneMutationEnabled reports whether all process-wide clone prerequisites are
// present. Per-remote allow_clone and destination state are checked by Clone.
func (s *RemoteService) CloneMutationEnabled() bool {
	return s != nil && s.Enabled() && s.cloneEnabled && s.cloneMaxBytes >= minRemoteCloneBytes &&
		s.cloneMaxEntries >= minRemoteCloneEntries && s.local != nil && s.local.WriteEnabled()
}

// Remotes returns safe summaries only. Endpoint paths and credential references
// remain operator-only configuration.
func (s *RemoteService) Remotes(ctx context.Context) []RemoteSummary {
	if s == nil || !s.Enabled() {
		return nil
	}
	out := make([]RemoteSummary, 0, len(s.ids))
	for _, id := range s.ids {
		if err := ctx.Err(); err != nil {
			break
		}
		remote := s.remotes[id]
		parsed, _ := url.Parse(remote.URL)
		out = append(out, RemoteSummary{
			ID: id, Repository: remote.Repository, Host: parsed.Hostname(),
			AuthenticationConfigured:           remote.TokenEnv != "",
			PushAllowed:                        s.PushMutationEnabled() && remote.AllowPush,
			BranchCreateAllowed:                s.BranchCreateMutationEnabled() && remote.AllowPush && remote.AllowBranchCreate,
			PullRequestReadAllowed:             s.GitHubPullRequestReadAccessEnabled() && remoteSupportsGitHubPullRequestRead(remote),
			PullRequestCreateAllowed:           s.GitHubPullRequestMutationEnabled() && remoteSupportsGitHubPullRequests(remote),
			PullRequestReplyAllowed:            s.GitHubPullRequestReplyMutationEnabled() && remoteSupportsGitHubPullRequestReply(remote),
			PullRequestThreadResolutionAllowed: s.GitHubPullRequestThreadResolutionMutationEnabled() && remoteSupportsGitHubPullRequestThreadResolution(remote),
			DefaultBranchPushAllowed:           s.PushMutationEnabled() && remote.AllowPush && remote.AllowDefaultBranchPush,
			CloneAllowed:                       s.CloneMutationEnabled() && remote.AllowClone,
		})
	}
	return out
}

// RemoteStatus contacts one exact configured HTTPS endpoint and returns a
// bounded set of advertised branch heads. The remote URL and secret are loaded
// internally and never appear in caller arguments or returned errors.
func (s *RemoteService) RemoteStatus(ctx context.Context, remoteID string) (*RemoteStatusResult, error) {
	if s == nil || !s.Enabled() {
		return nil, errRemoteDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if s.transport == nil {
		return nil, fmt.Errorf("remote Git transport is unavailable")
	}
	endpoint, err := transport.NewEndpoint(remote.URL)
	if err != nil {
		return nil, fmt.Errorf("remote %q endpoint is invalid", remoteID)
	}

	var auth transport.AuthMethod
	authenticated := false
	if remote.TokenEnv != "" {
		token, exists := s.lookupEnv(remote.TokenEnv)
		if !exists || token == "" {
			return nil, fmt.Errorf("remote %q credentials are unavailable", remoteID)
		}
		auth = &githttp.BasicAuth{Username: remote.Username, Password: token}
		authenticated = true
	}

	session, err := s.transport.NewUploadPackSession(endpoint, auth)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be opened", remoteID)
	}
	defer session.Close()
	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected", remoteID)
	}
	branchStateDigest := remoteBranchStateDigest(advertised)
	allRefs, err := advertised.AllReferences()
	if err != nil {
		return nil, fmt.Errorf("remote %q references could not be read", remoteID)
	}
	iterator, err := allRefs.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("remote %q references could not be iterated", remoteID)
	}
	defer iterator.Close()

	refs := make([]RemoteReference, 0, 32)
	truncated := false
	err = iterator.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference || !strings.HasPrefix(ref.Name().String(), "refs/heads/") {
			return nil
		}
		if len(refs) >= maxRemoteStatusRefs {
			truncated = true
			return nil
		}
		refs = append(refs, RemoteReference{Name: ref.Name().Short(), Hash: ref.Hash().String()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("remote %q references could not be read", remoteID)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	parsed, _ := url.Parse(remote.URL)
	return &RemoteStatusResult{
		Remote: remoteID, Repository: remote.Repository, Host: parsed.Hostname(),
		Authenticated: authenticated, References: refs, BranchStateDigest: branchStateDigest, Truncated: truncated,
	}, nil
}
