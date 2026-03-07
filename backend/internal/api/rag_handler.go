package api

import (
	"log"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// RAGHandler handles RAG-related API endpoints.
type RAGHandler struct {
	chunkRepo     *repository.ChunkRepo
	embeddingRepo *repository.EmbeddingRepo
	attachRepo    *repository.AttachmentRepo
	convoRepo     *repository.ConversationRepo
	settingsRepo  *repository.SettingsRepo
	llmSvc        *llm.Service
	storageDir    string
}

// NewRAGHandler creates a new RAGHandler.
func NewRAGHandler(
	chunkRepo *repository.ChunkRepo,
	embeddingRepo *repository.EmbeddingRepo,
	attachRepo *repository.AttachmentRepo,
	convoRepo *repository.ConversationRepo,
	settingsRepo *repository.SettingsRepo,
	llmSvc *llm.Service,
	storageDir string,
) *RAGHandler {
	return &RAGHandler{
		chunkRepo:     chunkRepo,
		embeddingRepo: embeddingRepo,
		attachRepo:    attachRepo,
		convoRepo:     convoRepo,
		settingsRepo:  settingsRepo,
		llmSvc:        llmSvc,
		storageDir:    storageDir,
	}
}

// ListChunks returns all chunks for a conversation.
func (h *RAGHandler) ListChunks(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")
	chunks, err := h.chunkRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if chunks == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, chunks)
}

// ListAttachmentChunks returns all chunks for a specific attachment.
func (h *RAGHandler) ListAttachmentChunks(w http.ResponseWriter, r *http.Request) {
	attachID := chi.URLParam(r, "attachmentId")
	att, err := h.attachRepo.GetByID(attachID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if att == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, att.ConversationID) {
		return
	}
	chunks, err := h.chunkRepo.ListByAttachment(attachID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if chunks == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, chunks)
}

// Reindex re-chunks and re-embeds all text attachments for a conversation.
func (h *RAGHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")

	convo, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	embeddingProvider := ""
	if convo != nil && convo.DefaultProvider != nil {
		embeddingProvider = *convo.DefaultProvider
	}

	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !settings.RAGEnabled {
		respondError(w, http.StatusBadRequest, "RAG is not enabled")
		return
	}

	// Delete existing chunks + embeddings for this conversation
	if err := h.embeddingRepo.DeleteByConversation(convoID); err != nil {
		log.Printf("ERROR: delete embeddings for conversation %s: %v", convoID, err)
		respondError(w, http.StatusInternalServerError, "failed to reindex")
		return
	}
	if err := h.chunkRepo.DeleteByConversation(convoID); err != nil {
		log.Printf("ERROR: delete chunks for conversation %s: %v", convoID, err)
		respondError(w, http.StatusInternalServerError, "failed to reindex")
		return
	}

	// Get all attachments for the conversation
	attachments, err := h.attachRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	chunkOpts := rag.ChunkOptions{
		ChunkSize: settings.RAGChunkSize,
		Overlap:   settings.RAGChunkOverlap,
	}
	if chunkOpts.ChunkSize <= 0 {
		chunkOpts = rag.DefaultChunkOptions()
	}

	var totalChunks int
	var totalEmbeddings int

	for _, att := range attachments {
		if !canExtractAttachmentText(att.MimeType) {
			continue
		}

		safePath, pathErr := SafeJoin(h.storageDir, att.StoragePath)
		if pathErr != nil {
			log.Printf("[rag] unsafe attachment path %s: %v", att.ID, pathErr)
			continue
		}
		content, err := extractAttachmentText(safePath, att.MimeType)
		if err != nil {
			log.Printf("[rag] failed to extract text for attachment %s (%s): %v", att.ID, att.MimeType, err)
			continue
		}

		dbChunks := rag.DetectAndChunk(content, att.MimeType, att.ID, convoID, chunkOpts)
		if len(dbChunks) == 0 {
			continue
		}

		if err := h.chunkRepo.CreateBatch(dbChunks); err != nil {
			log.Printf("[rag] failed to create chunks for attachment %s: %v", att.ID, err)
			continue
		}
		totalChunks += len(dbChunks)

		// Embed the chunks
		texts := make([]string, len(dbChunks))
		for i, c := range dbChunks {
			texts[i] = c.Content
		}

		embResp, err := h.llmSvc.Embed(r.Context(), llm.EmbeddingRequest{
			Provider: embeddingProvider,
			Model:    settings.RAGEmbeddingModel,
			Input:    texts,
		})
		if err != nil {
			log.Printf("[rag] failed to embed chunks for attachment %s: %v", att.ID, err)
			continue
		}

		// Store embeddings
		embModels := make([]models.DocumentEmbedding, len(embResp.Embeddings))
		for i, vec := range embResp.Embeddings {
			embModels[i] = models.DocumentEmbedding{
				ChunkID:    dbChunks[i].ID,
				Embedding:  vec,
				Model:      embResp.Model,
				Dimensions: embResp.Dimensions,
			}
		}

		if err := h.embeddingRepo.UpsertBatch(embModels); err != nil {
			log.Printf("[rag] failed to store embeddings for attachment %s: %v", att.ID, err)
			continue
		}
		totalEmbeddings += len(embModels)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversation_id":   convoID,
		"chunks_created":    totalChunks,
		"embeddings_stored": totalEmbeddings,
	})
}

