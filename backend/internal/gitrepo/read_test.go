package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestServiceReadOnlyRepositoryOperations(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "hello\nworld\n")
	if _, err := worktree.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	firstHash, err := worktree.Commit("initial commit", &git.CommitOptions{Author: &object.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Unix(1_700_000_000, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "hello\nchanged\n")
	writeTestFile(t, filepath.Join(dir, "new.txt"), "new file\n")

	svc := NewService(map[string]string{"test": dir})
	ctx := context.Background()

	status, err := svc.Status(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if status.Clean || len(status.Files) != 2 {
		t.Fatalf("unexpected status: %#v", status)
	}

	logResult, err := svc.Log(ctx, "test", "HEAD", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logResult.Commits) != 1 || logResult.Commits[0].Hash != firstHash.String() {
		t.Fatalf("unexpected log: %#v", logResult)
	}

	shown, err := svc.Show(ctx, "test", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if shown.Hash != firstHash.String() || shown.Message != "initial commit" {
		t.Fatalf("unexpected show result: %#v", shown)
	}

	branches, err := svc.Branches(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches.Branches) != 1 || !branches.Branches[0].Current {
		t.Fatalf("unexpected branches: %#v", branches)
	}

	blame, err := svc.Blame(ctx, "test", "hello.txt", "HEAD", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(blame.Lines) != 2 || blame.Lines[1].Text != "world" || blame.Lines[1].Hash != firstHash.String() {
		t.Fatalf("unexpected blame: %#v", blame)
	}
}

func TestServiceDoesNotExposeConfiguredPathInErrors(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "private", "repo")
	svc := NewService(map[string]string{"secret": secretPath})
	_, err := svc.Status(context.Background(), "secret")
	if err == nil {
		t.Fatal("Status() error = nil, want unavailable repository error")
	}
	if strings.Contains(err.Error(), secretPath) {
		t.Fatalf("error leaked configured path: %v", err)
	}
}

func TestBlameRejectsEscapingPaths(t *testing.T) {
	svc := NewService(map[string]string{"repo": t.TempDir()})
	for _, value := range []string{"../secret", "/etc/passwd", ".git/config"} {
		if _, err := svc.Blame(context.Background(), "repo", value, "HEAD", 0, 0); err == nil {
			t.Fatalf("Blame(%q) error = nil, want containment error", value)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
