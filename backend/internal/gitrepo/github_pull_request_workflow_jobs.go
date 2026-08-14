package gitrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	maxGitHubWorkflowRuns       = 10
	maxGitHubJobsPerWorkflowRun = 20
	maxGitHubWorkflowJobsTotal  = 50
	maxGitHubStepsPerJob        = 30
	maxGitHubWorkflowStepsTotal = 200
)

// GitHubPullRequestWorkflowJobsReader is the bounded PR-scoped GitHub Actions
// metadata boundary. Implementations derive the exact hosted head and all
// provider run/job IDs internally; callers never provide a commit SHA, run ID,
// job ID, API URL, or token.
type GitHubPullRequestWorkflowJobsReader interface {
	GetPullRequestWorkflowJobs(ctx context.Context, remoteID string, number int) (*GitHubPullRequestWorkflowJobsResult, error)
}

// GitHubPullRequestWorkflowJobsResult contains bounded workflow/job metadata for
// the exact pull-request head. It intentionally omits provider object IDs, URLs,
// logs, artifacts, runner names, and output text.
type GitHubPullRequestWorkflowJobsResult struct {
	Remote        string                    `json:"remote"`
	Repository    string                    `json:"repository"`
	PullRequest   int                       `json:"pull_request"`
	Head          string                    `json:"head"`
	WorkflowRuns  []GitHubWorkflowRunResult `json:"workflow_runs"`
	RunsTruncated bool                      `json:"runs_truncated,omitempty"`
	JobsTruncated bool                      `json:"jobs_truncated,omitempty"`
	StepsTruncated bool                     `json:"steps_truncated,omitempty"`
}

// GitHubWorkflowRunResult is bounded untrusted metadata for one Actions run.
type GitHubWorkflowRunResult struct {
	Name          string                    `json:"name,omitempty"`
	Event         string                    `json:"event,omitempty"`
	Status        string                    `json:"status,omitempty"`
	Conclusion    string                    `json:"conclusion,omitempty"`
	RunNumber     int                       `json:"run_number,omitempty"`
	Attempt       int                       `json:"attempt,omitempty"`
	CreatedAt     string                    `json:"created_at,omitempty"`
	UpdatedAt     string                    `json:"updated_at,omitempty"`
	Jobs          []GitHubWorkflowJobResult `json:"jobs,omitempty"`
	JobsTruncated bool                      `json:"jobs_truncated,omitempty"`
}

// GitHubWorkflowJobResult is bounded untrusted metadata for one Actions job.
type GitHubWorkflowJobResult struct {
	Name           string                     `json:"name,omitempty"`
	Status         string                     `json:"status,omitempty"`
	Conclusion     string                     `json:"conclusion,omitempty"`
	StartedAt      string                     `json:"started_at,omitempty"`
	CompletedAt    string                     `json:"completed_at,omitempty"`
	Steps          []GitHubWorkflowStepResult `json:"steps,omitempty"`
	StepsTruncated bool                       `json:"steps_truncated,omitempty"`
}

