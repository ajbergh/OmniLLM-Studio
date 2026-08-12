package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
)

func newTestWorkspaceRegistry(t *testing.T) *WorkspaceRegistry {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(database) })
	registry, err := NewWorkspaceRegistry(database)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestWorkspaceRegistryScopesOpaqueIDsByOwner(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	workspace, err := registry.Register("user-1", "repo", root, MountReadWriteNoDelete)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ID != "repo" || workspace.Mode != MountReadWriteNoDelete {
		t.Fatalf("workspace = %#v", workspace)
	}
	if workspace.RootPath == "" || !filepath.IsAbs(workspace.RootPath) {
		t.Fatalf("canonical root = %q", workspace.RootPath)
	}
	if _, err := registry.Get("user-2", "repo"); err == nil {
		t.Fatal("cross-owner workspace lookup should fail")
	}
	listed, err := registry.List("user-1")
	if err != nil || len(listed) != 1 || listed[0].ID != "repo" {
		t.Fatalf("List() = %#v err=%v", listed, err)
	}
}

func TestWorkspaceRegistryRejectsInvalidGrant(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	for _, tc := range []struct {
		owner string
		id    string
		mode  MountMode
	}{
		{"", "repo", MountReadOnly},
		{"user-1", "../repo", MountReadOnly},
		{"user-1", "repo", "host_root"},
	} {
		if _, err := registry.Register(tc.owner, tc.id, root, tc.mode); err == nil {
			t.Fatalf("expected grant %#v to be rejected", tc)
		}
	}
}

func TestWorkspaceJournalStoresHashesWithoutHostPath(t *testing.T) {
	registry := newTestWorkspaceRegistry(t)
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, MountReadWrite); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "example.txt")
	if err := os.WriteFile(file, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := CaptureFileState(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := CaptureFileState(file)
	if err != nil {
		t.Fatal(err)
	}
	change, err := registry.RecordWorkspaceChange(context.Background(), OwnerScope{
		UserID: "user-1", ConversationID: "conversation-1", AgentRunID: "run-1",
	}, "repo", "example.txt", "write", "sbx-1", "exec-1", before, after)
	if err != nil {
		t.Fatal(err)
	}
	if change.BeforeSHA256 == "" || change.AfterSHA256 == "" || change.BeforeSHA256 == change.AfterSHA256 {
		t.Fatalf("change hashes = %#v", change)
	}
	if strings.Contains(change.RelativePath, root) || change.RelativePath != "example.txt" {
		t.Fatalf("journal leaked/changed host path: %#v", change)
	}
	if !change.Revertable {
		t.Fatal("small regular file should be revertable")
	}

	changes, err := registry.ListWorkspaceChanges(context.Background(), "user-1", "repo", 10)
	if err != nil || len(changes) != 1 || changes[0].ID != change.ID {
		t.Fatalf("ListWorkspaceChanges() = %#v err=%v", changes, err)
	}
	if other, err := registry.ListWorkspaceChanges(context.Background(), "user-2", "repo", 10); err != nil || len(other) != 0 {
		t.Fatalf("cross-owner changes = %#v err=%v", other, err)
	}
}

func TestCaptureFileStateBoundsRevertSnapshot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "large.bin")
	payload := make([]byte, maxRevertSnapshotBytes+1)
	if err := os.WriteFile(file, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := CaptureFileState(file)
	if err != nil {
		t.Fatal(err)
	}
	if state.SHA256 == "" || state.Revertable {
		t.Fatalf("large state = %#v", state)
	}
	if len(state.Content) != maxRevertSnapshotBytes {
		t.Fatalf("snapshot bytes = %d, want %d", len(state.Content), maxRevertSnapshotBytes)
	}
}
