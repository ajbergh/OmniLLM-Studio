// Package gitrepo provides read-only access to explicitly configured local Git repositories.
package gitrepo

import (
	"context"
	"time"
)

const (
	defaultLogLimit      = 20
	maxLogLimit          = 100
	maxBlameLines        = 300
	maxBlameFileBytes    = 512 << 10
	maxDiffFiles         = 100
	maxDiffFileBytes     = 512 << 10
	maxDiffOutputBytes   = 120 << 10
	defaultDiffContext   = 3
	maxRepositoryIDBytes = 64
)

// Reader is the model-facing read-only repository contract. Implementations
// must resolve repository IDs without accepting arbitrary filesystem paths.
type Reader interface {
	Repositories(ctx context.Context) []RepositorySummary
	Status(ctx context.Context, repositoryID string) (*StatusResult, error)
	Diff(ctx context.Context, repositoryID, from, to string) (*DiffResult, error)
	Log(ctx context.Context, repositoryID, revision string, limit int) (*LogResult, error)
	Show(ctx context.Context, repositoryID, revision string) (*CommitDetail, error)
	Branches(ctx context.Context, repositoryID string) (*BranchesResult, error)
	Blame(ctx context.Context, repositoryID, filePath, revision string, startLine, endLine int) (*BlameResult, error)
}

// RepositorySummary describes a configured repository without exposing its local path.
type RepositorySummary struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Branch    string `json:"branch,omitempty"`
	Head      string `json:"head,omitempty"`
	Detached  bool   `json:"detached,omitempty"`
	Clean     bool   `json:"clean,omitempty"`
	Changes   int    `json:"changes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FileStatus describes staging and worktree state for a repository-relative path.
type FileStatus struct {
	Path     string `json:"path"`
	Staging  string `json:"staging"`
	Worktree string `json:"worktree"`
	Previous string `json:"previous,omitempty"`
}

// StatusResult is a read-only snapshot of HEAD and working-tree state.
type StatusResult struct {
	Repository string       `json:"repository"`
	Branch     string       `json:"branch,omitempty"`
	Head       string       `json:"head,omitempty"`
	Detached   bool         `json:"detached,omitempty"`
	Clean      bool         `json:"clean"`
	Files      []FileStatus `json:"files"`
}

// DiffResult contains either a worktree-vs-HEAD diff or a revision-to-revision diff.
type DiffResult struct {
	Repository string   `json:"repository"`
	Mode       string   `json:"mode"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Files      []string `json:"files,omitempty"`
	Patch      string   `json:"patch"`
	Truncated  bool     `json:"truncated,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// CommitSummary is a compact log entry.
type CommitSummary struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	When    time.Time `json:"when"`
	Subject string    `json:"subject"`
}

// LogResult contains recent commits from a revision.
type LogResult struct {
	Repository string          `json:"repository"`
	Revision   string          `json:"revision"`
	Commits    []CommitSummary `json:"commits"`
}

// CommitDetail describes one commit without exposing repository-local filesystem data.
type CommitDetail struct {
	Repository string    `json:"repository"`
	Hash       string    `json:"hash"`
	Author     string    `json:"author"`
	When       time.Time `json:"when"`
	Message    string    `json:"message"`
	Parents    []string  `json:"parents"`
}

// BranchInfo describes a local branch.
type BranchInfo struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Current bool   `json:"current"`
}

// BranchesResult contains local branches and detached-HEAD state.
type BranchesResult struct {
	Repository string       `json:"repository"`
	Detached   bool         `json:"detached"`
	Branches   []BranchInfo `json:"branches"`
}

// BlameLine contains repository-relative line attribution.
type BlameLine struct {
	Line   int       `json:"line"`
	Hash   string    `json:"hash"`
	Author string    `json:"author"`
	When   time.Time `json:"when"`
	Text   string    `json:"text"`
}

// BlameResult contains bounded blame output for one committed file.
type BlameResult struct {
	Repository string      `json:"repository"`
	Revision   string      `json:"revision"`
	Path       string      `json:"path"`
	StartLine  int         `json:"start_line"`
	EndLine    int         `json:"end_line"`
	Truncated  bool        `json:"truncated,omitempty"`
	Lines      []BlameLine `json:"lines"`
}
