package urlcontext

import (
	"net/url"
	"strings"
)

// ClassifyURL determines what kind of resource a URL points to.
func ClassifyURL(rawURL string) URLKind {
	u, err := url.Parse(rawURL)
	if err != nil {
		return URLKindUnknown
	}

	host := strings.ToLower(u.Hostname())
	path := u.Path

	// Raw GitHub content
	if host == "raw.githubusercontent.com" {
		return URLKindGitHubRaw
	}

	// GitHub API
	if host == "api.github.com" {
		if strings.HasPrefix(path, "/repos/") {
			return URLKindGitHubFile
		}
		return URLKindUnknown
	}

	// GitHub main site
	if host == "github.com" {
		return classifyGitHubPath(path)
	}

	// PDF by extension
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".pdf") {
		return URLKindPDF
	}

	return URLKindWebPage
}

// classifyGitHubPath inspects the URL path under github.com.
func classifyGitHubPath(path string) URLKind {
	// Trim leading slash and split
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 5)

	switch len(parts) {
	case 0, 1:
		// github.com/ or github.com/owner — not a specific repo
		return URLKindUnknown
	case 2:
		// github.com/owner/repo
		return URLKindGitHubRepo
	default:
		// github.com/owner/repo/<something>/...
		switch parts[2] {
		case "blob":
			return URLKindGitHubFile
		case "tree":
			return URLKindGitHubDirectory
		case "raw":
			return URLKindGitHubRaw
		default:
			// github.com/owner/repo/issues etc. — treat as repo
			return URLKindGitHubRepo
		}
	}
}

// ParseGitHubURL extracts structured components from a GitHub URL.
// Returns nil if the URL is not a recognizable GitHub URL.
func ParseGitHubURL(rawURL string) *GitHubParseResult {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	host := strings.ToLower(u.Hostname())
	path := strings.TrimPrefix(u.Path, "/")

	switch host {
	case "raw.githubusercontent.com":
		// raw.githubusercontent.com/{owner}/{repo}/{ref}/{path}
		parts := strings.SplitN(path, "/", 4)
		if len(parts) < 3 {
			return nil
		}
		r := &GitHubParseResult{
			Owner: parts[0],
			Repo:  parts[1],
			Kind:  URLKindGitHubRaw,
		}
		if len(parts) >= 3 {
			r.Ref = parts[2]
		}
		if len(parts) == 4 {
			r.Path = parts[3]
		}
		return r

	case "github.com":
		parts := strings.SplitN(path, "/", 5)
		if len(parts) < 2 {
			return nil
		}
		r := &GitHubParseResult{
			Owner: parts[0],
			Repo:  parts[1],
		}
		if len(parts) == 2 {
			r.Kind = URLKindGitHubRepo
			return r
		}
		switch parts[2] {
		case "blob":
			r.Kind = URLKindGitHubFile
			if len(parts) >= 4 {
				r.Ref = parts[3]
			}
			if len(parts) == 5 {
				r.Path = parts[4]
			}
		case "tree":
			r.Kind = URLKindGitHubDirectory
			if len(parts) >= 4 {
				r.Ref = parts[3]
			}
			if len(parts) == 5 {
				r.Path = parts[4]
			}
		default:
			r.Kind = URLKindGitHubRepo
		}
		return r

	case "api.github.com":
		// api.github.com/repos/{owner}/{repo}/contents/{path}
		parts := strings.SplitN(path, "/", 5)
		if len(parts) < 3 || parts[0] != "repos" {
			return nil
		}
		r := &GitHubParseResult{
			Owner: parts[1],
			Repo:  parts[2],
			Kind:  URLKindGitHubFile,
		}
		if len(parts) == 5 {
			r.Path = parts[4]
		}
		return r
	}

	return nil
}
