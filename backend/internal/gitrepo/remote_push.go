package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/revlist"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	maxRemotePushPackBytes int64 = 64 << 20
	maxRemotePushObjects         = 100_000
)

var (
	errRemotePushDisabled = errors.New("remote Git push is disabled")
	errRemotePushTooLarge = errors.New("remote Git push exceeded the transfer limit")
)

// RemotePusher is the remote-side mutation contract. Implementations must bind
// the push to operator-configured endpoints and reviewed local/remote state.
type RemotePusher interface {
	Push(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemotePushResult, error)
}

// RemotePushResult describes one guarded fast-forward branch push.
type RemotePushResult struct {
	Remote             string `json:"remote"`
	Repository         string `json:"repository"`
	Branch             string `json:"branch"`
	PreviousRemoteHead string `json:"previous_remote_head"`
	RemoteHead         string `json:"remote_head"`
	ObjectsSent        int    `json:"objects_sent"`
	BytesSent          int64  `json:"bytes_sent"`
	Updated            bool   `json:"updated"`
}

// Push updates the same-named remote branch to the exact reviewed local HEAD.
// It never creates/deletes refs, pushes tags, forces, changes Git config, or
// accepts a caller-provided refspec/URL. The remote branch's advertised old hash
// must equal expectedRemoteHead and is also sent as the receive-pack command's
// old object ID, giving the server a compare-and-swap precondition.
func (s *RemoteService) Push(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteHead string) (*RemotePushResult, error) {
	if !s.PushMutationEnabled() {
		return nil, errRemotePushDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if !remote.AllowPush {
		return nil, fmt.Errorf("remote %q does not allow push", remoteID)
	}
	if s.transport == nil {
		return nil, fmt.Errorf("remote Git transport is unavailable")
	}
	if !validRemoteHash(expectedRemoteHead) {
		return nil, fmt.Errorf("expected_remote_head must be the branch hash from git_remote_status")
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
		return nil, fmt.Errorf("current branch cannot be pushed safely")
	}
	localHead := plumbing.NewHash(localHeadString)
	expectedRemoteHash := plumbing.NewHash(strings.TrimSpace(expectedRemoteHead))

	trackingRef := remoteTrackingReference(remoteID, branch)
	tracking, err := repo.Reference(trackingRef, true)
	if err != nil || tracking.Type() != plumbing.HashReference || tracking.Hash() != expectedRemoteHash {
		return nil, fmt.Errorf("remote branch has not been fetched at the reviewed head; run git_remote_status and git_fetch before pushing")
	}
	remoteCommit, err := repo.CommitObject(expectedRemoteHash)
	if err != nil {
		return nil, fmt.Errorf("reviewed remote head is not available locally; run git_fetch before pushing")
	}
	localCommit, err := repo.CommitObject(localHead)
	if err != nil {
		return nil, fmt.Errorf("local HEAD commit could not be read")
	}
	if localHead != expectedRemoteHash {
		ancestor, err := remoteCommit.IsAncestor(localCommit)
		if err != nil {
			return nil, fmt.Errorf("fast-forward relationship could not be verified")
		}
		if !ancestor {
			return nil, fmt.Errorf("push would not be a fast-forward; fetch/reconcile the remote branch before pushing")
		}
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
		return nil, fmt.Errorf("remote %q could not be opened for push", remoteID)
	}
	defer session.Close()

	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected before push", remoteID)
	}
	remoteHead, exists := advertised.References[branchRef.String()]
	if !exists {
		return nil, fmt.Errorf("remote %q does not advertise existing branch %q; guarded push does not create branches", remoteID, branch)
	}
	if remoteHead != expectedRemoteHash {
		return nil, fmt.Errorf("remote branch changed; run git_remote_status and git_fetch again before pushing")
	}
	if !remote.AllowDefaultBranchPush && isProtectedRemoteBranch(advertised, branch) {
		return nil, fmt.Errorf("remote %q default branch %q is protected; operator opt-in is required for direct default-branch pushes", remoteID, branch)
	}

	result := &RemotePushResult{
		Remote: remoteID, Repository: remote.Repository, Branch: branch,
		PreviousRemoteHead: remoteHead.String(), RemoteHead: localHead.String(),
	}
	if localHead == remoteHead {
		return result, nil
	}

	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	if _, err := ensureExpectedBranch(repo, expectedBranch); err != nil {
		return nil, err
	}
	freshTracking, err := repo.Reference(trackingRef, true)
	if err != nil || freshTracking.Hash() != expectedRemoteHash {
		return nil, fmt.Errorf("fetched remote state changed locally; run git_remote_status and git_fetch again before pushing")
	}

	request := packp.NewReferenceUpdateRequestFromCapabilities(advertised.Capabilities)
	request.Commands = []*packp.Command{{Name: branchRef, Old: remoteHead, New: localHead}}

	haves := advertisedHashes(advertised)
	hashesToPush, err := revlist.Objects(repo.Storer, []plumbing.Hash{localHead}, haves)
	if err != nil {
		return nil, fmt.Errorf("objects required for push could not be enumerated")
	}
	if len(hashesToPush) > maxRemotePushObjects {
		return nil, fmt.Errorf("push requires %d objects, exceeding the guarded limit of %d", len(hashesToPush), maxRemotePushObjects)
	}

	bytesSent, report, err := pushRemoteHashes(ctx, session, repo, request, hashesToPush, !advertised.Capabilities.Supports(capability.OFSDelta))
	if err != nil {
		if errors.Is(err, errRemotePushTooLarge) {
			return nil, fmt.Errorf("remote %q push exceeded the %d MiB pack limit", remoteID, maxRemotePushPackBytes>>20)
		}
		return nil, fmt.Errorf("remote %q push failed", remoteID)
	}
	if report != nil {
		if err := report.Error(); err != nil {
			return nil, fmt.Errorf("remote %q rejected the guarded branch update", remoteID)
		}
	}

	// The server accepted the command whose Old field was the reviewed remote
	// hash. Updating the isolated tracking ref records that known remote state.
	if err := repo.Storer.SetReference(plumbing.NewHashReference(trackingRef, localHead)); err != nil {
		return nil, fmt.Errorf("remote push succeeded but local tracking state could not be updated; run git_remote_status before another remote mutation")
	}
	result.ObjectsSent = len(hashesToPush)
	result.BytesSent = bytesSent
	result.Updated = true
	return result, nil
}