// GitHubWorkflowStepResult is bounded status-only step metadata. Step output and
// command text are deliberately not exposed.
type GitHubWorkflowStepResult struct {
	Name       string `json:"name,omitempty"`
	Number     int    `json:"number,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
}

type githubWorkflowRunsResponse struct {
	TotalCount   int `json:"total_count"`
	WorkflowRuns []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Event      string `json:"event"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		RunNumber  int    `json:"run_number"`
		RunAttempt int    `json:"run_attempt"`
		HeadSHA    string `json:"head_sha"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	} `json:"workflow_runs"`
}

type githubWorkflowJobsResponse struct {
	TotalCount int `json:"total_count"`
	Jobs       []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		Conclusion  string `json:"conclusion"`
		StartedAt   string `json:"started_at"`
		CompletedAt string `json:"completed_at"`
		Steps       []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			Number     int    `json:"number"`
		} `json:"steps"`
	} `json:"jobs"`
}

// GetPullRequestWorkflowJobs fetches Actions metadata only for workflow runs
// whose provider-side head SHA matches a freshly fetched pull-request head. The
// method accepts no provider object IDs and returns no log/artifact/output URLs.
func (s *RemoteService) GetPullRequestWorkflowJobs(ctx context.Context, remoteID string, number int) (*GitHubPullRequestWorkflowJobsResult, error) {
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
	head := strings.ToLower(pull.Head.SHA)

	query := url.Values{}
	query.Set("head_sha", head)
	query.Set("per_page", fmt.Sprintf("%d", maxGitHubWorkflowRuns+1))
	endpoint := fmt.Sprintf("/repos/%s/%s/actions/runs?%s", owner, repository, query.Encode())
	var response githubWorkflowRunsResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &response); err != nil {
		return nil, fmt.Errorf("GitHub workflow runs could not be inspected")
	}

	runsTruncated := response.TotalCount > maxGitHubWorkflowRuns || len(response.WorkflowRuns) > maxGitHubWorkflowRuns
	if len(response.WorkflowRuns) > maxGitHubWorkflowRuns {
		response.WorkflowRuns = response.WorkflowRuns[:maxGitHubWorkflowRuns]
	}
	resultRuns := make([]GitHubWorkflowRunResult, 0, len(response.WorkflowRuns))
	jobsTruncated := false
	stepsTruncated := false
	totalJobs := 0
	totalSteps := 0

	for _, run := range response.WorkflowRuns {
		if run.ID <= 0 || !strings.EqualFold(strings.TrimSpace(run.HeadSHA), head) {
			return nil, fmt.Errorf("GitHub workflow run response was not bound to the pull request head")
		}
		if totalJobs >= maxGitHubWorkflowJobsTotal {
			jobsTruncated = true
			break
		}

		jobQuery := url.Values{}
		jobQuery.Set("filter", "latest")
		jobQuery.Set("per_page", fmt.Sprintf("%d", maxGitHubJobsPerWorkflowRun+1))
		jobEndpoint := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs?%s", owner, repository, run.ID, jobQuery.Encode())
		var jobResponse githubWorkflowJobsResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, jobEndpoint, nil, http.StatusOK, &jobResponse); err != nil {
			return nil, fmt.Errorf("GitHub workflow jobs could not be inspected")
		}
		perRunTruncated := jobResponse.TotalCount > maxGitHubJobsPerWorkflowRun || len(jobResponse.Jobs) > maxGitHubJobsPerWorkflowRun
		if len(jobResponse.Jobs) > maxGitHubJobsPerWorkflowRun {
			jobResponse.Jobs = jobResponse.Jobs[:maxGitHubJobsPerWorkflowRun]
		}
		jobs := make([]GitHubWorkflowJobResult, 0, len(jobResponse.Jobs))
		for _, job := range jobResponse.Jobs {
			if totalJobs >= maxGitHubWorkflowJobsTotal {
				perRunTruncated = true
				jobsTruncated = true
				break
			}
			if job.ID <= 0 {
				return nil, fmt.Errorf("GitHub workflow job response was incomplete")
			}
			jobStepsTruncated := len(job.Steps) > maxGitHubStepsPerJob
			if len(job.Steps) > maxGitHubStepsPerJob {
				job.Steps = job.Steps[:maxGitHubStepsPerJob]
			}
			steps := make([]GitHubWorkflowStepResult, 0, len(job.Steps))
			for _, step := range job.Steps {
				if totalSteps >= maxGitHubWorkflowStepsTotal {
					jobStepsTruncated = true
					stepsTruncated = true
					break
				}
				steps = append(steps, GitHubWorkflowStepResult{
					Name:       boundedGitHubDiagnosticText(step.Name, maxGitHubDiagnosticTitleRunes),
					Number:     step.Number,
					Status:     boundedGitHubDiagnosticText(step.Status, 32),
					Conclusion: boundedGitHubDiagnosticText(step.Conclusion, 32),
				})
				totalSteps++
			}
			if jobStepsTruncated {
				stepsTruncated = true
			}
			jobs = append(jobs, GitHubWorkflowJobResult{
				Name:           boundedGitHubDiagnosticText(job.Name, maxGitHubPullRequestTitleRunes),
				Status:         boundedGitHubDiagnosticText(job.Status, 32),
				Conclusion:     boundedGitHubDiagnosticText(job.Conclusion, 32),
				StartedAt:      boundedGitHubDiagnosticText(job.StartedAt, 64),
				CompletedAt:    boundedGitHubDiagnosticText(job.CompletedAt, 64),
				Steps:          steps,
				StepsTruncated: jobStepsTruncated,
			})
			totalJobs++
		}
		if perRunTruncated {
			jobsTruncated = true
		}
		resultRuns = append(resultRuns, GitHubWorkflowRunResult{
			Name:          boundedGitHubDiagnosticText(run.Name, maxGitHubPullRequestTitleRunes),
			Event:         boundedGitHubDiagnosticText(run.Event, 64),
			Status:        boundedGitHubDiagnosticText(run.Status, 32),
			Conclusion:    boundedGitHubDiagnosticText(run.Conclusion, 32),
			RunNumber:     run.RunNumber,
			Attempt:       run.RunAttempt,
			CreatedAt:     boundedGitHubDiagnosticText(run.CreatedAt, 64),
			UpdatedAt:     boundedGitHubDiagnosticText(run.UpdatedAt, 64),
			Jobs:          jobs,
			JobsTruncated: perRunTruncated,
		})
	}

	return &GitHubPullRequestWorkflowJobsResult{
		Remote:         strings.TrimSpace(remoteID),
		Repository:     remote.Repository,
		PullRequest:    number,
		Head:           head,
		WorkflowRuns:   resultRuns,
		RunsTruncated:  runsTruncated,
		JobsTruncated:  jobsTruncated,
		StepsTruncated: stepsTruncated,
	}, nil
}
