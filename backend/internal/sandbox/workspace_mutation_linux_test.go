//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxWorkspaceMutationTargetPinsParentDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	parent := filepath.Join(root, "safe")
	detached := filepath.Join(root, "safe-detached")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	insidePath := filepath.Join(parent, "victim.txt")
	outsidePath := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(insidePath, []byte("inside-old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	target, err := openWorkspaceMutationTarget(root, "safe/victim.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	before, err := target.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if string(before.Content) != "inside-old" {
		t.Fatalf("unexpected initial content %q", before.Content)
	}

	if err := os.Rename(parent, detached); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}

	if err := target.Write([]byte("inside-new"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotInside, err := os.ReadFile(filepath.Join(detached, "victim.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotInside) != "inside-new" {
		t.Fatalf("pinned directory was not updated: %q", gotInside)
	}
	gotOutside, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOutside) != "outside" {
		t.Fatalf("outside file was modified through swapped parent: %q", gotOutside)
	}

	if err := target.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(detached, "victim.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected pinned file deletion, got %v", err)
	}
	gotOutside, err = os.ReadFile(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOutside) != "outside" {
		t.Fatalf("outside file changed after pinned delete: %q", gotOutside)
	}

	if err := target.Restore(before); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(detached, "victim.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != "inside-old" {
		t.Fatalf("unexpected restored content %q", restored)
	}
	gotOutside, err = os.ReadFile(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOutside) != "outside" {
		t.Fatalf("outside file changed after pinned restore: %q", gotOutside)
	}
}

func TestLinuxWorkspaceMutationTargetRejectsFinalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	target, err := openWorkspaceMutationTarget(root, "link.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.Capture(); err == nil {
		t.Fatal("expected final symlink capture to fail closed")
	}
}
