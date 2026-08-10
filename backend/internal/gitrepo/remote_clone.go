package gitrepo

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const (
	maxRemoteCloneObjects      = 200_000
	cloneLargeObjectThreshold = 4 << 20
)

var errRemoteCloneDisabled = errors.New("remote Git clone is disabled")

// RemoteCloner is the remote repository creation contract. The selected remote
// determines both the endpoint and the already operator-configured destination
// repository ID; callers never provide a filesystem path.
type RemoteCloner interface {
	Clone(ctx context.Context, remoteID, expectedBranch, expectedRemoteHead string) (*RemoteCloneResult, error)
}

// RemoteCloneResult describes a successfully promoted clone without revealing
// its configured filesystem destination or remote URL.
type RemoteCloneResult struct {
	Remote            string `json:"remote"`
	Repository        string `json:"repository"`
	Branch            string `json:"branch"`
	Head              string `json:"head"`
	BytesReceived     int64  `json:"bytes_received"`
	StorageBytesUsed  int64  `json:"storage_bytes_used"`
	EntriesCreated    int64  `json:"entries_created"`
	StorageByteLimit  int64  `json:"storage_byte_limit"`
	StorageEntryLimit int64  `json:"storage_entry_limit"`
}

// Clone creates one non-bare repository at the selected remote's preconfigured
// local repository path. The destination must not exist. All Git data and
// checkout writes occur first in a private sibling temporary directory under a
// shared quota, then the completed repository is atomically renamed into place.
func (s *RemoteService) Clone(ctx context.Context, remoteID, expectedBranch, expectedRemoteHead string) (*RemoteCloneResult, error) {
	if !s.CloneMutationEnabled() {
		return nil, errRemoteCloneDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if !remote.AllowClone {
		return nil, fmt.Errorf("remote %q does not allow clone", remoteID)
	}
	if s.transport == nil {
		return nil, fmt.Errorf("remote Git transport is unavailable")
	}
	if !validRemoteHash(expectedRemoteHead) {
		return nil, fmt.Errorf("expected_remote_head must be the branch hash from git_remote_status")
	}
	branch, branchRef, err := cleanBranchName(expectedBranch)
	if err != nil {
		return nil, fmt.Errorf("expected_branch must be a branch advertised by git_remote_status")
	}

	// Serialize destination creation with all local Git mutations made through
	// this service. The configured destination path itself remains immutable.
	s.local.writeMu.Lock()
	defer s.local.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	destination, err := s.cloneDestination(remote.Repository)
	if err != nil {
		return nil, err
	}
	parent := filepath.Dir(destination)
	temporary, err := os.MkdirTemp(parent, ".omnillm-clone-")
	if err != nil {
		return nil, safeRepositoryError(remote.Repository, "temporary clone workspace could not be created")
	}
	if err := os.Chmod(temporary, 0o700); err != nil {
		_ = os.RemoveAll(temporary)
		return nil, safeRepositoryError(remote.Repository, "temporary clone workspace could not be secured")
	}
	promoted := false
	defer func() {
		if !promoted {
			_ = os.RemoveAll(temporary)
		}
	}()

	// BoundOS uses secure path joining for every filesystem operation. This is
	// stricter than Billy's legacy soft-chroot default and ensures later symlink
	// creation cannot redirect clone writes outside the private staging root.
	worktreeFS := newCloneQuotaFilesystem(osfs.New(temporary, osfs.WithBoundOS()), s.cloneMaxBytes, s.cloneMaxEntries)
	if err := worktreeFS.MkdirAll(git.GitDirName, 0o700); err != nil {
		return nil, s.safeCloneCheckoutError(remote.Repository, err)
	}
	dotGitFS, err := worktreeFS.Chroot(git.GitDirName)
	if err != nil {
		return nil, safeRepositoryError(remote.Repository, "temporary Git storage could not be initialized")
	}
	storage := filesystem.NewStorageWithOptions(dotGitFS, cache.NewObjectLRUDefault(), filesystem.Options{
		ExclusiveAccess:      true,
		LargeObjectThreshold: cloneLargeObjectThreshold,
	})
	repo, err := git.InitWithOptions(storage, worktreeFS, git.InitOptions{DefaultBranch: branchRef})
	if err != nil {
		return nil, safeRepositoryError(remote.Repository, "temporary Git repository could not be initialized")
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
		return nil, fmt.Errorf("remote %q could not be opened for clone", remoteID)
	}
	defer session.Close()

	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected before clone", remoteID)
	}
	remoteHead, exists := advertised.References[branchRef.String()]
	if !exists {
		return nil, fmt.Errorf("remote %q does not advertise branch %q", remoteID, branch)
	}
	if !strings.EqualFold(remoteHead.String(), strings.TrimSpace(expectedRemoteHead)) {
		return nil, fmt.Errorf("remote branch changed; run git_remote_status again before cloning")
	}

	request := packp.NewUploadPackRequestFromCapabilities(advertised.Capabilities)
	request.Capabilities.Delete(capability.Sideband)
	request.Capabilities.Delete(capability.Sideband64k)
	request.Capabilities.Delete(capability.ThinPack)
	request.Wants = []plumbing.Hash{remoteHead}

	response, err := session.UploadPack(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("remote %q clone could not be started", remoteID)
	}
	packLimit := s.cloneMaxBytes
	if packLimit > maxRemoteClonePackBytes {
		packLimit = maxRemoteClonePackBytes
	}
	limited := &remoteFetchLimitReader{reader: response, remaining: packLimit}
	buffered := bufio.NewReaderSize(limited, 32*1024)
	if err := validateClonePackHeader(buffered); err != nil {
		_ = response.Close()
		return nil, fmt.Errorf("remote %q clone pack was rejected: %w", remoteID, err)
	}
	storageErr := packfile.UpdateObjectStorage(repo.Storer, buffered)
	closeErr := response.Close()
	if storageErr != nil {
		return nil, s.safeCloneStorageError(remoteID, storageErr, packLimit)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("remote %q clone response could not be closed cleanly", remoteID)
	}
	if _, err := repo.CommitObject(remoteHead); err != nil {
		return nil, fmt.Errorf("remote %q clone did not produce the expected branch object", remoteID)
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(branchRef, remoteHead)); err != nil {
		return nil, safeRepositoryError(remote.Repository, "cloned branch reference could not be created")
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branchRef)); err != nil {
		return nil, safeRepositoryError(remote.Repository, "cloned HEAD could not be set")
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, safeRepositoryError(remote.Repository, "cloned worktree could not be opened")
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: branchRef, Force: true}); err != nil {
		return nil, s.safeCloneCheckoutError(remote.Repository, err)
	}
	status, err := worktree.Status()
	if err != nil || !status.IsClean() {
		return nil, safeRepositoryError(remote.Repository, "cloned worktree did not validate as clean")
	}
	head, err := repo.Head()
	if err != nil || head.Name() != branchRef || head.Hash() != remoteHead {
		return nil, safeRepositoryError(remote.Repository, "cloned HEAD did not match the reviewed remote state")
	}

	bytesUsed, entriesUsed := worktreeFS.quota.usage()
	if closer, ok := any(storage).(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return nil, safeRepositoryError(remote.Repository, "temporary Git storage could not be closed")
		}
	}
	if _, err := os.Lstat(destination); err == nil {
		return nil, safeRepositoryError(remote.Repository, "clone destination already exists")
	} else if !os.IsNotExist(err) {
		return nil, safeRepositoryError(remote.Repository, "clone destination could not be checked")
	}
	if err := os.Rename(temporary, destination); err != nil {
		return nil, safeRepositoryError(remote.Repository, "completed clone could not be promoted")
	}
	promoted = true

	return &RemoteCloneResult{
		Remote: remoteID, Repository: remote.Repository, Branch: branch,
		Head: remoteHead.String(), BytesReceived: limited.read,
		StorageBytesUsed: bytesUsed, EntriesCreated: entriesUsed,
		StorageByteLimit: s.cloneMaxBytes, StorageEntryLimit: s.cloneMaxEntries,
	}, nil
}

