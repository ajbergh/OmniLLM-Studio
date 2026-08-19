package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initAgentWorktreeTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable unavailable")
	}
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	run("init")
	run("config", "user.name", "OmniLLM Test")
	run("config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-m", "initial")
	return root
}

func TestAgentWorktreeReviewAndPromotionAreDigestBound(t *testing.T) {
	repoRoot := initAgentWorktreeTestRepo(t)
	managerRoot := filepath.Join(t.TempDir(), "agent-worktrees")
	service := NewServiceWithWriteAccess(map[string]string{"omni": repoRoot}, true)
	manager, err := NewAgentWorktreeManager(service, managerRoot)
	if err != nil {
		t.Fatal(err)
	}
	owner := AgentWorktreeOwner{UserID: "user-1", WorkspaceID: "workspace-1", AgentRunID: "run-1"}
	worktree, err := manager.Create(context.Background(), "omni", owner, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	path, err := manager.InternalPath(worktree.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(worktree.ID, path) && strings.Contains(worktree.ID, string(filepath.Separator)) {
		t.Fatal("public worktree id unexpectedly resembles a path")
	}
	if err := os.WriteFile(filepath.Join(path, "tracked.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "new.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	review, err := manager.Review(context.Background(), worktree.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if review.PatchSHA256 == "" || !strings.Contains(review.Patch, "tracked.txt") || !strings.Contains(review.Patch, "new.txt") {
		t.Fatalf("unexpected review: %#v", review)
	}
	if err := manager.Promote(context.Background(), worktree.ID, owner, "deadbeef"); err == nil {
		t.Fatal("expected stale review digest to fail closed")
	}
	if err := manager.Promote(context.Background(), worktree.ID, owner, review.PatchSHA256); err != nil {
		t.Fatal(err)
	}
	tracked, err := os.ReadFile(filepath.Join(repoRoot, "tracked.txt"))
	if err != nil || string(tracked) != "changed\n" {
		t.Fatalf("tracked promotion = %q err=%v", tracked, err)
	}
	added, err := os.ReadFile(filepath.Join(repoRoot, "new.txt"))
	if err != nil || string(added) != "new\n" {
		t.Fatalf("new-file promotion = %q err=%v", added, err)
	}
}

func TestAgentWorktreeOwnerIsolation(t *testing.T) {
	repoRoot := initAgentWorktreeTestRepo(t)
	service := NewServiceWithWriteAccess(map[string]string{"omni": repoRoot}, true)
	manager, err := NewAgentWorktreeManager(service, filepath.Join(t.TempDir(), "agent-worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	owner := AgentWorktreeOwner{UserID: "user-1", AgentRunID: "run-1"}
	worktree, err := manager.Create(context.Background(), "omni", owner, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	other := AgentWorktreeOwner{UserID: "user-2", AgentRunID: "run-1"}
	if _, err := manager.InternalPath(worktree.ID, other); err == nil {
		t.Fatal("expected cross-owner worktree access to be rejected")
	}
}
