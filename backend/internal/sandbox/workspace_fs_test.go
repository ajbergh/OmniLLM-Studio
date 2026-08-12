package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceFSRejectsTraversalSymlinksAndGitMetadata(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadWrite); err != nil {
		t.Fatal(err)
	}
	fs, _ := NewWorkspaceFS(registry)

	for _, path := range []string{"../outside", "/etc/passwd", `C:\Windows\system.ini`, ".git/config"} {
		if _, _, _, err := fs.Resolve("user-1", "repo", path, true); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err == nil {
		if _, _, _, err := fs.Resolve("user-1", "repo", "escape/file.txt", true); err == nil {
			t.Fatal("expected symlinked parent to be rejected")
		}
	}
}

func TestWorkspaceFSWritePatchDeleteAndStaleProtection(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadWrite); err != nil {
		t.Fatal(err)
	}
	fs, _ := NewWorkspaceFS(registry)
	owner := OwnerScope{UserID: "user-1", ConversationID: "conversation-1"}

	created, err := fs.WriteFile(context.Background(), owner, "repo", "example.txt", []byte("alpha\nbeta\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Operation != "create" || created.AfterSHA256 == "" {
		t.Fatalf("create change = %#v", created)
	}
	_, sha, err := fs.ReadFile("user-1", "repo", "example.txt", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile(context.Background(), owner, "repo", "example.txt", []byte("new"), strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected stale write hash to be rejected")
	}

	patched, err := fs.ApplyExactPatch(context.Background(), owner, "repo", "example.txt", sha, []TextEdit{{OldText: "beta", NewText: "gamma"}})
	if err != nil {
		t.Fatal(err)
	}
	data, patchedSHA, err := fs.ReadFile("user-1", "repo", "example.txt", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alpha\ngamma\n" || patched.AfterSHA256 != patchedSHA {
		t.Fatalf("patched data=%q change=%#v", data, patched)
	}

	if _, err := fs.DeleteFile(context.Background(), owner, "repo", "example.txt", sha); err == nil {
		t.Fatal("expected stale delete hash to be rejected")
	}
	deleted, err := fs.DeleteFile(context.Background(), owner, "repo", "example.txt", patchedSHA)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.AfterExists {
		t.Fatalf("delete change = %#v", deleted)
	}
	if _, err := os.Stat(filepath.Join(root, "example.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file stat error = %v", err)
	}

	reverted, err := fs.RevertChange(context.Background(), owner, deleted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reverted.AfterExists {
		t.Fatalf("revert change = %#v", reverted)
	}
	data, _, err = fs.ReadFile("user-1", "repo", "example.txt", 1024)
	if err != nil || string(data) != "alpha\ngamma\n" {
		t.Fatalf("reverted data=%q err=%v", data, err)
	}
}

func TestWorkspaceFSRevertRefusesNewerContent(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadWrite); err != nil {
		t.Fatal(err)
	}
	fs, _ := NewWorkspaceFS(registry)
	owner := OwnerScope{UserID: "user-1"}
	change, err := fs.WriteFile(context.Background(), owner, "repo", "example.txt", []byte("first"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "example.txt"), []byte("external-newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.RevertChange(context.Background(), owner, change.ID); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale revert error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "example.txt"))
	if err != nil || string(data) != "external-newer" {
		t.Fatalf("newer content overwritten: %q err=%v", data, err)
	}
}

func TestWorkspaceFSRespectsNoDeleteMode(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadWriteNoDelete); err != nil {
		t.Fatal(err)
	}
	fs, _ := NewWorkspaceFS(registry)
	owner := OwnerScope{UserID: "user-1"}
	change, err := fs.WriteFile(context.Background(), owner, "repo", "example.txt", []byte("ok"), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.DeleteFile(context.Background(), owner, "repo", "example.txt", change.AfterSHA256); err == nil {
		t.Fatal("expected read_write_no_delete workspace to reject delete")
	}
}
