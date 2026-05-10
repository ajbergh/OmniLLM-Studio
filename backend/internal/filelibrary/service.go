package filelibrary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// LibraryService provides file-library operations backed by existing RAG infrastructure.
type LibraryService struct {
	libraryRepo      *repository.LibraryFileRepo
	attachmentRepo   *repository.AttachmentRepo
	chunkRepo        *repository.ChunkRepo
	settingsRepo     *repository.SettingsRepo
	providerRepo     *repository.ProviderRepo
	conversationRepo *repository.ConversationRepo
	llmSvc           *llm.Service
	vectorStore      *rag.VectorStore
	storageDir       string
}

// NewService creates a file-library service.
func NewService(
	libraryRepo *repository.LibraryFileRepo,
	attachmentRepo *repository.AttachmentRepo,
	chunkRepo *repository.ChunkRepo,
	settingsRepo *repository.SettingsRepo,
	providerRepo *repository.ProviderRepo,
	conversationRepo *repository.ConversationRepo,
	llmSvc *llm.Service,
	vectorStore *rag.VectorStore,
	storageDir string,
) *LibraryService {
	return &LibraryService{
		libraryRepo:      libraryRepo,
		attachmentRepo:   attachmentRepo,
		chunkRepo:        chunkRepo,
		settingsRepo:     settingsRepo,
		providerRepo:     providerRepo,
		conversationRepo: conversationRepo,
		llmSvc:           llmSvc,
		vectorStore:      vectorStore,
		storageDir:       storageDir,
	}
}

func CollectionName(scope string, userID string, workspaceID string, conversationID string) string {
	switch scope {
	case "conversation":
		if conversationID == "" {
			return "conversation:unknown"
		}
		return "conversation:" + conversationID
	case "workspace":
		if workspaceID == "" {
			return "workspace:unknown"
		}
		return "workspace:" + workspaceID
	case "global":
		if userID == "" {
			return "global:unknown"
		}
		return "global:" + userID
	default:
		if conversationID != "" {
			return "conversation:" + conversationID
		}
		if workspaceID != "" {
			return "workspace:" + workspaceID
		}
		if userID != "" {
			return "global:" + userID
		}
		return "conversation:unknown"
	}
}

// IngestFile ingests an attachment-backed file into the library and indexes it for retrieval.
func (s *LibraryService) IngestFile(ctx context.Context, req IngestFileRequest) (*models.LibraryFile, error) {
	if strings.TrimSpace(req.AttachmentID) == "" {
		return nil, fmt.Errorf("attachment_id is required")
	}

	scope := normalizeScope(req.Scope)
	att, err := s.attachmentRepo.GetByID(req.AttachmentID)
	if err != nil {
		return nil, err
	}
	if att == nil {
		return nil, fmt.Errorf("attachment not found")
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = att.ConversationID
	}

	safePath, err := safeJoin(s.storageDir, att.StoragePath)
	if err != nil {
		return nil, err
	}
	bytes, err := readFile(safePath)
	if err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(bytes)
	checksumHex := hex.EncodeToString(checksum[:])

	existing, err := s.libraryRepo.GetByChecksum(req.OwnerUserID, scope, checksumHex)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = filepath.Base(att.StoragePath)
	}
	metaJSON := "{}"
	if len(req.Metadata) > 0 {
		b, _ := json.Marshal(req.Metadata)
		metaJSON = string(b)
	}

	lib := &models.LibraryFile{
		OwnerUserID:      stringPtrOrNil(req.OwnerUserID),
		WorkspaceID:      stringPtrOrNil(req.WorkspaceID),
		ConversationID:   stringPtrOrNil(conversationID),
		AttachmentID:     &att.ID,
		SourceType:       "attachment",
		Scope:            scope,
		DisplayName:      displayName,
		OriginalFilename: stringPtrOrNil(filepath.Base(att.StoragePath)),
		MimeType:         &att.MimeType,
		FileExt:          fileExtPtr(att.StoragePath),
		StoragePath:      &att.StoragePath,
		SizeBytes:        att.Bytes,
		ChecksumSHA256:   &checksumHex,
		Status:           "extracting",
		MetadataJSON:     metaJSON,
	}
	if err := s.libraryRepo.Create(lib); err != nil {
		return nil, err
	}

	if err := s.libraryRepo.UpdateStatus(lib.ID, "extracting", nil, nil); err != nil {
		return nil, err
	}
	content, err := extractFileText(safePath, att.MimeType)
	if err != nil {
		errMsg := err.Error()
		_ = s.libraryRepo.UpdateStatus(lib.ID, "failed", &errMsg, nil)
		return nil, err
	}

	if err := s.libraryRepo.UpdateStatus(lib.ID, "chunking", nil, nil); err != nil {
		return nil, err
	}
	settings, err := s.settingsRepo.GetTyped()
	if err != nil {
		return nil, err
	}
	chunkOpts := rag.ChunkOptions{ChunkSize: settings.RAGChunkSize, Overlap: settings.RAGChunkOverlap}
	if chunkOpts.ChunkSize <= 0 {
		chunkOpts = rag.DefaultChunkOptions()
	}

	dbChunks := rag.DetectAndChunk(content, att.MimeType, att.ID, conversationID, chunkOpts)
	for i := range dbChunks {
		dbChunks[i].LibraryFileID = &lib.ID
		dbChunks[i].Scope = &scope
		dbChunks[i].WorkspaceID = stringPtrOrNil(req.WorkspaceID)
		dbChunks[i].SourceType = stringPtrOrNil("attachment")
		if strings.TrimSpace(dbChunks[i].ChunkMetaJSON) == "" {
			dbChunks[i].ChunkMetaJSON = "{}"
		}
	}
	if len(dbChunks) > 0 {
		if err := s.chunkRepo.CreateBatch(dbChunks); err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(lib.ID, "failed", &errMsg, nil)
			return nil, err
		}
	}

	if err := s.libraryRepo.UpdateStatus(lib.ID, "embedding", nil, nil); err != nil {
		return nil, err
	}
	if settings.RAGEnabled && len(dbChunks) > 0 {
		embedProvider, embedModel, err := s.resolveEmbeddingProvider(conversationID, settings)
		if err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(lib.ID, "failed", &errMsg, nil)
			return nil, err
		}
		embedFunc := rag.NewLLMEmbeddingFunc(s.llmSvc, embedProvider, embedModel)
		providerType := s.providerTypeFor(embedProvider)
		collectionName := CollectionName(scope, req.OwnerUserID, req.WorkspaceID, conversationID)
		if err := s.vectorStore.IndexChunks(ctx, collectionName, dbChunks, providerType, embedFunc); err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(lib.ID, "failed", &errMsg, nil)
			return nil, err
		}
	}

	now := time.Now().UTC()
	if err := s.libraryRepo.UpdateStatus(lib.ID, "indexed", nil, &now); err != nil {
		return nil, err
	}
	return s.libraryRepo.GetByID(lib.ID)
}

