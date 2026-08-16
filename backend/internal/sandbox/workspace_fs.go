package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxWorkspaceMutationBytes = 2 << 20

var windowsAbsolutePathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// WorkspaceFS provides containment-checked filesystem operations over opaque
// grants. Tool adapters use this service rather than joining host paths.
type WorkspaceFS struct {
	registry *WorkspaceRegistry
}

func NewWorkspaceFS(registry *WorkspaceRegistry) (*WorkspaceFS, error) {
	if registry == nil {
		return nil, fmt.Errorf("workspace registry is required")
	}
	return &WorkspaceFS{registry: registry}, nil
}

// Resolve returns an internal host path only after owner/grant/path checks. All
// existing path components, including the final component, must be non-symlink.
func (s *WorkspaceFS) Resolve(ownerUserID, workspaceID, relativePath string, allowMissingFinal bool) (*FileWorkspace, string, string, error) {
	workspace, err := s.registry.Get(ownerUserID, workspaceID)
	if err != nil {
		return nil, "", "", err
	}
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, "", "", err
	}
	root, err := canonicalWorkspaceRoot(workspace.RootPath)
	if err != nil || root != workspace.RootPath {
		return nil, "", "", fmt.Errorf("workspace root is no longer safely canonical")
	}
	candidate := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", "", fmt.Errorf("workspace path escapes root")
	}
	current := root
	parts := strings.Split(filepath.FromSlash(clean), string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && allowMissingFinal && index == len(parts)-1 {
			return workspace, candidate, clean, nil
		}
		if statErr != nil {
			return nil, "", "", fmt.Errorf("inspect workspace path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, "", "", fmt.Errorf("workspace path contains a symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, "", "", fmt.Errorf("workspace parent is not a directory")
		}
	}
	return workspace, candidate, clean, nil
}

func cleanWorkspaceRelativePath(input string) (string, error) {
	input = strings.TrimSpace(strings.ReplaceAll(input, "\\", "/"))
	if input == "" || input == "." || strings.HasPrefix(input, "/") || windowsAbsolutePathPattern.MatchString(input) {
		return "", fmt.Errorf("workspace path must be a non-empty relative path")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(input)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsRune(clean, '\x00') {
		return "", fmt.Errorf("workspace path escapes root")
	}
	if clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return "", fmt.Errorf("direct .git mutation is not permitted through workspace tools")
	}
	return clean, nil
}

// ReadFile reads a bounded regular file and returns its current SHA-256. The
// platform read boundary is responsible for keeping the file lookup bound to
// the granted root while bytes are opened and consumed.
func (s *WorkspaceFS) ReadFile(ownerUserID, workspaceID, relativePath string, maxBytes int64) ([]byte, string, error) {
	workspace, _, clean, err := s.Resolve(ownerUserID, workspaceID, relativePath, false)
	if err != nil {
		return nil, "", err
	}
	if maxBytes <= 0 || maxBytes > 8<<20 {
		maxBytes = 2 << 20
	}
	return readWorkspaceRegularFile(workspace.RootPath, clean, maxBytes)
}

