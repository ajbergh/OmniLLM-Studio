//go:build linux

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestLinuxWorkspaceReadRejectsFinalAndParentSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "final-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := readWorkspaceRegularFile(root, "final-link", 1024); err == nil {
		t.Fatal("expected final symlink to be rejected")
	}
	if err := os.Symlink(outside, filepath.Join(root, "parent-link")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readWorkspaceRegularFile(root, "parent-link/secret.txt", 1024); err == nil {
		t.Fatal("expected parent symlink to be rejected")
	}
}

// TestLinuxWorkspaceReadResistsParentSymlinkSwap repeatedly races a granted
// directory against an outside symlink. The read may succeed against the pinned
// in-workspace directory or fail while the name is being replaced, but it must
// never return bytes from the outside target.
func TestLinuxWorkspaceReadResistsParentSymlinkSwap(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	insideDir := filepath.Join(root, "safe")
	parkedDir := filepath.Join(root, "safe-parked")
	if err := os.Mkdir(insideDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(insideDir, "target.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "target.txt"), []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stop atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			if err := os.Rename(insideDir, parkedDir); err != nil {
				runtime.Gosched()
				continue
			}
			if err := os.Symlink(outside, insideDir); err == nil {
				_ = os.Remove(insideDir)
			}
			if err := os.Rename(parkedDir, insideDir); err != nil && !errors.Is(err, os.ErrNotExist) {
				_ = os.Remove(insideDir)
				_ = os.Rename(parkedDir, insideDir)
			}
			runtime.Gosched()
		}
	}()

	for i := 0; i < 2000; i++ {
		data, _, err := readWorkspaceRegularFile(root, "safe/target.txt", 1024)
		if err != nil {
			continue
		}
		if string(data) != "inside" {
			stop.Store(true)
			<-done
			t.Fatalf("workspace read escaped granted directory: %q", data)
		}
	}
	stop.Store(true)
	<-done
}
