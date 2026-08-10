package gitrepo

import (
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// RemotesEnv contains operator-controlled remote definitions as a JSON object
	// keyed by stable remote ID. Remote URLs and credential references are never
	// accepted from model arguments.
	RemotesEnv = "OMNILLM_GIT_REMOTES_JSON"
	// RemoteEnabledEnv is the operator gate for outbound Git network access.
	RemoteEnabledEnv = "OMNILLM_GIT_REMOTE_ENABLED"
	// RemotePushEnabledEnv is an additional operator gate reserved for push.
	// Enabling remote reads does not implicitly enable remote writes.
	RemotePushEnabledEnv = "OMNILLM_GIT_REMOTE_PUSH_ENABLED"
)

var credentialEnvPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// RemoteConfig binds a stable model-facing remote ID to one configured local
// repository and one exact HTTPS endpoint. TokenEnv names an operator-provided
// environment variable; the token value itself is never stored in this struct.
// Push, default-branch push, and clone permissions are independent explicit
// opt-ins layered on top of their process-wide gates.
type RemoteConfig struct {
	Repository             string `json:"repository"`
	URL                    string `json:"url"`
	Username               string `json:"username,omitempty"`
	TokenEnv               string `json:"token_env,omitempty"`
	AllowPush              bool   `json:"allow_push,omitempty"`
	AllowDefaultBranchPush bool   `json:"allow_default_branch_push,omitempty"`
	AllowClone             bool   `json:"allow_clone,omitempty"`
}

// RemoteSummary describes a configured remote without exposing its URL,
// credential-variable name, credential value, or filesystem destination.
type RemoteSummary struct {
	ID                       string `json:"id"`
	Repository               string `json:"repository"`
	Host                     string `json:"host"`
	AuthenticationConfigured bool   `json:"authentication_configured"`
	PushAllowed              bool   `json:"push_allowed"`
	DefaultBranchPushAllowed bool   `json:"default_branch_push_allowed"`
	CloneAllowed             bool   `json:"clone_allowed"`
}

// ParseRemoteConfig parses the operator-controlled JSON remote map. Invalid
// entries are ignored rather than partially normalized into a weaker policy.
func ParseRemoteConfig(raw string) map[string]RemoteConfig {
	configured := map[string]RemoteConfig{}
	if strings.TrimSpace(raw) == "" {
		return configured
	}
	var decoded map[string]RemoteConfig
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return configured
	}
	for id, candidate := range decoded {
		id = strings.TrimSpace(id)
		if !ValidRepositoryID(id) {
			continue
		}
		normalized, ok := normalizeRemoteConfig(candidate)
		if !ok {
			continue
		}
		configured[id] = normalized
	}
	return configured
}

func normalizeRemoteConfig(candidate RemoteConfig) (RemoteConfig, bool) {
	candidate.Repository = strings.TrimSpace(candidate.Repository)
	candidate.URL = strings.TrimSpace(candidate.URL)
	candidate.Username = strings.TrimSpace(candidate.Username)
	candidate.TokenEnv = strings.TrimSpace(candidate.TokenEnv)
	if !ValidRepositoryID(candidate.Repository) || candidate.URL == "" {
		return RemoteConfig{}, false
	}
	parsed, err := url.Parse(candidate.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return RemoteConfig{}, false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" && parsed.Port() != "443" {
		return RemoteConfig{}, false
	}
	if candidate.TokenEnv != "" && !credentialEnvPattern.MatchString(candidate.TokenEnv) {
		return RemoteConfig{}, false
	}
	if candidate.TokenEnv != "" && candidate.Username == "" {
		candidate.Username = "git"
	}
	if candidate.AllowDefaultBranchPush && !candidate.AllowPush {
		return RemoteConfig{}, false
	}
	return candidate, true
}

func boolEnvironment(name string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return err == nil && value
}

func sortedRemoteIDs(configured map[string]RemoteConfig) []string {
	ids := make([]string, 0, len(configured))
	for id := range configured {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