// ListFiles returns files owned by a user, filtered by scope/query.
func (s *LibraryService) ListFiles(ctx context.Context, ownerUserID string, scope string, query string) ([]models.LibraryFile, error) {
	_ = ctx
	scopes := expandScopes(scope)
	if strings.TrimSpace(query) != "" {
		return s.libraryRepo.SearchMetadata(ownerUserID, query, scopes)
	}
	all := make([]models.LibraryFile, 0)
	for _, sc := range scopes {
		files, err := s.libraryRepo.ListByScope(ownerUserID, sc, nil, nil)
		if err != nil {
			return nil, err
		}
		all = append(all, files...)
	}
	return all, nil
}

func normalizeScope(scope string) string {
	s := strings.TrimSpace(strings.ToLower(scope))
	switch s {
	case "conversation", "workspace", "global":
		return s
	default:
		return "conversation"
	}
}

func expandScopes(scope string) []string {
	s := strings.TrimSpace(strings.ToLower(scope))
	switch s {
	case "conversation", "workspace", "global":
		return []string{s}
	case "all", "auto", "":
		return []string{"conversation", "workspace", "global"}
	default:
		return []string{"conversation", "workspace", "global"}
	}
}

func stringPtrOrNil(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	vv := v
	return &vv
}

func fileExtPtr(path string) *string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return nil
	}
	return &ext
}

func (s *LibraryService) resolveEmbeddingProvider(conversationID string, settings models.AppSettings) (string, string, error) {
	activeProvider := ""
	if conversationID != "" {
		if convo, err := s.conversationRepo.GetByID(conversationID); err == nil && convo != nil && convo.DefaultProvider != nil {
			activeProvider = *convo.DefaultProvider
		}
	}
	return rag.ResolveEmbeddingProvider(activeProvider, settings, s.providerRepo)
}

func ownerMatches(fileOwner *string, ownerUserID string) bool {
	if strings.TrimSpace(ownerUserID) == "" {
		return fileOwner == nil || strings.TrimSpace(*fileOwner) == ""
	}
	if fileOwner == nil {
		return false
	}
	return *fileOwner == ownerUserID
}

func (s *LibraryService) providerTypeFor(providerName string) string {
	if s.providerRepo == nil || providerName == "" {
		return ""
	}
	all, err := s.providerRepo.List()
	if err != nil {
		return ""
	}
	for _, p := range all {
		if p.Name == providerName {
			return p.Type
		}
	}
	return ""
}
