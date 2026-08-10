package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeRemoteCloner struct {
	result *gitrepo.RemoteCloneResult
}

func (f *fakeRemoteCloner) Clone(context.Context, string, string, string) (*gitrepo.RemoteCloneResult, error) {
	return f.result, nil
}

func TestGitCloneDefaultsToApproval(t *testing.T) {
	tool := &gitRemoteCloneTool{service: &fakeRemoteCloner{}}
	definition := tool.Definition().Normalized()
	if definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || definition.SupportsParallel || definition.Risk != RiskCritical {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitCloneAcceptsOnlyReviewedRemoteState(t *testing.T) {
	tool := &gitRemoteCloneTool{service: &fakeRemoteCloner{}}
	head := strings.Repeat("a", 40)
	valid := json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"` + head + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"` + head + `","path":"/tmp/repo"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"` + head + `","url":"https://example.com/repo.git"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"` + head + `","max_bytes":999999999}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"short"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"","expected_remote_head":"` + head + `"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitCloneReturnsStructuredResult(t *testing.T) {
	cloner := &fakeRemoteCloner{result: &gitrepo.RemoteCloneResult{
		Remote: "origin", Repository: "repo", Branch: "main", Head: strings.Repeat("a", 40),
		BytesReceived: 1024, StorageBytesUsed: 2048, EntriesCreated: 20,
	}}
	tool := &gitRemoteCloneTool{service: cloner}
	args := json.RawMessage(`{"remote":"origin","expected_branch":"main","expected_remote_head":"` + strings.Repeat("a", 40) + `"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if len(result.Structured) == 0 {
		t.Fatal("expected structured clone result")
	}
}

func TestRegistryGitCloneRequiresExplicitBudgetsAndGate(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "clone-target")
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://example.com/repo.git","allow_clone":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteCloneEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteCloneMaxBytesEnv, "")
	t.Setenv(gitrepo.RemoteCloneMaxEntriesEnv, "")

	withoutBudgets := NewRegistry()
	if _, ok := withoutBudgets.Get("git_clone"); ok {
		t.Fatal("git_clone registered without explicit storage budgets")
	}

	t.Setenv(gitrepo.RemoteCloneMaxBytesEnv, "268435456")
	t.Setenv(gitrepo.RemoteCloneMaxEntriesEnv, "25000")
	withBudgets := NewRegistry()
	if _, ok := withBudgets.Get("git_clone"); !ok {
		t.Fatal("git_clone not registered when all global clone prerequisites are enabled")
	}

	t.Setenv(gitrepo.RemoteCloneEnabledEnv, "false")
	withoutGate := NewRegistry()
	if _, ok := withoutGate.Get("git_clone"); ok {
		t.Fatal("git_clone registered while clone gate is disabled")
	}
}