func isProtectedRemoteBranch(advertised *packp.AdvRefs, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" || advertised == nil || advertised.Capabilities == nil {
		return branch == "main" || branch == "master"
	}
	for _, value := range advertised.Capabilities.Get(capability.SymRef) {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 && parts[0] == "HEAD" && parts[1] == plumbing.NewBranchReferenceName(branch).String() {
			return true
		}
	}
	// Conservative fallback for servers that do not advertise HEAD symref.
	return branch == "main" || branch == "master"
}

func advertisedHashes(advertised *packp.AdvRefs) []plumbing.Hash {
	if advertised == nil {
		return nil
	}
	seen := make(map[plumbing.Hash]struct{}, len(advertised.References))
	hashes := make([]plumbing.Hash, 0, len(advertised.References))
	for _, hash := range advertised.References {
		if hash == plumbing.ZeroHash {
			continue
		}
		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}
		hashes = append(hashes, hash)
	}
	return hashes
}

func pushRemoteHashes(ctx context.Context, session transport.ReceivePackSession, repo *git.Repository, request *packp.ReferenceUpdateRequest, hashes []plumbing.Hash, useRefDeltas bool) (int64, *packp.ReportStatus, error) {
	reader, writer := io.Pipe()
	config, err := repo.Storer.Config()
	if err != nil {
		return 0, nil, err
	}
	request.Packfile = reader
	limited := &remotePushLimitWriter{writer: writer, remaining: maxRemotePushPackBytes}
	done := make(chan error, 1)
	go func() {
		encoder := packfile.NewEncoder(limited, repo.Storer, useRefDeltas)
		if _, encodeErr := encoder.Encode(hashes, config.Pack.Window); encodeErr != nil {
			done <- writer.CloseWithError(encodeErr)
			return
		}
		done <- writer.Close()
	}()

	report, receiveErr := session.ReceivePack(ctx, request)
	if receiveErr != nil {
		_ = reader.Close()
		return limited.written, nil, receiveErr
	}
	if encodeErr := <-done; encodeErr != nil {
		return limited.written, report, encodeErr
	}
	return limited.written, report, nil
}

type remotePushLimitWriter struct {
	writer    io.Writer
	remaining int64
	written   int64
}

func (w *remotePushLimitWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errRemotePushTooLarge
	}
	if int64(len(p)) > w.remaining {
		p = p[:w.remaining]
		n, err := w.writer.Write(p)
		w.remaining -= int64(n)
		w.written += int64(n)
		if err != nil {
			return n, err
		}
		return n, errRemotePushTooLarge
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	w.written += int64(n)
	return n, err
}
