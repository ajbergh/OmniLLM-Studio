package gitrepo

import (
	"bytes"
	"context"
	"os"
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
	if diff.Mode != "worktree" || len(diff.WorktreeDigest) != 64 || !strings.Contains(diff.Patch, "-world") || !strings.Contains(diff.Patch, "+changed") || !strings.Contains(diff.Patch, "+new file") {
		t.Fatalf("unexpected worktree diff: %#v", diff)
	}
}

func TestServiceWorktreeDigestCoversOversizedOmittedContent(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "README.md"), "base\n")
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("initial", &git.CommitOptions{Author: testSignature()}); err != nil {
		t.Fatal(err)
	}

	large := bytes.Repeat([]byte{'x'}, maxDiffFileBytes+128)
	if err := os.WriteFile(filepath.Join(dir, "large.bin"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewService(map[string]string{"test": dir})
	first, err := svc.Diff(context.Background(), "test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(first.Warnings, "\n"), "exceeds") {
		t.Fatalf("expected oversized omission warning, got %#v", first.Warnings)
	}
	if len(first.WorktreeDigest) != 64 {
		t.Fatalf("first worktree digest length = %d, want 64", len(first.WorktreeDigest))
	}

	large[len(large)-1] = 'y'
	if err := os.WriteFile(filepath.Join(dir, "large.bin"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Diff(context.Background(), "test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.WorktreeDigest == first.WorktreeDigest {
		t.Fatal("worktree digest did not change when omitted oversized content changed")
	}
}

func TestServiceWorktreeDiffDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "README.md"), "base\n")
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("initial", &git.CommitOptions{Author: testSignature()}); err != nil {
		t.Fatal(err)
	}

	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	writeTestFile(t, secretPath, "do-not-leak-this-content\n")
	if err := os.Symlink(secretPath, filepath.Join(dir, "outside-link")); err != nil {
		t.Skipf("symlinks unavailable in test environment: %v", err)
	}

	diff, err := NewService(map[string]string{"test": dir}).Diff(context.Background(), "test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(diff.Patch, "do-not-leak-this-content") {
		t.Fatalf("worktree diff followed symlink outside repository: %q", diff.Patch)
	}
	if !strings.Contains(strings.Join(diff.Warnings, "\n"), "symlink content is not rendered") {
		t.Fatalf("expected symlink omission warning, got %#v", diff.Warnings)
	}
	if len(diff.WorktreeDigest) != 64 {
		t.Fatalf("symlink worktree digest length = %d, want 64", len(diff.WorktreeDigest))
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
	if diff.Mode != "revision" || diff.From != first.String() || diff.To != second.String() || diff.WorktreeDigest != "" {
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
