package repository_test

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestChunkRepoFTSFindsExactIdentifiersAndConversationScope(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	conversations := repository.NewConversationRepo(database)
	conversationA, err := conversations.Create("", "A", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	conversationB, err := conversations.Create("", "B", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	attachments := repository.NewAttachmentRepo(database)
	for _, attachment := range []*models.Attachment{
		{ID: "att-a", ConversationID: conversationA.ID, Type: "file", MimeType: "text/plain", StoragePath: "a.txt"},
		{ID: "att-b", ConversationID: conversationB.ID, Type: "file", MimeType: "text/plain", StoragePath: "b.txt"},
	} {
		if err := attachments.Create(attachment); err != nil {
			t.Fatal(err)
		}
	}
	chunks := repository.NewChunkRepo(database)
	if err := chunks.CreateBatch([]models.DocumentChunk{
		{ID: "target", AttachmentID: "att-a", ConversationID: conversationA.ID, Content: "Failure code OLLAMA-EMBED-429 requires retry-after handling."},
		{ID: "distractor", AttachmentID: "att-b", ConversationID: conversationB.ID, Content: "The same OLLAMA-EMBED-429 text exists in another conversation."},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := chunks.SearchFTSByConversation(conversationA.ID, "OLLAMA-EMBED-429", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Chunk.ID != "target" {
		t.Fatalf("unexpected scoped FTS results: %#v", hits)
	}
	for _, hit := range hits {
		if hit.Chunk.ConversationID != conversationA.ID {
			t.Fatalf("cross-scope result leaked: %#v", hit)
		}
	}
}
