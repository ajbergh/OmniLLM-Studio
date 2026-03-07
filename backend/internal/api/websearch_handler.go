package api

import (
	"log"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// WebSearchHandler exposes the /v1/websearch endpoint.
type WebSearchHandler struct {
	orchestrator *websearch.Orchestrator
}

// NewWebSearchHandler creates a new WebSearchHandler.
func NewWebSearchHandler(orch *websearch.Orchestrator) *WebSearchHandler {
	return &WebSearchHandler{orchestrator: orch}
}

type webSearchAPIRequest struct {
	Query      string `json:"query"`
	TimeRange  string `json:"timeRange"`
	Region     string `json:"region"`
	Locale     string `json:"locale"`
	MaxResults int    `json:"maxResults"`
}

// Search handles POST /v1/websearch
func (h *WebSearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req webSearchAPIRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	// Apply defaults
	if req.TimeRange == "" {
		req.TimeRange = "24h"
	}
	if req.Region == "" {
		req.Region = "US"
	}
	if req.Locale == "" {
		req.Locale = "en-US"
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 10
	}

	searchReq := websearch.SearchRequest{
		Query:      req.Query,
		TimeRange:  req.TimeRange,
		Region:     req.Region,
		Locale:     req.Locale,
		MaxResults: req.MaxResults,
	}

	resp, err := h.orchestrator.DirectSearch(r.Context(), searchReq)
	if err != nil {
		log.Printf("ERROR: web search: %v", err)
		respondError(w, http.StatusBadGateway, "web search failed")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
