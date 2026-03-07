package repository_test

import (
	"database/sql"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// newTestDB creates an in-memory SQLite database with all migrations applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// ---------- Conversation Repo ----------

func TestConversationCRUD(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewConversationRepo(database)

	// Create
	convo, err := repo.Create("", "Test Chat", nil, nil, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if convo.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if convo.Title != "Test Chat" {
		t.Errorf("title: got %q, want 'Test Chat'", convo.Title)
	}
	if convo.Kind != models.ConversationKindChat {
		t.Errorf("kind: got %q, want %q", convo.Kind, models.ConversationKindChat)
	}

	// Get
	fetched, err := repo.GetByID(convo.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Title != "Test Chat" {
		t.Errorf("fetched title: got %q, want 'Test Chat'", fetched.Title)
	}

	// List
	list, err := repo.List("", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list count: got %d, want 1", len(list))
	}

	// Update
	newTitle := "Renamed"
	if _, err := repo.Update(convo.ID, "", repository.ConversationUpdate{Title: &newTitle}); err != nil {
		t.Fatalf("update: %v", err)
	}
	fetched, _ = repo.GetByID(convo.ID)
	if fetched.Title != "Renamed" {
		t.Errorf("updated title: got %q, want 'Renamed'", fetched.Title)
	}

	// Delete
	if err := repo.Delete(convo.ID, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ = repo.List("", false)
	if len(list) != 0 {
		t.Errorf("after delete: got %d conversations, want 0", len(list))
	}
}

func TestConversationListSeparatesImageSessions(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewConversationRepo(database)

	if _, err := repo.Create("", "Chat Conversation", nil, nil, nil); err != nil {
		t.Fatalf("create chat conversation: %v", err)
	}
	imageConvo, err := repo.CreateWithKind("", "Image Session", models.ConversationKindImage, nil, nil, nil)
	if err != nil {
		t.Fatalf("create image conversation: %v", err)
	}

	chatList, err := repo.List("", false)
	if err != nil {
		t.Fatalf("list chat conversations: %v", err)
	}
	if len(chatList) != 1 {
		t.Fatalf("chat list count: got %d, want 1", len(chatList))
	}
	if chatList[0].Kind != models.ConversationKindChat {
		t.Fatalf("chat list kind: got %q, want %q", chatList[0].Kind, models.ConversationKindChat)
	}

	imageList, err := repo.ListByKind("", false, models.ConversationKindImage)
	if err != nil {
		t.Fatalf("list image conversations: %v", err)
	}
	if len(imageList) != 1 {
		t.Fatalf("image list count: got %d, want 1", len(imageList))
	}
	if imageList[0].ID != imageConvo.ID {
		t.Fatalf("image list id: got %q, want %q", imageList[0].ID, imageConvo.ID)
	}
}

func TestConversationArchive(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewConversationRepo(database)

	convo, _ := repo.Create("", "Archivable", nil, nil, nil)

	// Archive
	archived := true
	if _, err := repo.Update(convo.ID, "", repository.ConversationUpdate{Archived: &archived}); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// Should not appear without include_archived
	list, _ := repo.List("", false)
	if len(list) != 0 {
		t.Errorf("expected 0 non-archived, got %d", len(list))
	}

	// Should appear with include_archived
	list, _ = repo.List("", true)
	if len(list) != 1 {
		t.Errorf("expected 1 with archived, got %d", len(list))
	}
}

// ---------- Settings Repo ----------

func TestSettingsCRUD(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewSettingsRepo(database)

	// Set
	if err := repo.Set("test_key", "test_value"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Get
	val, err := repo.Get("test_key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if val != "test_value" {
		t.Errorf("got %q, want 'test_value'", val)
	}

	// Get nonexistent
	val, err = repo.Get("nonexistent")
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}

	// SetMany
	if err := repo.SetMany(map[string]string{"k1": "v1", "k2": "v2"}); err != nil {
		t.Fatalf("set many: %v", err)
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if all["k1"] != "v1" || all["k2"] != "v2" {
		t.Errorf("SetMany: got %v", all)
	}

	// Delete
	if err := repo.Delete("k1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	val, _ = repo.Get("k1")
	if val != "" {
		t.Errorf("after delete expected empty, got %q", val)
	}
}

func TestSettingsTyped(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewSettingsRepo(database)

	input := models.AppSettings{
		WebSearchProvider: "brave",
		BraveAPIKey:       "my-key-123",
		JinaReaderEnabled: false,
		JinaReaderMaxLen:  5000,
	}

	if err := repo.SetTyped(input); err != nil {
		t.Fatalf("set typed: %v", err)
	}

	got, err := repo.GetTyped()
	if err != nil {
		t.Fatalf("get typed: %v", err)
	}

	if got.WebSearchProvider != input.WebSearchProvider {
		t.Errorf("WebSearchProvider: got %q, want %q", got.WebSearchProvider, input.WebSearchProvider)
	}
	if got.BraveAPIKey != input.BraveAPIKey {
		t.Errorf("BraveAPIKey: got %q, want %q", got.BraveAPIKey, input.BraveAPIKey)
	}
	if got.JinaReaderEnabled != input.JinaReaderEnabled {
		t.Errorf("JinaReaderEnabled: got %v, want %v", got.JinaReaderEnabled, input.JinaReaderEnabled)
	}
	if got.JinaReaderMaxLen != input.JinaReaderMaxLen {
		t.Errorf("JinaReaderMaxLen: got %d, want %d", got.JinaReaderMaxLen, input.JinaReaderMaxLen)
	}
}

// ---------- Message Repo ----------

func TestMessageCRUD(t *testing.T) {
	database := newTestDB(t)
	convoRepo := repository.NewConversationRepo(database)
	msgRepo := repository.NewMessageRepo(database)

	convo, _ := convoRepo.Create("", "Msg Test", nil, nil, nil)

	// Create
	msg, err := msgRepo.Create(&models.Message{
		ConversationID: convo.ID,
		Role:           "user",
		Content:        "Hello!",
	})
	if err != nil {
		t.Fatalf("create msg: %v", err)
	}
	if msg.ID == "" {
		t.Fatal("expected non-empty message ID")
	}

	// List
	msgs, err := msgRepo.ListByConversation(convo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Hello!" {
		t.Errorf("content: got %q, want 'Hello!'", msgs[0].Content)
	}

	// Delete
	if err := msgRepo.Delete(convo.ID, msg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	msgs, _ = msgRepo.ListByConversation(convo.ID)
	if len(msgs) != 0 {
		t.Errorf("after delete: got %d messages, want 0", len(msgs))
	}
}

// ---------- Attachment Repo ----------

func TestAttachmentCRUD(t *testing.T) {
	database := newTestDB(t)
	convoRepo := repository.NewConversationRepo(database)
	attachRepo := repository.NewAttachmentRepo(database)

	convo, _ := convoRepo.Create("", "Attach Test", nil, nil, nil)

	// Create
	a := &models.Attachment{
		ConversationID: convo.ID,
		Type:           "file",
		MimeType:       "text/plain",
		StoragePath:    "test-file.txt",
		Bytes:          42,
		MetadataJSON:   "{}",
	}
	if err := attachRepo.Create(a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID
	fetched, err := attachRepo.GetByID(a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected non-nil attachment")
	}
	if fetched.MimeType != "text/plain" {
		t.Errorf("mime: got %q, want 'text/plain'", fetched.MimeType)
	}

	// List
	list, err := attachRepo.ListByConversation(convo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list count: got %d, want 1", len(list))
	}

	// Delete
	if err := attachRepo.Delete(a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	fetched, _ = attachRepo.GetByID(a.ID)
	if fetched != nil {
		t.Error("expected nil after delete")
	}
}
