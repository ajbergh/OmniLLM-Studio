package gitrepo

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestServiceWorktreeDiffIncludesTrackedAndUntrackedFiles(t *testing.T) {
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
	if _, err := worktree.Commit("initial", &git.CommitOptions{Author: testSignature()}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "hello.txt"), "hello\nchanged\n")
	writeTestFile(t, filepath.Join(dir, "new.txt"), "new file\n")

	diff, err := NewService(map[string]string{"test": dir}).Diff(context.Background(), "test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if diff.Mode != "worktree" || !strings.Contains(diff.Patch, "-world") || !strings.Contains(diff.Patch, "+changed") || !strings.Contains(diff.Patch, "+new file") {
		t.Fatalf("unexpected worktree diff: %#v", diff)
	}
}

func TestServiceRevisionDiff(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "one\n")
	if _, err := worktree.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	first, err := worktree.Commit("first", &git.CommitOptions{Author: testSignature()})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "two\n")
	if _, err := worktree.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	sig := testSignature()
	sig.When = sig.When.Add(time.Minute)
	second, err := worktree.Commit("second", &git.CommitOptions{Author: sig})
	if err != nil {
		t.Fatal(err)
	}

	diff, err := NewService(map[string]string{"test": dir}).Diff(context.Background(), "test", first.String(), second.String())
	if err != nil {
		t.Fatal(err)
	}
	if diff.Mode != "revision" || diff.From != first.String() || diff.To != second.String() {
		t.Fatalf("unexpected revision diff metadata: %#v", diff)
	}
	if !strings.Contains(diff.Patch, "-one") || !strings.Contains(diff.Patch, "+two") {
		t.Fatalf("unexpected revision patch: %q", diff.Patch)
	}
}

func testSignature() *object.Signature {
	return &object.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Unix(1_700_000_000, 0).UTC(),
	}
}