// WriteFile atomically creates/replaces a small regular file. The optional
// expectedSHA256 binds replacement to a previously reviewed state. Existing
// files larger than the journal snapshot cap or non-regular files are rejected.
func (s *WorkspaceFS) WriteFile(ctx context.Context, owner OwnerScope, workspaceID, relativePath string, data []byte, expectedSHA256 string) (*WorkspaceChange, error) {
	workspace, _, clean, err := s.Resolve(owner.UserID, workspaceID, relativePath, true)
	if err != nil {
		return nil, err
	}
	if workspace.Mode == MountReadOnly {
		return nil, fmt.Errorf("workspace is read-only")
	}
	if len(data) > maxWorkspaceMutationBytes {
		return nil, fmt.Errorf("workspace write exceeds %d bytes", maxWorkspaceMutationBytes)
	}
	target, err := openWorkspaceMutationTarget(workspace.RootPath, clean)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	before, err := target.Capture()
	if err != nil {
		return nil, err
	}
	if before.Exists && !before.Revertable {
		return nil, fmt.Errorf("workspace mutation requires a small regular existing file")
	}
	if expectedSHA256 != "" && before.SHA256 != strings.ToLower(strings.TrimSpace(expectedSHA256)) {
		return nil, fmt.Errorf("workspace file changed since it was reviewed")
	}
	mode := os.FileMode(0o600)
	if before.Exists && before.Mode.Perm() != 0 {
		mode = before.Mode.Perm()
	}
	if err := target.Write(data, mode); err != nil {
		return nil, err
	}
	after, err := target.Capture()
	if err != nil {
		_ = target.Restore(before)
		return nil, err
	}
	operation := "write"
	if !before.Exists {
		operation = "create"
	}
	change, err := s.registry.RecordWorkspaceChange(ctx, owner, workspaceID, clean, operation, "", "", before, after)
	if err != nil {
		_ = target.Restore(before)
		return nil, fmt.Errorf("workspace write journal failed; mutation rolled back: %w", err)
	}
	return change, nil
}

// DeleteFile removes one small regular file only from a full read_write grant
// and requires its current hash so delete approval cannot apply to new bytes.
func (s *WorkspaceFS) DeleteFile(ctx context.Context, owner OwnerScope, workspaceID, relativePath, expectedSHA256 string) (*WorkspaceChange, error) {
	workspace, _, clean, err := s.Resolve(owner.UserID, workspaceID, relativePath, false)
	if err != nil {
		return nil, err
	}
	if workspace.Mode != MountReadWrite {
		return nil, fmt.Errorf("workspace does not permit deletion")
	}
	target, err := openWorkspaceMutationTarget(workspace.RootPath, clean)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	before, err := target.Capture()
	if err != nil {
		return nil, err
	}
	if !before.Exists || !before.Revertable {
		return nil, fmt.Errorf("workspace delete requires a small regular file")
	}
	if expectedSHA256 == "" || before.SHA256 != strings.ToLower(strings.TrimSpace(expectedSHA256)) {
		return nil, fmt.Errorf("workspace delete requires the current reviewed sha256")
	}
	if err := target.Delete(); err != nil {
		return nil, err
	}
	after := FileState{Exists: false, Revertable: true}
	change, err := s.registry.RecordWorkspaceChange(ctx, owner, workspaceID, clean, "delete", "", "", before, after)
	if err != nil {
		_ = target.Restore(before)
		return nil, fmt.Errorf("workspace delete journal failed; mutation rolled back: %w", err)
	}
	return change, nil
}

// ApplyExactPatch performs deterministic exact-text replacements against a
// required expected file hash, then delegates to the atomic journaled write.
func (s *WorkspaceFS) ApplyExactPatch(ctx context.Context, owner OwnerScope, workspaceID, relativePath, expectedSHA256 string, edits []TextEdit) (*WorkspaceChange, error) {
	if expectedSHA256 == "" {
		return nil, fmt.Errorf("workspace patch requires expected_sha256")
	}
	data, currentSHA, err := s.ReadFile(owner.UserID, workspaceID, relativePath, maxWorkspaceMutationBytes)
	if err != nil {
		return nil, err
	}
	if currentSHA != strings.ToLower(strings.TrimSpace(expectedSHA256)) {
		return nil, fmt.Errorf("workspace file changed since it was reviewed")
	}
	text := string(data)
	if len(edits) == 0 || len(edits) > 32 {
		return nil, fmt.Errorf("workspace patch requires 1-32 edits")
	}
	for _, edit := range edits {
		if edit.OldText == "" {
			return nil, fmt.Errorf("workspace patch old_text cannot be empty")
		}
		count := strings.Count(text, edit.OldText)
		if count != 1 {
			return nil, fmt.Errorf("workspace patch old_text must match exactly once; matched %d", count)
		}
		text = strings.Replace(text, edit.OldText, edit.NewText, 1)
	}
	return s.WriteFile(ctx, owner, workspaceID, relativePath, []byte(text), currentSHA)
}

