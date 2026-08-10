package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestGitRepositoryToolDefinitionsAreReadOnly(t *testing.T) {
	svc := gitrepo.NewService(map[string]string{"repo": t.TempDir()})
	for _, tool := range NewGitRepositoryTools(svc) {
		def := tool.Definition().Normalized()
		if !def.ReadOnly || def.SideEffecting || def.RequiresNetwork || def.Risk != RiskLow {
			t.Fatalf("%s definition is not read-only low-risk: %#v", def.Name, def)
		}
		if def.Category != "git" {
			t.Fatalf("%s category = %q, want git", def.Name, def.Category)
		}
	}
}

func TestGitRepositoryMutationDefinitionsRequireApproval(t *testing.T) {
	svc := gitrepo.NewServiceWithWriteAccess(map[string]string{"repo": t.TempDir()}, true)
	for _, tool := range NewGitRepositoryMutationTools(svc) {
		def := tool.Definition().Normalized()
		if def.ReadOnly || !def.SideEffecting || def.RequiresNetwork || def.SupportsParallel || def.Risk != RiskHigh {
			t.Fatalf("%s definition has unsafe mutation metadata: %#v", def.Name, def)
		}
		if def.Category != "git" {
			t.Fatalf("%s category = %q, want git", def.Name, def.Category)
		}
		if policy := EffectivePolicy(def, ""); policy != "ask" {
			t.Fatalf("%s default policy = %q, want ask", def.Name, policy)
		}
	}
}

func TestGitRepositoryToolValidation(t *testing.T) {
	svc := gitrepo.NewService(map[string]string{"repo": t.TempDir()})
	var diffTool Tool
	var blameTool Tool
	for _, tool := range NewGitRepositoryTools(svc) {
		switch tool.Definition().Name {
		case "git_diff":
			diffTool = tool
		case "git_blame":
			blameTool = tool
		}
	}
	if err := diffTool.Validate(json.RawMessage(`{"repository":"repo","to":"HEAD"}`)); err == nil {
		t.Fatal("git_diff accepted to without from")
	}
	if err := blameTool.Validate(json.RawMessage(`{"repository":"repo","path":""}`)); err == nil {
		t.Fatal("git_blame accepted empty path")
	}
}

func TestGitRepositoryMutationToolValidation(t *testing.T) {
	svc := gitrepo.NewServiceWithWriteAccess(map[string]string{"repo": t.TempDir()}, true)
	mutation := map[string]Tool{}
	for _, tool := range NewGitRepositoryMutationTools(svc) {
		mutation[tool.Definition().Name] = tool
	}
	head := strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	worktreeDigest := strings.Repeat("c", 64)

	if err := mutation["git_create_branch"].Validate(json.RawMessage(`{"repository":"repo","name":"","expected_head":"` + head + `"}`)); err == nil {
		t.Fatal("git_create_branch accepted empty name")
	}
	if err := mutation["git_checkout"].Validate(json.RawMessage(`{"repository":"repo","branch":"feature","expected_head":"not-a-hash"}`)); err == nil {
		t.Fatal("git_checkout accepted invalid expected_head")
	}
	if err := mutation["git_stage"].Validate(json.RawMessage(`{"repository":"repo","paths":[],"expected_branch":"main","expected_head":"` + head + `","expected_index_digest":"` + digest + `","expected_worktree_digest":"` + worktreeDigest + `"}`)); err == nil {
		t.Fatal("git_stage accepted empty paths")
	}
	if err := mutation["git_stage"].Validate(json.RawMessage(`{"repository":"repo","paths":["a.txt"],"expected_branch":"","expected_head":"` + head + `","expected_index_digest":"` + digest + `","expected_worktree_digest":"` + worktreeDigest + `"}`)); err == nil {
		t.Fatal("git_stage accepted empty expected_branch")
	}
	if err := mutation["git_stage"].Validate(json.RawMessage(`{"repository":"repo","paths":["a.txt"],"expected_branch":"main","expected_head":"` + head + `","expected_index_digest":"` + digest + `","expected_worktree_digest":"short"}`)); err == nil {
		t.Fatal("git_stage accepted invalid expected_worktree_digest")
	}
	if err := mutation["git_stage"].Validate(json.RawMessage(`{"repository":"repo","paths":["a.txt"],"expected_branch":"main","expected_head":"` + head + `","expected_index_digest":"` + digest + `","expected_worktree_digest":"` + worktreeDigest + `"}`)); err != nil {
		t.Fatalf("git_stage valid arguments error = %v", err)
	}
	if err := mutation["git_commit"].Validate(json.RawMessage(`{"repository":"repo","message":"commit","expected_branch":"main","expected_head":"` + head + `","expected_index_digest":"short"}`)); err == nil {
		t.Fatal("git_commit accepted invalid expected_index_digest")
	}
	if err := mutation["git_commit"].Validate(json.RawMessage(`{"repository":"repo","message":"commit","expected_branch":"main","expected_head":"` + head + `","expected_index_digest":"` + digest + `"}`)); err != nil {
		t.Fatalf("git_commit valid arguments error = %v", err)
	}
}

func TestGitRepositoriesToolDoesNotReturnConfiguredPath(t *testing.T) {
	secret := t.TempDir()
	svc := gitrepo.NewService(map[string]string{"repo": secret})
	var catalog Tool
	for _, tool := range NewGitRepositoryTools(svc) {
		if tool.Definition().Name == "git_repositories" {
			catalog = tool
			break
		}
	}
	result, err := catalog.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content, secret) {
		t.Fatalf("catalog leaked configured path: %s", result.Content)
	}
}

func TestRegistryGitWriteGate(t *testing.T) {
	t.Setenv(gitrepo.RepositoriesEnv, "")
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	withoutGit := NewRegistry()
	if _, ok := withoutGit.Get("git_status"); ok {
		t.Fatal("git_status registered without configured repositories")
	}
	if _, ok := withoutGit.Get("git_commit"); ok {
		t.Fatal("git_commit registered without configured repositories")
	}

	t.Setenv(gitrepo.RepositoriesEnv, "repo=/path/that/does/not/need/to/exist/for-registration")
	t.Setenv(gitrepo.WriteEnabledEnv, "")
	readOnly := NewRegistry()
	if tool, ok := readOnly.Get("git_status"); !ok || tool == nil {
		t.Fatal("git_status not registered with configured repository ID")
	}
	if _, ok := readOnly.Get("git_commit"); ok {
		t.Fatal("git_commit registered without explicit write gate")
	}

	t.Setenv(gitrepo.WriteEnabledEnv, "not-a-bool")
	invalidGate := NewRegistry()
	if _, ok := invalidGate.Get("git_stage"); ok {
		t.Fatal("git_stage registered for invalid write gate")
	}

	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	writable := NewRegistry()
	for _, name := range []string{"git_create_branch", "git_checkout", "git_stage", "git_commit"} {
		if tool, ok := writable.Get(name); !ok || tool == nil {
			t.Fatalf("%s not registered with explicit write gate", name)
		}
	}
}
