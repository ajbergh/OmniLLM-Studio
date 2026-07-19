package urlcontext

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const githubAPIBase = "https://api.github.com"

// ghRepo is the GitHub API response for GET /repos/{owner}/{repo}.
type ghRepo struct {
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ghContent is the GitHub Contents API response.
type ghContent struct {
	Type        string `json:"type"` // "file" or "dir"
	Encoding    string `json:"encoding"`
	Content     string `json:"content"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

// ghCommit holds the latest commit SHA for a ref.
type ghCommit struct {
	SHA string `json:"sha"`
}

// GitHubInspector fetches structured repository context from the GitHub API.
type GitHubInspector struct {
	fetcher *Fetcher
	token   string
	cfg     *Config
}

// NewGitHubInspector creates an inspector using the service config.
func NewGitHubInspector(fetcher *Fetcher, cfg *Config) *GitHubInspector {
	return &GitHubInspector{
		fetcher: fetcher,
		token:   cfg.GitHubToken,
		cfg:     cfg,
	}
}

// InspectRepo fetches structured context for a GitHub repository.
func (g *GitHubInspector) InspectRepo(
	ctx context.Context,
	parsed *GitHubParseResult,
	goal AnalysisGoal,
	streamStatus func(event string, payload any),
) (*GitHubRepoContext, error) {
	owner, repo := parsed.Owner, parsed.Repo

	notify := func(status, detail string) {
		if streamStatus != nil {
			streamStatus("url_context", map[string]any{
				"status": status,
				"url":    fmt.Sprintf("https://github.com/%s/%s", owner, repo),
				"kind":   string(URLKindGitHubRepo),
				"detail": detail,
			})
		}
	}

	notify("fetching", "repository metadata")

	// 1. Repo metadata
	meta, err := g.getRepoMeta(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	branch := meta.DefaultBranch
	if parsed.Ref != "" {
		branch = parsed.Ref
	}
	if branch == "" {
		branch = "main"
	}

	repoCtx := &GitHubRepoContext{
		Owner:         owner,
		Repo:          repo,
		DefaultBranch: branch,
		Description:   meta.Description,
	}

	// 2. Latest commit SHA
	commitSHA, _ := g.getCommitSHA(ctx, owner, repo, branch)
	repoCtx.CommitSHA = commitSHA

	// 3. README
	notify("fetching", "README")
	readme, err := g.getREADME(ctx, owner, repo, branch)
	if err == nil {
		limit := 8000
		if len(readme) > limit {
			readme = readme[:limit] + "\n[...truncated]"
		}
		repoCtx.README = readme
	}

	// 4. File tree
	notify("fetching", "file tree")
	tree, treeTruncated, err := g.getTree(ctx, owner, repo, branch)
	if err != nil {
		repoCtx.Warnings = append(repoCtx.Warnings, fmt.Sprintf("Could not fetch file tree: %v", err))
	} else {
		if treeTruncated {
			repoCtx.Warnings = append(repoCtx.Warnings, "Repository tree was truncated by GitHub API (very large repo).")
		}
		repoCtx.FileTree = tree
	}

	// 5. Select and fetch files
	notify("fetching", "selected files")
	selected := SelectFiles(tree, goal, g.cfg)
	fetched, manifests, docs, warnings := g.fetchSelectedFiles(ctx, owner, repo, branch, selected)
	repoCtx.SelectedFiles = fetched
	repoCtx.Manifests = manifests
	repoCtx.Docs = docs
	repoCtx.Warnings = append(repoCtx.Warnings, warnings...)

	// 6. Architecture signals
	repoCtx.ArchitectureSignals = detectArchitectureSignals(tree, fetched, manifests)

	return repoCtx, nil
}

func (g *GitHubInspector) getRepoMeta(ctx context.Context, owner, repo string) (*ghRepo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repo)
	result, err := g.fetcher.FetchWithAuth(ctx, url, g.token)
	if err != nil {
		return nil, err
	}
	var meta ghRepo
	if err := json.Unmarshal(result.Body, &meta); err != nil {
		return nil, fmt.Errorf("parse repo metadata: %w", err)
	}
	return &meta, nil
}

func (g *GitHubInspector) getCommitSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", githubAPIBase, owner, repo, branch)
	result, err := g.fetcher.FetchWithAuth(ctx, url, g.token)
	if err != nil {
		return "", err
	}
	var commit ghCommit
	if err := json.Unmarshal(result.Body, &commit); err != nil {
		return "", nil
	}
	return commit.SHA, nil
}

func (g *GitHubInspector) getREADME(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/readme?ref=%s", githubAPIBase, owner, repo, branch)
	result, err := g.fetcher.FetchWithAuth(ctx, url, g.token)
	if err != nil {
		return "", err
	}
	var content ghContent
	if err := json.Unmarshal(result.Body, &content); err != nil {
		return "", fmt.Errorf("parse readme response: %w", err)
	}
	return decodeGitHubContent(content)
}

func (g *GitHubInspector) getTree(ctx context.Context, owner, repo, branch string) ([]GitHubTreeEntry, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", githubAPIBase, owner, repo, branch)
	result, err := g.fetcher.FetchWithAuth(ctx, url, g.token)
	if err != nil {
		return nil, false, err
	}

	var resp struct {
		Tree      []ghTreeNode `json:"tree"`
		Truncated bool         `json:"truncated"`
	}
	if err := json.Unmarshal(result.Body, &resp); err != nil {
		return nil, false, fmt.Errorf("parse tree: %w", err)
	}

	limit := g.cfg.GitHubMaxTreeEntries
	if limit <= 0 {
		limit = 100_000
	}

	entries := make([]GitHubTreeEntry, 0, len(resp.Tree))
	for i, node := range resp.Tree {
		if i >= limit {
			break
		}
		entries = append(entries, GitHubTreeEntry{
			Path: node.Path,
			Type: node.Type,
			Size: node.Size,
			SHA:  node.SHA,
		})
	}

	return entries, resp.Truncated, nil
}

type ghTreeNode struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
}

func (g *GitHubInspector) fetchSelectedFiles(
	ctx context.Context,
	owner, repo, branch string,
	selected []SelectedFile,
) (files, manifests, docs []GitHubFileContext, warnings []string) {
	maxFiles := g.cfg.GitHubMaxFiles
	if maxFiles <= 0 {
		maxFiles = 80
	}
	maxBytes := g.cfg.GitHubMaxBytesPerFile
	if maxBytes <= 0 {
		maxBytes = 120_000
	}

	fetched := 0
	skipped := 0

	for _, sel := range selected {
		if fetched >= maxFiles {
			skipped++
			continue
		}

		// Create a child context with short deadline per file
		fileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		content, err := g.fetchFileContent(fileCtx, owner, repo, branch, sel.Path, maxBytes)
		cancel()

		if err != nil {
			skipped++
			warnings = append(warnings, fmt.Sprintf("Skipped %s: %v", sel.Path, err))
			continue
		}

		fc := GitHubFileContext{
			Path:     sel.Path,
			Language: languageFromPath(sel.Path),
			Size:     int64(len(content)),
			Content:  content,
			Reason:   sel.Reason,
		}

		fetched++

		switch sel.Category {
		case CategoryManifest:
			manifests = append(manifests, fc)
		case CategoryDoc:
			docs = append(docs, fc)
		default:
			files = append(files, fc)
		}
	}

	if skipped > 0 {
		warnings = append(warnings, fmt.Sprintf("Skipped %d files (max files reached or fetch error).", skipped))
	}

	return
}

func (g *GitHubInspector) fetchFileContent(ctx context.Context, owner, repo, branch, path string, maxBytes int64) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", githubAPIBase, owner, repo, path, branch)
	result, err := g.fetcher.FetchWithAuth(ctx, url, g.token)
	if err != nil {
		return "", err
	}

	var content ghContent
	if err := json.Unmarshal(result.Body, &content); err != nil {
		return "", fmt.Errorf("parse content response: %w", err)
	}

	if content.Type != "file" {
		return "", fmt.Errorf("not a file: %s", content.Type)
	}

	text, err := decodeGitHubContent(content)
	if err != nil {
		return "", err
	}

	if IsBinaryContent([]byte(text)) {
		return "", fmt.Errorf("binary file")
	}

	if int64(len(text)) > maxBytes {
		text = text[:maxBytes] + "\n[...file truncated at configured size limit]"
	}

	return text, nil
}

func decodeGitHubContent(c ghContent) (string, error) {
	switch c.Encoding {
	case "base64":
		// GitHub returns base64 with newlines; strip them before decoding
		cleaned := strings.ReplaceAll(c.Content, "\n", "")
		b, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", fmt.Errorf("base64 decode: %w", err)
		}
		return string(b), nil
	case "":
		return c.Content, nil
	default:
		return "", fmt.Errorf("unsupported encoding: %s", c.Encoding)
	}
}

// detectArchitectureSignals infers technology signals from the tree and files.
func detectArchitectureSignals(tree []GitHubTreeEntry, files, manifests []GitHubFileContext) []string {
	pathSet := make(map[string]bool, len(tree))
	for _, e := range tree {
		pathSet[e.Path] = true
	}

	hasPath := func(substr string) bool {
		for p := range pathSet {
			if strings.Contains(p, substr) {
				return true
			}
		}
		return false
	}

	var signals []string

	if hasPath("go.mod") || hasPath(".go") {
		signals = append(signals, "Go backend detected")
	}
	if hasPath("package.json") {
		signals = append(signals, "Node.js / npm project detected")
	}
	if hasPath("react") || hasPath(".tsx") || hasPath(".jsx") {
		signals = append(signals, "React frontend detected")
	}
	if hasPath("chi") || hasPath("router.go") {
		signals = append(signals, "Chi router detected")
	}
	if hasPath(".db") || hasPath("sqlite") || hasPath("db.go") {
		signals = append(signals, "SQLite persistence detected")
	}
	if hasPath("rag/") || hasPath("chromem") {
		signals = append(signals, "RAG / vector store detected")
	}
	if hasPath("tools/") {
		signals = append(signals, "Tool registry detected")
	}
	if hasPath("sports/") {
		signals = append(signals, "Sports direct preflight detected")
	}
	if hasPath("news/") {
		signals = append(signals, "News direct preflight detected")
	}
	if hasPath("websearch/") {
		signals = append(signals, "Web search package detected")
	}
	if hasPath("artifacts/") || hasPath("wordgen/") {
		signals = append(signals, "Artifact generation detected")
	}
	if hasPath("wails") || hasPath("desktop") {
		signals = append(signals, "Wails desktop build detected")
	}
	if hasPath("plugins/") {
		signals = append(signals, "Plugin system detected")
	}
	if hasPath("agent/") {
		signals = append(signals, "Agent mode detected")
	}
	if hasPath("auth/") {
		signals = append(signals, "Authentication system detected")
	}

	return signals
}

// languageFromPath returns a short language label for a file path.
func languageFromPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "Go"
	case strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx"):
		return "TypeScript"
	case strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".jsx"):
		return "JavaScript"
	case strings.HasSuffix(lower, ".py"):
		return "Python"
	case strings.HasSuffix(lower, ".md"):
		return "Markdown"
	case strings.HasSuffix(lower, ".json"):
		return "JSON"
	case strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"):
		return "YAML"
	case strings.HasSuffix(lower, ".toml"):
		return "TOML"
	case strings.HasSuffix(lower, ".sh"):
		return "Shell"
	case strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd"):
		return "Batch"
	case strings.HasSuffix(lower, ".sql"):
		return "SQL"
	case strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm"):
		return "HTML"
	case strings.HasSuffix(lower, ".css"):
		return "CSS"
	case strings.HasSuffix(lower, ".rs"):
		return "Rust"
	case strings.HasSuffix(lower, ".java"):
		return "Java"
	case strings.HasSuffix(lower, ".c") || strings.HasSuffix(lower, ".cpp"):
		return "C/C++"
	}
	return ""
}
