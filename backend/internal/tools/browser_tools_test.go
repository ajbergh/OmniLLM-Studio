package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBrowserNavigateRejectsGitHubURLs(t *testing.T) {
	t.Parallel()

	tool := NewBrowserNavigateTool(nil, nil)
	for _, rawURL := range []string{
		"https://github.com/owner/repo",
		"https://api.github.com/repos/owner/repo",
		"https://raw.githubusercontent.com/owner/repo/main/README.md",
	} {
		args, err := json.Marshal(map[string]string{"url": rawURL})
		if err != nil {
			t.Fatal(err)
		}
		err = tool.Validate(args)
		if err == nil || !strings.Contains(err.Error(), "github_repo_inspect") {
			t.Fatalf("Validate(%q) error = %v, want GitHub inspection guidance", rawURL, err)
		}
	}
}

func TestBrowserNavigateAllowsNonGitHubURL(t *testing.T) {
	t.Parallel()

	tool := NewBrowserNavigateTool(nil, nil)
	if err := tool.Validate(json.RawMessage(`{"url":"https://example.com/article"}`)); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
