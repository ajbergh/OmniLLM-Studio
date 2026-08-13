package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestMergeEligibilityReader struct {
	fakeGitHubPullRequestMergePolicyEvidenceReader
	eligibility *gitrepo.GitHubPullRequestMergeEligibilityResult
}

func (f *fakeGitHubPullRequestMergeEligibilityReader) GetPullRequestMergeEligibility(context.Context, string, int) (*gitrepo.GitHubPullRequestMergeEligibilityResult, error) {
	return f.eligibility, nil
}

func TestGitHubPullRequestMergeEligibilityToolIsAdditiveReadOnly(t *testing.T) {
	reader := &fakeGitHubPullRequestMergeEligibilityReader{}
	registered := NewGitHubPullRequestMergeRequirementsTools(reader)
	if len(registered) != 3 {
		t.Fatalf("tool count = %d, want 3", len(registered))
	}
	definition := registered[2].Definition().Normalized()
	if definition.Name != "github_get_pull_request_merge_eligibility" || definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || definition.SupportsParallel {
		t.Fatalf("unexpected eligibility definition: %#v", definition)
	}
}

func TestGitHubPullRequestMergeEligibilityToolRejectsHostedSelectors(t *testing.T) {
	tool := &githubPullRequestMergeEligibilityTool{service: &fakeGitHubPullRequestMergeEligibilityReader{}}
	if err := tool.Validate(json.RawMessage(`{"remote":"origin","number":7}`)); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		json.RawMessage(`{"remote":"origin","number":7,"base":"other"}`),
		json.RawMessage(`{"remote":"origin","number":7,"check":"Quality Gate"}`),
		json.RawMessage(`{"remote":"origin","number":7,"environment":"production"}`),
		json.RawMessage(`{"remote":"origin","number":0}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestMergeEligibilityToolReturnsStructuredReadOnlyResult(t *testing.T) {
	head := strings.Repeat("c", 40)
	reader := &fakeGitHubPullRequestMergeEligibilityReader{eligibility: &gitrepo.GitHubPullRequestMergeEligibilityResult{
		Remote: "origin", Repository: "repo", PullRequest: 9, Head: head, BaseBranch: "main",
		PolicyEvidenceComplete: true, EligibilityComplete: true, Eligible: true, DirectMergeSupported: false,
	}}
	tool := &githubPullRequestMergeEligibilityTool{service: reader}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":9}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if strings.Contains(result.Content, "direct_merge_supported\":true") {
		t.Fatalf("M3A result unexpectedly enabled direct merge: %s", result.Content)
	}
}
