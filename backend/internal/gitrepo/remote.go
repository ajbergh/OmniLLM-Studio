package gitrepo

import (
	"context"
	"fmt"
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
// configured endpoint URL or credentials.
type RemoteStatusResult struct {
	Remote       string            `json:"remote"`
	Repository   string            `json:"repository"`
	Host         string            `json:"host"`
	Authenticated bool             `json:"authenticated"`
	References   []RemoteReference `json:"references"`
	Truncated    bool              `json:"truncated,omitempty"`
}

// RemoteService owns operator-configured outbound Git endpoints. The transport
// is dedicated to this service so remote status does not alter process-wide HTTP
// or go-git transport behavior.
type RemoteService struct {
	remotes      map[string]RemoteConfig
	ids          []string
	enabled      bool
	pushEnabled  bool
	transport    transport.Transport
	lookupEnv    func(string) (string, bool)
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
	return newRemoteService(filtered, boolEnvironment(RemoteEnabledEnv), boolEnvironment(RemotePushEnabledEnv), newRemoteStatusTransport(), os.LookupEnv)
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
		pushEnabled: pushEnabled, transport: remoteTransport, lookupEnv: lookupEnv,
	}
}

// Configured reports whether at least one valid remote is bound to a configured
// local repository.
func (s *RemoteService) Configured() bool { return s != nil && len(s.ids) > 0 }

// Enabled reports whether the operator allowed outbound Git network access.
func (s *RemoteService) Enabled() bool { return s != nil && s.enabled }

// PushEnabled reports the independent operator push gate. Per-remote AllowPush
// and normal tool approval are additional gates when push is implemented.
func (s *RemoteService) PushEnabled() bool { return s != nil && s.pushEnabled }

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
			AuthenticationConfigured: remote.TokenEnv != "",
			PushAllowed: s.pushEnabled && remote.AllowPush,
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
		Authenticated: authenticated, References: refs, Truncated: truncated,
	}, nil
}
