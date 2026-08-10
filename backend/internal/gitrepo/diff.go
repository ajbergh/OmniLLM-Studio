package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pmezard/go-difflib/difflib"
)

// Diff returns a revision diff when from is supplied, otherwise it returns a
// combined worktree diff against HEAD. The worktree mode intentionally does not
// distinguish staged from unstaged hunks; git_status preserves that distinction.
func (s *Service) Diff(ctx context.Context, repositoryID, from, to string) (*DiffResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo, err := s.open(repositoryID)
	if err != nil {
		return nil, err
	}
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" && to == "" {
		return s.worktreeDiff(ctx, repositoryID, repo)
	}
	if from == "" {
		return nil, fmt.Errorf("from revision is required when to is provided")
	}
	if to == "" {
		to = "HEAD"
	}
	fromCommit, err := resolveCommit(repo, from)
	if err != nil {
		return nil, fmt.Errorf("revision %q could not be resolved", from)
	}
	toCommit, err := resolveCommit(repo, to)
	if err != nil {
		return nil, fmt.Errorf("revision %q could not be resolved", to)
	}
	patch, err := fromCommit.PatchContext(ctx, toCommit)
	if err != nil {
		return nil, fmt.Errorf("diff could not be computed")
	}
	patchText, truncated := truncateString(patch.String(), maxDiffOutputBytes)
	files := make([]string, 0, len(patch.FilePatches()))
	for _, filePatch := range patch.FilePatches() {
		fromFile, toFile := filePatch.Files()
		if toFile != nil {
			files = append(files, toFile.Path())
		} else if fromFile != nil {
			files = append(files, fromFile.Path())
		}
	}
	return &DiffResult{
		Repository: repositoryID,
		Mode:       "revision",
		From:       fromCommit.Hash.String(),
		To:         toCommit.Hash.String(),
		Files:      files,
		Patch:      patchText,
		Truncated:  truncated,
	}, nil
}

func (s *Service) worktreeDiff(ctx context.Context, repositoryID string, repo *git.Repository) (*DiffResult, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "does not expose a worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "status could not be read")
	}
	worktreeDigest, err := s.worktreeStateDigest(repositoryID, status)
	if err != nil {
		return nil, fmt.Errorf("worktree state could not be fingerprinted safely: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "HEAD could not be resolved")
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "HEAD commit could not be read")
	}

	paths := make([]string, 0, len(status))
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified && fileStatus.Worktree == git.Unmodified {
			continue
		}
		paths = append(paths, filepath.ToSlash(filePath))
	}
	sort.Strings(paths)
	files := append([]string(nil), paths...)
	result := &DiffResult{
		Repository:     repositoryID,
		Mode:           "worktree",
		From:           headCommit.Hash.String(),
		To:             "WORKTREE",
		WorktreeDigest: worktreeDigest,
		Files:          files,
	}
	if len(paths) == 0 {
		return result, nil
	}
	if len(paths) > maxDiffFiles {
		result.Warnings = append(result.Warnings, fmt.Sprintf("diff limited to first %d changed files", maxDiffFiles))
		paths = paths[:maxDiffFiles]
		result.Truncated = true
	}

	repositoryRoot := s.repositories[repositoryID]
	var output strings.Builder
	for _, filePath := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		oldPath := filePath
		oldContent, oldExists, oldBinary, oldTooLarge := committedFileContent(headCommit, oldPath)
		newContent, newExists, newBinary, newTooLarge, newErr := worktreeFileContent(repositoryRoot, filePath)
		if newErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s omitted: %v", filePath, newErr))
			continue
		}
		if oldTooLarge || newTooLarge {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s omitted because it exceeds %d bytes", filePath, maxDiffFileBytes))
			continue
		}
		if oldBinary || newBinary {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s omitted because binary content is not rendered", filePath))
			continue
		}
		if !oldExists && !newExists {
			continue
		}
		fromName := "a/" + oldPath
		toName := "b/" + filePath
		if !oldExists {
			fromName = "/dev/null"
		}
		if !newExists {
			toName = "/dev/null"
		}
		patch, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(oldContent),
			B:        difflib.SplitLines(newContent),
			FromFile: fromName,
			ToFile:   toName,
			Context:  defaultDiffContext,
		})
		if err != nil {
			return nil, fmt.Errorf("worktree diff could not be formatted")
		}
		if patch == "" {
			continue
		}
		if output.Len()+len(patch) > maxDiffOutputBytes {
			remaining := maxDiffOutputBytes - output.Len()
			if remaining > 0 {
				output.WriteString(patch[:min(remaining, len(patch))])
			}
			result.Truncated = true
			result.Warnings = append(result.Warnings, "diff output was truncated")
			break
		}
		output.WriteString(patch)
	}
	result.Patch = output.String()
	return result, nil
}

func committedFileContent(commit *object.Commit, filePath string) (content string, exists, binary, tooLarge bool) {
	file, err := commit.File(filePath)
	if err != nil {
		return "", false, false, false
	}
	reader, err := file.Reader()
	if err != nil {
		return "", true, false, false
	}
	defer reader.Close()
	data, exceeded, err := readBounded(reader, maxDiffFileBytes)
	if err != nil {
		return "", true, false, false
	}
	return string(data), true, bytes.IndexByte(data, 0) >= 0, exceeded
}

func worktreeFileContent(repositoryRoot, filePath string) (content string, exists, binary, tooLarge bool, err error) {
	cleanPath, err := cleanRepositoryPath(filePath)
	if err != nil {
		return "", false, false, false, fmt.Errorf("path is not repository-relative")
	}
	fullPath := filepath.Join(repositoryRoot, filepath.FromSlash(cleanPath))
	if !pathWithinRoot(repositoryRoot, fullPath) {
		return "", false, false, false, fmt.Errorf("path escapes the configured repository root")
	}

	info, err := os.Lstat(fullPath)
	if os.IsNotExist(err) {
		return "", false, false, false, nil
	}
	if err != nil {
		return "", true, false, false, fmt.Errorf("worktree file metadata could not be read")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", true, false, false, fmt.Errorf("symlink content is not rendered")
	}
	if info.IsDir() {
		return "", true, false, false, fmt.Errorf("directory content is not rendered")
	}

	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", true, false, false, fmt.Errorf("worktree path could not be resolved safely")
	}
	if !pathWithinRoot(repositoryRoot, resolvedPath) {
		return "", true, false, false, fmt.Errorf("resolved path escapes the configured repository root")
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", true, false, false, fmt.Errorf("worktree file could not be read")
	}
	defer file.Close()
	data, exceeded, err := readBounded(file, maxDiffFileBytes)
	if err != nil {
		return "", true, false, false, fmt.Errorf("worktree file could not be read")
	}
	return string(data), true, bytes.IndexByte(data, 0) >= 0, exceeded, nil
}

func readBounded(reader io.Reader, maxBytes int) ([]byte, bool, error) {
	limited := io.LimitReader(reader, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}
