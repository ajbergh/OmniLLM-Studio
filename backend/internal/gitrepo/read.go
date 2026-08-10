package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
)

// Repositories returns bounded status summaries for configured repositories.
func (s *Service) Repositories(ctx context.Context) []RepositorySummary {
	if s == nil {
		return nil
	}
	out := make([]RepositorySummary, 0, len(s.ids))
	for _, id := range s.ids {
		if ctx.Err() != nil {
			break
		}
		summary := RepositorySummary{ID: id}
		status, err := s.Status(ctx, id)
		if err != nil {
			summary.Error = err.Error()
			out = append(out, summary)
			continue
		}
		summary.Available = true
		summary.Branch = status.Branch
		summary.Head = status.Head
		summary.Detached = status.Detached
		summary.Clean = status.Clean
		summary.Changes = len(status.Files)
		out = append(out, summary)
	}
	return out
}

// Status returns the current local HEAD and working-tree status.
func (s *Service) Status(ctx context.Context, repositoryID string) (*StatusResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo, err := s.open(repositoryID)
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

	branch, head, detached := headState(repo)
	files := make([]FileStatus, 0, len(status))
	paths := make([]string, 0, len(status))
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified && fileStatus.Worktree == git.Unmodified {
			continue
		}
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	for _, filePath := range paths {
		fileStatus := status[filePath]
		files = append(files, FileStatus{
			Path:     filepath.ToSlash(filePath),
			Staging:  statusCodeName(fileStatus.Staging),
			Worktree: statusCodeName(fileStatus.Worktree),
		})
	}

	return &StatusResult{
		Repository: repositoryID,
		Branch:     branch,
		Head:       head,
		Detached:   detached,
		Clean:      status.IsClean(),
		Files:      files,
	}, nil
}

// Log returns recent commits reachable from a revision.
func (s *Service) Log(ctx context.Context, repositoryID, revision string, limit int) (*LogResult, error) {
	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultLogLimit
	}
	if limit > maxLogLimit {
		limit = maxLogLimit
	}
	if strings.TrimSpace(revision) == "" {
		revision = "HEAD"
	}
	hash, err := resolveRevision(repo, revision)
	if err != nil {
		return nil, fmt.Errorf("revision %q could not be resolved", revision)
	}
	iter, err := repo.Log(&git.LogOptions{From: hash})
	if err != nil {
		return nil, fmt.Errorf("commit log could not be read")
	}
	defer iter.Close()

	commits := make([]CommitSummary, 0, limit)
	for len(commits) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		commit, err := iter.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("commit log could not be read")
		}
		commits = append(commits, CommitSummary{
			Hash:    commit.Hash.String(),
			Author:  commit.Author.Name,
			When:    commit.Author.When,
			Subject: commitSubject(commit.Message),
		})
	}
	return &LogResult{Repository: repositoryID, Revision: hash.String(), Commits: commits}, nil
}

// Show returns one resolved commit.
func (s *Service) Show(ctx context.Context, repositoryID, revision string) (*CommitDetail, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(revision) == "" {
		revision = "HEAD"
	}
	commit, err := resolveCommit(repo, revision)
	if err != nil {
		return nil, fmt.Errorf("revision %q could not be resolved", revision)
	}
	parents := make([]string, 0, len(commit.ParentHashes))
	for _, parentHash := range commit.ParentHashes {
		parents = append(parents, parentHash.String())
	}
	return &CommitDetail{
		Repository: repositoryID,
		Hash:       commit.Hash.String(),
		Author:     commit.Author.Name,
		When:       commit.Author.When,
		Message:    commit.Message,
		Parents:    parents,
	}, nil
}

// Branches returns local branches only; remote-tracking references remain a later capability.
func (s *Service) Branches(ctx context.Context, repositoryID string) (*BranchesResult, error) {
	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	head, _ := repo.Head()
	current := ""
	detached := true
	if head != nil && head.Name().IsBranch() {
		current = head.Name().Short()
		detached = false
	}
	iter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("branches could not be read")
	}
	defer iter.Close()
	branches := make([]BranchInfo, 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ref, err := iter.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("branches could not be read")
		}
		branches = append(branches, BranchInfo{
			Name:    ref.Name().Short(),
			Hash:    ref.Hash().String(),
			Current: ref.Name().Short() == current,
		})
	}
	sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
	return &BranchesResult{Repository: repositoryID, Detached: detached, Branches: branches}, nil
}
