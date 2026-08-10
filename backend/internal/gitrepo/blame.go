package gitrepo

import (
	"context"
	"fmt"
	"strings"

	git "github.com/go-git/go-git/v5"
)

// Blame returns bounded line attribution for a committed repository-relative file.
func (s *Service) Blame(ctx context.Context, repositoryID, filePath, revision string, startLine, endLine int) (*BlameResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cleanPath, err := cleanRepositoryPath(filePath)
	if err != nil {
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
	file, err := commit.File(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("path %q could not be blamed at revision %q", cleanPath, revision)
	}
	if file.Size > maxBlameFileBytes {
		return nil, fmt.Errorf("path %q exceeds the blame size limit", cleanPath)
	}
	blame, err := git.Blame(commit, cleanPath)
	if err != nil {
		return nil, fmt.Errorf("path %q could not be blamed at revision %q", cleanPath, revision)
	}
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(blame.Lines) {
		endLine = len(blame.Lines)
	}
	if startLine > endLine || startLine > len(blame.Lines) {
		return nil, fmt.Errorf("requested line range is outside the file")
	}

	// Advance at most maxBlameLines positions instead of deriving an allocation
	// size from caller-provided line numbers. This also avoids overflow-prone
	// arithmetic when callers provide values near the platform integer limit.
	boundedEnd := startLine
	for count := 1; count < maxBlameLines && boundedEnd < endLine; count++ {
		boundedEnd++
	}
	truncated := boundedEnd < endLine
	endLine = boundedEnd

	lines := make([]BlameLine, 0, maxBlameLines)
	for i := startLine - 1; i < endLine; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := blame.Lines[i]
		lines = append(lines, BlameLine{
			Line:   i + 1,
			Hash:   line.Hash.String(),
			Author: line.AuthorName,
			When:   line.Date,
			Text:   line.Text,
		})
	}
	return &BlameResult{
		Repository: repositoryID,
		Revision:   commit.Hash.String(),
		Path:       cleanPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Truncated:  truncated,
		Lines:      lines,
	}, nil
}
