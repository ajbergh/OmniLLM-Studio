package gitrepo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	git "github.com/go-git/go-git/v5"
)

type changedStatusPath struct {
	raw   string
	clean string
}

// worktreeStateDigest fingerprints a status snapshot using the configured
// repository's rooted worktree filesystem. The repository root comes only from
// the immutable operator configuration, never from model arguments.
func (s *Service) worktreeStateDigest(ctx context.Context, repositoryID string, status git.Status) (string, error) {
	repo, err := s.open(repositoryID)
	if err != nil {
		return "", err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return "", safeRepositoryError(repositoryID, "does not expose a worktree")
	}
	return s.worktreeStateDigestWithWorktree(ctx, repositoryID, worktree, status)
}

// worktreeStateDigestWithWorktree fingerprints the complete set of changed
// worktree paths, including staged/worktree status codes, file modes,
// regular-file bytes, symlink targets, and deletions. Content is streamed
// through the repository-rooted worktree filesystem so binary and oversized
// files remain covered without process-wide filesystem access.
func (s *Service) worktreeStateDigestWithWorktree(ctx context.Context, repositoryID string, worktree *git.Worktree, status git.Status) (string, error) {
	root, ok := s.repositories[repositoryID]
	if !ok {
		return "", fmt.Errorf("repository %q is not configured", repositoryID)
	}
	if worktree == nil || worktree.Filesystem == nil {
		return "", fmt.Errorf("repository worktree filesystem is unavailable")
	}
	paths := make([]changedStatusPath, 0, len(status))
	for filePath, fileStatus := range status {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if fileStatus.Staging == git.Unmodified && fileStatus.Worktree == git.Unmodified {
			continue
		}
		cleanPath, err := cleanRepositoryPath(filePath)
		if err != nil {
			return "", fmt.Errorf("worktree state contains an unsafe path")
		}
		paths = append(paths, changedStatusPath{raw: filePath, clean: filepath.ToSlash(cleanPath)})
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].clean < paths[j].clean })

	h := sha256.New()
	_, _ = io.WriteString(h, "omnillm-worktree-state-v1\n")
	for _, item := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fileStatus := status[item.raw]
		fingerprint, err := worktreePathFingerprintWithWorktree(ctx, worktree, root, item.clean)
		if err != nil {
			return "", fmt.Errorf("path %q could not be fingerprinted safely", item.clean)
		}
		fmt.Fprintf(h, "%s\x00%c\x00%c\x00%s\n", item.clean, fileStatus.Staging, fileStatus.Worktree, fingerprint)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// worktreePathFingerprint preserves the mutation service's existing call shape
// while ensuring final reads occur through a repository-rooted worktree
// filesystem. repositoryRoot is trusted operator configuration.
func worktreePathFingerprint(ctx context.Context, repositoryRoot, cleanPath string) (string, error) {
	repo, err := git.PlainOpen(repositoryRoot)
	if err != nil {
		return "", fmt.Errorf("repository worktree could not be opened")
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("repository does not expose a worktree")
	}
	return worktreePathFingerprintWithWorktree(ctx, worktree, repositoryRoot, cleanPath)
}

// worktreePathFingerprintWithWorktree hashes the Git-relevant current state of
// one path. Symlinked parent directories are rejected first. Final
// metadata/content reads use go-git's repository-rooted billy filesystem; a
// final symlink is hashed by target string rather than dereferenced.
func worktreePathFingerprintWithWorktree(ctx context.Context, worktree *git.Worktree, repositoryRoot, cleanPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if worktree == nil || worktree.Filesystem == nil {
		return "", fmt.Errorf("repository worktree filesystem is unavailable")
	}
	if err := validateStageFilesystemPath(repositoryRoot, cleanPath); err != nil {
		return "", err
	}
	localPath := filepath.FromSlash(cleanPath)
	info, err := worktree.Filesystem.Lstat(localPath)
	if os.IsNotExist(err) {
		return digestText("missing"), nil
	}
	if err != nil {
		return "", fmt.Errorf("path metadata could not be read")
	}

	h := sha256.New()
	fmt.Fprintf(h, "mode:%d\x00", uint32(info.Mode()))
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := worktree.Filesystem.Readlink(localPath)
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
	file, err := worktree.Filesystem.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("path content could not be read")
	}
	defer file.Close()
	_, _ = io.WriteString(h, "file\x00")
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := h.Write(buffer[:n]); err != nil {
				return "", fmt.Errorf("path content could not be hashed")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("path content could not be read")
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func digestText(value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", h[:])
}
