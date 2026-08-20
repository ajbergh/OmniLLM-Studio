//go:build !windows

package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAgentWorktreeSnapshotDoesNotRunHooksOrSmudgeFilters(t *testing.T) {
	repoRoot := initAgentWorktreeTestRepo(t)
	markerDir := t.TempDir()
	hookMarker := filepath.Join(markerDir, "hook-ran")
	filterMarker := filepath.Join(markerDir, "filter-ran")

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hook := filepath.Join(hooksDir, "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nprintf ran > \""+hookMarker+"\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	attributes := "filtered.txt filter=omnillm-malicious\n"
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitattributes"), []byte(attributes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "filtered.txt"), []byte("raw\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	// Configure a repository-local smudge command that would create a marker if
	// a trusted Git checkout were used. Snapshot creation must read commit blobs
	// directly through go-git, so this command is never invoked.
	run("config", "filter.omnillm-malicious.smudge", "sh -c 'printf ran > "+filterMarker+"; cat'")
	run("config", "filter.omnillm-malicious.clean", "cat")
	run("add", ".gitattributes", "filtered.txt")
	run("commit", "-m", "add filtered fixture")
	_ = os.Remove(hookMarker)
	_ = os.Remove(filterMarker)

	service := NewServiceWithWriteAccess(map[string]string{"omni": repoRoot}, true)
	manager, err := NewAgentWorktreeManager(service, filepath.Join(t.TempDir(), "agent-worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	owner := AgentWorktreeOwner{UserID: "user-1", AgentRunID: "run-1"}
	created, err := manager.Create(context.Background(), "omni", owner, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	path, err := manager.InternalPath(created.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
		t.Fatalf("sandbox-visible snapshot unexpectedly contains .git: %v", err)
	}
	if _, err := os.Stat(hookMarker); !os.IsNotExist(err) {
		t.Fatalf("repository post-checkout hook executed during snapshot creation: %v", err)
	}
	if _, err := os.Stat(filterMarker); !os.IsNotExist(err) {
		t.Fatalf("repository smudge filter executed during snapshot creation: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(path, "filtered.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "raw\n" {
		t.Fatalf("snapshot did not preserve committed blob bytes: %q", contents)
	}
}
