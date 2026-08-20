//go:build windows

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsWorkspaceGrantRejectsReplacedRootIdentity(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace, err := registry.Register("user-1", "repo", root, MountReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(workspace.RootIdentity, "windows:") {
		t.Fatalf("root identity = %q", workspace.RootIdentity)
	}

	detached := filepath.Join(parent, "workspace-original")
	if err := os.Rename(root, detached); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := registry.Get("user-1", "repo"); err == nil || !strings.Contains(err.Error(), "root identity changed") {
		t.Fatalf("Get() err = %v, want root identity mismatch", err)
	}
	listed, err := registry.List("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("List() exposed replaced grant: %#v", listed)
	}
	fs, err := NewWorkspaceFS(registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := fs.Resolve("user-1", "repo", "inside.txt", false); err == nil || !strings.Contains(err.Error(), "root identity changed") {
		t.Fatalf("Resolve() err = %v, want root identity mismatch", err)
	}
}

func TestWindowsWorkspaceLegacyGrantRequiresReregistration(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadOnly); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.db.Exec(`UPDATE sandbox_workspaces SET root_identity=NULL WHERE id=? AND owner_user_id=?`, "repo", "user-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Get("user-1", "repo"); err == nil || !strings.Contains(err.Error(), "predates durable root identity") {
		t.Fatalf("Get() err = %v, want legacy grant failure", err)
	}
	workspace, err := registry.Register("user-1", "repo", root, MountReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(workspace.RootIdentity, "windows:") {
		t.Fatalf("re-registration root identity = %q", workspace.RootIdentity)
	}
}

func TestWindowsWorkspaceReadAndSearchRejectReparseCandidates(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("needle inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("needle outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.txt")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Skipf("Windows runner cannot create symlink for reparse assurance: %v", err)
	}
	if _, _, err := readWorkspaceRegularFile(root, "linked.txt", 1024); err == nil {
		t.Fatal("expected reparse candidate read to fail closed")
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
		t.Fatalf("missing in-workspace candidate: %#v", seen)
	}
	if _, ok := seen["linked.txt"]; ok {
		t.Fatal("search followed a reparse candidate")
	}
}

func TestWindowsWorkspaceMutationTargetPinsRenamedParent(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "safe")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "victim.txt"), []byte("before"), 0o600); err != nil {
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
	detached := filepath.Join(root, "safe-renamed")
	if err := os.Rename(parent, detached); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "victim.txt"), []byte("replacement-path"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := target.Write([]byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(detached, "victim.txt")); err != nil || string(got) != "after" {
		t.Fatalf("pinned renamed parent not updated: %q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(parent, "victim.txt")); err != nil || string(got) != "replacement-path" {
		t.Fatalf("replacement pathname was modified: %q err=%v", got, err)
	}
	if err := target.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(detached, "victim.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected pinned target deletion, got %v", err)
	}
	if err := target.Restore(before); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(detached, "victim.txt")); err != nil || string(got) != "before" {
		t.Fatalf("restore did not target pinned parent: %q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(parent, "victim.txt")); err != nil || string(got) != "replacement-path" {
		t.Fatalf("replacement pathname changed after delete/restore: %q err=%v", got, err)
	}
}

func TestWindowsWorkspaceSearchRejectsRootReplacementAndStopsGlobally(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := captureWorkspaceRootIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	parked := filepath.Join(parent, "workspace-original")
	if err := os.Rename(root, parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := enumerateWorkspaceSearchCandidates(root, identity, 1024, func(string, []byte) bool { return true }); err == nil {
		t.Fatal("expected search to reject replacement root identity")
	}

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
		t.Fatalf("expected search to stop globally after one candidate, visited %d", visits)
	}
}
