package gitrepo

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	securejoin "github.com/cyphar/filepath-securejoin"
	git "github.com/go-git/go-git/v5"
)

type changedStatusPath struct {
	raw   string
	clean string
}

// worktreeStateDigest fingerprints the complete set of changed worktree paths,
// including their staged/worktree status codes, file modes, regular-file bytes,
// symlink targets, and deletions. It streams file content instead of buffering it
// so the digest can cover binary and oversized files that git_diff does not render.
func (s *Service) worktreeStateDigest(repositoryID string, status git.Status) (string, error) {
	root, ok := s.repositories[repositoryID]
	if !ok {
		return "", fmt.Errorf("repository %q is not configured", repositoryID)
	}
	paths := make([]changedStatusPath, 0, len(status))
	for filePath, fileStatus := range status {
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
		fileStatus := status[item.raw]
		fingerprint, err := worktreePathFingerprint(root, item.clean)
		if err != nil {
			return "", fmt.Errorf("path %q could not be fingerprinted safely", item.clean)
		}
		fmt.Fprintf(h, "%s\x00%c\x00%c\x00%s\n", item.clean, fileStatus.Staging, fileStatus.Worktree, fingerprint)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// worktreePathFingerprint hashes the Git-relevant current state of one path.
// The filesystem path is constructed from a securely resolved parent and a
// single basename. A final symlink is hashed by link target rather than
// dereferenced, matching Git's object model.
func worktreePathFingerprint(repositoryRoot, cleanPath string) (string, error) {
	fullPath, err := containedFingerprintPath(repositoryRoot, cleanPath)
	if err != nil {
		return "", err
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

// containedFingerprintPath returns a lexical path below a securely resolved
// parent. Symlinked parents are rejected instead of followed, and the basename
// cannot introduce another path component.
func containedFingerprintPath(repositoryRoot, cleanPath string) (string, error) {
	localPath := filepath.FromSlash(cleanPath)
	parentRelative := filepath.Dir(localPath)
	resolvedParent := repositoryRoot
	if parentRelative != "." {
		var err error
		resolvedParent, err = securejoin.SecureJoin(repositoryRoot, parentRelative)
		if err != nil {
			return "", fmt.Errorf("parent path could not be resolved within the repository")
		}
		lexicalParent := filepath.Join(repositoryRoot, parentRelative)
		relative, err := filepath.Rel(lexicalParent, resolvedParent)
		if err != nil || relative != "." {
			return "", fmt.Errorf("parent path contains a symlink and cannot be fingerprinted safely")
		}
	}
	base := filepath.Base(localPath)
	if base == "" || base == "." || base == ".." || filepath.Base(base) != base {
		return "", fmt.Errorf("path basename is invalid")
	}
	return filepath.Join(resolvedParent, base), nil
}

func digestText(value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", h[:])
}
