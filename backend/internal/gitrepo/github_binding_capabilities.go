package gitrepo

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	// GitHubBindingCapabilitiesEnv contains operator-owned authorization policy
	// for binding-derived GitHub remotes. Keys are startup-allowlisted local
	// repository IDs. User-owned repository bindings never populate or modify
	// this policy.
	GitHubBindingCapabilitiesEnv = "OMNILLM_GITHUB_BINDING_CAPABILITIES_JSON"
)

// GitHubBindingCapabilities is the explicit operator authorization overlay for
// one binding-derived GitHub remote. The first policy version intentionally
// excludes default-branch push and clone so connected repositories enter the
// reviewed feature-branch/pull-request workflow rather than gaining broader
// repository mutation authority.
type GitHubBindingCapabilities struct {
	AllowPush                        bool   `json:"allow_push,omitempty"`
	AllowBranchCreate                bool   `json:"allow_branch_create,omitempty"`
	AllowPullRequestRead             bool   `json:"allow_pull_request_read,omitempty"`
	AllowPullRequestCreate           bool   `json:"allow_pull_request_create,omitempty"`
	AllowPullRequestReply            bool   `json:"allow_pull_request_reply,omitempty"`
	AllowPullRequestThreadResolution bool   `json:"allow_pull_request_thread_resolution,omitempty"`
	AllowPullRequestReady            bool   `json:"allow_pull_request_ready,omitempty"`
	AllowPullRequestMerge            bool   `json:"allow_pull_request_merge,omitempty"`
	PullRequestMergeMethod           string `json:"pull_request_merge_method,omitempty"`
}

// ParseGitHubBindingCapabilities parses the operator-owned per-local-repository
// binding policy. Invalid entries are ignored independently so one malformed
// repository policy cannot broaden or weaken another entry. Unknown fields are
// rejected for that entry rather than silently accepted.
func ParseGitHubBindingCapabilities(raw string) map[string]GitHubBindingCapabilities {
	configured := map[string]GitHubBindingCapabilities{}
	if strings.TrimSpace(raw) == "" {
		return configured
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return configured
	}
	for rawID, encoded := range entries {
		id := strings.TrimSpace(rawID)
		if !ValidRepositoryID(id) {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var candidate GitHubBindingCapabilities
		if err := decoder.Decode(&candidate); err != nil {
			continue
		}
		if decoder.More() {
			continue
		}
		normalized, ok := normalizeGitHubBindingCapabilities(candidate)
		if !ok {
			continue
		}
		configured[id] = normalized
	}
	return configured
}

func normalizeGitHubBindingCapabilities(candidate GitHubBindingCapabilities) (GitHubBindingCapabilities, bool) {
	candidate.PullRequestMergeMethod = strings.ToLower(strings.TrimSpace(candidate.PullRequestMergeMethod))
	if candidate.AllowBranchCreate && !candidate.AllowPush {
		return GitHubBindingCapabilities{}, false
	}
	if candidate.AllowPullRequestMerge {
		if !candidate.AllowPullRequestRead || !validGitHubMergeMethod(candidate.PullRequestMergeMethod) {
			return GitHubBindingCapabilities{}, false
		}
	} else if candidate.PullRequestMergeMethod != "" {
		return GitHubBindingCapabilities{}, false
	}
	return candidate, true
}

func applyGitHubBindingCapabilities(remote RemoteConfig, policy GitHubBindingCapabilities) RemoteConfig {
	remote.AllowPush = policy.AllowPush
	remote.AllowBranchCreate = policy.AllowBranchCreate
	remote.AllowPullRequestRead = policy.AllowPullRequestRead
	remote.AllowPullRequestCreate = policy.AllowPullRequestCreate
	remote.AllowPullRequestReply = policy.AllowPullRequestReply
	remote.AllowPullRequestThreadResolution = policy.AllowPullRequestThreadResolution
	remote.AllowPullRequestReady = policy.AllowPullRequestReady
	remote.AllowPullRequestMerge = policy.AllowPullRequestMerge
	remote.PullRequestMergeMethod = policy.PullRequestMergeMethod
	// Binding policy deliberately never grants these broader operations.
	remote.AllowDefaultBranchPush = false
	remote.AllowClone = false
	return remote
}
