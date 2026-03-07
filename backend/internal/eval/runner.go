package eval

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// LLMClient is the interface for making LLM calls during evaluation.
type LLMClient interface {
	ChatComplete(ctx context.Context, providerName, userMessage, systemPrompt string) (string, error)
}

// Runner executes evaluation suites against LLM providers.
type Runner struct {
	llm LLMClient
}

// NewRunner creates a new eval runner.
func NewRunner(llm LLMClient) *Runner {
	return &Runner{llm: llm}
}

// RunSuite executes all cases in an eval suite and returns a report.
func (r *Runner) RunSuite(ctx context.Context, suite models.EvalSuite, provider, model string) (Report, error) {
	var results []models.EvalCaseResult

	systemPrompt := fmt.Sprintf(
		"You are being evaluated. Provider: %s, Model: %s. Respond thoroughly and accurately.",
		provider, model,
	)

	for _, c := range suite.Cases {
		response, err := r.llm.ChatComplete(ctx, provider, c.Input, systemPrompt)
		if err != nil {
			// Record failed case with zero score
			results = append(results, models.EvalCaseResult{
				CaseID:   c.ID,
				Input:    c.Input,
				Response: fmt.Sprintf("ERROR: %v", err),
				Score:    0,
			})
			continue
		}

		result := ScoreResponse(c, response)
		results = append(results, result)
	}

	return GenerateReport(suite, provider, model, results), nil
}

// ParseSuite parses an eval suite from JSON data.
func ParseSuite(data []byte) (*models.EvalSuite, error) {
	var suite models.EvalSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse eval suite: %w", err)
	}
	if suite.Name == "" {
		return nil, fmt.Errorf("eval suite name is required")
	}
	if len(suite.Cases) == 0 {
		return nil, fmt.Errorf("eval suite must have at least one case")
	}
	for i, c := range suite.Cases {
		if c.ID == "" {
			return nil, fmt.Errorf("case %d: id is required", i)
		}
		if c.Input == "" {
			return nil, fmt.Errorf("case %d: input is required", i)
		}
	}
	return &suite, nil
}
