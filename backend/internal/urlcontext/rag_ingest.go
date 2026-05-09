package urlcontext

import (
	"fmt"
	"strings"
)

// SourceToRAGText converts a ResolvedSource to flat text suitable for chunking
// and indexing into the RAG pipeline. For GitHub repos, each fetched file is
// emitted as a labelled section. For web pages, the full markdown is returned.
// The result is keyed by src.ID (a "urlsrc_…" UUID) when stored as chunks.
func SourceToRAGText(src ResolvedSource) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# URL Source: %s\n", src.Title)
	fmt.Fprintf(&sb, "URL: %s\n", src.URL)
	fmt.Fprintf(&sb, "Type: %s\n\n", src.Kind)

	switch src.Kind {
	case URLKindGitHubRepo:
		if src.Repo != nil {
			writeRAGRepoContent(&sb, src.Repo)
		}
	case URLKindGitHubFile, URLKindGitHubRaw:
		if src.ContentText != "" {
			if src.Path != "" {
				fmt.Fprintf(&sb, "## File: %s\n\n", src.Path)
			}
			sb.WriteString(src.ContentText)
			sb.WriteString("\n\n")
		}
	default:
		if src.ContentMarkdown != "" {
			sb.WriteString(src.ContentMarkdown)
		} else if src.ContentText != "" {
			sb.WriteString(src.ContentText)
		}
	}

	return sb.String()
}

func writeRAGRepoContent(sb *strings.Builder, repo *GitHubRepoContext) {
	fmt.Fprintf(sb, "## Repository: %s/%s\n", repo.Owner, repo.Repo)
	if repo.Description != "" {
		fmt.Fprintf(sb, "Description: %s\n", repo.Description)
	}
	fmt.Fprintf(sb, "Default Branch: %s\n\n", repo.DefaultBranch)

	if repo.README != "" {
		sb.WriteString("## README\n\n")
		sb.WriteString(repo.README)
		sb.WriteString("\n\n")
	}

	for _, f := range repo.Manifests {
		fmt.Fprintf(sb, "## File: %s\n\n", f.Path)
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}

	for _, f := range repo.Docs {
		fmt.Fprintf(sb, "## File: %s\n\n", f.Path)
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}

	for _, f := range repo.SelectedFiles {
		fmt.Fprintf(sb, "## File: %s\n\n", f.Path)
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}
}
