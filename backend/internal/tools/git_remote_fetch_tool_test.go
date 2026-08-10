package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeRemoteFetcher struct {
	result *gitrepo.RemoteFetchResult
}

func (f *fakeRemoteFetcher) Fetch(context.Context, string, string, string, string) (*gitrepo.RemoteFetchResult, error) {
	return f.result, nil
}

func TestGitFetchDefaultsToApproval(t *testing.T) {
	tool := &gitRemoteFetchTool{service: &fakeRemoteFetcher{}}
	definition := tool.Definition().Normalized()
	if definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || definition.SupportsParallel || definition.Risk != RiskHigh {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitFetchRequiresReviewedStateAndRejectsURLCredentials(t *testing.T) {
	tool := &gitRemoteFetchTool{service: &fakeRemoteFetcher{}}
	head := strings.Repeat("a", 40)
	remoteHead := strings.Repeat("b", 40)
	valid := json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","url":"https://example.com/repo.git"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","refspec":"refs/heads/main:refs/heads/main"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"short","expected_remote_head":"` + remoteHead + `"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + head + `","expected_remote_head":"short"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitFetchReturnsStructuredResult(t *testing.T) {
	fetcher := &fakeRemoteFetcher{result: &gitrepo.RemoteFetchResult{
		Remote: "origin", Repository: "repo", Branch: "main", LocalHead: strings.Repeat("a", 40),
		RemoteHead: strings.Repeat("b", 40), TrackingRef: "refs/remotes/omnillm/0123456789abcdef/main",
	}}
	tool := &gitRemoteFetchTool{service: fetcher}
	args := json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_head":"` + strings.Repeat("a", 40) + `","expected_remote_head":"` + strings.Repeat("b", 40) + `"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if len(result.Structured) == 0 {
		t.Fatal("expected structured result")
	}
}
