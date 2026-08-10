package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestServiceMutationsRequireExplicitWriteAccess(t *testing.T) {
	dir, _, _, head := setupWritableRepository(t)
	svc := NewService(map[string]string{"repo": dir})
	if svc.WriteEnabled() {
		t.Fatal("NewService unexpectedly enabled writes")
	}
	if _, err := svc.CreateBranch(context.Background(), "repo", "feature", "HEAD", head); err == nil {
		t.Fatal("CreateBranch() with writes disabled error = nil")
	}
}

func TestServiceGuardedBranchStageAndCommitFlow(t *testing.T) {
	dir, repo, _, initialHead := setupWritableRepository(t)
	svc := NewServiceWithWriteAccess(map[string]string{"repo": dir}, true)
	ctx := context.Background()

	initialStatus, err := svc.Status(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(initialStatus.IndexDigest) != 64 {
		t.Fatalf("initial index digest length = %d, want 64", len(initialStatus.IndexDigest))
	}

	created, err := svc.CreateBranch(ctx, "repo", "feature/test", "HEAD", initialHead)
	if err != nil {
		t.Fatal(err)
	}
	if created.Hash != initialHead || created.Branch != "feature/test" {
		t.Fatalf("unexpected branch result: %#v", created)
	}
	statusAfterCreate, err := svc.Status(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if statusAfterCreate.Branch == "feature/test" {
		t.Fatal("CreateBranch switched HEAD")
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "dirty\n")
	if _, err := svc.Checkout(ctx, "repo", "feature/test", initialHead); err == nil {
		t.Fatal("Checkout() dirty worktree error = nil")
	}
	writeTestFile(t, filepath.Join(dir, "hello.txt"), "base\n")
	checkedOut, err := svc.Checkout(ctx, "repo", "feature/test", initialHead)
	if err != nil {
		t.Fatal(err)
	}
	if checkedOut.Branch != "feature/test" || checkedOut.Head != initialHead {
		t.Fatalf("unexpected checkout result: %#v", checkedOut)
	}

	writeTestFile(t, filepath.Join(dir, "hello.txt"), "feature change\n")
	beforeStage, err := svc.Status(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Stage(ctx, "repo", []string{"hello.txt"}, strings.Repeat("0", 40)); err == nil {
		t.Fatal("Stage() stale expected_head error = nil")
	}
	staged, err := svc.Stage(ctx, "repo", []string{"hello.txt"}, beforeStage.Head)
	if err != nil {
		t.Fatal(err)
	}
	if staged.IndexDigest == beforeStage.IndexDigest {
		t.Fatal("Stage() did not change index digest")
	}

	// An unstaged file created after staging must not be swept into the commit.
	writeTestFile(t, filepath.Join(dir, "unstaged.txt"), "leave me unstaged\n")
	ready, err := svc.Status(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commit(ctx, "repo", "feature commit", ready.Head, strings.Repeat("0", 64)); err == nil {
		t.Fatal("Commit() stale index digest error = nil")
	}
	committed, err := svc.Commit(ctx, "repo", "feature commit", ready.Head, ready.IndexDigest)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Previous != initialHead || committed.Hash == initialHead || committed.Author != "Configured Author" {
		t.Fatalf("unexpected commit result: %#v", committed)
	}
	commit, err := repo.CommitObject(plumbing.NewHash(committed.Hash))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := commit.File("unstaged.txt"); err == nil {
		t.Fatal("Commit() included an unstaged file")
	}
	afterCommit, err := svc.Status(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if afterCommit.Head != committed.Hash || afterCommit.Clean {
		t.Fatalf("unexpected post-commit status: %#v", afterCommit)
	}
	if _, err := svc.CreateBranch(ctx, "repo", "stale", "HEAD", initialHead); err == nil {
		t.Fatal("CreateBranch() stale expected_head error = nil")
	}
}

func TestStageRejectsUnsafeAndNonFilePaths(t *testing.T) {
	dir, _, _, head := setupWritableRepository(t)
	svc := NewServiceWithWriteAccess(map[string]string{"repo": dir}, true)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "nested", "file.txt"), "nested\n")
	if _, err := svc.Stage(ctx, "repo", []string{"nested"}, head); err == nil {
		t.Fatal("Stage() directory error = nil")
	}
	if _, err := svc.Stage(ctx, "repo", []string{"../outside"}, head); err == nil {
		t.Fatal("Stage() traversal error = nil")
	}

	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "secret\n")
	link := filepath.Join(dir, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}
	if err := validateStageFilesystemPath(dir, "escape/secret.txt"); err == nil {
		t.Fatal("validateStageFilesystemPath() followed parent symlink outside repository")
	}
}

func setupWritableRepository(t *testing.T) (string, *git.Repository, *git.Worktree, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.User.Name = "Configured Author"
	cfg.User.Email = "configured@example.com"
	if err := repo.Storer.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "hello.txt"), "base\n")
	if _, err := worktree.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := worktree.Commit("initial", &git.CommitOptions{Author: &object.Signature{
		Name: "Configured Author", Email: "configured@example.com", When: time.Unix(1_700_000_000, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return dir, repo, worktree, hash.String()
}