// IndexAttachment chunks and embeds a single attachment. Called internally
// (or via API) when a new attachment is uploaded.
func (h *RAGHandler) IndexAttachment(w http.ResponseWriter, r *http.Request) {
	attachID := chi.URLParam(r, "attachmentId")

	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !settings.RAGEnabled {
		respondError(w, http.StatusBadRequest, "RAG is not enabled")
		return
	}

	att, err := h.attachRepo.GetByID(attachID)
	if err != nil || att == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// Verify user owns the parent conversation
	if !verifyConversationAccessByID(w, r, h.convoRepo, att.ConversationID) {
		return
	}

	if !canExtractAttachmentText(att.MimeType) {
		respondError(w, http.StatusBadRequest, "attachment type is not supported for RAG indexing")
		return
	}

	// Delete any existing chunks + embeddings for this attachment
	_ = h.embeddingRepo.DeleteByAttachment(attachID)
	_ = h.chunkRepo.DeleteByAttachment(attachID)

	convo, err := h.convoRepo.GetByID(att.ConversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	embeddingProvider := ""
	if convo != nil && convo.DefaultProvider != nil {
		embeddingProvider = *convo.DefaultProvider
	}

	safePath, pathErr := SafeJoin(h.storageDir, att.StoragePath)
	if pathErr != nil {
		respondError(w, http.StatusBadRequest, "invalid attachment path")
		return
	}
	content, err := extractAttachmentText(safePath, att.MimeType)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to extract text from attachment")
		return
	}

	chunkOpts := rag.ChunkOptions{
		ChunkSize: settings.RAGChunkSize,
		Overlap:   settings.RAGChunkOverlap,
	}
	if chunkOpts.ChunkSize <= 0 {
		chunkOpts = rag.DefaultChunkOptions()
	}

	dbChunks := rag.DetectAndChunk(content, att.MimeType, att.ID, att.ConversationID, chunkOpts)
	if len(dbChunks) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"attachment_id":     attachID,
			"chunks_created":    0,
			"embeddings_stored": 0,
		})
		return
	}

	if err := h.chunkRepo.CreateBatch(dbChunks); err != nil {
		log.Printf("ERROR: create chunks for attachment %s: %v", attachID, err)
		respondError(w, http.StatusInternalServerError, "failed to index attachment")
		return
	}

	// Embed
	texts := make([]string, len(dbChunks))
	for i, c := range dbChunks {
		texts[i] = c.Content
	}

	embResp, err := h.llmSvc.Embed(r.Context(), llm.EmbeddingRequest{
		Provider: embeddingProvider,
		Model:    settings.RAGEmbeddingModel,
		Input:    texts,
	})
	if err != nil {
		log.Printf("ERROR: embed chunks for attachment %s: %v", attachID, err)
		respondError(w, http.StatusBadGateway, "embedding failed")
		return
	}

	embModels := make([]models.DocumentEmbedding, len(embResp.Embeddings))
	for i, vec := range embResp.Embeddings {
		embModels[i] = models.DocumentEmbedding{
			ChunkID:    dbChunks[i].ID,
			Embedding:  vec,
			Model:      embResp.Model,
			Dimensions: embResp.Dimensions,
		}
	}

	if err := h.embeddingRepo.UpsertBatch(embModels); err != nil {
		log.Printf("ERROR: store embeddings for attachment %s: %v", attachID, err)
		respondError(w, http.StatusInternalServerError, "failed to store embeddings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"attachment_id":     attachID,
		"chunks_created":    len(dbChunks),
		"embeddings_stored": len(embModels),
	})
}
