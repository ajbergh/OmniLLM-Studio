package gitrepo

import (
	"fmt"
	"path"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (s *Service) open(repositoryID string) (*git.Repository, error) {
	if s == nil || !ValidRepositoryID(repositoryID) {
		return nil, fmt.Errorf("unknown repository %q", repositoryID)
	}
	repositoryPath, ok := s.repositories[repositoryID]
	if !ok {
		return nil, fmt.Errorf("unknown repository %q", repositoryID)
	}
	repo, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "could not be opened as a Git repository")
	}
	return repo, nil
}

func resolveRevision(repo *git.Repository, revision string) (plumbing.Hash, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(strings.TrimSpace(revision)))
	if err != nil || hash == nil {
		return plumbing.ZeroHash, fmt.Errorf("revision could not be resolved")
	}
	return *hash, nil
}

func resolveCommit(repo *git.Repository, revision string) (*object.Commit, error) {
	hash, err := resolveRevision(repo, revision)
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(hash)
}

func headState(repo *git.Repository) (branch, hash string, detached bool) {
	ref, err := repo.Head()
	if err != nil {
		return "", "", false
	}
	hash = ref.Hash().String()
	if ref.Name().IsBranch() {
		return ref.Name().Short(), hash, false
	}
	return "", hash, true
}

func safeRepositoryError(repositoryID, detail string) error {
	return fmt.Errorf("repository %q %s", repositoryID, detail)
}

func statusCodeName(code git.StatusCode) string {
	switch code {
	case git.Unmodified:
		return "unmodified"
	case git.Untracked:
		return "untracked"
	case git.Modified:
		return "modified"
	case git.Added:
		return "added"
	case git.Deleted:
		return "deleted"
	case git.Renamed:
		return "renamed"
	case git.Copied:
		return "copied"
	case git.UpdatedButUnmerged:
		return "unmerged"
	default:
		return "unknown"
	}
}

func cleanRepositoryPath(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("path must be repository-relative")
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == ".git" || strings.HasPrefix(cleaned, ".git/") {
		return "", fmt.Errorf("path must stay within the repository worktree")
	}
	return cleaned, nil
}

func commitSubject(message string) string {
	message = strings.TrimSpace(message)
	if idx := strings.IndexByte(message, '\n'); idx >= 0 {
		message = strings.TrimSpace(message[:idx])
	}
	return message
}

func truncateString(value string, limit int) (string, bool) {
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	return value[:limit], true
}
