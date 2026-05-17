package api

import (
	"log"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	intentrouter "github.com/ajbergh/omnillm-studio/internal/router"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// SettingsHandler handles application settings API endpoints.
type SettingsHandler struct {
	repo         *repository.SettingsRepo
	providerRepo *repository.ProviderRepo
	orchestrator *websearch.Orchestrator
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(repo *repository.SettingsRepo, providerRepo *repository.ProviderRepo, orchestrator *websearch.Orchestrator) *SettingsHandler {
	return &SettingsHandler{repo: repo, providerRepo: providerRepo, orchestrator: orchestrator}
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
	if v, ok := raw["jina_api_key"]; ok {
		if s, ok := v.(string); ok && s != "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" {
			// Only update if not the masked placeholder
			existing.JinaAPIKey = s
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

	// Music Studio settings
	if v, ok := raw["default_music_provider"]; ok {
		if s, ok := v.(string); ok {
			existing.DefaultMusicProvider = s
		}
	}
	if v, ok := raw["default_music_model_openrouter"]; ok {
		if s, ok := v.(string); ok {
			existing.DefaultMusicModelOpenRouter = s
		}
	}
	if v, ok := raw["default_music_model_gemini"]; ok {
		if s, ok := v.(string); ok {
			existing.DefaultMusicModelGemini = s
		}
	}
	if v, ok := raw["custom_gemini_lyria_model"]; ok {
		if s, ok := v.(string); ok {
			existing.CustomGeminiLyriaModel = s
		}
	}
	if v, ok := raw["auto_enhance_music_prompts"]; ok {
		if b, ok := v.(bool); ok {
			existing.AutoEnhanceMusicPrompts = b
		}
	}
	if v, ok := raw["save_music_generation_metadata"]; ok {
		if b, ok := v.(bool); ok {
			existing.SaveMusicGenerationMetadata = b
		}
	}
	if v, ok := raw["music_output_directory"]; ok {
		if s, ok := v.(string); ok {
			existing.MusicOutputDirectory = s
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

	// Router / intent classification settings
	if v, ok := raw["router_enabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.RouterEnabled = b
		}
	}
	if v, ok := raw["router_mode"]; ok {
		if s, ok := v.(string); ok {
			existing.RouterMode = s
		}
	}
	if v, ok := raw["router_provider"]; ok {
		if s, ok := v.(string); ok {
			existing.RouterProvider = s
		}
	}
	if v, ok := raw["router_model"]; ok {
		if s, ok := v.(string); ok {
			existing.RouterModel = s
		}
	}
	if v, ok := raw["router_structured_output_mode"]; ok {
		if s, ok := v.(string); ok {
			existing.RouterStructuredOutputMode = s
		}
	}
	if v, ok := raw["router_confidence_threshold"]; ok {
		if n, ok := v.(float64); ok {
			existing.RouterConfidenceThreshold = n
		}
	}
	if v, ok := raw["router_fallback_behavior"]; ok {
		if s, ok := v.(string); ok {
			existing.RouterFallbackBehavior = s
		}
	}
	if v, ok := raw["router_timeout_ms"]; ok {
		if n, ok := v.(float64); ok {
			existing.RouterTimeoutMS = int(n)
		}
	}
	if v, ok := raw["router_max_tokens"]; ok {
		if n, ok := v.(float64); ok {
			existing.RouterMaxTokens = int(n)
		}
	}
	if v, ok := raw["router_temperature"]; ok {
		if n, ok := v.(float64); ok {
			existing.RouterTemperature = n
		}
	}
	if v, ok := raw["router_show_trace"]; ok {
		if b, ok := v.(bool); ok {
			existing.RouterShowTrace = b
		}
	}
	if v, ok := raw["router_cache_enabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.RouterCacheEnabled = b
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

func (h *SettingsHandler) RouterSuggestions(w http.ResponseWriter, r *http.Request) {
	providerKey := r.URL.Query().Get("provider")
	providers, err := h.providerRepo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	for _, provider := range providers {
		if providerKey == "" || provider.ID == providerKey || provider.Name == providerKey || provider.Type == providerKey {
			respondJSON(w, http.StatusOK, intentrouter.SuggestionsForProvider(provider))
			return
		}
	}
	respondError(w, http.StatusNotFound, "provider not found")
}
