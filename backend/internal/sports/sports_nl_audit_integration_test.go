//go:build integration

package sports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type nlAuditQuestion struct {
	Number   int    `json:"number"`
	Question string `json:"question"`
	Target   string `json:"target"`
	Note     string `json:"note"`
}

type nlAuditResult struct {
	Number       int               `json:"number"`
	Question     string            `json:"question"`
	Target       string            `json:"target"`
	Status       string            `json:"status"`
	Reason       string            `json:"reason,omitempty"`
	Intent       SportsIntentType  `json:"intent,omitempty"`
	League       string            `json:"league,omitempty"`
	Sport        string            `json:"sport,omitempty"`
	TeamQuery    string            `json:"team_query,omitempty"`
	AthleteQuery string            `json:"athlete_query,omitempty"`
	DateLabel    string            `json:"date_label,omitempty"`
	Season       int               `json:"season,omitempty"`
	DurationMS   int64             `json:"duration_ms"`
	Error        string            `json:"error,omitempty"`
	UserMessage  string            `json:"user_message,omitempty"`
	Preview      string            `json:"preview,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func TestIntegration_ESPNNLQuestionSetAudit(t *testing.T) {
	if os.Getenv("ESPN_NL_AUDIT") != "1" {
		t.Skip("set ESPN_NL_AUDIT=1 to run the 100-question live ESPN NL audit")
	}

	repoRoot := sportsRepoRoot(t)
	questionsPath := filepath.Join(repoRoot, "docs", "internal_docs", "espn_go_nl_question_test_set.md")
	questions, err := loadNLAuditQuestions(questionsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 100 {
		t.Fatalf("loaded %d questions, want 100", len(questions))
	}

	now := time.Now()
	client := NewESPNClient()
	results := make([]nlAuditResult, 0, len(questions))

	for _, q := range questions {
		result := runNLAuditQuestion(t, client, q, now)
		results = append(results, result)
		t.Logf("Q%03d %-18s intent=%s league=%s reason=%s", result.Number, result.Status, result.Intent, result.League, result.Reason)
		time.Sleep(150 * time.Millisecond)
	}

	outDir := filepath.Join(repoRoot, "output", "espn_nl_audit")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(outDir, "espn_100_live_results.json")
	mdPath := filepath.Join(outDir, "espn_100_live_results.md")

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mdPath, []byte(renderNLAuditMarkdown(results, now)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", jsonPath)
	t.Logf("wrote %s", mdPath)
}

func runNLAuditQuestion(t *testing.T, client *ESPNClient, q nlAuditQuestion, now time.Time) nlAuditResult {
	t.Helper()
	start := time.Now()
	out := nlAuditResult{
		Number:   q.Number,
		Question: q.Question,
		Target:   q.Target,
		Metadata: map[string]string{"note": q.Note},
	}

	req, ok := DetectSportsIntent(q.Question, now)
	if !ok {
		out.Status = "fail"
		out.Reason = "intent_not_detected"
		out.DurationMS = time.Since(start).Milliseconds()
		return out
	}
	out.Intent = req.Intent
	out.League = req.League
	out.Sport = req.Sport
	out.TeamQuery = req.TeamQuery
	out.AthleteQuery = req.AthleteQuery
	out.DateLabel = req.DateLabel
	out.Season = req.Season

	if err := ValidateDateInQuery(q.Question, now); err != nil {
		out.Status = "fail"
		out.Reason = "date_validation_failed"
		out.Error = err.Error()
		out.UserMessage = UserFacingError(*req, err)
		out.DurationMS = time.Since(start).Milliseconds()
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := client.Lookup(ctx, *req)
	out.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		out.Error = err.Error()
		out.UserMessage = UserFacingError(*req, err)
		if isGracefulSportsError(err) && strings.TrimSpace(out.UserMessage) != "" {
			out.Status = "graceful_fallback"
			out.Reason = sportsErrorReason(err)
			return out
		}
		out.Status = "fail"
		out.Reason = "lookup_error"
		return out
	}
	if result == nil || strings.TrimSpace(result.Markdown) == "" {
		out.Status = "fail"
		out.Reason = "empty_result"
		return out
	}
	out.Preview = compactPreview(result.Markdown)
	out.Warnings = auditWarnings(*req, result.Markdown)
	if len(out.Warnings) > 0 {
		out.Status = "needs_review"
		out.Reason = strings.Join(out.Warnings, "; ")
		return out
	}
	out.Status = "pass"
	out.Reason = "live_response"
	return out
}

func loadNLAuditQuestions(path string) ([]nlAuditQuestion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rowRe := regexp.MustCompile(`^\|\s*([0-9]+)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|`)
	var questions []nlAuditQuestion
	for _, line := range strings.Split(string(raw), "\n") {
		m := rowRe.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		if m[1] == "---" {
			continue
		}
		n := 0
		for _, r := range m[1] {
			n = n*10 + int(r-'0')
		}
		if n <= 0 {
			continue
		}
		questions = append(questions, nlAuditQuestion{
			Number:   n,
			Question: cleanMarkdownCell(m[2]),
			Target:   cleanMarkdownCell(m[3]),
			Note:     cleanMarkdownCell(m[4]),
		})
	}
	return questions, nil
}

func sportsRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func cleanMarkdownCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

func isGracefulSportsError(err error) bool {
	return errors.Is(err, ErrUnsupportedLeague) ||
		errors.Is(err, ErrNoGames) ||
		errors.Is(err, ErrNoMatchingGames) ||
		errors.Is(err, ErrNoStandings) ||
		errors.Is(err, ErrNoNews) ||
		errors.Is(err, ErrNoOdds) ||
		errors.Is(err, ErrNoSportsData) ||
		errors.Is(err, ErrTeamNotFound) ||
		errors.Is(err, ErrAthleteNotFound)
}

func sportsErrorReason(err error) string {
	switch {
	case errors.Is(err, ErrUnsupportedLeague):
		return "unsupported_league_or_endpoint"
	case errors.Is(err, ErrNoGames):
		return "no_games"
	case errors.Is(err, ErrNoMatchingGames):
		return "no_matching_games"
	case errors.Is(err, ErrNoStandings):
		return "no_standings"
	case errors.Is(err, ErrNoNews):
		return "no_news"
	case errors.Is(err, ErrNoOdds):
		return "no_odds"
	case errors.Is(err, ErrNoSportsData):
		return "no_sports_data"
	case errors.Is(err, ErrTeamNotFound):
		return "team_not_found"
	case errors.Is(err, ErrAthleteNotFound):
		return "athlete_not_found"
	default:
		return "graceful_error"
	}
}

func auditWarnings(req SportsRequest, markdown string) []string {
	normMarkdown := normalizeText(markdown)
	var warnings []string
	rawMarkers := []string{
		"sports.core.api.espn.com",
		"site.api.espn.com/apis",
		"now.core.api.espn.com",
		"$ref",
		"| Ref URL |",
		"System.Object",
	}
	for _, marker := range rawMarkers {
		if strings.Contains(markdown, marker) {
			warnings = append(warnings, "raw_api_artifact")
			break
		}
	}
	emptyStateMarkers := []string{
		"No ESPN Rows Returned",
		"No Games Found",
		"No ESPN Articles Found",
		"No ESPN Odds Found",
		"No data returned",
		"No games returned",
		"No news returned",
		"No odds returned",
	}
	for _, marker := range emptyStateMarkers {
		if strings.Contains(markdown, marker) {
			warnings = append(warnings, "empty_state_response")
			break
		}
	}
	if strings.TrimSpace(req.TeamQuery) != "" && !strings.Contains(normMarkdown, normalizeText(req.TeamQuery)) {
		warnings = append(warnings, "team_not_visible_in_response")
	}
	if strings.TrimSpace(req.AthleteQuery) != "" && !strings.Contains(normMarkdown, normalizeText(req.AthleteQuery)) {
		warnings = append(warnings, "athlete_not_visible_in_response")
	}
	return warnings
}

func compactPreview(markdown string) string {
	preview := strings.TrimSpace(strings.Join(strings.Fields(stripHTMLTags(markdown)), " "))
	if len(preview) > 220 {
		return preview[:217] + "..."
	}
	return preview
}

func renderNLAuditMarkdown(results []nlAuditResult, now time.Time) string {
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Status]++
	}
	var b strings.Builder
	b.WriteString("# ESPN NL 100 Live Audit\n\n")
	b.WriteString("Run time: ")
	b.WriteString(now.Format(time.RFC3339))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("- pass: %d\n", counts["pass"]))
	b.WriteString(fmt.Sprintf("- graceful_fallback: %d\n", counts["graceful_fallback"]))
	b.WriteString(fmt.Sprintf("- needs_review: %d\n", counts["needs_review"]))
	b.WriteString(fmt.Sprintf("- fail: %d\n\n", counts["fail"]))
	b.WriteString("| # | Status | Intent | League | Reason | Question |\n")
	b.WriteString("|---:|---|---|---|---|---|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n",
			r.Number,
			escapeMarkdownCell(r.Status),
			escapeMarkdownCell(string(r.Intent)),
			escapeMarkdownCell(r.League),
			escapeMarkdownCell(r.Reason),
			escapeMarkdownCell(r.Question),
		))
	}
	return b.String()
}
