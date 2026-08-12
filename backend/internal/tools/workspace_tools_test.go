package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

func setupWorkspaceTools(t *testing.T, mode sandbox.MountMode) (context.Context, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(database) })
	registry, err := sandbox.NewWorkspaceRegistry(database)
	if err != nil {
		t.Fatal(err)
	}
	sandbox.SetDefaultWorkspaceRegistry(registry)
	t.Cleanup(func() { sandbox.SetDefaultWorkspaceRegistry(nil) })
	root := t.TempDir()
	if _, err := registry.Register("user-1", "repo", root, mode); err != nil {
		t.Fatal(err)
	}
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-1", ConversationID: "conversation-1"})
	return ctx, root
}

func workspaceToolByName(t *testing.T, name string) Tool {
	t.Helper()
	for _, tool := range NewWorkspaceTools() {
		if tool.Definition().Name == name {
			return tool
		}
	}
	t.Fatalf("workspace tool %q not found", name)
	return nil
}

func TestWorkspaceToolsNeverExposeConfiguredHostRoot(t *testing.T) {
	ctx, root := setupWorkspaceTools(t, sandbox.MountReadWrite)
	result, err := workspaceToolByName(t, "workspace_list").Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content, root) {
		t.Fatalf("workspace_list exposed host root: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"id":"repo"`) || !strings.Contains(result.Content, `"mode":"read_write"`) {
		t.Fatalf("workspace_list result = %s", result.Content)
	}
}

func TestWorkspaceToolWriteReadPatchAndRevert(t *testing.T) {
	ctx, root := setupWorkspaceTools(t, sandbox.MountReadWrite)
	write := workspaceToolByName(t, "workspace_write")
	read := workspaceToolByName(t, "workspace_read")
	patch := workspaceToolByName(t, "workspace_apply_patch")
	revert := workspaceToolByName(t, "workspace_revert")

	written, err := write.Execute(ctx, json.RawMessage(`{"workspace_id":"repo","path":"main.go","content":"package main\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	var writeChange sandbox.WorkspaceChange
	if err := json.Unmarshal(written.Structured, &writeChange); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}

	readResult, err := read.Execute(ctx, json.RawMessage(`{"workspace_id":"repo","path":"main.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	var readPayload struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(readResult.Structured, &readPayload); err != nil {
		t.Fatal(err)
	}
	patchArgs, _ := json.Marshal(map[string]any{
		"workspace_id":    "repo",
		"path":            "main.go",
		"expected_sha256": readPayload.SHA256,
		"edits":           []map[string]string{{"old_text": "package main", "new_text": "package app"}},
	})
	patched, err := patch.Execute(ctx, patchArgs)
	if err != nil {
		t.Fatal(err)
	}
	var patchChange sandbox.WorkspaceChange
	if err := json.Unmarshal(patched.Structured, &patchChange); err != nil {
		t.Fatal(err)
	}

	revertArgs, _ := json.Marshal(map[string]string{"change_id": patchChange.ID})
	if _, err := revert.Execute(ctx, revertArgs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(data) != "package main\n" {
		t.Fatalf("reverted data=%q err=%v", data, err)
	}
}

func TestWorkspaceMutationDefinitionsAreHighRiskAskCandidates(t *testing.T) {
	for _, name := range []string{"workspace_write", "workspace_apply_patch", "workspace_delete", "workspace_revert"} {
		def := workspaceToolByName(t, name).Definition().Normalized()
		if def.Risk != RiskHigh || !def.SideEffecting || def.ReadOnly {
			t.Fatalf("%s definition = %#v", name, def)
		}
	}
}

func TestWorkspaceToolsRejectCrossOwnerAccess(t *testing.T) {
	_, _ = setupWorkspaceTools(t, sandbox.MountReadWrite)
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-2"})
	if _, err := workspaceToolByName(t, "workspace_read").Execute(ctx, json.RawMessage(`{"workspace_id":"repo","path":"main.go"}`)); err == nil {
		t.Fatal("expected cross-owner workspace read to fail")
	}
}
