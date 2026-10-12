package api

// File overview: implements administrator-facing RAG footprint reporting and full indexed-conversation rebuild operations.

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

// Repair non-destructively rebuilds every conversation that currently has persisted chunks; it does not limit work to detected inconsistencies.
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

// Diagnose compares persisted relational chunk IDs against the union of the
// corresponding physical vector-collection IDs. It is intentionally read-only,
// bounded and administrator-only (see router); /rag/repair retains its existing
// full-rebuild semantics until targeted recovery is implemented.
func (h *RAGHandler) Diagnose(w http.ResponseWriter, r *http.Request) {
	conversationIDs, err := h.chunkRepo.DistinctConversationIDsWithChunks()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	const maxDiagnosticConversations = 250
	if len(conversationIDs) > maxDiagnosticConversations {
		respondError(w, http.StatusRequestEntityTooLarge, "too many indexed conversations for a single integrity scan")
		return
	}
	results := make([]map[string]interface{}, 0, len(conversationIDs))
	mismatched := 0
	for _, conversationID := range conversationIDs {
		if err := r.Context().Err(); err != nil {
			return
		}
		chunks, err := h.chunkRepo.ListByConversation(conversationID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		chunkIDs := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			chunkIDs = append(chunkIDs, chunk.ID)
		}
		report, err := h.vectorStore.InspectChunkIDs(r.Context(), conversationID, chunkIDs)
		if err != nil {
			respondError(w, http.StatusServiceUnavailable, "index integrity inspection did not complete")
			return
		}
		if !report.AllExpectedPresent {
			mismatched++
		}
		results = append(results, map[string]interface{}{
			"conversation_id": conversationID,
			"summary": report,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversations_scanned": len(results),
		"conversations_with_missing_vectors": mismatched,
		"results": results,
		"scope": "presence in any legacy/physical embedding-space collection; active-space completeness not verified",
		"repair_performed": false,
		"orphan_only_collections_scanned": false,
	})
}
