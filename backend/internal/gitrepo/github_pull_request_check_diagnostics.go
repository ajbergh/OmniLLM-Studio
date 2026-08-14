package gitrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxGitHubDiagnosticChecks       = 10
	maxGitHubAnnotationsPerCheck    = 20
	maxGitHubDiagnosticAnnotations  = 50
	maxGitHubDiagnosticPathRunes    = 512
	maxGitHubDiagnosticTitleRunes   = 256
	maxGitHubDiagnosticMessageRunes = 4096
)

// GitHubPullRequestCheckDiagnosticsReader is the bounded PR-scoped CI diagnostic
// boundary. Implementations derive the exact hosted head and check-run IDs
// internally; callers never provide a commit SHA, check ID, API URL, or token.
type GitHubPullRequestCheckDiagnosticsReader interface {
	GetPullRequestCheckDiagnostics(ctx context.Context, remoteID string, number int) (*GitHubPullRequestCheckDiagnosticsResult, error)
}

// GitHubPullRequestCheckDiagnosticsResult contains bounded annotations from
// failing checks for one exact pull-request head. Annotation text is untrusted
// provider content for diagnosis only and is never authorization evidence.
type GitHubPullRequestCheckDiagnosticsResult struct {
	Remote               string                        `json:"remote"`
	Repository           string                        `json:"repository"`
	PullRequest          int                           `json:"pull_request"`
	Head                 string                        `json:"head"`
	Checks               []GitHubCheckDiagnosticResult `json:"checks"`
	ChecksTruncated      bool                          `json:"checks_truncated,omitempty"`
	AnnotationsTruncated bool                          `json:"annotations_truncated,omitempty"`
}

// GitHubCheckDiagnosticResult describes one completed non-successful check and
// the bounded annotations GitHub exposes for it. URLs and raw check output are
// intentionally omitted.
type GitHubCheckDiagnosticResult struct {
	Name                 string                        `json:"name"`
	Status               string                        `json:"status"`
	Conclusion           string                        `json:"conclusion,omitempty"`
	App                  string                        `json:"app,omitempty"`
	Annotations          []GitHubCheckAnnotationResult `json:"annotations,omitempty"`
	AnnotationsTruncated bool                          `json:"annotations_truncated,omitempty"`
}

// GitHubCheckAnnotationResult is a bounded repository-relative diagnostic. The
// message/title originate from a hosted check and must be treated as untrusted
// reference data.
type GitHubCheckAnnotationResult struct {
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Level     string `json:"level,omitempty"`
	Title     string `json:"title,omitempty"`
	Message   string `json:"message,omitempty"`
}

type githubDiagnosticCheckRunsResponse struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"app"`
	} `json:"check_runs"`
}

type githubDiagnosticAnnotationResponse struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Message         string `json:"message"`
	Title           string `json:"title"`
}

// GetPullRequestCheckDiagnostics inspects only failing checks associated with
// the exact head returned by a fresh PR read. It deliberately returns annotations
// instead of raw workflow logs/artifacts, which can be much larger and may carry
// unrelated or sensitive output.
func (s *RemoteService) GetPullRequestCheckDiagnostics(ctx context.Context, remoteID string, number int) (*GitHubPullRequestCheckDiagnosticsResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if !validRemoteHash(pull.Head.SHA) {
		return nil, fmt.Errorf("GitHub pull request head could not be validated")
	}

	query := url.Values{}
	query.Set("filter", "latest")
	query.Set("per_page", fmt.Sprintf("%d", maxGitHubCheckResults+1))
	endpoint := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?%s", owner, repository, pull.Head.SHA, query.Encode())
	var response githubDiagnosticCheckRunsResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &response); err != nil {
		return nil, fmt.Errorf("GitHub check diagnostics could not be inspected")
	}

	checks := make([]GitHubCheckDiagnosticResult, 0, maxGitHubDiagnosticChecks)
	checksTruncated := false
	annotationsTruncated := false
	totalAnnotations := 0
	for _, check := range response.CheckRuns {
		if !githubCheckNeedsDiagnostics(check.Status, check.Conclusion) {
			continue
		}
		if len(checks) >= maxGitHubDiagnosticChecks {
			checksTruncated = true
			break
		}
		if check.ID <= 0 {
			return nil, fmt.Errorf("GitHub check diagnostics response was incomplete")
		}

		annotationQuery := url.Values{}
		annotationQuery.Set("per_page", fmt.Sprintf("%d", maxGitHubAnnotationsPerCheck+1))
		annotationEndpoint := fmt.Sprintf("/repos/%s/%s/check-runs/%d/annotations?%s", owner, repository, check.ID, annotationQuery.Encode())
		var annotationResponse []githubDiagnosticAnnotationResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, annotationEndpoint, nil, http.StatusOK, &annotationResponse); err != nil {
			return nil, fmt.Errorf("GitHub check annotations could not be inspected")
		}

		perCheckTruncated := len(annotationResponse) > maxGitHubAnnotationsPerCheck
		if perCheckTruncated {
			annotationResponse = annotationResponse[:maxGitHubAnnotationsPerCheck]
			annotationsTruncated = true
		}
		annotations := make([]GitHubCheckAnnotationResult, 0, len(annotationResponse))
		for _, annotation := range annotationResponse {
			if totalAnnotations >= maxGitHubDiagnosticAnnotations {
				perCheckTruncated = true
				annotationsTruncated = true
				break
			}
			annotations = append(annotations, GitHubCheckAnnotationResult{
				Path:      boundedGitHubDiagnosticText(annotation.Path, maxGitHubDiagnosticPathRunes),
				StartLine: annotation.StartLine,
				EndLine:   annotation.EndLine,
				Level:     boundedGitHubDiagnosticText(annotation.AnnotationLevel, 32),
				Title:     boundedGitHubDiagnosticText(annotation.Title, maxGitHubDiagnosticTitleRunes),
				Message:   boundedGitHubDiagnosticText(annotation.Message, maxGitHubDiagnosticMessageRunes),
			})
			totalAnnotations++
		}
		app := check.App.Slug
		if app == "" {
			app = check.App.Name
		}
		checks = append(checks, GitHubCheckDiagnosticResult{
			Name:                 boundedGitHubDiagnosticText(check.Name, maxGitHubPullRequestTitleRunes),
			Status:               boundedGitHubDiagnosticText(check.Status, 32),
			Conclusion:           boundedGitHubDiagnosticText(check.Conclusion, 32),
			App:                  boundedGitHubDiagnosticText(app, 128),
			Annotations:          annotations,
			AnnotationsTruncated: perCheckTruncated,
		})
	}

	return &GitHubPullRequestCheckDiagnosticsResult{
		Remote:               strings.TrimSpace(remoteID),
		Repository:           remote.Repository,
		PullRequest:          number,
		Head:                 strings.ToLower(pull.Head.SHA),
		Checks:               checks,
		ChecksTruncated:      checksTruncated,
		AnnotationsTruncated: annotationsTruncated,
	}, nil
}

func githubCheckNeedsDiagnostics(status, conclusion string) bool {
	if !strings.EqualFold(strings.TrimSpace(status), "completed") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "", "success", "neutral", "skipped":
		return false
	default:
		return true
	}
}

func boundedGitHubDiagnosticText(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}
