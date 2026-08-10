package gitrepo

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
)

func TestCloneQuotaFilesystemRejectsByteOverflowBeforeWrite(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 5, 10)
	file, err := fs.Create("data.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if n, err := file.Write([]byte("abcde")); err != nil || n != 5 {
		t.Fatalf("first Write() = (%d, %v), want (5, nil)", n, err)
	}
	if n, err := file.Write([]byte("x")); n != 0 || !errors.Is(err, errCloneStorageQuotaExceeded) {
		t.Fatalf("overflow Write() = (%d, %v), want quota error", n, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abcde" {
		t.Fatalf("file content = %q, want abcde", got)
	}
}

func TestCloneQuotaFilesystemChargesSparseLogicalExpansion(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 8, 10)
	file, err := fs.Create("sparse.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if _, err := file.Seek(8, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if n, err := file.Write([]byte("x")); n != 0 || !errors.Is(err, errCloneStorageQuotaExceeded) {
		t.Fatalf("sparse Write() = (%d, %v), want quota error", n, err)
	}
	info, err := fs.Stat("sparse.bin")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("sparse file size = %d, want 0 after rejected write", info.Size())
	}
}

func TestCloneQuotaFilesystemChargesTruncateExpansion(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 4, 10)
	file, err := fs.Create("expand.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := file.Truncate(4); err != nil {
		t.Fatalf("Truncate(4): %v", err)
	}
	if err := file.Truncate(5); !errors.Is(err, errCloneStorageQuotaExceeded) {
		t.Fatalf("Truncate(5) error = %v, want quota error", err)
	}
	info, err := fs.Stat("expand.bin")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 4 {
		t.Fatalf("file size = %d, want 4", info.Size())
	}
}

func TestCloneQuotaFilesystemBoundsEntryCreation(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 1024, 3)
	if err := fs.MkdirAll("a/b", 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	file, err := fs.Create("a/b/empty")
	if err != nil {
		t.Fatalf("Create third entry: %v", err)
	}
	_ = file.Close()
	if _, err := fs.Create("fourth"); !errors.Is(err, errCloneEntryQuotaExceeded) {
		t.Fatalf("Create fourth entry error = %v, want entry quota error", err)
	}
	if _, err := os.Stat(fs.Join(fs.Root(), "fourth")); !os.IsNotExist(err) {
		t.Fatalf("fourth entry exists after rejected creation: %v", err)
	}
}

func TestCloneQuotaFilesystemChrootSharesQuota(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 6, 10)
	if err := fs.MkdirAll(".git", 0o700); err != nil {
		t.Fatal(err)
	}
	childRaw, err := fs.Chroot(".git")
	if err != nil {
		t.Fatal(err)
	}
	child, ok := childRaw.(*cloneQuotaFilesystem)
	if !ok {
		t.Fatalf("child type = %T, want *cloneQuotaFilesystem", childRaw)
	}

	object, err := child.Create("object")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := object.Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	_ = object.Close()

	worktree, err := fs.Create("file")
	if err != nil {
		t.Fatal(err)
	}
	defer worktree.Close()
	if n, err := worktree.Write([]byte("xyz")); n != 0 || !errors.Is(err, errCloneStorageQuotaExceeded) {
		t.Fatalf("shared quota Write() = (%d, %v), want quota error", n, err)
	}
	bytes, entries := fs.quota.usage()
	if bytes != 4 {
		t.Fatalf("used bytes = %d, want 4", bytes)
	}
	if entries < 3 {
		t.Fatalf("used entries = %d, want at least 3", entries)
	}
}

func TestCloneQuotaFilesystemCountsSymlinkTargetBytesAndEntry(t *testing.T) {
	fs := newCloneQuotaFilesystem(osfs.New(t.TempDir()), 4, 1)
	if err := fs.Symlink("dest", "link"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if _, err := fs.Create("other"); !errors.Is(err, errCloneEntryQuotaExceeded) {
		t.Fatalf("second entry error = %v, want entry quota error", err)
	}
	bytes, entries := fs.quota.usage()
	if bytes != 4 || entries != 1 {
		t.Fatalf("usage = (%d bytes, %d entries), want (4, 1)", bytes, entries)
	}
}
