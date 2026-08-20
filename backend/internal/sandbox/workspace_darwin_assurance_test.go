//go:build darwin

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestDarwinWorkspaceReadRejectsFinalAndParentSymlinks(t *testing.T) {
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

func TestDarwinWorkspaceReadResistsParentSymlinkSwap(t *testing.T) {
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
	defer func() {
		stop.Store(true)
		<-done
	}()

	for i := 0; i < 2000; i++ {
		data, _, err := readWorkspaceRegularFile(root, "safe/target.txt", 1024)
		if err != nil {
			continue
		}
		if string(data) != "inside" {
			t.Fatalf("workspace read escaped granted directory: %q", data)
		}
	}
}

func TestDarwinWorkspaceMutationTargetPinsParentDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	parent := filepath.Join(root, "safe")
	detached := filepath.Join(root, "safe-detached")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	insidePath := filepath.Join(parent, "victim.txt")
	outsidePath := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(insidePath, []byte("inside-old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	target, err := openWorkspaceMutationTarget(root, "safe/victim.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	before, err := target.Capture()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(parent, detached); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}

	if err := target.Write([]byte("inside-new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(detached, "victim.txt")); err != nil || string(got) != "inside-new" {
		t.Fatalf("pinned directory was not updated: %q err=%v", got, err)
	}
	if got, err := os.ReadFile(outsidePath); err != nil || string(got) != "outside" {
		t.Fatalf("outside file changed after write: %q err=%v", got, err)
	}

	if err := target.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(detached, "victim.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected pinned file deletion, got %v", err)
	}
	if got, err := os.ReadFile(outsidePath); err != nil || string(got) != "outside" {
		t.Fatalf("outside file changed after delete: %q err=%v", got, err)
	}

	if err := target.Restore(before); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(detached, "victim.txt")); err != nil || string(got) != "inside-old" {
		t.Fatalf("unexpected restored content %q err=%v", got, err)
	}
	if got, err := os.ReadFile(outsidePath); err != nil || string(got) != "outside" {
		t.Fatalf("outside file changed after restore: %q err=%v", got, err)
	}
}

func TestDarwinWorkspaceMutationTargetRejectsFinalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	target, err := openWorkspaceMutationTarget(root, "link.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.Capture(); err == nil {
		t.Fatal("expected final symlink capture to fail closed")
	}
}

func TestDarwinWorkspaceSearchRejectsSymlinkCandidatesAndRootReplacement(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	outside := t.TempDir()
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
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

	parked := filepath.Join(parent, "workspace-original")
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

func TestDarwinWorkspaceSearchResistsParentSymlinkSwapAndStopsGlobally(t *testing.T) {
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
		escaped := false
		_ = enumerateWorkspaceSearchCandidates(root, identity, 1024, func(_ string, data []byte) bool {
			if string(data) == "outside-secret" {
				escaped = true
			}
			return true
		})
		if escaped {
			stop.Store(true)
			<-done
			t.Fatal("workspace search escaped through swapped parent symlink")
		}
	}
	stop.Store(true)
	<-done

	cleanRoot := t.TempDir()
	for _, rel := range []string{"a/one.txt", "a/two.txt", "b/three.txt"} {
		path := filepath.Join(cleanRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("needle"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cleanIdentity, err := captureWorkspaceRootIdentity(cleanRoot)
	if err != nil {
		t.Fatal(err)
	}
	visits := 0
	if err := enumerateWorkspaceSearchCandidates(cleanRoot, cleanIdentity, 1024, func(string, []byte) bool {
		visits++
		return false
	}); err != nil {
		t.Fatal(err)
	}
	if visits != 1 {
		t.Fatalf("expected traversal to stop after one candidate, visited %d", visits)
	}
}
