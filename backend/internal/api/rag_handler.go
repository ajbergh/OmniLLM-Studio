package api

import (
	"log"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// RAGHandler handles RAG-related API endpoints.
type RAGHandler struct {
	chunkRepo    *repository.ChunkRepo
	vectorStore  *rag.VectorStore
	attachRepo   *repository.AttachmentRepo
	convoRepo    *repository.ConversationRepo
	settingsRepo *repository.SettingsRepo
	providerRepo *repository.ProviderRepo
	llmSvc       *llm.Service
	storageDir   string
}

// NewRAGHandler creates a new RAGHandler.
func NewRAGHandler(
	chunkRepo *repository.ChunkRepo,
	vectorStore *rag.VectorStore,
	attachRepo *repository.AttachmentRepo,
	convoRepo *repository.ConversationRepo,
	settingsRepo *repository.SettingsRepo,
	providerRepo *repository.ProviderRepo,
	llmSvc *llm.Service,
	storageDir string,
) *RAGHandler {
	return &RAGHandler{
		chunkRepo:    chunkRepo,
		vectorStore:  vectorStore,
		attachRepo:   attachRepo,
		convoRepo:    convoRepo,
		settingsRepo: settingsRepo,
		providerRepo: providerRepo,
		llmSvc:       llmSvc,
		storageDir:   storageDir,
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

	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !settings.RAGEnabled {
		respondError(w, http.StatusBadRequest, "RAG is not enabled")
		return
	}

	activeProvider := ""
	if convo != nil && convo.DefaultProvider != nil {
		activeProvider = *convo.DefaultProvider
	}

	embedProvider, embedModel, err := rag.ResolveEmbeddingProvider(activeProvider, settings, h.providerRepo)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Delete existing chunks + chromem collection for this conversation
	if err := h.vectorStore.DeleteCollection(convoID); err != nil {
		log.Printf("[rag] delete chromem collection for %s: %v", convoID, err)
	}
	if err := h.chunkRepo.DeleteByConversation(convoID); err != nil {
		log.Printf("ERROR: delete chunks for conversation %s: %v", convoID, err)
		respondError(w, http.StatusInternalServerError, "failed to reindex")
		return
	}

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

	embedFunc := rag.NewLLMEmbeddingFunc(h.llmSvc, embedProvider, embedModel)
	providerType := h.providerTypeFor(embedProvider)

	var totalChunks int

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

		if err := h.vectorStore.IndexChunks(r.Context(), convoID, dbChunks, providerType, embedFunc); err != nil {
			log.Printf("[rag] failed to index chunks for attachment %s: %v", att.ID, err)
			continue
		}
		totalChunks += len(dbChunks)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversation_id": convoID,
		"chunks_indexed":  totalChunks,
		"embed_provider":  embedProvider,
		"embed_model":     embedModel,
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

	if !verifyConversationAccessByID(w, r, h.convoRepo, att.ConversationID) {
		return
	}

	if !canExtractAttachmentText(att.MimeType) {
		respondError(w, http.StatusBadRequest, "attachment type is not supported for RAG indexing")
		return
	}

	convo, err := h.convoRepo.GetByID(att.ConversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	activeProvider := ""
	if convo != nil && convo.DefaultProvider != nil {
		activeProvider = *convo.DefaultProvider
	}

	embedProvider, embedModel, err := rag.ResolveEmbeddingProvider(activeProvider, settings, h.providerRepo)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Remove any existing chunks + vector entries for this attachment
	existing, _ := h.chunkRepo.ListByAttachment(attachID)
	if len(existing) > 0 {
		ids := make([]string, len(existing))
		for i, c := range existing {
			ids[i] = c.ID
		}
		_ = h.vectorStore.DeleteDocuments(r.Context(), att.ConversationID, ids...)
	}
	_ = h.chunkRepo.DeleteByAttachment(attachID)

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
			"attachment_id":  attachID,
			"chunks_indexed": 0,
		})
		return
	}

	if err := h.chunkRepo.CreateBatch(dbChunks); err != nil {
		log.Printf("ERROR: create chunks for attachment %s: %v", attachID, err)
		respondError(w, http.StatusInternalServerError, "failed to index attachment")
		return
	}

	embedFunc := rag.NewLLMEmbeddingFunc(h.llmSvc, embedProvider, embedModel)
	providerType := h.providerTypeFor(embedProvider)

	if err := h.vectorStore.IndexChunks(r.Context(), att.ConversationID, dbChunks, providerType, embedFunc); err != nil {
		log.Printf("ERROR: index chunks for attachment %s: %v", attachID, err)
		respondError(w, http.StatusBadGateway, "embedding failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"attachment_id":  attachID,
		"chunks_indexed": len(dbChunks),
		"embed_provider": embedProvider,
		"embed_model":    embedModel,
	})
}

// ReindexAll drops every chromem collection so the next query against each
// conversation triggers a lazy re-migration from legacy SQL embeddings (or
// returns empty if none exist). Admin-only — wired in router under /rag/reindex-all.
func (h *RAGHandler) ReindexAll(w http.ResponseWriter, r *http.Request) {
	rows, err := h.chunkRepo.DistinctConversationIDsWithChunks()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	dropped := 0
	for _, convoID := range rows {
		if err := h.vectorStore.DeleteCollection(convoID); err != nil {
			log.Printf("[rag] reindex-all: drop %s: %v", convoID, err)
			continue
		}
		dropped++
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversations_dropped": dropped,
		"note":                  "next retrieval per conversation will lazy-migrate from legacy embeddings (if any) or remain empty until a per-conversation reindex is triggered",
	})
}

// providerTypeFor returns the provider type ("openai", "ollama", ...) for a
// provider profile name. Used to tune indexing concurrency.
func (h *RAGHandler) providerTypeFor(providerName string) string {
	if h.providerRepo == nil || providerName == "" {
		return ""
	}
	all, err := h.providerRepo.List()
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
