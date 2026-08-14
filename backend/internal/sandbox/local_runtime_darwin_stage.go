//go:build darwin

package sandbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	maxDarwinStagedWorkspaceBytes = int64(256 << 20)
	maxDarwinStagedWorkspaceFiles = 20_000
)

func stageDarwinReadOnlyWorkspace(source, destination string) error {
	source, err := darwinCanonicalDirectory(source)
	if err != nil {
		return err
	}
	files := 0
	var bytesCopied int64
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("workspace staging path escapes source root")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace staging rejects symbolic link %q", relative)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace staging rejects symbolic link %q", relative)
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace staging rejects non-regular file %q", relative)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink > 1 {
			return fmt.Errorf("workspace staging rejects hard-linked file %q", relative)
		}
		files++
		if files > maxDarwinStagedWorkspaceFiles {
			return fmt.Errorf("workspace staging exceeds %d files", maxDarwinStagedWorkspaceFiles)
		}
		if info.Size() < 0 || bytesCopied+info.Size() > maxDarwinStagedWorkspaceBytes {
			return fmt.Errorf("workspace staging exceeds %d bytes", maxDarwinStagedWorkspaceBytes)
		}
		written, err := stageDarwinReadOnlyFile(path, target, relative, info)
		if err != nil {
			return err
		}
		bytesCopied += written
		return nil
	})
}

// stageDarwinReadOnlyFile copies one path only if the opened source is still the
// same regular inode observed by the directory walk. Keeping this check in a
// focused helper makes the source-swap defense deterministic to test in 13D.
func stageDarwinReadOnlyFile(path, target, relative string, observed os.FileInfo) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, err
	}
	input, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	openedInfo, statErr := input.Stat()
	if statErr != nil {
		_ = input.Close()
		return 0, statErr
	}
	if !os.SameFile(observed, openedInfo) || !openedInfo.Mode().IsRegular() {
		_ = input.Close()
		return 0, fmt.Errorf("workspace source changed while staging %q", relative)
	}
	mode := os.FileMode(0o400)
	if openedInfo.Mode()&0o111 != 0 {
		mode = 0o500
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = input.Close()
		return 0, err
	}
	written, copyErr := io.Copy(output, input)
	closeOutErr := output.Close()
	closeInErr := input.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if closeOutErr != nil {
		return 0, closeOutErr
	}
	if closeInErr != nil {
		return 0, closeInErr
	}
	if written != openedInfo.Size() {
		return 0, fmt.Errorf("workspace source changed size while staging %q", relative)
	}
	return written, nil
}

type darwinBoundedOutput struct {
	limit     int64
	buf       bytes.Buffer
	truncated bool
}

func (w *darwinBoundedOutput) Write(p []byte) (int, error) {
	original := len(p)
	remaining := w.limit - int64(w.buf.Len())
	if remaining <= 0 {
		w.truncated = w.truncated || original > 0
		return original, nil
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
		w.truncated = true
	}
	_, _ = w.buf.Write(p)
	return original, nil
}

func (w *darwinBoundedOutput) String() string { return w.buf.String() }
