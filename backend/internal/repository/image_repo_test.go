package repository_test

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// ---------- ImageSession Repo ----------

func TestImageSessionCRUD(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewImageSessionRepo(database)

	// Create a conversation first (needed for FK).
	convoRepo := repository.NewConversationRepo(database)
	convo, err := convoRepo.Create("", "Test Conversation", nil, nil, nil)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	// Create session
	session, err := repo.Create(convo.ID, "My Session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.Title != "My Session" {
		t.Errorf("got title %q, want %q", session.Title, "My Session")
	}
	if session.ActiveNodeID != nil {
		t.Errorf("expected nil active node, got %v", session.ActiveNodeID)
	}

	// Get by ID
	got, err := repo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("got id %q, want %q", got.ID, session.ID)
	}

	// List by conversation
	sessions, err := repo.ListByConversation(convo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}

	// Update active node
	if err := repo.UpdateActiveNode(session.ID, "fake-node-id"); err != nil {
		t.Fatalf("update active node: %v", err)
	}
	got, _ = repo.GetByID(session.ID)
	if got.ActiveNodeID == nil || *got.ActiveNodeID != "fake-node-id" {
		t.Errorf("active node not updated")
	}

	// Delete
	if err := repo.Delete(session.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	sessions, _ = repo.ListByConversation(convo.ID)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
}

// ---------- ImageNode Repo ----------

func TestImageNodeCRUD(t *testing.T) {
	database := newTestDB(t)
	sessionRepo := repository.NewImageSessionRepo(database)
	nodeRepo := repository.NewImageNodeRepo(database)
	convoRepo := repository.NewConversationRepo(database)

	convo, _ := convoRepo.Create("", "Test", nil, nil, nil)
	session, _ := sessionRepo.Create(convo.ID, "Session")

	// Create root node
	root := &models.ImageNode{
		SessionID:     session.ID,
		OperationType: "generate",
		Instruction:   "A red car",
		Provider:      "openai",
		Model:         "dall-e-3",
	}
	if err := nodeRepo.Create(root); err != nil {
		t.Fatalf("create root node: %v", err)
	}
	if root.ID == "" {
		t.Fatal("expected ID to be set")
	}

	// Get by ID
	got, err := nodeRepo.GetByID(root.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Instruction != "A red car" {
		t.Errorf("got instruction %q, want %q", got.Instruction, "A red car")
	}

	// Create child node
	child := &models.ImageNode{
		SessionID:     session.ID,
		ParentNodeID:  &root.ID,
		OperationType: "edit",
		Instruction:   "Make it blue",
		Provider:      "openai",
		Model:         "dall-e-3",
	}
	if err := nodeRepo.Create(child); err != nil {
		t.Fatalf("create child node: %v", err)
	}

	// List by session
	nodes, err := nodeRepo.ListBySession(session.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}

	// Get children
	children, err := nodeRepo.GetChildren(root.ID)
	if err != nil {
		t.Fatalf("get children: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("got %d children, want 1", len(children))
	}
	if children[0].ID != child.ID {
		t.Errorf("child ID mismatch")
	}
}

// ---------- ImageNodeAsset Repo ----------

func TestImageNodeAssetCRUD(t *testing.T) {
	database := newTestDB(t)
	sessionRepo := repository.NewImageSessionRepo(database)
	nodeRepo := repository.NewImageNodeRepo(database)
	assetRepo := repository.NewImageNodeAssetRepo(database)
	convoRepo := repository.NewConversationRepo(database)
	attachRepo := repository.NewAttachmentRepo(database)

	convo, _ := convoRepo.Create("", "Test", nil, nil, nil)
	session, _ := sessionRepo.Create(convo.ID, "Session")

	node := &models.ImageNode{SessionID: session.ID, OperationType: "generate", Instruction: "test"}
	nodeRepo.Create(node)

	// Create an attachment for the asset
	att := &models.Attachment{ConversationID: convo.ID, Type: "image", MimeType: "image/png", StoragePath: "test.png", Bytes: 1024}
	if err := attachRepo.Create(att); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	// Create assets
	a1 := &models.ImageNodeAsset{NodeID: node.ID, AttachmentID: att.ID, VariantIndex: 0, IsSelected: true}
	if err := assetRepo.Create(a1); err != nil {
		t.Fatalf("create asset 1: %v", err)
	}

	att2 := &models.Attachment{ConversationID: convo.ID, Type: "image", MimeType: "image/png", StoragePath: "test2.png", Bytes: 1024}
	attachRepo.Create(att2)
	a2 := &models.ImageNodeAsset{NodeID: node.ID, AttachmentID: att2.ID, VariantIndex: 1, IsSelected: false}
	if err := assetRepo.Create(a2); err != nil {
		t.Fatalf("create asset 2: %v", err)
	}

	// List
	assets, err := assetRepo.ListByNode(node.ID)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("got %d assets, want 2", len(assets))
	}

	// Set selected
	if err := assetRepo.SetSelected(node.ID, a2.ID); err != nil {
		t.Fatalf("set selected: %v", err)
	}
	assets, _ = assetRepo.ListByNode(node.ID)
	for _, a := range assets {
		if a.ID == a2.ID && !a.IsSelected {
			t.Error("a2 should be selected")
		}
		if a.ID == a1.ID && a.IsSelected {
			t.Error("a1 should be deselected")
		}
	}

	// Delete by node
	if err := assetRepo.DeleteByNode(node.ID); err != nil {
		t.Fatalf("delete by node: %v", err)
	}
	assets, _ = assetRepo.ListByNode(node.ID)
	if len(assets) != 0 {
		t.Errorf("expected 0 assets after delete, got %d", len(assets))
	}
}
