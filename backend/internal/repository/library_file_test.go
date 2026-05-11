package repository_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func strPtr(s string) *string {
	return &s
}

func seedUser(t *testing.T, repoDB *sql.DB, id, username string) {
	t.Helper()
	if _, err := repoDB.Exec(
		"INSERT INTO users (id, username, display_name, password_hash, role) VALUES (?, ?, ?, ?, 'member')",
		id, username, username, "test-hash",
	); err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func TestLibraryFileRepoCRUDAndStatus(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewLibraryFileRepo(database)
	seedUser(t, database, "user-1", "user1")

	f := &models.LibraryFile{
		OwnerUserID:  strPtr("user-1"),
		SourceType:   "attachment",
		Scope:        "conversation",
		DisplayName:  "Design Notes",
		MimeType:     strPtr("text/markdown"),
		FileExt:      strPtr("md"),
		SizeBytes:    128,
		MetadataJSON: `{"topic":"architecture"}`,
	}
	if err := repo.Create(f); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.ID == "" {
		t.Fatal("expected id to be set")
	}

	got, err := repo.GetByID(f.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil {
		t.Fatal("expected file record")
	}
	if got.DisplayName != "Design Notes" {
		t.Fatalf("display name mismatch: got %q", got.DisplayName)
	}

	indexedAt := time.Now().UTC()
	if err := repo.UpdateStatus(f.ID, "indexed", nil, &indexedAt); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err = repo.GetByID(f.ID)
	if err != nil {
		t.Fatalf("get after update status: %v", err)
	}
	if got.Status != "indexed" {
		t.Fatalf("status mismatch: got %q", got.Status)
	}
	if got.IndexedAt == nil {
		t.Fatal("expected indexed_at to be set")
	}

	if err := repo.MarkDeleted(f.ID); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	got, err = repo.GetByID(f.ID)
	if err != nil {
		t.Fatalf("get after mark deleted: %v", err)
	}
	if got.Status != "deleted" {
		t.Fatalf("expected deleted status, got %q", got.Status)
	}

	if err := repo.Delete(f.ID); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	got, err = repo.GetByID(f.ID)
	if err != nil {
		t.Fatalf("get after hard delete: %v", err)
	}
	if got != nil {
		t.Fatal("expected no record after hard delete")
	}
}

func TestLibraryFileRepoListSearchAndChecksum(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewLibraryFileRepo(database)
	seedUser(t, database, "user-1", "user1")
	seedUser(t, database, "user-2", "user2")

	checksum := "abc123"

	conversationFile := &models.LibraryFile{
		OwnerUserID:    strPtr("user-1"),
		SourceType:     "attachment",
		Scope:          "conversation",
		DisplayName:    "Kyndryl Notes",
		MetadataJSON:   `{"project":"Kyndryl"}`,
		ChecksumSHA256: &checksum,
	}
	workspaceFile := &models.LibraryFile{
		OwnerUserID:  strPtr("user-1"),
		SourceType:   "upload",
		Scope:        "workspace",
		DisplayName:  "DXC Backup Requirements",
		MetadataJSON: `{"project":"DXC"}`,
	}
	otherUserFile := &models.LibraryFile{
		OwnerUserID:  strPtr("user-2"),
		SourceType:   "upload",
		Scope:        "global",
		DisplayName:  "Other User File",
		MetadataJSON: `{}`,
	}

	for _, f := range []*models.LibraryFile{conversationFile, workspaceFile, otherUserFile} {
		if err := repo.Create(f); err != nil {
			t.Fatalf("create %q: %v", f.DisplayName, err)
		}
	}

	list, err := repo.ListByScope("user-1", "conversation", nil, nil)
	if err != nil {
		t.Fatalf("list conversation scope: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 conversation scope file, got %d", len(list))
	}
	if list[0].DisplayName != "Kyndryl Notes" {
		t.Fatalf("unexpected list result: %q", list[0].DisplayName)
	}

	searchResults, err := repo.SearchMetadata("user-1", "dxc", []string{"workspace", "global"})
	if err != nil {
		t.Fatalf("search metadata: %v", err)
	}
	if len(searchResults) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchResults))
	}
	if searchResults[0].DisplayName != "DXC Backup Requirements" {
		t.Fatalf("unexpected search result: %q", searchResults[0].DisplayName)
	}

	byChecksum, err := repo.GetByChecksum("user-1", "conversation", checksum)
	if err != nil {
		t.Fatalf("get by checksum: %v", err)
	}
	if byChecksum == nil {
		t.Fatal("expected checksum match")
	}
	if byChecksum.DisplayName != "Kyndryl Notes" {
		t.Fatalf("unexpected checksum result: %q", byChecksum.DisplayName)
	}

	missing, err := repo.GetByChecksum("user-1", "workspace", checksum)
	if err != nil {
		t.Fatalf("get missing by checksum: %v", err)
	}
	if missing != nil {
		t.Fatal("expected no checksum match for mismatched scope")
	}
}
