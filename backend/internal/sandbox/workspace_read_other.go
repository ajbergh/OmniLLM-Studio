//go:build !linux && !darwin

package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// readWorkspaceRegularFile preserves the existing pathname-based workspace read
// behavior on platforms without a proven descriptor-relative implementation.
func readWorkspaceRegularFile(root, relativePath string, maxBytes int64) ([]byte, string, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(root, filepath.FromSlash(clean))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("workspace path is not a regular file")
	}
	if info.Size() > maxBytes {
		return nil, "", fmt.Errorf("workspace file exceeds %d bytes", maxBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("workspace file exceeds %d bytes", maxBytes)
	}
	hash := sha256.Sum256(data)
	return data, hex.EncodeToString(hash[:]), nil
}