// RevertChange restores the before-state only if the current file still matches
// the recorded after-state, preventing a stale revert from overwriting newer work.
func (s *WorkspaceFS) RevertChange(ctx context.Context, owner OwnerScope, changeID string) (*WorkspaceChange, error) {
	change, beforeContent, beforeMode, err := s.registry.loadChangeForRevert(ctx, owner.UserID, changeID)
	if err != nil {
		return nil, err
	}
	if !change.Revertable {
		return nil, fmt.Errorf("workspace change is not automatically revertable")
	}
	workspace, _, clean, err := s.Resolve(owner.UserID, change.WorkspaceID, change.RelativePath, !change.AfterExists)
	if err != nil {
		return nil, err
	}
	if workspace.Mode == MountReadOnly {
		return nil, fmt.Errorf("workspace is read-only")
	}
	target, err := openWorkspaceMutationTarget(workspace.RootPath, clean)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	current, err := target.Capture()
	if err != nil {
		return nil, err
	}
	if current.Exists != change.AfterExists || current.SHA256 != change.AfterSHA256 {
		return nil, fmt.Errorf("workspace change can no longer be reverted because the file changed")
	}
	before := FileState{Exists: change.BeforeExists, SHA256: change.BeforeSHA256, Mode: beforeMode, Content: beforeContent, Revertable: true}
	if err := target.Restore(before); err != nil {
		return nil, err
	}
	after, err := target.Capture()
	if err != nil {
		return nil, err
	}
	revertRecord, err := s.registry.RecordWorkspaceChange(ctx, owner, change.WorkspaceID, clean, "revert", "", "", current, after)
	if err != nil {
		_ = target.Restore(current)
		return nil, fmt.Errorf("workspace revert journal failed; revert rolled back: %w", err)
	}
	return revertRecord, nil
}

// TextEdit is one exact text replacement used by ApplyExactPatch.
type TextEdit struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// Search finds bounded text matches without following symlinked directories or
// reading oversized files. Platform search enumeration owns candidate discovery
// and bounded reads so Linux can keep both on one descriptor-relative lineage.
type SearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Preview string `json:"preview"`
}

func (s *WorkspaceFS) Search(ownerUserID, workspaceID, query string, maxMatches int) ([]SearchMatch, error) {
	workspace, err := s.registry.Get(ownerUserID, workspaceID)
	if err != nil {
		return nil, err
	}
	root, err := canonicalWorkspaceRoot(workspace.RootPath)
	if err != nil || root != workspace.RootPath {
		return nil, fmt.Errorf("workspace root is no longer safely canonical")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("workspace search query is required")
	}
	if maxMatches <= 0 || maxMatches > 200 {
		maxMatches = 50
	}
	matches := make([]SearchMatch, 0, min(maxMatches, 50))
	lowerQuery := strings.ToLower(query)
	err = enumerateWorkspaceSearchCandidates(workspace.RootPath, workspace.RootIdentity, 1<<20, func(rel string, data []byte) bool {
		if strings.IndexByte(string(data), 0) >= 0 {
			return true
		}
		for index, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				preview := strings.TrimSpace(line)
				if len(preview) > 300 {
					preview = preview[:300]
				}
				matches = append(matches, SearchMatch{Path: rel, Line: index + 1, Preview: preview})
				if len(matches) >= maxMatches {
					return false
				}
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].Line < matches[j].Line
		}
		return matches[i].Path < matches[j].Path
	})
	return matches, nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace parent directory is unavailable or unsafe")
	}
	temp, err := os.CreateTemp(parent, ".omnillm-write-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(mode.Perm()); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func restoreFileState(path string, state FileState) error {
	if !state.Exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if !state.Revertable {
		return fmt.Errorf("file state is not revertable")
	}
	mode := state.Mode.Perm()
	if mode == 0 {
		mode = 0o600
	}
	return atomicWriteFile(path, state.Content, mode)
}
