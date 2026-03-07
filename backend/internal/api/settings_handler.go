package api

import (
	"log"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// SettingsHandler handles application settings API endpoints.
type SettingsHandler struct {
	repo         *repository.SettingsRepo
	orchestrator *websearch.Orchestrator
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(repo *repository.SettingsRepo, orchestrator *websearch.Orchestrator) *SettingsHandler {
	return &SettingsHandler{repo: repo, orchestrator: orchestrator}
}

func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	s, err := h.repo.GetTypedMasked()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, s)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Read existing settings first (for merge with partial updates)
	existing, err := h.repo.GetTyped()
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// Decode the incoming partial update into a raw map to detect which fields were provided
	var raw map[string]interface{}
	if err := decodeJSON(r, &raw); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Merge only provided fields over existing settings
	if v, ok := raw["web_search_provider"]; ok {
		if s, ok := v.(string); ok {
			existing.WebSearchProvider = s
		}
	}
	if v, ok := raw["brave_api_key"]; ok {
		if s, ok := v.(string); ok && s != "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" {
			// Only update if not the masked placeholder
			existing.BraveAPIKey = s
		}
	}
	if v, ok := raw["jina_reader_enabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.JinaReaderEnabled = b
		}
	}
	if v, ok := raw["jina_reader_max_len"]; ok {
		if n, ok := v.(float64); ok {
			existing.JinaReaderMaxLen = int(n)
		}
	}

	// RAG settings
	if v, ok := raw["rag_enabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.RAGEnabled = b
		}
	}
	if v, ok := raw["rag_embedding_model"]; ok {
		if s, ok := v.(string); ok {
			existing.RAGEmbeddingModel = s
		}
	}
	if v, ok := raw["rag_chunk_size"]; ok {
		if n, ok := v.(float64); ok {
			existing.RAGChunkSize = int(n)
		}
	}
	if v, ok := raw["rag_chunk_overlap"]; ok {
		if n, ok := v.(float64); ok {
			existing.RAGChunkOverlap = int(n)
		}
	}
	if v, ok := raw["rag_top_k"]; ok {
		if n, ok := v.(float64); ok {
			existing.RAGTopK = int(n)
		}
	}

	if err := h.repo.SetTyped(existing); err != nil {
		respondInternalError(w, err)
		return
	}

	// Re-initialize websearch provider from updated settings.
	if h.orchestrator != nil {
		wsProvider := websearch.NewProviderFromSettings(h.repo)
		jinaReader := websearch.NewJinaReaderFromSettings(h.repo)
		h.orchestrator.Reconfigure(wsProvider, jinaReader)
		log.Println("[settings] web search orchestrator reconfigured")
	}

	s, err := h.repo.GetTypedMasked()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, s)
}
