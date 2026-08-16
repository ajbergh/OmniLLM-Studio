//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxWorkspaceGrantRejectsReplacedRootIdentity(t *testing.T) {
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
	if workspace.RootIdentity == "" || !strings.HasPrefix(workspace.RootIdentity, "linux:") {
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
	outside, err := os.ReadFile(filepath.Join(root, "inside.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(outside) != "replacement" {
		t.Fatalf("replacement root content changed: %q", outside)
	}
}

func TestLinuxWorkspaceLegacyGrantRequiresReregistration(t *testing.T) {
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
	listed, err := registry.List("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("List() exposed legacy grant: %#v", listed)
	}

	workspace, err := registry.Register("user-1", "repo", root, MountReadOnly)
	if err != nil {
		t.Fatalf("trusted re-registration failed: %v", err)
	}
	if workspace.RootIdentity == "" {
		t.Fatal("re-registration did not persist root identity")
	}
}
