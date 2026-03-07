package repository

import (
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// PricingRepo manages pricing rules in the DB.
type PricingRepo struct {
	db *sql.DB
}

// NewPricingRepo creates a PricingRepo.
func NewPricingRepo(db *sql.DB) *PricingRepo {
	return &PricingRepo{db: db}
}

// List returns all pricing rules ordered by provider_type.
func (r *PricingRepo) List() ([]models.PricingRule, error) {
	rows, err := r.db.Query(`
		SELECT id, provider_type, model_pattern, input_cost_per_mtok,
		       output_cost_per_mtok, currency, effective_from, created_at
		FROM pricing_rules
		ORDER BY provider_type, model_pattern
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.PricingRule
	for rows.Next() {
		var p models.PricingRule
		if err := rows.Scan(
			&p.ID, &p.ProviderType, &p.ModelPattern,
			&p.InputCostPerMTok, &p.OutputCostPerMTok,
			&p.Currency, &p.EffectiveFrom, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, p)
	}
	return rules, rows.Err()
}

// Upsert inserts or updates a pricing rule.
func (r *PricingRepo) Upsert(rule models.PricingRule) error {
	_, err := r.db.Exec(`
		INSERT INTO pricing_rules (id, provider_type, model_pattern,
			input_cost_per_mtok, output_cost_per_mtok, currency, effective_from)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider_type = excluded.provider_type,
			model_pattern = excluded.model_pattern,
			input_cost_per_mtok = excluded.input_cost_per_mtok,
			output_cost_per_mtok = excluded.output_cost_per_mtok,
			currency = excluded.currency,
			effective_from = excluded.effective_from
	`, rule.ID, rule.ProviderType, rule.ModelPattern,
		rule.InputCostPerMTok, rule.OutputCostPerMTok,
		rule.Currency, rule.EffectiveFrom)
	return err
}

// Delete removes a pricing rule by ID.
func (r *PricingRepo) Delete(id string) error {
	res, err := r.db.Exec("DELETE FROM pricing_rules WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindMatch returns the best matching pricing rule for a given provider type
// and model name. It uses glob-style matching on model_pattern (e.g. "gpt-4*"
// matches "gpt-4-turbo"). Returns nil if no match found.
func (r *PricingRepo) FindMatch(providerType, model string) (*models.PricingRule, error) {
	rules, err := r.List()
	if err != nil {
		return nil, err
	}

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
		// Prefer more-specific patterns (longer pattern = more specific)
		specificity := len(rule.ModelPattern)
		if specificity > bestSpecificity {
			bestSpecificity = specificity
			best = &rules[i]
		}
	}
	return best, nil
}
