package analytics

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// Service provides aggregated usage and cost analytics.
type Service struct {
	db          *sql.DB
	pricingRepo *repository.PricingRepo
}

// NewService creates an analytics service.
func NewService(db *sql.DB, pricingRepo *repository.PricingRepo) *Service {
	return &Service{db: db, pricingRepo: pricingRepo}
}

// periodRange returns (start, end) timestamps for the given period.
func periodRange(period string) (time.Time, time.Time) {
	now := time.Now().UTC()
	var start time.Time
	switch period {
	case "day":
		start = now.AddDate(0, 0, -1)
	case "week":
		start = now.AddDate(0, 0, -7)
	case "month":
		start = now.AddDate(0, -1, 0)
	default: // "all"
		start = time.Time{}
	}
	return start, now
}

// UsageParams controls what data to aggregate.
type UsageParams struct {
	Period         string // "day", "week", "month", "all"
	ConversationID string // optional: filter to single conversation
}

// GetUsage returns aggregated usage statistics.
// It executes a single GROUP BY query and derives all aggregations in Go.
func (s *Service) GetUsage(params UsageParams) (*models.UsageSummary, error) {
	start, _ := periodRange(params.Period)

	summary := &models.UsageSummary{
		Period: params.Period,
	}

	// Build WHERE clause
	where := "WHERE m.role = 'assistant'"
	args := []interface{}{}

	if !start.IsZero() {
		where += " AND m.created_at >= ?"
		args = append(args, start.Format(time.RFC3339))
	}
	if params.ConversationID != "" {
		where += " AND m.conversation_id = ?"
		args = append(args, params.ConversationID)
	}

	// Single query grouped by (provider, model) — derive totals and
	// by-provider rollups in Go instead of three separate scans.
	query := fmt.Sprintf(`
		SELECT
			COALESCE(m.provider, 'unknown'),
			COALESCE(m.model, 'unknown'),
			COALESCE(SUM(COALESCE(m.token_input, 0)), 0),
			COALESCE(SUM(COALESCE(m.token_output, 0)), 0),
			COUNT(*),
			COALESCE(AVG(CASE WHEN m.latency_ms > 0 THEN m.latency_ms END), 0)
		FROM messages m %s
		GROUP BY m.provider, m.model
		ORDER BY SUM(COALESCE(m.token_input, 0)) + SUM(COALESCE(m.token_output, 0)) DESC
	`, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage: %w", err)
	}
	defer rows.Close()

	providerMap := map[string]*models.ProviderUsage{}

	for rows.Next() {
		var mu models.ModelUsage
		if err := rows.Scan(&mu.Provider, &mu.Model, &mu.InputTokens, &mu.OutputTokens, &mu.MessageCount, &mu.AvgLatencyMs); err != nil {
			return nil, err
		}
		summary.ByModel = append(summary.ByModel, mu)

		// Accumulate totals
		summary.TotalInputTok += mu.InputTokens
		summary.TotalOutputTok += mu.OutputTokens
		summary.TotalMessages += mu.MessageCount

		// Accumulate per-provider
		pu, ok := providerMap[mu.Provider]
		if !ok {
			pu = &models.ProviderUsage{Provider: mu.Provider}
			providerMap[mu.Provider] = pu
		}
		pu.InputTokens += mu.InputTokens
		pu.OutputTokens += mu.OutputTokens
		pu.MessageCount += mu.MessageCount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Compute overall average latency from the per-model data (weighted by message count).
	var totalWeightedLatency float64
	for _, mu := range summary.ByModel {
		totalWeightedLatency += mu.AvgLatencyMs * float64(mu.MessageCount)
	}
	if summary.TotalMessages > 0 {
		summary.AvgLatencyMs = totalWeightedLatency / float64(summary.TotalMessages)
	}

	// Flatten provider map to slice, compute average latency per provider
	for _, pu := range providerMap {
		// Compute provider-level average latency
		var wl float64
		var mc int
		for _, mu := range summary.ByModel {
			if mu.Provider == pu.Provider {
				wl += mu.AvgLatencyMs * float64(mu.MessageCount)
				mc += mu.MessageCount
			}
		}
		if mc > 0 {
			pu.AvgLatencyMs = wl / float64(mc)
		}
		summary.ByProvider = append(summary.ByProvider, *pu)
	}

	return summary, nil
}

// GetUsageWithCost enhances a usage summary with cost estimates from pricing rules.
// It loads pricing rules once and matches in-memory to avoid N+1 DB calls.
func (s *Service) GetUsageWithCost(params UsageParams) (*models.UsageSummary, error) {
	summary, err := s.GetUsage(params)
	if err != nil {
		return nil, err
	}

	// Load all pricing rules once (avoids N+1 DB round-trips).
	rules, err := s.pricingRepo.List()
	if err != nil {
		return nil, fmt.Errorf("load pricing rules: %w", err)
	}

	// Calculate costs per model breakdown
	var totalCost float64
	for i, mu := range summary.ByModel {
		rule := findMatchingRule(rules, mu.Provider, mu.Model)
		if rule == nil {
			continue
		}
		inputCost := float64(mu.InputTokens) / 1_000_000 * rule.InputCostPerMTok
		outputCost := float64(mu.OutputTokens) / 1_000_000 * rule.OutputCostPerMTok
		summary.ByModel[i].EstimatedCost = inputCost + outputCost
		totalCost += inputCost + outputCost
	}

	// Calculate costs per provider breakdown
	for i, pu := range summary.ByProvider {
		var providerCost float64
		for _, mu := range summary.ByModel {
			if mu.Provider == pu.Provider {
				providerCost += mu.EstimatedCost
			}
		}
		summary.ByProvider[i].EstimatedCost = providerCost
	}

	summary.EstimatedCost = totalCost
	return summary, nil
}

// findMatchingRule picks the best matching pricing rule for a (provider, model)
// pair, using the same glob logic as PricingRepo.FindMatch but without hitting
// the DB. Returns nil if no rule matches.
func findMatchingRule(rules []models.PricingRule, providerType, model string) *models.PricingRule {
	var best *models.PricingRule
	bestSpecificity := -1

	for i, rule := range rules {
		if !strings.EqualFold(rule.ProviderType, providerType) {
			continue
		}
		matched, err := filepath.Match(strings.ToLower(rule.ModelPattern), strings.ToLower(model))
		if err != nil || !matched {
			continue
		}
		specificity := len(strings.ReplaceAll(rule.ModelPattern, "*", ""))
		if specificity > bestSpecificity {
			bestSpecificity = specificity
			best = &rules[i]
		}
	}
	return best
}
