package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestMergeRequirementsReader struct {
	result *gitrepo.GitHubPullRequestMergeRequirementsResult
}

func (f *fakeGitHubPullRequestMergeRequirementsReader) GetPullRequestMergeRequirements(context.Context, string, int) (*gitrepo.GitHubPullRequestMergeRequirementsResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestMergeRequirementsToolIsLowRiskReadOnly(t *testing.T) {
	tools := NewGitHubPullRequestMergeRequirementsTools(&fakeGitHubPullRequestMergeRequirementsReader{})
	if len(tools) != 1 {
		t.Fatalf("tool count = %d, want 1", len(tools))
	}
	definition := tools[0].Definition().Normalized()
	if definition.Name != "github_get_pull_request_merge_requirements" || definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || !definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestMergeRequirementsToolRejectsHostedControlArguments(t *testing.T) {
	tool := &githubPullRequestMergeRequirementsTool{service: &fakeGitHubPullRequestMergeRequirementsReader{}}
	if err := tool.Validate(json.RawMessage(`{"remote":"origin","number":7}`)); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"base":"other"}`),
		json.RawMessage(`{"remote":"origin","number":7,"head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		json.RawMessage(`{"remote":"origin","number":7,"merge_method":"squash"}`),
		json.RawMessage(`{"remote":"origin","number":0}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestMergeRequirementsToolReturnsStructuredResult(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestMergeRequirementsReader{result: &gitrepo.GitHubPullRequestMergeRequirementsResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head, BaseBranch: "main",
		MergePolicyComplete: false, ClassicProtectionStatus: "unavailable_or_unprotected",
	}}
	tool := &githubPullRequestMergeRequirementsTool{service: reader}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}