func (s *RemoteService) cloneDestination(repositoryID string) (string, error) {
	if s == nil || s.local == nil {
		return "", errRemoteCloneDisabled
	}
	destination, ok := s.local.repositories[repositoryID]
	if !ok {
		return "", fmt.Errorf("repository %q is not configured", repositoryID)
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", safeRepositoryError(repositoryID, "clone destination already exists")
	} else if !os.IsNotExist(err) {
		return "", safeRepositoryError(repositoryID, "clone destination could not be checked")
	}
	parent := filepath.Dir(destination)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", safeRepositoryError(repositoryID, "clone destination parent must be an existing real directory")
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Clean(resolvedParent) != filepath.Clean(parent) {
		return "", safeRepositoryError(repositoryID, "clone destination parent cannot traverse symlinked directories")
	}
	return destination, nil
}

func validateClonePackHeader(reader *bufio.Reader) error {
	header, err := reader.Peek(12)
	if err != nil {
		return fmt.Errorf("pack header could not be read")
	}
	if string(header[:4]) != "PACK" {
		return fmt.Errorf("invalid Git pack header")
	}
	version := binary.BigEndian.Uint32(header[4:8])
	if version != packfile.VersionSupported {
		return fmt.Errorf("unsupported Git pack version %d", version)
	}
	objects := binary.BigEndian.Uint32(header[8:12])
	if objects > maxRemoteCloneObjects {
		return fmt.Errorf("pack declares %d objects, exceeding the guarded limit of %d", objects, maxRemoteCloneObjects)
	}
	return nil
}

func (s *RemoteService) safeCloneStorageError(remoteID string, err error, packLimit int64) error {
	switch {
	case errors.Is(err, errRemoteFetchTooLarge):
		return fmt.Errorf("remote %q clone exceeded the %d MiB compressed pack limit", remoteID, packLimit>>20)
	case errors.Is(err, errCloneStorageQuotaExceeded):
		return fmt.Errorf("remote %q clone exceeded the configured storage byte quota", remoteID)
	case errors.Is(err, errCloneEntryQuotaExceeded):
		return fmt.Errorf("remote %q clone exceeded the configured filesystem entry quota", remoteID)
	default:
		return fmt.Errorf("remote %q clone pack could not be stored", remoteID)
	}
}

func (s *RemoteService) safeCloneCheckoutError(repositoryID string, err error) error {
	switch {
	case errors.Is(err, errCloneStorageQuotaExceeded):
		return safeRepositoryError(repositoryID, "clone exceeded the configured storage byte quota during checkout")
	case errors.Is(err, errCloneEntryQuotaExceeded):
		return safeRepositoryError(repositoryID, "clone exceeded the configured filesystem entry quota during checkout")
	default:
		return safeRepositoryError(repositoryID, "cloned worktree could not be checked out")
	}
}
