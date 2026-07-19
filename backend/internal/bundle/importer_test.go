package bundle

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestImportAttachmentGeneratesSafeStoragePath(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO conversations (id) VALUES (?)`, "conversation-1"); err != nil {
		t.Fatal(err)
	}

	const archiveName = "escape.txt"
	const contents = "bundle attachment"
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("attachments/files/" + archiveName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}

	attachmentsDir := t.TempDir()
	attachment := &models.Attachment{
		ID:             "attachment-1",
		ConversationID: "conversation-1",
		Type:           "file",
		MimeType:       "text/plain",
		StoragePath:    "../" + archiveName,
		Bytes:          int64(len(contents)),
		CreatedAt:      time.Now().UTC(),
		MetadataJSON:   `{}`,
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	importer := NewImporter(database, attachmentsDir, nil)
	imported, err := importer.importAttachment(tx, reader, attachment, ImportSkip)
	if err != nil {
		t.Fatalf("importAttachment() error = %v", err)
	}
	if !imported {
		t.Fatal("expected attachment to be imported")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if attachment.StoragePath == archiveName || attachment.StoragePath == "../"+archiveName {
		t.Fatalf("storage path %q was derived from the archive name", attachment.StoragePath)
	}
	if filepath.Base(attachment.StoragePath) != attachment.StoragePath {
		t.Fatalf("storage path %q is not an opaque filename", attachment.StoragePath)
	}
	data, err := os.ReadFile(filepath.Join(attachmentsDir, attachment.StoragePath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != contents {
		t.Fatalf("attachment contents = %q, want %q", data, contents)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(attachmentsDir), archiveName)); !os.IsNotExist(err) {
		t.Fatalf("archive-controlled path was written outside the attachments directory: %v", err)
	}
}

func TestImportConversationRestoresCurrentFields(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}

	workspaceID := "workspace-1"
	userID := "user-1"
	parentID := "message-parent"
	now := time.Now().UTC().Truncate(time.Second)
	bundle := &ConversationBundle{
		Conversation: models.Conversation{
			ID:           "conversation-1",
			Title:        "Imported",
			CreatedAt:    now,
			UpdatedAt:    now,
			Kind:         models.ConversationKindChat,
			MetadataJSON: `{}`,
			WorkspaceID:  &workspaceID,
			UserID:       &userID,
		},
		Messages: []models.Message{
			{
				ID:             parentID,
				ConversationID: "conversation-1",
				Role:           "user",
				Content:        "parent",
				CreatedAt:      now,
				MetadataJSON:   `{}`,
				BranchID:       "main",
				UserID:         &userID,
			},
			{
				ID:              "message-child",
				ConversationID:  "conversation-1",
				Role:            "assistant",
				Content:         "child",
				CreatedAt:       now,
				MetadataJSON:    `{}`,
				BranchID:        "branch-1",
				ParentMessageID: &parentID,
				UserID:          &userID,
			},
		},
	}

	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	importer := NewImporter(database, t.TempDir(), nil)
	imported, err := importer.importConversation(tx, bundle, ImportSkip)
	if err != nil {
		t.Fatalf("importConversation() error = %v", err)
	}
	if !imported {
		t.Fatal("expected conversation to be imported")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var gotWorkspace, gotUser string
	if err := database.QueryRow(`SELECT workspace_id, user_id FROM conversations WHERE id = ?`, bundle.Conversation.ID).Scan(&gotWorkspace, &gotUser); err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != workspaceID || gotUser != userID {
		t.Fatalf("conversation ownership = (%q, %q), want (%q, %q)", gotWorkspace, gotUser, workspaceID, userID)
	}

	var gotBranch, gotParent, gotMessageUser string
	if err := database.QueryRow(`SELECT branch_id, parent_message_id, user_id FROM messages WHERE id = ?`, "message-child").Scan(&gotBranch, &gotParent, &gotMessageUser); err != nil {
		t.Fatal(err)
	}
	if gotBranch != "branch-1" || gotParent != parentID || gotMessageUser != userID {
		t.Fatalf("message linkage = (%q, %q, %q)", gotBranch, gotParent, gotMessageUser)
	}
}
