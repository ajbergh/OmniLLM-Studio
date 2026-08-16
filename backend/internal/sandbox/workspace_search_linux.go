//go:build linux

package sandbox

import (
	"fmt"
	"io"
	"os"
	"sort"

	"golang.org/x/sys/unix"
)

// enumerateWorkspaceSearchCandidates keeps directory enumeration and candidate
// reads on one descriptor-relative lineage rooted in the registered filesystem
// object. Names may be concurrently changed, but traversal never follows a
// symlink and candidate bytes always come from the file descriptor opened from
// the directory descriptor that produced the candidate.
func enumerateWorkspaceSearchCandidates(root, expectedRootIdentity string, maxBytes int64, visit func(string, []byte) bool) error {
	if maxBytes <= 0 {
		return fmt.Errorf("workspace search read limit must be positive")
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open workspace search root: %w", err)
	}
	defer unix.Close(rootFD)

	var rootStat unix.Stat_t
	if err := unix.Fstat(rootFD, &rootStat); err != nil {
		return fmt.Errorf("stat workspace search root: %w", err)
	}
	if rootStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("workspace search root is not a directory")
	}
	currentIdentity := fmt.Sprintf("linux:%d:%d", uint64(rootStat.Dev), uint64(rootStat.Ino))
	if expectedRootIdentity == "" || currentIdentity != expectedRootIdentity {
		return fmt.Errorf("workspace root identity changed before search")
	}

	return walkWorkspaceSearchDirectory(rootFD, "", maxBytes, visit)
}

func walkWorkspaceSearchDirectory(dirFD int, prefix string, maxBytes int64, visit func(string, []byte) bool) error {
	dupFD, err := unix.Dup(dirFD)
	if err != nil {
		return fmt.Errorf("duplicate workspace search directory: %w", err)
	}
	dir := os.NewFile(uintptr(dupFD), "workspace-search-dir")
	if dir == nil {
		_ = unix.Close(dupFD)
		return fmt.Errorf("open workspace search directory handle")
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." || (prefix == "" && name == ".git") {
			continue
		}
		rel := name
		if prefix != "" {
			rel = prefix + "/" + name
		}

		var stat unix.Stat_t
		if err := unix.Fstatat(dirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			continue
		}
		switch stat.Mode & unix.S_IFMT {
		case unix.S_IFLNK:
			continue
		case unix.S_IFDIR:
			childFD, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				continue
			}
			if err := walkWorkspaceSearchDirectory(childFD, rel, maxBytes, visit); err != nil {
				_ = unix.Close(childFD)
				return err
			}
			_ = unix.Close(childFD)
		case unix.S_IFREG:
			fileFD, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				continue
			}
			file := os.NewFile(uintptr(fileFD), name)
			if file == nil {
				_ = unix.Close(fileFD)
				continue
			}
			var opened unix.Stat_t
			if err := unix.Fstat(fileFD, &opened); err != nil || opened.Mode&unix.S_IFMT != unix.S_IFREG || opened.Size > maxBytes {
				_ = file.Close()
				continue
			}
			data, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
			_ = file.Close()
			if readErr != nil || int64(len(data)) > maxBytes {
				continue
			}
			if !visit(rel, data) {
				return nil
			}
		}
	}
	return nil
}
