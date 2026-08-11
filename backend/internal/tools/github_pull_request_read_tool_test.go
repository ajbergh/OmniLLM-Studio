package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestReader struct {
	pull   *gitrepo.GitHubPullRequestReadResult
	list   *gitrepo.GitHubPullRequestListResult
	checks *gitrepo.GitHubPullRequestChecksResult
}

func (f *fakeGitHubPullRequestReader) GetPullRequest(context.Context, string, int) (*gitrepo.GitHubPullRequestReadResult, error) {
	return f.pull, nil
}

func (f *fakeGitHubPullRequestReader) ListPullRequests(context.Context, string, string, string, int) (*gitrepo.GitHubPullRequestListResult, error) {
	return f.list, nil
}

func (f *fakeGitHubPullRequestReader) GetPullRequestChecks(context.Context, string, int) (*gitrepo.GitHubPullRequestChecksResult, error) {
	return f.checks, nil
}

func TestGitHubPullRequestReadToolsAreLowRiskReadOnly(t *testing.T) {
	tools := NewGitHubPullRequestReadTools(&fakeGitHubPullRequestReader{})
	if len(tools) != 3 {
		t.Fatalf("tool count = %d, want 3", len(tools))
	}
	for _, tool := range tools {
		definition := tool.Definition().Normalized()
		if definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || !definition.SupportsParallel {
			t.Fatalf("unexpected definition for %s: %#v", definition.Name, definition)
		}
	}
}

func TestGitHubPullRequestReadToolsRejectHostedControlArguments(t *testing.T) {
	reader := &fakeGitHubPullRequestReader{}
	getTool := &githubPullRequestReadTool{service: reader, name: "github_get_pull_request"}
	if err := getTool.Validate(json.RawMessage(`{"remote":"origin","number":7}`)); err != nil {
		t.Fatalf("valid get args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":0}`),
	} {
		if err := getTool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}

	listTool := &githubPullRequestReadTool{service: reader, name: "github_list_pull_requests"}
	if err := listTool.Validate(json.RawMessage(`{"remote":"origin","state":"open","head_branch":"feature/read","limit":20}`)); err != nil {
		t.Fatalf("valid list args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","state":"invalid"}`),
		json.RawMessage(`{"remote":"origin","limit":21}`),
		json.RawMessage(`{"remote":"origin","base":"main"}`),
	} {
		if err := listTool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestReadToolsReturnStructuredResults(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestReader{
		pull: &gitrepo.GitHubPullRequestReadResult{Remote: "origin", Repository: "repo", Number: 9, Head: head, HeadBranch: "feature/read", BaseBranch: "main"},
		list: &gitrepo.GitHubPullRequestListResult{Remote: "origin", Repository: "repo", State: "open", PullRequests: []gitrepo.GitHubPullRequestReadResult{{Number: 9, Head: head}}},
		checks: &gitrepo.GitHubPullRequestChecksResult{Remote: "origin", Repository: "repo", PullRequest: 9, Head: head, CombinedStatus: "success"},
	}
	cases := []struct {
		name string
		args json.RawMessage
	}{
		{name: "github_get_pull_request", args: json.RawMessage(`{"remote":"origin","number":9}`)},
		{name: "github_list_pull_requests", args: json.RawMessage(`{"remote":"origin"}`)},
		{name: "github_get_pull_request_checks", args: json.RawMessage(`{"remote":"origin","number":9}`)},
	}
	for _, test := range cases {
		tool := &githubPullRequestReadTool{service: reader, name: test.name}
		result, err := tool.Execute(context.Background(), test.args)
		if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
			t.Fatalf("%s Execute() = %#v, %v", test.name, result, err)
		}
	}
}

func TestRegistryGitHubPullRequestReadGateIsIndependent(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_create":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")

	withoutRead := NewRegistry()
	for _, name := range []string{"github_get_pull_request", "github_list_pull_requests", "github_get_pull_request_checks"} {
		if _, ok := withoutRead.Get(name); ok {
			t.Fatalf("%s registered without independent read gate", name)
		}
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	withRead := NewRegistry()
	for _, name := range []string{"github_get_pull_request", "github_list_pull_requests", "github_get_pull_request_checks"} {
		if _, ok := withRead.Get(name); !ok {
			t.Fatalf("%s not registered with remote/read gates enabled", name)
		}
	}
	if _, ok := withRead.Get("github_create_draft_pull_request"); ok {
		t.Fatal("draft PR creation should remain disabled when its independent gate is off")
	}
	if _, ok := withRead.Get("git_push"); ok {
		t.Fatal("Git push should remain disabled when its independent write/push gates are off")
	}
}
