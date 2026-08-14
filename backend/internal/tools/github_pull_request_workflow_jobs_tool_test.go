package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestWorkflowJobsReader struct {
	result *gitrepo.GitHubPullRequestWorkflowJobsResult
}

func (f *fakeGitHubPullRequestWorkflowJobsReader) GetPullRequest(context.Context, string, int) (*gitrepo.GitHubPullRequestReadResult, error) {
	return &gitrepo.GitHubPullRequestReadResult{}, nil
}

func (f *fakeGitHubPullRequestWorkflowJobsReader) ListPullRequests(context.Context, string, string, string, int) (*gitrepo.GitHubPullRequestListResult, error) {
	return &gitrepo.GitHubPullRequestListResult{}, nil
}

func (f *fakeGitHubPullRequestWorkflowJobsReader) GetPullRequestChecks(context.Context, string, int) (*gitrepo.GitHubPullRequestChecksResult, error) {
	return &gitrepo.GitHubPullRequestChecksResult{}, nil
}

func (f *fakeGitHubPullRequestWorkflowJobsReader) GetPullRequestWorkflowJobs(context.Context, string, int) (*gitrepo.GitHubPullRequestWorkflowJobsResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestWorkflowJobsToolIsLowRiskReadOnly(t *testing.T) {
	reader := &fakeGitHubPullRequestWorkflowJobsReader{}
	registered := NewGitHubPullRequestReadTools(reader)
	var workflowTool Tool
	for _, candidate := range registered {
		if candidate.Definition().Name == "github_get_pull_request_workflow_jobs" {
			workflowTool = candidate
			break
		}
	}
	if workflowTool == nil {
		t.Fatal("github_get_pull_request_workflow_jobs was not registered for a workflow-capable reader")
	}
	definition := workflowTool.Definition().Normalized()
	if definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || !definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestWorkflowJobsToolRejectsProviderControlArguments(t *testing.T) {
	tool := NewGitHubPullRequestWorkflowJobsTool(&fakeGitHubPullRequestWorkflowJobsReader{})
	if err := tool.Validate(json.RawMessage(`{"remote":"origin","number":7}`)); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		json.RawMessage(`{"remote":"origin","number":7,"run_id":123}`),
		json.RawMessage(`{"remote":"origin","number":7,"job_id":456}`),
		json.RawMessage(`{"remote":"origin","number":7,"api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":0}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestWorkflowJobsToolReturnsStructuredResult(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestWorkflowJobsReader{result: &gitrepo.GitHubPullRequestWorkflowJobsResult{
		Remote: "origin", Repository: "repo", PullRequest: 9, Head: head,
		WorkflowRuns: []gitrepo.GitHubWorkflowRunResult{{Name: "Quality Gate", Conclusion: "failure"}},
	}}
	tool := NewGitHubPullRequestWorkflowJobsTool(reader)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":9}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}

func TestRegistryGitHubPullRequestWorkflowJobsUsesReadGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")

	withoutRead := NewRegistry()
	if _, ok := withoutRead.Get("github_get_pull_request_workflow_jobs"); ok {
		t.Fatal("workflow job tool registered without independent PR read gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	withRead := NewRegistry()
	if _, ok := withRead.Get("github_get_pull_request_workflow_jobs"); !ok {
		t.Fatal("workflow job tool not registered with remote/read gates enabled")
	}
}
