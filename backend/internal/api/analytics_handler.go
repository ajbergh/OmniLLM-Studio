package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/analytics"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AnalyticsHandler exposes usage and cost analytics endpoints.
type AnalyticsHandler struct {
	analyticsSvc *analytics.Service
	pricingRepo  *repository.PricingRepo
	convoRepo    *repository.ConversationRepo
}

// NewAnalyticsHandler creates an AnalyticsHandler.
func NewAnalyticsHandler(svc *analytics.Service, pricingRepo *repository.PricingRepo, convoRepo *repository.ConversationRepo) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsSvc: svc,
		pricingRepo:  pricingRepo,
		convoRepo:    convoRepo,
	}
}

// GetUsage returns aggregated usage statistics.
// Query params: ?period=day|week|month|all
func (h *AnalyticsHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}

	params := analytics.UsageParams{Period: period}
	summary, err := h.analyticsSvc.GetUsageWithCost(params)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, summary)
}

// GetConversationUsage returns usage for a specific conversation.
func (h *AnalyticsHandler) GetConversationUsage(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}

	params := analytics.UsageParams{
		Period:         period,
		ConversationID: convoID,
	}
	summary, err := h.analyticsSvc.GetUsageWithCost(params)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, summary)
}

// ListPricing returns all pricing rules.
func (h *AnalyticsHandler) ListPricing(w http.ResponseWriter, r *http.Request) {
	rules, err := h.pricingRepo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if rules == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, rules)
}

// UpsertPricing creates or updates a pricing rule.
func (h *AnalyticsHandler) UpsertPricing(w http.ResponseWriter, r *http.Request) {
	var rule models.PricingRule
	if err := decodeJSON(r, &rule); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if rule.ProviderType == "" || rule.ModelPattern == "" {
		respondError(w, http.StatusBadRequest, "provider_type and model_pattern are required")
		return
	}

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.Currency == "" {
		rule.Currency = "USD"
	}

	if err := h.pricingRepo.Upsert(rule); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, rule)
}

// DeletePricing removes a pricing rule by ID.
func (h *AnalyticsHandler) DeletePricing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "pricingId")
	if err := h.pricingRepo.Delete(id); err != nil {
		respondError(w, http.StatusNotFound, "pricing rule not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
