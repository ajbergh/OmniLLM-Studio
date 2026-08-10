package gitrepo

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	git "github.com/go-git/go-git/v5"
)

// worktreeStateDigest fingerprints the complete set of changed worktree paths,
// including their staged/worktree status codes, file modes, regular-file bytes,
// symlink targets, and deletions. It streams file content instead of buffering it
// so the digest can cover binary and oversized files that git_diff does not render.
func (s *Service) worktreeStateDigest(repositoryID string, status git.Status) (string, error) {
	root, ok := s.repositories[repositoryID]
	if !ok {
		return "", fmt.Errorf("repository %q is not configured", repositoryID)
	}
	paths := make([]string, 0, len(status))
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified && fileStatus.Worktree == git.Unmodified {
			continue
		}
		paths = append(paths, filepath.ToSlash(filePath))
	}
	sort.Strings(paths)

	h := sha256.New()
	_, _ = io.WriteString(h, "omnillm-worktree-state-v1\n")
	for _, filePath := range paths {
		cleanPath, err := cleanRepositoryPath(filePath)
		if err != nil {
			return "", fmt.Errorf("worktree state contains an unsafe path")
		}
		cleanPath = filepath.ToSlash(cleanPath)
		fileStatus := status[filePath]
		fingerprint, err := worktreePathFingerprint(root, cleanPath)
		if err != nil {
			return "", fmt.Errorf("path %q could not be fingerprinted safely", cleanPath)
		}
		fmt.Fprintf(h, "%s\x00%c\x00%c\x00%s\n", cleanPath, fileStatus.Staging, fileStatus.Worktree, fingerprint)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// worktreePathFingerprint hashes the Git-relevant current state of one path.
// Symlink parents are rejected before filesystem access; a final symlink is
// hashed by link target rather than dereferenced, matching Git's object model.
func worktreePathFingerprint(repositoryRoot, cleanPath string) (string, error) {
	if err := validateStageFilesystemPath(repositoryRoot, cleanPath); err != nil {
		return "", err
	}
	fullPath := filepath.Join(repositoryRoot, filepath.FromSlash(cleanPath))
	if !pathWithinRoot(repositoryRoot, fullPath) {
		return "", fmt.Errorf("path escapes the configured repository root")
	}
	info, err := os.Lstat(fullPath)
	if os.IsNotExist(err) {
		return digestText("missing"), nil
	}
	if err != nil {
		return "", fmt.Errorf("path metadata could not be read")
	}

	h := sha256.New()
	fmt.Fprintf(h, "mode:%d\x00", uint32(info.Mode()))
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return "", fmt.Errorf("symlink target could not be read")
		}
		_, _ = io.WriteString(h, "symlink\x00")
		_, _ = io.WriteString(h, target)
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file or symlink")
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("path content could not be read")
	}
	defer file.Close()
	_, _ = io.WriteString(h, "file\x00")
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("path content could not be read")
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func digestText(value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", h[:])
}
