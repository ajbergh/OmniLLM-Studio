package eval

import (
	"encoding/json"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// Report holds the full results of an evaluation run.
type Report struct {
	SuiteName  string                  `json:"suite_name"`
	Provider   string                  `json:"provider"`
	Model      string                  `json:"model"`
	TotalScore float64                 `json:"total_score"`
	CaseCount  int                     `json:"case_count"`
	Results    []models.EvalCaseResult `json:"results"`
}

// GenerateReport creates a summary report from individual case results.
func GenerateReport(suite models.EvalSuite, provider, model string, results []models.EvalCaseResult) Report {
	report := Report{
		SuiteName: suite.Name,
		Provider:  provider,
		Model:     model,
		CaseCount: len(results),
		Results:   results,
	}

	if len(results) > 0 {
		total := 0.0
		for _, r := range results {
			total += r.Score
		}
		report.TotalScore = total / float64(len(results))
	}

	return report
}

// ReportToJSON serializes a report to JSON string for DB storage.
func ReportToJSON(report Report) string {
	data, _ := json.Marshal(report.Results)
	return string(data)
}
