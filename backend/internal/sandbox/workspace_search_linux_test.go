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

func TestLinuxWorkspaceSearchRejectsSymlinkCandidates(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("needle inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("needle outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	identity, err := captureWorkspaceRootIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	if err := enumerateWorkspaceSearchCandidates(root, identity, 1024, func(rel string, data []byte) bool {
		seen[rel] = string(data)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if seen["inside.txt"] != "needle inside" {
		t.Fatalf("expected inside candidate, got %#v", seen)
	}
	if _, ok := seen["linked.txt"]; ok {
		t.Fatal("final symlink was enumerated")
	}
	if _, ok := seen["linked-dir/secret.txt"]; ok {
		t.Fatal("symlinked directory was traversed")
	}
}

// TestLinuxWorkspaceSearchResistsParentSymlinkSwap races a candidate directory
// against an outside symlink. Search may see the pinned in-workspace directory
// or skip the name while it is changing, but candidate bytes must never come
// from the outside target.
func TestLinuxWorkspaceSearchResistsParentSymlinkSwap(t *testing.T) {
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
	identity, err := captureWorkspaceRootIdentity(root)
	if err != nil {
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

	for i := 0; i < 1000; i++ {
		err := enumerateWorkspaceSearchCandidates(root, identity, 1024, func(_ string, data []byte) bool {
			if string(data) == "outside-secret" {
				stop.Store(true)
			}
			return true
		})
		if err != nil {
			continue
		}
		if stop.Load() {
			<-done
			t.Fatal("workspace search escaped through swapped parent symlink")
		}
	}
	stop.Store(true)
	<-done
}

func TestLinuxWorkspaceSearchRejectsRootReplacementAfterGrantCheck(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	parked := filepath.Join(parent, "workspace-original")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := captureWorkspaceRootIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := enumerateWorkspaceSearchCandidates(root, identity, 1024, func(string, []byte) bool { return true }); err == nil {
		t.Fatal("expected replacement root identity to be rejected")
	}
}

func TestLinuxWorkspaceSearchPropagatesMatchLimitStop(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"a/one.txt", "a/two.txt", "b/three.txt"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("needle"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := captureWorkspaceRootIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	visits := 0
	if err := enumerateWorkspaceSearchCandidates(root, identity, 1024, func(string, []byte) bool {
		visits++
		return false
	}); err != nil {
		t.Fatal(err)
	}
	if visits != 1 {
		t.Fatalf("expected traversal to stop after one candidate, visited %d", visits)
	}
}
