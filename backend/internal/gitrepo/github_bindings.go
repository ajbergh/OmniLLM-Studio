package gitrepo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
)

const githubBindingRemotePrefix = "github-"

// GitHubRemoteBinding is a secret-free, owner-scoped association between one
// startup-allowlisted local repository and one validated GitHub repository.
// Implementations must not place local filesystem paths, tokens, or arbitrary
// remote URLs in this structure.
type GitHubRemoteBinding struct {
	LocalRepositoryID string
	GitHubFullName    string
	Disabled          bool
}

// GitHubRemoteBindingResolver returns only bindings active for the invocation
// owner. It must be local/non-network so git_remotes cannot trigger token refresh
// or provider access.
type GitHubRemoteBindingResolver interface {
	GitHubRemoteBindings(ctx context.Context) ([]GitHubRemoteBinding, error)
}

// GitHubRemoteBindingResolverFunc adapts one local binding lookup function.
type GitHubRemoteBindingResolverFunc func(context.Context) ([]GitHubRemoteBinding, error)

func (f GitHubRemoteBindingResolverFunc) GitHubRemoteBindings(ctx context.Context) ([]GitHubRemoteBinding, error) {
	if f == nil {
		return nil, nil
	}
	return f(ctx)
}

// GitHubBindingRemoteID deterministically maps a local repository ID to a safe
// model-facing remote ID. Long repository IDs are truncated with a stable hash
// suffix so the result remains within the existing repository-ID bound.
func GitHubBindingRemoteID(localRepositoryID string) string {
	localRepositoryID = strings.TrimSpace(localRepositoryID)
	if !ValidRepositoryID(localRepositoryID) {
		return ""
	}
	candidate := githubBindingRemotePrefix + localRepositoryID
	if ValidRepositoryID(candidate) {
		return candidate
	}
	digest := sha256.Sum256([]byte(localRepositoryID))
	suffix := fmt.Sprintf("-%x", digest[:4])
	maxLocal := maxRepositoryIDBytes - len(githubBindingRemotePrefix) - len(suffix)
	if maxLocal <= 0 {
		return ""
	}
	trimmed := localRepositoryID
	if len(trimmed) > maxLocal {
		trimmed = trimmed[:maxLocal]
	}
	candidate = githubBindingRemotePrefix + trimmed + suffix
	if !ValidRepositoryID(candidate) {
		return ""
	}
	return candidate
}

func githubRemoteConfigFromBinding(binding GitHubRemoteBinding) (string, RemoteConfig, bool) {
	localRepositoryID := strings.TrimSpace(binding.LocalRepositoryID)
	if binding.Disabled || !ValidRepositoryID(localRepositoryID) {
		return "", RemoteConfig{}, false
	}
	owner, repository, ok := githubRepositoryFromFullName(binding.GitHubFullName)
	if !ok {
		return "", RemoteConfig{}, false
	}
	remoteID := GitHubBindingRemoteID(localRepositoryID)
	if remoteID == "" {
		return "", RemoteConfig{}, false
	}
	return remoteID, RemoteConfig{
		Repository: localRepositoryID,
		URL:        "https://github.com/" + owner + "/" + repository + ".git",
		Username:   "x-access-token",
	}, true
}

func githubRepositoryFromFullName(fullName string) (string, string, bool) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" || strings.Count(fullName, "/") != 1 {
		return "", "", false
	}
	parts := strings.SplitN(fullName, "/", 2)
	owner := strings.TrimSpace(parts[0])
	repository := strings.TrimSpace(parts[1])
	if !githubOwnerPattern.MatchString(owner) || !githubRepositoryPattern.MatchString(repository) {
		return "", "", false
	}
	return owner, repository, true
}

func (s *UserScopedRemoteService) serviceWithBindings(ctx context.Context) (*RemoteService, error) {
	if s == nil || s.RemoteService == nil || s.githubBindings == nil {
		if s == nil {
			return nil, nil
		}
		return s.RemoteService, nil
	}
	bindings, err := s.githubBindings.GitHubRemoteBindings(ctx)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return s.RemoteService, nil
	}
	clone := *s.RemoteService
	clone.remotes = cloneRemoteConfigs(s.remotes)
	changed := false
	for _, binding := range bindings {
		remoteID, remote, ok := githubRemoteConfigFromBinding(binding)
		if !ok || s.local == nil || !s.local.HasRepository(remote.Repository) {
			continue
		}
		// Static operator configuration always wins a deterministic ID collision.
		if _, exists := clone.remotes[remoteID]; exists {
			continue
		}
		if policy, configured := s.githubBindingCapabilities[remote.Repository]; configured {
			remote = applyGitHubBindingCapabilities(remote, policy)
		}
		clone.remotes[remoteID] = remote
		changed = true
	}
	if !changed {
		return s.RemoteService, nil
	}
	clone.ids = sortedRemoteIDs(clone.remotes)
	return &clone, nil
}
