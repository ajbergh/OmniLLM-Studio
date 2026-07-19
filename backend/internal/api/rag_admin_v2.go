package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/rag"
)

// Health reports the effective RAG configuration and persisted index footprint.
func (h *RAGHandler) Health(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	conversationIDs, err := h.chunkRepo.DistinctConversationIDsWithChunks()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	totalChunks := 0
	for _, conversationID := range conversationIDs {
		chunks, chunkErr := h.chunkRepo.ListByConversation(conversationID)
		if chunkErr != nil {
			respondInternalError(w, chunkErr)
			return
		}
		totalChunks += len(chunks)
	}
	collections := h.vectorStore.CollectionCounts()
	vectorRecords := 0
	for _, count := range collections {
		vectorRecords += count
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":                  settings.RAGEnabled,
		"embedding_selection":      settings.RAGEmbeddingModel,
		"chunk_size":               settings.RAGChunkSize,
		"chunk_overlap":            settings.RAGChunkOverlap,
		"top_k":                    settings.RAGTopK,
		"conversations_indexed":    len(conversationIDs),
		"chunks":                   totalChunks,
		"vector_records":           vectorRecords,
		"physical_collections":     len(collections),
		"backend":                  "chromem-exact+sqlite-fts5",
		"pure_go":                  true,
		"embedding_schema_version": rag.EmbeddingSchemaVersion,
	})
}

// Repair non-destructively rebuilds all conversations that currently have chunks.
func (h *RAGHandler) Repair(w http.ResponseWriter, r *http.Request) {
	conversationIDs, err := h.chunkRepo.DistinctConversationIDsWithChunks()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	repaired := 0
	indexed := 0
	failures := []string{}
	for _, conversationID := range conversationIDs {
		result, reindexErr := h.reindexConversationSafe(r.Context(), conversationID)
		if reindexErr != nil {
			failures = append(failures, conversationID+": "+reindexErr.Error())
			continue
		}
		repaired++
		indexed += result.ChunksIndexed
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversations_repaired": repaired,
		"chunks_indexed":         indexed,
		"failures":               failures,
	})
}
