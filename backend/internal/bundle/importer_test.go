package bundle

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

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
			ID:          "conversation-1",
			Title:       "Imported",
			CreatedAt:   now,
			UpdatedAt:   now,
			Kind:        models.ConversationKindChat,
			MetadataJSON: `{}`,
			WorkspaceID: &workspaceID,
			UserID:      &userID,
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
