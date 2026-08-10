package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeRemoteReader struct {
	remotes []gitrepo.RemoteSummary
	status  *gitrepo.RemoteStatusResult
}

func (f *fakeRemoteReader) Remotes(context.Context) []gitrepo.RemoteSummary { return f.remotes }

func (f *fakeRemoteReader) RemoteStatus(context.Context, string) (*gitrepo.RemoteStatusResult, error) {
	return f.status, nil
}

func TestGitRemoteStatusDefaultsToApproval(t *testing.T) {
	tool := &gitRemoteTool{service: &fakeRemoteReader{}, name: "git_remote_status"}
	definition := tool.Definition().Normalized()
	if !definition.ReadOnly || !definition.RequiresNetwork || definition.Risk != RiskHigh {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitRemoteStatusRejectsURLAndCredentialArguments(t *testing.T) {
	tool := &gitRemoteTool{service: &fakeRemoteReader{}, name: "git_remote_status"}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","url":"https://example.com/repo.git"}`),
		json.RawMessage(`{"remote":"origin","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","token_env":"SECRET_ENV"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
	if err := tool.Validate(json.RawMessage(`{"remote":"origin"}`)); err != nil {
		t.Fatalf("valid remote ID rejected: %v", err)
	}
}

func TestGitRemotesReturnsSafeSummary(t *testing.T) {
	reader := &fakeRemoteReader{remotes: []gitrepo.RemoteSummary{{ID: "origin", Repository: "omni", Host: "github.com", AuthenticationConfigured: true}}}
	tool := &gitRemoteTool{service: reader, name: "git_remotes"}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if string(result.Structured) == "" {
		t.Fatal("expected structured result")
	}
}
