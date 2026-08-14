package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestCheckDiagnosticsReader struct {
	result *gitrepo.GitHubPullRequestCheckDiagnosticsResult
}

func (f *fakeGitHubPullRequestCheckDiagnosticsReader) GetPullRequest(context.Context, string, int) (*gitrepo.GitHubPullRequestReadResult, error) {
	return &gitrepo.GitHubPullRequestReadResult{}, nil
}

func (f *fakeGitHubPullRequestCheckDiagnosticsReader) ListPullRequests(context.Context, string, string, string, int) (*gitrepo.GitHubPullRequestListResult, error) {
	return &gitrepo.GitHubPullRequestListResult{}, nil
}

func (f *fakeGitHubPullRequestCheckDiagnosticsReader) GetPullRequestChecks(context.Context, string, int) (*gitrepo.GitHubPullRequestChecksResult, error) {
	return &gitrepo.GitHubPullRequestChecksResult{}, nil
}

func (f *fakeGitHubPullRequestCheckDiagnosticsReader) GetPullRequestCheckDiagnostics(context.Context, string, int) (*gitrepo.GitHubPullRequestCheckDiagnosticsResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestCheckDiagnosticsToolIsLowRiskReadOnly(t *testing.T) {
	reader := &fakeGitHubPullRequestCheckDiagnosticsReader{}
	registered := NewGitHubPullRequestReadTools(reader)
	var diagnostic Tool
	for _, candidate := range registered {
		if candidate.Definition().Name == "github_get_pull_request_check_diagnostics" {
			diagnostic = candidate
			break
		}
	}
	if diagnostic == nil {
		t.Fatal("github_get_pull_request_check_diagnostics was not registered for a diagnostic-capable reader")
	}
	definition := diagnostic.Definition().Normalized()
	if definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || !definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestCheckDiagnosticsToolRejectsHostedControlArguments(t *testing.T) {
	tool := NewGitHubPullRequestCheckDiagnosticsTool(&fakeGitHubPullRequestCheckDiagnosticsReader{})
	if err := tool.Validate(json.RawMessage(`{"remote":"origin","number":7}`)); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		json.RawMessage(`{"remote":"origin","number":7,"check_run_id":123}`),
		json.RawMessage(`{"remote":"origin","number":7,"api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":0}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestCheckDiagnosticsToolReturnsStructuredResult(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestCheckDiagnosticsReader{result: &gitrepo.GitHubPullRequestCheckDiagnosticsResult{
		Remote: "origin", Repository: "repo", PullRequest: 9, Head: head,
		Checks: []gitrepo.GitHubCheckDiagnosticResult{{Name: "Quality Gate", Conclusion: "failure"}},
	}}
	tool := NewGitHubPullRequestCheckDiagnosticsTool(reader)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":9}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}

func TestRegistryGitHubPullRequestCheckDiagnosticsUsesReadGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")

	withoutRead := NewRegistry()
	if _, ok := withoutRead.Get("github_get_pull_request_check_diagnostics"); ok {
		t.Fatal("diagnostic tool registered without independent PR read gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	withRead := NewRegistry()
	if _, ok := withRead.Get("github_get_pull_request_check_diagnostics"); !ok {
		t.Fatal("diagnostic tool not registered with remote/read gates enabled")
	}
}
