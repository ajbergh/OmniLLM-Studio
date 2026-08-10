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

func TestRegistryAddsGitToolsOnlyWhenRepositoriesConfigured(t *testing.T) {
	t.Setenv(gitrepo.RepositoriesEnv, "")
	withoutGit := NewRegistry()
	if _, ok := withoutGit.Get("git_status"); ok {
		t.Fatal("git_status registered without configured repositories")
	}

	t.Setenv(gitrepo.RepositoriesEnv, "repo=/path/that/does/not/need/to/exist/for-registration")
	withGit := NewRegistry()
	if tool, ok := withGit.Get("git_status"); !ok || tool == nil {
		t.Fatal("git_status not registered with configured repository ID")
	}
}
