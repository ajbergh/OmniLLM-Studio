package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

type seedResult struct {
	Title          string `json:"title"`
	ConversationID string `json:"conversation_id"`
	SessionID      string `json:"session_id"`
	NodeID         string `json:"node_id"`
	AssetID        string `json:"asset_id"`
	AttachmentID   string `json:"attachment_id"`
}

func main() {
	title := flag.String("title", "Toolbar Smoke Session", "image session title")
	instruction := flag.String("instruction", "Fixture image for toolbar smoke test", "seed node instruction")
	fixtureImage := flag.String("image", os.Getenv("OMNILLM_PLAYWRIGHT_FIXTURE_IMAGE"), "path to the source image fixture")
	flag.Parse()

	dbPath := os.Getenv("OMNILLM_DB_PATH")
	attachmentsDir := os.Getenv("OMNILLM_ATTACHMENTS_DIR")

	if dbPath == "" {
		exitf("OMNILLM_DB_PATH is required")
	}
	if attachmentsDir == "" {
		exitf("OMNILLM_ATTACHMENTS_DIR is required")
	}
	if *fixtureImage == "" {
		exitf("fixture image path is required")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		exitErr("open db", err)
	}
	defer db.Close(database)

	convoRepo := repository.NewConversationRepo(database)
	sessionRepo := repository.NewImageSessionRepo(database)
	nodeRepo := repository.NewImageNodeRepo(database)
	assetRepo := repository.NewImageNodeAssetRepo(database)
	attachmentRepo := repository.NewAttachmentRepo(database)

	convo, err := convoRepo.CreateWithKind("", *title, models.ConversationKindImage, nil, nil, nil)
	if err != nil {
		exitErr("create conversation", err)
	}

	session, err := sessionRepo.Create(convo.ID, *title)
	if err != nil {
		exitErr("create image session", err)
	}

	fixtureBytes, err := os.ReadFile(*fixtureImage)
	if err != nil {
		exitErr("read fixture image", err)
	}

	if err := os.MkdirAll(attachmentsDir, 0o755); err != nil {
		exitErr("create attachments dir", err)
	}

	ext := filepath.Ext(*fixtureImage)
	if ext == "" {
		ext = ".png"
	}
	storageName := uuid.New().String() + ext
	if err := os.WriteFile(filepath.Join(attachmentsDir, storageName), fixtureBytes, 0o644); err != nil {
		exitErr("write fixture image", err)
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/png"
	}

	attachment := &models.Attachment{
		ConversationID: convo.ID,
		Type:           "image",
		MimeType:       mimeType,
		StoragePath:    storageName,
		Bytes:          int64(len(fixtureBytes)),
		MetadataJSON:   `{"seeded":true,"source":"playwright-smoke"}`,
	}
	if err := attachmentRepo.Create(attachment); err != nil {
		exitErr("create attachment", err)
	}

	node := &models.ImageNode{
		SessionID:     session.ID,
		OperationType: "generate",
		Instruction:   *instruction,
		Provider:      "fixture",
		Model:         "fixture",
	}
	if err := nodeRepo.Create(node); err != nil {
		exitErr("create image node", err)
	}

	asset := &models.ImageNodeAsset{
		NodeID:       node.ID,
		AttachmentID: attachment.ID,
		VariantIndex: 0,
		IsSelected:   true,
	}
	if err := assetRepo.Create(asset); err != nil {
		exitErr("create image node asset", err)
	}

	if err := sessionRepo.UpdateActiveNode(session.ID, node.ID); err != nil {
		exitErr("set active image node", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(seedResult{
		Title:          *title,
		ConversationID: convo.ID,
		SessionID:      session.ID,
		NodeID:         node.ID,
		AssetID:        asset.ID,
		AttachmentID:   attachment.ID,
	}); err != nil {
		exitErr("encode seed result", err)
	}
}

func exitErr(action string, err error) {
	exitf("%s: %v", action, err)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
