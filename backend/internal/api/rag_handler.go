package api

// File overview: contains public conversation and attachment RAG HTTP handlers.

import (
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

// Reindex non-destructively rebuilds every supported attachment in a conversation, preserving prior searchable data for failed attachments.
func (h *RAGHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	result, err := h.reindexConversationSafe(r.Context(), chi.URLParam(r, "conversationId"))
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// IndexAttachment chunks and embeds a single attachment. Called internally
// (or via API) when a new attachment is uploaded.
func (h *RAGHandler) IndexAttachment(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "attachmentId")
	attachment, err := h.attachRepo.GetByID(attachmentID)
	if err != nil || attachment == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, attachment.ConversationID) {
		return
	}
	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	conversation, err := h.convoRepo.GetByID(attachment.ConversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	activeProvider := ""
	if conversation != nil && conversation.DefaultProvider != nil {
		activeProvider = *conversation.DefaultProvider
	}
	embedProvider, embedModel, err := rag.ResolveEmbeddingProvider(activeProvider, settings, h.providerRepo)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.reindexAttachmentSafe(r.Context(), attachment, settings, embedProvider, embedModel)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"attachment_id":  result.AttachmentID,
		"chunks_indexed": result.ChunksIndexed,
		"stale_removed":  result.StaleRemoved,
		"embed_provider": embedProvider,
		"embed_model":    embedModel,
	})
}

// ReindexAll non-destructively rebuilds every conversation that currently has persisted chunks and reports per-conversation failures.
func (h *RAGHandler) ReindexAll(w http.ResponseWriter, r *http.Request) {
	conversationIDs, err := h.chunkRepo.DistinctConversationIDsWithChunks()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	rebuilt := 0
	chunks := 0
	failures := []string{}
	for _, conversationID := range conversationIDs {
		result, rebuildErr := h.reindexConversationSafe(r.Context(), conversationID)
		if rebuildErr != nil {
			failures = append(failures, conversationID+": "+rebuildErr.Error())
			continue
		}
		rebuilt++
		chunks += result.ChunksIndexed
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversations_reindexed": rebuilt,
		"chunks_indexed":          chunks,
		"failures":                failures,
		"note":                    "replacement vectors are built before relational activation; failed attachments retain their previous index",
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
