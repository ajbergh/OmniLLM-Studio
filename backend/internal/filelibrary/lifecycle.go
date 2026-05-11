package filelibrary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
)

// UpdateFile updates editable metadata on a library file.
func (s *LibraryService) UpdateFile(ctx context.Context, req UpdateFileRequest) (*models.LibraryFile, error) {
	_ = ctx
	if strings.TrimSpace(req.LibraryFileID) == "" {
		return nil, fmt.Errorf("library_file_id is required")
	}
	f, err := s.libraryRepo.GetByID(req.LibraryFileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("library file not found")
	}
	if !ownerMatches(f.OwnerUserID, req.OwnerUserID) {
		return nil, fmt.Errorf("library file not found")
	}

	if req.Scope != nil {
		s := normalizeScope(*req.Scope)
		req.Scope = &s
	}
	if err := s.libraryRepo.UpdateFields(req.LibraryFileID, req.DisplayName, req.Scope, req.Metadata); err != nil {
		return nil, err
	}
	return s.libraryRepo.GetByID(req.LibraryFileID)
}

// DeleteFile removes a library file and its indexed chunks/vectors.
func (s *LibraryService) DeleteFile(ctx context.Context, req DeleteFileRequest) error {
	if strings.TrimSpace(req.LibraryFileID) == "" {
		return fmt.Errorf("library_file_id is required")
	}
	f, err := s.libraryRepo.GetByID(req.LibraryFileID)
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("library file not found")
	}
	if !ownerMatches(f.OwnerUserID, req.OwnerUserID) {
		return fmt.Errorf("library file not found")
	}

	chunks, err := s.chunkRepo.ListByLibraryFileID(f.ID)
	if err != nil {
		return err
	}
	if len(chunks) > 0 {
		chunkIDs := make([]string, 0, len(chunks))
		for _, c := range chunks {
			chunkIDs = append(chunkIDs, c.ID)
		}
		workspaceID := ""
		if f.WorkspaceID != nil {
			workspaceID = *f.WorkspaceID
		}
		conversationID := ""
		if f.ConversationID != nil {
			conversationID = *f.ConversationID
		}
		collectionName := CollectionName(f.Scope, req.OwnerUserID, workspaceID, conversationID)
		_ = s.vectorStore.DeleteDocuments(ctx, collectionName, chunkIDs...)
		if err := s.chunkRepo.DeleteByLibraryFileID(f.ID); err != nil {
			return err
		}
	}

	if req.HardDelete {
		return s.libraryRepo.Delete(f.ID)
	}
	return s.libraryRepo.MarkDeleted(f.ID)
}

// ReindexFile re-extracts and reindexes one existing library file.
func (s *LibraryService) ReindexFile(ctx context.Context, req ReindexFileRequest) (*ReindexFileResponse, error) {
	if strings.TrimSpace(req.LibraryFileID) == "" {
		return nil, fmt.Errorf("library_file_id is required")
	}
	f, err := s.libraryRepo.GetByID(req.LibraryFileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("library file not found")
	}
	if !ownerMatches(f.OwnerUserID, req.OwnerUserID) {
		return nil, fmt.Errorf("library file not found")
	}
	if f.StoragePath == nil || strings.TrimSpace(*f.StoragePath) == "" {
		return nil, fmt.Errorf("library file has no storage path")
	}

	if err := s.libraryRepo.UpdateStatus(f.ID, "extracting", nil, nil); err != nil {
		return nil, err
	}
	path, err := safeJoin(s.storageDir, *f.StoragePath)
	if err != nil {
		return nil, err
	}
	mime := ""
	if f.MimeType != nil {
		mime = *f.MimeType
	}
	content, err := extractFileText(path, mime)
	if err != nil {
		errMsg := err.Error()
		_ = s.libraryRepo.UpdateStatus(f.ID, "failed", &errMsg, nil)
		return nil, err
	}

	oldChunks, err := s.chunkRepo.ListByLibraryFileID(f.ID)
	if err != nil {
		return nil, err
	}
	if len(oldChunks) > 0 {
		ids := make([]string, 0, len(oldChunks))
		for _, c := range oldChunks {
			ids = append(ids, c.ID)
		}
		workspaceID := ""
		if f.WorkspaceID != nil {
			workspaceID = *f.WorkspaceID
		}
		conversationID := ""
		if f.ConversationID != nil {
			conversationID = *f.ConversationID
		}
		collectionName := CollectionName(f.Scope, req.OwnerUserID, workspaceID, conversationID)
		_ = s.vectorStore.DeleteDocuments(ctx, collectionName, ids...)
		if err := s.chunkRepo.DeleteByLibraryFileID(f.ID); err != nil {
			return nil, err
		}
	}

	if err := s.libraryRepo.UpdateStatus(f.ID, "chunking", nil, nil); err != nil {
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

	conversationID := ""
	if f.ConversationID != nil {
		conversationID = *f.ConversationID
	}
	dbChunks := rag.DetectAndChunk(content, mime, valueOrEmpty(f.AttachmentID), conversationID, chunkOpts)
	for i := range dbChunks {
		dbChunks[i].LibraryFileID = &f.ID
		dbChunks[i].Scope = &f.Scope
		dbChunks[i].WorkspaceID = f.WorkspaceID
		dbChunks[i].SourceType = &f.SourceType
		if strings.TrimSpace(dbChunks[i].ChunkMetaJSON) == "" {
			dbChunks[i].ChunkMetaJSON = "{}"
		}
	}
	if len(dbChunks) > 0 {
		if err := s.chunkRepo.CreateBatch(dbChunks); err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(f.ID, "failed", &errMsg, nil)
			return nil, err
		}
	}

	embeddingsStored := 0
	if err := s.libraryRepo.UpdateStatus(f.ID, "embedding", nil, nil); err != nil {
		return nil, err
	}
	if settings.RAGEnabled && len(dbChunks) > 0 {
		embedProvider, embedModel, err := s.resolveEmbeddingProvider(conversationID, settings)
		if err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(f.ID, "failed", &errMsg, nil)
			return nil, err
		}
		embedFunc := rag.NewLLMEmbeddingFunc(s.llmSvc, embedProvider, embedModel)
		providerType := s.providerTypeFor(embedProvider)
		workspaceID := ""
		if f.WorkspaceID != nil {
			workspaceID = *f.WorkspaceID
		}
		collectionName := CollectionName(f.Scope, req.OwnerUserID, workspaceID, conversationID)
		if err := s.vectorStore.IndexChunks(ctx, collectionName, dbChunks, providerType, embedFunc); err != nil {
			errMsg := err.Error()
			_ = s.libraryRepo.UpdateStatus(f.ID, "failed", &errMsg, nil)
			return nil, err
		}
		embeddingsStored = len(dbChunks)
	}

	now := time.Now().UTC()
	if err := s.libraryRepo.UpdateStatus(f.ID, "indexed", nil, &now); err != nil {
		return nil, err
	}

	return &ReindexFileResponse{
		LibraryFileID:    f.ID,
		ChunksCreated:    len(dbChunks),
		EmbeddingsStored: embeddingsStored,
	}, nil
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
