package gitrepo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var errWriteDisabled = errors.New("local Git write access is disabled")

// CreateBranch creates a local branch reference without changing HEAD.
func (s *Service) CreateBranch(ctx context.Context, repositoryID, branchName, fromRevision, expectedHead string) (*CreateBranchResult, error) {
	if err := s.requireWriteEnabled(); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	branchName, branchRef, err := cleanBranchName(branchName)
	if err != nil {
		return nil, err
	}
	if _, err := repo.Reference(branchRef, false); err == nil {
		return nil, fmt.Errorf("branch %q already exists", branchName)
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, fmt.Errorf("branch %q could not be inspected", branchName)
	}
	if strings.TrimSpace(fromRevision) == "" {
		fromRevision = "HEAD"
	}
	commit, err := resolveCommit(repo, fromRevision)
	if err != nil {
		return nil, fmt.Errorf("revision %q could not be resolved", fromRevision)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(branchRef, commit.Hash)); err != nil {
		return nil, fmt.Errorf("branch %q could not be created", branchName)
	}
	return &CreateBranchResult{Repository: repositoryID, Branch: branchName, Hash: commit.Hash.String()}, nil
}

// Checkout switches to an existing local branch only when the worktree is clean.
func (s *Service) Checkout(ctx context.Context, repositoryID, branchName, expectedHead string) (*CheckoutResult, error) {
	if err := s.requireWriteEnabled(); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	branchName, branchRef, err := cleanBranchName(branchName)
	if err != nil {
		return nil, err
	}
	if _, err := repo.Reference(branchRef, false); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, fmt.Errorf("branch %q does not exist", branchName)
		}
		return nil, fmt.Errorf("branch %q could not be inspected", branchName)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "does not expose a worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "status could not be read")
	}
	if !status.IsClean() {
		return nil, fmt.Errorf("repository %q has local changes; checkout requires a clean worktree", repositoryID)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: branchRef}); err != nil {
		return nil, fmt.Errorf("branch %q could not be checked out", branchName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "HEAD could not be resolved after checkout")
	}
	return &CheckoutResult{Repository: repositoryID, Branch: branchName, Head: head.Hash().String()}, nil
}

// Stage adds exact repository-relative paths with unstaged changes to the index.
// Directory, glob, and stage-all semantics are deliberately not exposed.
func (s *Service) Stage(ctx context.Context, repositoryID string, rawPaths []string, expectedHead string) (*StageResult, error) {
	if err := s.requireWriteEnabled(); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(rawPaths) == 0 || len(rawPaths) > maxStagePaths {
		return nil, fmt.Errorf("between 1 and %d paths are required", maxStagePaths)
	}

	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	head, err := ensureExpectedHead(repo, expectedHead)
	if err != nil {
		return nil, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "does not expose a worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "status could not be read")
	}

	paths, err := s.cleanStagePaths(repositoryID, rawPaths, status)
	if err != nil {
		return nil, err
	}
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	originalIndex, err := repo.Storer.Index()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "index could not be read")
	}
	rollback := func() {
		_ = repo.Storer.SetIndex(originalIndex)
	}
	for _, filePath := range paths {
		if err := ctx.Err(); err != nil {
			rollback()
			return nil, err
		}
		if _, err := worktree.Add(filepath.FromSlash(filePath)); err != nil {
			rollback()
			return nil, fmt.Errorf("path %q could not be staged", filePath)
		}
	}
	digest, err := indexDigest(repo)
	if err != nil {
		rollback()
		return nil, safeRepositoryError(repositoryID, "index state could not be verified")
	}
	return &StageResult{Repository: repositoryID, Head: head, Paths: paths, IndexDigest: digest}, nil
}

// Commit creates a commit from the existing staged index only. It never stages
// files automatically, amends, creates empty commits, or operates on detached HEAD.
func (s *Service) Commit(ctx context.Context, repositoryID, message, expectedHead, expectedIndexDigest string) (*CommitResult, error) {
	if err := s.requireWriteEnabled(); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("commit message is required")
	}
	if len(message) > maxCommitMessageBytes {
		return nil, fmt.Errorf("commit message exceeds %d bytes", maxCommitMessageBytes)
	}
	if strings.TrimSpace(expectedIndexDigest) == "" {
		return nil, fmt.Errorf("expected_index_digest is required; run git_status again")
	}

	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	previousHead, err := ensureExpectedHead(repo, expectedHead)
	if err != nil {
		return nil, err
	}
	branch, _, detached := headState(repo)
	if detached || branch == "" {
		return nil, fmt.Errorf("repository %q has detached HEAD; commit requires a local branch", repositoryID)
	}
	currentDigest, err := indexDigest(repo)
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "index state could not be verified")
	}
	if !strings.EqualFold(strings.TrimSpace(expectedIndexDigest), currentDigest) {
		return nil, fmt.Errorf("repository %q index changed; run git_status again before committing", repositoryID)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "does not expose a worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "status could not be read")
	}
	staged := false
	for _, fileStatus := range status {
		if fileStatus.Staging != git.Unmodified && fileStatus.Staging != git.Untracked {
			staged = true
			break
		}
	}
	if !staged {
		return nil, fmt.Errorf("repository %q has no staged changes to commit", repositoryID)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	finalDigest, err := indexDigest(repo)
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "index state could not be verified")
	}
	if !strings.EqualFold(strings.TrimSpace(expectedIndexDigest), finalDigest) {
		return nil, fmt.Errorf("repository %q index changed; run git_status again before committing", repositoryID)
	}
	hash, err := worktree.Commit(message, &git.CommitOptions{All: false, Amend: false, AllowEmptyCommits: false})
	if err != nil {
		switch {
		case errors.Is(err, git.ErrEmptyCommit):
			return nil, fmt.Errorf("repository %q has no staged changes to commit", repositoryID)
		case errors.Is(err, git.ErrMissingAuthor):
			return nil, fmt.Errorf("repository %q does not have a configured Git author name and email", repositoryID)
		default:
			return nil, fmt.Errorf("repository %q commit could not be created", repositoryID)
		}
	}
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "new commit could not be read")
	}
	return &CommitResult{
		Repository: repositoryID,
		Branch:     branch,
		Previous:   previousHead,
		Hash:       hash.String(),
		Author:     commit.Author.Name,
		Subject:    commitSubject(commit.Message),
	}, nil
}

