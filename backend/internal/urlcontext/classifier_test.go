package urlcontext

import "testing"

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		url      string
		wantKind URLKind
	}{
		{"https://github.com/ajbergh/OmniLLM-Studio", URLKindGitHubRepo},
		{"https://github.com/owner/repo/blob/main/README.md", URLKindGitHubFile},
		{"https://github.com/owner/repo/tree/main/docs", URLKindGitHubDirectory},
		{"https://raw.githubusercontent.com/owner/repo/main/file.go", URLKindGitHubRaw},
		{"https://api.github.com/repos/owner/repo/contents/path", URLKindGitHubFile},
		{"https://example.com/page", URLKindWebPage},
		{"https://example.com/doc.pdf", URLKindPDF},
		{"https://docs.example.com/guide", URLKindWebPage},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := ClassifyURL(tt.url)
			if got != tt.wantKind {
				t.Errorf("ClassifyURL(%q) = %q, want %q", tt.url, got, tt.wantKind)
			}
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
		wantRef   string
		wantPath  string
		wantKind  URLKind
	}{
		{
			url: "https://github.com/ajbergh/OmniLLM-Studio",
			wantOwner: "ajbergh", wantRepo: "OmniLLM-Studio",
			wantKind: URLKindGitHubRepo,
		},
		{
			url: "https://github.com/owner/repo/blob/main/README.md",
			wantOwner: "owner", wantRepo: "repo",
			wantRef: "main", wantPath: "README.md",
			wantKind: URLKindGitHubFile,
		},
		{
			url: "https://github.com/owner/repo/tree/main/docs",
			wantOwner: "owner", wantRepo: "repo",
			wantRef: "main", wantPath: "docs",
			wantKind: URLKindGitHubDirectory,
		},
		{
			url: "https://raw.githubusercontent.com/owner/repo/main/file.go",
			wantOwner: "owner", wantRepo: "repo",
			wantRef: "main", wantPath: "file.go",
			wantKind: URLKindGitHubRaw,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := ParseGitHubURL(tt.url)
			if got == nil {
				t.Fatalf("ParseGitHubURL(%q) returned nil", tt.url)
			}
			if got.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.wantOwner)
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.wantRepo)
			}
			if tt.wantRef != "" && got.Ref != tt.wantRef {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.wantRef)
			}
			if tt.wantPath != "" && got.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tt.wantPath)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.wantKind)
			}
		})
	}
}
