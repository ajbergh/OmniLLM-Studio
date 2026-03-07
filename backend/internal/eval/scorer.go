package eval

import (
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// ScoreResponse scores an LLM response against an eval case.
func ScoreResponse(c models.EvalCase, response string) models.EvalCaseResult {
	result := models.EvalCaseResult{
		CaseID:    c.ID,
		Input:     c.Input,
		Response:  response,
		Breakdown: make(map[string]float64),
	}

	lowerResp := strings.ToLower(response)

	// Keyword coverage scoring
	if len(c.ExpectedKeywords) > 0 {
		hits := 0
		for _, kw := range c.ExpectedKeywords {
			if strings.Contains(lowerResp, strings.ToLower(kw)) {
				result.KeywordHits = append(result.KeywordHits, kw)
				hits++
			} else {
				result.KeywordMisses = append(result.KeywordMisses, kw)
			}
		}
		coverage := float64(hits) / float64(len(c.ExpectedKeywords))
		result.Breakdown["keyword_coverage"] = coverage
	}

	// Coherence scoring: simple heuristic based on response length and structure.
	// A more sophisticated version would use an LLM judge.
	coherence := scoreCoherence(response)
	result.Breakdown["coherence"] = coherence

	// Tool call accuracy (check if expected tool names appear in response)
	if len(c.ExpectedToolCalls) > 0 {
		matched := 0
		for _, tool := range c.ExpectedToolCalls {
			if strings.Contains(lowerResp, strings.ToLower(tool)) {
				result.ToolCallsMatched = append(result.ToolCallsMatched, tool)
				matched++
			}
		}
		if len(c.ExpectedToolCalls) > 0 {
			result.Breakdown["tool_accuracy"] = float64(matched) / float64(len(c.ExpectedToolCalls))
		}
	}

	// Compute weighted total score from scoring weights
	if len(c.Scoring) > 0 {
		total := 0.0
		for metric, weight := range c.Scoring {
			if score, ok := result.Breakdown[metric]; ok {
				total += score * weight
			}
		}
		result.Score = total
	} else {
		// Default: average all breakdown scores
		if len(result.Breakdown) > 0 {
			sum := 0.0
			for _, v := range result.Breakdown {
				sum += v
			}
			result.Score = sum / float64(len(result.Breakdown))
		}
	}

	return result
}

// scoreCoherence is a simple heuristic scorer for response coherence.
// Returns a score between 0.0 and 1.0.
func scoreCoherence(response string) float64 {
	if len(response) == 0 {
		return 0.0
	}

	score := 0.0

	// Minimum length check (at least 50 characters for a meaningful response)
	if len(response) >= 50 {
		score += 0.3
	} else if len(response) >= 20 {
		score += 0.15
	}

	// Sentence structure: check for proper sentences (period-terminated)
	sentences := strings.Count(response, ". ") + strings.Count(response, ".\n")
	if strings.HasSuffix(strings.TrimSpace(response), ".") {
		sentences++
	}
	if sentences >= 3 {
		score += 0.4
	} else if sentences >= 1 {
		score += 0.2
	}

	// Paragraph structure
	paragraphs := strings.Count(response, "\n\n")
	if paragraphs >= 1 {
		score += 0.15
	}

	// Not just repeating the same thing (unique words ratio)
	words := strings.Fields(strings.ToLower(response))
	if len(words) > 5 {
		unique := make(map[string]bool)
		for _, w := range words {
			unique[w] = true
		}
		ratio := float64(len(unique)) / float64(len(words))
		if ratio > 0.5 {
			score += 0.15
		}
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