func ensureExpectedHead(repo *git.Repository, expected string) (string, error) {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return "", fmt.Errorf("expected_head is required; run git_status again")
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("HEAD could not be resolved")
	}
	current := head.Hash().String()
	if !strings.EqualFold(expected, current) {
		return "", fmt.Errorf("repository HEAD changed; run git_status again before mutating")
	}
	return current, nil
}

func cleanBranchName(raw string) (string, plumbing.ReferenceName, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > maxBranchNameBytes {
		return "", "", fmt.Errorf("branch name must be between 1 and %d bytes", maxBranchNameBytes)
	}
	ref := plumbing.NewBranchReferenceName(name)
	if err := ref.Validate(); err != nil {
		return "", "", fmt.Errorf("branch name %q is invalid", name)
	}
	return name, ref, nil
}

func (s *Service) cleanStagePaths(repositoryID string, rawPaths []string, status git.Status) ([]string, error) {
	repositoryRoot := s.repositories[repositoryID]
	unique := make(map[string]struct{}, len(rawPaths))
	paths := make([]string, 0, maxStagePaths)
	for _, rawPath := range rawPaths {
		cleanPath, err := cleanRepositoryPath(rawPath)
		if err != nil {
			return nil, err
		}
		cleanPath = filepath.ToSlash(cleanPath)
		if _, exists := unique[cleanPath]; exists {
			continue
		}
		fileStatus, ok := status[cleanPath]
		if !ok || (fileStatus.Worktree != git.Modified && fileStatus.Worktree != git.Untracked && fileStatus.Worktree != git.Deleted) {
			return nil, fmt.Errorf("path %q has no unstaged change to stage", cleanPath)
		}
		if fileStatus.Worktree != git.Deleted {
			if err := validateStageFilesystemPath(repositoryRoot, cleanPath); err != nil {
				return nil, fmt.Errorf("path %q cannot be staged safely: %v", cleanPath, err)
			}
		}
		unique[cleanPath] = struct{}{}
		paths = append(paths, cleanPath)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one changed path is required")
	}
	sort.Strings(paths)
	return paths, nil
}

// validateStageFilesystemPath rejects stage targets whose parent directory
// resolves through a symlink. The final path itself may be a symlink because
// Git stages the link object rather than following it. SecureJoin performs the
// filesystem-sensitive resolution under the canonical configured root; the
// equality check then enforces the stricter no-symlink-parent rule required
// before passing the original repository-relative path to go-git.
func validateStageFilesystemPath(repositoryRoot, cleanPath string) error {
	parentRelative := filepath.Dir(filepath.FromSlash(cleanPath))
	if parentRelative == "." {
		return nil
	}
	resolvedParent, err := securejoin.SecureJoin(repositoryRoot, parentRelative)
	if err != nil {
		return fmt.Errorf("parent path could not be resolved within the repository")
	}
	lexicalParent := filepath.Join(repositoryRoot, parentRelative)
	relative, err := filepath.Rel(lexicalParent, resolvedParent)
	if err != nil || relative != "." {
		return fmt.Errorf("parent path contains a symlink and cannot be staged safely")
	}
	return nil
}

func indexDigest(repo *git.Repository) (string, error) {
	idx, err := repo.Storer.Index()
	if err != nil {
		return "", err
	}
	ordered := make([]indexDigestEntry, 0, len(idx.Entries))
	for _, entry := range idx.Entries {
		ordered = append(ordered, indexDigestEntry{
			name:  filepath.ToSlash(entry.Name),
			hash:  entry.Hash.String(),
			mode:  uint32(entry.Mode),
			stage: uint8(entry.Stage),
		})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].name != ordered[j].name {
			return ordered[i].name < ordered[j].name
		}
		return ordered[i].stage < ordered[j].stage
	})
	h := sha256.New()
	for _, entry := range ordered {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\n", entry.name, entry.hash, entry.mode, entry.stage)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type indexDigestEntry struct {
	name  string
	hash  string
	mode  uint32
	stage uint8
}
