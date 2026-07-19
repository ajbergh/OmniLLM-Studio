package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
)

type ragAttachmentReindexResult struct {
	AttachmentID  string `json:"attachment_id"`
	ChunksIndexed int    `json:"chunks_indexed"`
	StaleRemoved  int    `json:"stale_removed"`
}

type ragConversationReindexResult struct {
	ConversationID string                       `json:"conversation_id"`
	ChunksIndexed  int                          `json:"chunks_indexed"`
	Attachments    []ragAttachmentReindexResult `json:"attachments"`
	Failures       []string                     `json:"failures,omitempty"`
	EmbedProvider  string                       `json:"embed_provider"`
	EmbedModel     string                       `json:"embed_model"`
}

func (h *RAGHandler) reindexAttachmentSafe(ctx context.Context, attachment *models.Attachment, settings models.AppSettings, embedProvider, embedModel string) (ragAttachmentReindexResult, error) {
	result := ragAttachmentReindexResult{AttachmentID: attachment.ID}
	if !canExtractAttachmentText(attachment.MimeType) {
		return result, fmt.Errorf("attachment type %q is not supported", attachment.MimeType)
	}
	safePath, err := SafeJoin(h.storageDir, attachment.StoragePath)
	if err != nil {
		return result, err
	}
	content, err := extractAttachmentText(safePath, attachment.MimeType)
	if err != nil {
		return result, err
	}
	chunkOptions := rag.ChunkOptions{ChunkSize: settings.RAGChunkSize, Overlap: settings.RAGChunkOverlap}
	if chunkOptions.ChunkSize <= 0 {
		chunkOptions = rag.DefaultChunkOptions()
	}
	chunks := rag.DetectAndChunk(content, attachment.MimeType, attachment.ID, attachment.ConversationID, chunkOptions)
	embedFunc := rag.NewLLMEmbeddingFunc(h.llmSvc, embedProvider, embedModel)
	providerType := h.providerTypeFor(embedProvider)

	// Build vectors first. If embedding or vector persistence fails, the
	// currently active relational and vector records remain untouched.
	if len(chunks) > 0 {
		if err := h.vectorStore.IndexChunks(ctx, attachment.ConversationID, chunks, providerType, embedFunc); err != nil {
			return result, fmt.Errorf("index replacement vectors: %w", err)
		}
	}
	staleIDs, err := h.chunkRepo.ReplaceAttachmentChunks(attachment.ID, chunks)
	if err != nil {
		return result, fmt.Errorf("activate replacement chunks: %w", err)
	}
	if len(staleIDs) > 0 {
		if err := h.vectorStore.DeleteDocuments(ctx, attachment.ConversationID, staleIDs...); err != nil {
			return result, fmt.Errorf("remove stale vectors: %w", err)
		}
	}
	result.ChunksIndexed = len(chunks)
	result.StaleRemoved = len(staleIDs)
	return result, nil
}

func (h *RAGHandler) reindexConversationSafe(ctx context.Context, conversationID string) (*ragConversationReindexResult, error) {
	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		return nil, err
	}
	if !settings.RAGEnabled {
		return nil, fmt.Errorf("RAG is not enabled")
	}
	conversation, err := h.convoRepo.GetByID(conversationID)
	if err != nil {
		return nil, err
	}
	activeProvider := ""
	if conversation != nil && conversation.DefaultProvider != nil {
		activeProvider = *conversation.DefaultProvider
	}
	embedProvider, embedModel, err := rag.ResolveEmbeddingProvider(activeProvider, settings, h.providerRepo)
	if err != nil {
		return nil, err
	}
	attachments, err := h.attachRepo.ListByConversation(conversationID)
	if err != nil {
		return nil, err
	}
	result := &ragConversationReindexResult{
		ConversationID: conversationID,
		EmbedProvider:  embedProvider,
		EmbedModel:     embedModel,
	}
	for index := range attachments {
		attachment := &attachments[index]
		if !canExtractAttachmentText(attachment.MimeType) {
			continue
		}
		item, itemErr := h.reindexAttachmentSafe(ctx, attachment, settings, embedProvider, embedModel)
		if itemErr != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", attachment.ID, itemErr))
			continue
		}
		result.Attachments = append(result.Attachments, item)
		result.ChunksIndexed += item.ChunksIndexed
	}
	if len(result.Attachments) == 0 && len(result.Failures) > 0 {
		return result, fmt.Errorf("all attachment reindex operations failed: %s", strings.Join(result.Failures, "; "))
	}
	return result, nil
}
