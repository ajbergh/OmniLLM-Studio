package gitrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	// maxRemoteFetchPackBytes is a hard ceiling on compressed pack data read
	// from a remote for one fetch. Fetch never checks out files, so the normal
	// filesystem storer persists this bounded pack rather than expanding it into
	// a worktree. Clone remains a separate problem because checkout expansion
	// needs its own storage quota.
	maxRemoteFetchPackBytes int64 = 64 << 20
	maxRemoteFetchHaves           = 256
)

var (
	errRemoteFetchDisabled = errors.New("remote Git fetch is disabled")
	errRemoteFetchTooLarge = errors.New("remote Git fetch exceeded the transfer limit")
)

// RemoteFetcher is the side-effecting remote Git contract. Fetch is deliberately
// separate from RemoteReader so remote inspection can remain enabled without
// granting local repository mutation capability.
type RemoteFetcher interface {
	Fetch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemoteFetchResult, error)
}

// RemoteFetchResult describes a guarded fetch. The current local branch and HEAD
// are never moved; only a dedicated OmniLLM remote-tracking reference is updated.
type RemoteFetchResult struct {
	Remote        string `json:"remote"`
	Repository    string `json:"repository"`
	Branch        string `json:"branch"`
	LocalHead     string `json:"local_head"`
	RemoteHead    string `json:"remote_head"`
	TrackingRef   string `json:"tracking_ref"`
	BytesReceived int64  `json:"bytes_received"`
	Downloaded    bool   `json:"downloaded"`
}

// FetchEnabled reports whether outbound remote access and local Git writes are
// both operator-enabled. Fetch mutates the local object database/tracking ref,
// so OMNILLM_GIT_WRITE_ENABLED remains an independent required gate.
func (s *RemoteService) FetchEnabled() bool {
	return s != nil && s.Enabled() && s.local != nil && s.local.WriteEnabled()
}

// Fetch downloads the current local branch's exact previously-reviewed remote
// head into the local object database and updates an isolated tracking ref. It
// never changes HEAD, the current branch, the index, the worktree, Git config,
// tags, submodules, or arbitrary refs.
func (s *RemoteService) Fetch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemoteFetchResult, error) {
	if !s.FetchEnabled() {
		return nil, errRemoteFetchDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if s.transport == nil {
		return nil, fmt.Errorf("remote Git transport is unavailable")
	}
	if !validRemoteHash(expectedRemoteHead) {
		return nil, fmt.Errorf("expected_remote_head must be the branch hash from git_remote_status")
	}

	// Serialize with local branch/stage/commit operations performed through the
	// same configured Service. External Git clients are handled by the repeated
	// branch/HEAD precondition checks below.
	s.local.writeMu.Lock()
	defer s.local.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	repo, err := s.local.open(remote.Repository)
	if err != nil {
		return nil, err
	}
	localHead, err := ensureExpectedHead(repo, expectedHead)
	if err != nil {
		return nil, err
	}
	branch, err := ensureExpectedBranch(repo, expectedBranch)
	if err != nil {
		return nil, err
	}
	_, branchRef, err := cleanBranchName(branch)
	if err != nil {
		return nil, fmt.Errorf("current branch cannot be fetched safely")
	}

	endpoint, err := transport.NewEndpoint(remote.URL)
	if err != nil {
		return nil, fmt.Errorf("remote %q endpoint is invalid", remoteID)
	}
	auth, err := s.remoteAuth(remoteID, remote)
	if err != nil {
		return nil, err
	}
	session, err := s.transport.NewUploadPackSession(endpoint, auth)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be opened", remoteID)
	}
	defer session.Close()

	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected before fetch", remoteID)
	}
	remoteHead, ok := advertised.References[branchRef.String()]
	if !ok {
		return nil, fmt.Errorf("remote %q does not advertise branch %q", remoteID, branch)
	}
	if !strings.EqualFold(remoteHead.String(), strings.TrimSpace(expectedRemoteHead)) {
		return nil, fmt.Errorf("remote branch changed; run git_remote_status again before fetching")
	}
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	if _, err := ensureExpectedBranch(repo, expectedBranch); err != nil {
		return nil, err
	}

	trackingRef := remoteTrackingReference(remoteID, branch)
	result := &RemoteFetchResult{
		Remote: remoteID, Repository: remote.Repository, Branch: branch,
		LocalHead: localHead, RemoteHead: remoteHead.String(), TrackingRef: trackingRef.String(),
	}

	if _, err := repo.CommitObject(remoteHead); err == nil {
		if err := setFetchedTrackingRef(repo, trackingRef, remoteHead, expectedHead, expectedBranch); err != nil {
			return nil, err
		}
		return result, nil
	} else if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, fmt.Errorf("repository %q object database could not be inspected", remote.Repository)
	}

	request := packp.NewUploadPackRequestFromCapabilities(advertised.Capabilities)
	// A sideband stream must be demultiplexed before pack parsing; disabling it
	// keeps the bounded reader directly over raw pack bytes. Thin packs are also
	// disabled so the received pack remains self-contained in object storage.
	request.Capabilities.Delete(capability.Sideband)
	request.Capabilities.Delete(capability.Sideband64k)
	request.Capabilities.Delete(capability.ThinPack)
	request.Wants = []plumbing.Hash{remoteHead}
	request.Haves = localCommitHaves(repo, maxRemoteFetchHaves)

	response, err := session.UploadPack(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("remote %q fetch could not be started", remoteID)
	}
	limited := &remoteFetchLimitReader{reader: response, remaining: maxRemoteFetchPackBytes}
	storageErr := packfile.UpdateObjectStorage(repo.Storer, limited)
	closeErr := response.Close()
	if storageErr != nil {
		if errors.Is(storageErr, errRemoteFetchTooLarge) {
			return nil, fmt.Errorf("remote %q fetch exceeded the %d MiB pack limit", remoteID, maxRemoteFetchPackBytes>>20)
		}
		return nil, fmt.Errorf("remote %q fetch pack could not be stored", remoteID)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("remote %q fetch response could not be closed cleanly", remoteID)
	}
	if _, err := repo.CommitObject(remoteHead); err != nil {
		return nil, fmt.Errorf("remote %q fetch did not produce the expected branch object", remoteID)
	}
	if err := setFetchedTrackingRef(repo, trackingRef, remoteHead, expectedHead, expectedBranch); err != nil {
		return nil, err
	}
	result.BytesReceived = limited.read
	result.Downloaded = true
	return result, nil
}

