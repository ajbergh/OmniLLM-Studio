package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/revlist"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var errRemoteBranchCreateDisabled = errors.New("remote Git branch creation is disabled")

// RemoteBranchPublisher is the remote branch-creation contract. Publication is
// deliberately separate from RemotePusher so creating a new remote ref requires
// its own operator and per-remote opt-ins.
type RemoteBranchPublisher interface {
	PublishBranch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest string) (*RemoteBranchPublishResult, error)
}

// RemoteBranchPublishResult describes one guarded same-name remote branch
// creation. The result never includes the configured endpoint or credentials.
type RemoteBranchPublishResult struct {
	Remote      string `json:"remote"`
	Repository  string `json:"repository"`
	Branch      string `json:"branch"`
	RemoteHead  string `json:"remote_head"`
	ObjectsSent int    `json:"objects_sent"`
	BytesSent   int64  `json:"bytes_sent"`
	Published   bool   `json:"published"`
}

// PublishBranch creates the same-named remote branch at the exact reviewed local
// HEAD. The remote branch must have been absent in the branch-state snapshot
// returned by git_remote_status and must still be absent immediately before the
// receive-pack request. The command uses a zero old object ID, so a concurrent
// branch creation is rejected by the server instead of overwritten.
func (s *RemoteService) PublishBranch(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest string) (*RemoteBranchPublishResult, error) {
	if !s.BranchCreateMutationEnabled() {
		return nil, errRemoteBranchCreateDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if !remote.AllowPush {
		return nil, fmt.Errorf("remote %q does not allow push", remoteID)
	}
	if !remote.AllowBranchCreate {
		return nil, fmt.Errorf("remote %q does not allow branch creation", remoteID)
	}
	if s.transport == nil {
		return nil, fmt.Errorf("remote Git transport is unavailable")
	}
	if !validRemoteStateDigest(expectedRemoteStateDigest) {
		return nil, fmt.Errorf("expected_remote_state_digest must be the branch-state digest from git_remote_status")
	}

	s.local.writeMu.Lock()
	defer s.local.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	repo, err := s.local.open(remote.Repository)
	if err != nil {
		return nil, err
	}
	localHeadString, err := ensureExpectedHead(repo, expectedHead)
	if err != nil {
		return nil, err
	}
	branch, err := ensureExpectedBranch(repo, expectedBranch)
	if err != nil {
		return nil, err
	}
	_, branchRef, err := cleanBranchName(branch)
	if err != nil {
		return nil, fmt.Errorf("current branch cannot be published safely")
	}
	localHead := plumbing.NewHash(localHeadString)
	if _, err := repo.CommitObject(localHead); err != nil {
		return nil, fmt.Errorf("local HEAD commit could not be read")
	}

	endpoint, err := transport.NewEndpoint(remote.URL)
	if err != nil {
		return nil, fmt.Errorf("remote %q endpoint is invalid", remoteID)
	}
	auth, err := s.remoteAuth(remoteID, remote)
	if err != nil {
		return nil, err
	}
	session, err := s.transport.NewReceivePackSession(endpoint, auth)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be opened for branch publication", remoteID)
	}
	defer session.Close()

	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected before branch publication", remoteID)
	}
	if remoteBranchStateDigest(advertised) != strings.ToLower(strings.TrimSpace(expectedRemoteStateDigest)) {
		return nil, fmt.Errorf("remote branch state changed; run git_remote_status again before publishing")
	}
	if _, exists := advertised.References[branchRef.String()]; exists {
		return nil, fmt.Errorf("remote %q already advertises branch %q; use git_fetch and git_push for existing branches", remoteID, branch)
	}
	if isProtectedRemoteBranch(advertised, branch) {
		return nil, fmt.Errorf("remote branch %q is protected from creation by guarded publication", branch)
	}

	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	if _, err := ensureExpectedBranch(repo, expectedBranch); err != nil {
		return nil, err
	}

	request := packp.NewReferenceUpdateRequestFromCapabilities(advertised.Capabilities)
	request.Commands = []*packp.Command{{Name: branchRef, Old: plumbing.ZeroHash, New: localHead}}

	haves := advertisedHashes(advertised)
	hashesToPush, err := revlist.Objects(repo.Storer, []plumbing.Hash{localHead}, haves)
	if err != nil {
		return nil, fmt.Errorf("objects required for branch publication could not be enumerated")
	}
	if len(hashesToPush) > maxRemotePushObjects {
		return nil, fmt.Errorf("branch publication requires %d objects, exceeding the guarded limit of %d", len(hashesToPush), maxRemotePushObjects)
	}

	bytesSent, report, err := pushRemoteHashes(ctx, session, repo, request, hashesToPush, !advertised.Capabilities.Supports(capability.OFSDelta))
	if err != nil {
		if errors.Is(err, errRemotePushTooLarge) {
			return nil, fmt.Errorf("remote %q branch publication exceeded the %d MiB pack limit", remoteID, maxRemotePushPackBytes>>20)
		}
		return nil, fmt.Errorf("remote %q branch publication failed", remoteID)
	}
	if report != nil {
		if err := report.Error(); err != nil {
			return nil, fmt.Errorf("remote %q rejected the guarded branch creation", remoteID)
		}
	}

	trackingRef := remoteTrackingReference(remoteID, branch)
	if err := repo.Storer.SetReference(plumbing.NewHashReference(trackingRef, localHead)); err != nil {
		return nil, fmt.Errorf("remote branch publication succeeded but local tracking state could not be updated; run git_remote_status before another remote mutation")
	}
	return &RemoteBranchPublishResult{
		Remote: remoteID, Repository: remote.Repository, Branch: branch,
		RemoteHead: localHead.String(), ObjectsSent: len(hashesToPush), BytesSent: bytesSent, Published: true,
	}, nil
}