func (s *RemoteService) remoteAuth(remoteID string, remote RemoteConfig) (transport.AuthMethod, error) {
	if remote.TokenEnv == "" {
		return nil, nil
	}
	token, exists := s.lookupEnv(remote.TokenEnv)
	if !exists || token == "" {
		return nil, fmt.Errorf("remote %q credentials are unavailable", remoteID)
	}
	return &githttp.BasicAuth{Username: remote.Username, Password: token}, nil
}

func setFetchedTrackingRef(repo *git.Repository, trackingRef plumbing.ReferenceName, remoteHead plumbing.Hash, expectedHead, expectedBranch string) error {
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return err
	}
	if _, err := ensureExpectedBranch(repo, expectedBranch); err != nil {
		return err
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(trackingRef, remoteHead)); err != nil {
		return fmt.Errorf("remote tracking reference could not be updated")
	}
	return nil
}

func remoteTrackingReference(remoteID, branch string) plumbing.ReferenceName {
	digest := sha256.Sum256([]byte(remoteID))
	return plumbing.ReferenceName(fmt.Sprintf("refs/remotes/omnillm/%x/%s", digest[:8], branch))
}

func validRemoteHash(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) != 40 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func localCommitHaves(repo *git.Repository, limit int) []plumbing.Hash {
	if repo == nil || limit <= 0 {
		return nil
	}
	haves := make([]plumbing.Hash, 0, limit)
	seen := map[plumbing.Hash]struct{}{}
	add := func(hash plumbing.Hash) {
		if len(haves) >= limit {
			return
		}
		if _, exists := seen[hash]; exists {
			return
		}
		if _, err := repo.CommitObject(hash); err != nil {
			return
		}
		seen[hash] = struct{}{}
		haves = append(haves, hash)
	}
	if head, err := repo.Head(); err == nil {
		add(head.Hash())
	}
	iter, err := repo.References()
	if err != nil {
		return haves
	}
	defer iter.Close()
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference {
			add(ref.Hash())
		}
		return nil
	})
	return haves
}

type remoteFetchLimitReader struct {
	reader    io.Reader
	remaining int64
	read      int64
}

func (r *remoteFetchLimitReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, errRemoteFetchTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	r.read += int64(n)
	return n, err
}
