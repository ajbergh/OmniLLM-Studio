package urlcontext

import (
	"path/filepath"
	"strings"
)

// FileCategory groups files for display in the prompt pack.
type FileCategory string

const (
	CategoryCode     FileCategory = "code"
	CategoryDoc      FileCategory = "doc"
	CategoryManifest FileCategory = "manifest"
)

// SelectedFile is a tree entry chosen for inclusion.
type SelectedFile struct {
	Path     string
	Category FileCategory
	Reason   string
	Priority int // lower = higher priority
}

// binaryExtensions are file types that are never fetched.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".ico": true, ".bmp": true, ".tiff": true,
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".wasm": true, ".bin": true, ".dat": true,
	".mp3": true, ".mp4": true, ".wav": true, ".ogg": true, ".flac": true,
	".ttf": true, ".woff": true, ".woff2": true, ".eot": true,
	".class": true, ".jar": true, ".pyc": true,
}

// lockFiles are excluded because they are auto-generated and rarely useful for analysis.
var lockFileNames = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"go.sum":            true, // fetched separately as manifest
	"composer.lock":     true,
	"Gemfile.lock":      true,
	"poetry.lock":       true,
	"Pipfile.lock":      true,
}

// SelectFiles picks the most relevant files from the tree for a given goal.
func SelectFiles(tree []GitHubTreeEntry, goal AnalysisGoal, cfg *Config) []SelectedFile {
	var candidates []SelectedFile

	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		if isBinaryFile(entry.Path) {
			continue
		}
		if isExcludedPath(entry.Path) {
			continue
		}

		prio, category, reason := scoreFile(entry.Path, goal)
		if prio < 0 {
			continue
		}

		candidates = append(candidates, SelectedFile{
			Path:     entry.Path,
			Category: category,
			Reason:   reason,
			Priority: prio,
		})
	}

	// Sort by priority (lower value = higher importance)
	sortByPriority(candidates)

	// Limit to max files
	maxFiles := cfg.GitHubMaxFiles
	if maxFiles <= 0 {
		maxFiles = 80
	}
	if len(candidates) > maxFiles {
		candidates = candidates[:maxFiles]
	}

	return candidates
}

// scoreFile returns (priority, category, reason) for a file path.
// Returns prio=-1 to exclude the file.
func scoreFile(path string, goal AnalysisGoal) (int, FileCategory, string) {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))

	// Lock files: always skip
	if lockFileNames[base] && base != "go.sum" {
		return -1, "", ""
	}

	// ---- Manifests (always included, highest priority) ----
	switch base {
	case "go.mod", "go.sum":
		return 1, CategoryManifest, "Go module manifest"
	case "package.json":
		if !strings.Contains(lower, "node_modules/") {
			return 2, CategoryManifest, "npm manifest"
		}
		return -1, "", ""
	case "cargo.toml":
		return 3, CategoryManifest, "Rust manifest"
	case "requirements.txt", "pyproject.toml", "setup.py":
		return 4, CategoryManifest, "Python manifest"
	case "dockerfile":
		return 5, CategoryManifest, "Docker build definition"
	case "docker-compose.yml", "docker-compose.yaml":
		return 6, CategoryManifest, "Docker Compose definition"
	}

	// ---- README / Docs (high priority for most goals) ----
	if base == "readme.md" || base == "readme.rst" || base == "readme.txt" || base == "readme" {
		return 10, CategoryDoc, "Primary documentation"
	}
	if strings.HasPrefix(lower, "docs/") || strings.HasPrefix(lower, "doc/") {
		return 20, CategoryDoc, "Documentation"
	}
	if strings.HasSuffix(lower, ".md") && !strings.Contains(lower, "node_modules") {
		return 25, CategoryDoc, "Markdown documentation"
	}

	// ---- Goal-specific priority ----
	switch goal {
	case GoalSecurityReview:
		return scoreForSecurity(lower, base)
	case GoalArchitectureReview:
		return scoreForArchitecture(lower, base)
	case GoalCodeReview:
		return scoreForCodeReview(lower, base)
	}

	// Default: feature gap review and general review
	return scoreForFeatureGap(lower, base)
}

func scoreForFeatureGap(lower, base string) (int, FileCategory, string) {
	// API handlers — show what endpoints/features exist
	if matchesAny(lower, "internal/api/", "/api/", "handlers/", "_handler.go") {
		return 30, CategoryCode, "API handler"
	}
	// Frontend components
	if matchesAny(lower, "frontend/src/components/", "src/components/", "pages/") {
		return 35, CategoryCode, "Frontend component"
	}
	// Tool registry
	if matchesAny(lower, "tools/", "tool_") {
		return 40, CategoryCode, "Tool/plugin"
	}
	// Config and settings
	if matchesAny(lower, "config/", "settings/", "config.go", "settings.go") {
		return 45, CategoryCode, "Configuration"
	}
	// Provider / LLM adapters
	if matchesAny(lower, "llm/", "provider/", "providers/") {
		return 50, CategoryCode, "LLM provider"
	}
	// Stores / state (frontend)
	if matchesAny(lower, "stores/", "store/", "zustand", "redux") {
		return 55, CategoryCode, "Frontend state"
	}
	// Scripts / deployment
	if matchesAny(lower, "scripts/", "deploy/", ".github/workflows/") {
		return 60, CategoryCode, "Deployment / scripts"
	}
	// Other source files
	if strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") {
		return 70, CategoryCode, "Source file"
	}
	if strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".jsx") {
		return 75, CategoryCode, "JavaScript source"
	}
	if strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".rs") || strings.HasSuffix(lower, ".java") {
		return 80, CategoryCode, "Source file"
	}
	// YAML/JSON config
	if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".json") {
		if !strings.Contains(lower, "node_modules") {
			return 85, CategoryManifest, "Config file"
		}
	}
	return -1, "", ""
}

func scoreForSecurity(lower, base string) (int, FileCategory, string) {
	if matchesAny(lower, "auth/", "crypto/", "secrets", "middleware/") {
		return 30, CategoryCode, "Security-sensitive: auth/crypto"
	}
	if matchesAny(lower, "plugins/", "plugin/") {
		return 35, CategoryCode, "Security-sensitive: plugin runtime"
	}
	if matchesAny(lower, "repository/", "repo/") {
		return 40, CategoryCode, "Database access layer"
	}
	if matchesAny(lower, "tools/", "tool_") {
		return 45, CategoryCode, "Tool execution"
	}
	if matchesAny(lower, "websearch/", "fetcher", "fetch") {
		return 50, CategoryCode, "Web fetcher"
	}
	if matchesAny(lower, "config/", "settings.go", "env") {
		return 55, CategoryCode, "Configuration / secrets"
	}
	if matchesAny(lower, "api/", "_handler.go") {
		return 60, CategoryCode, "API handler (security surface)"
	}
	if matchesAny(lower, ".github/", "workflows/", "dockerfile", "docker-compose") {
		return 65, CategoryManifest, "Deployment configuration"
	}
	if strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") {
		return 75, CategoryCode, "Source file"
	}
	return -1, "", ""
}

func scoreForArchitecture(lower, base string) (int, FileCategory, string) {
	if base == "router.go" || matchesAny(lower, "internal/api/router") {
		return 20, CategoryCode, "Composition root / router"
	}
	if matchesAny(lower, "_handler.go", "handlers/") {
		return 25, CategoryCode, "API handler"
	}
	if matchesAny(lower, "models/", "model.go", "models.go") {
		return 30, CategoryCode, "Domain models"
	}
	if matchesAny(lower, "repository/", "repo/") {
		return 35, CategoryCode, "Repository / data layer"
	}
	if matchesAny(lower, "service.go", "services/") {
		return 40, CategoryCode, "Service layer"
	}
	if matchesAny(lower, "stores/", "store/index") {
		return 45, CategoryCode, "Frontend state store"
	}
	if base == "api.ts" || matchesAny(lower, "frontend/src/api") {
		return 50, CategoryCode, "Frontend API client"
	}
	if matchesAny(lower, "config/", "config.go") {
		return 55, CategoryCode, "Configuration"
	}
	if matchesAny(lower, "scripts/", "deploy/", ".github/workflows/", "dockerfile") {
		return 60, CategoryManifest, "Deployment"
	}
	if strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") {
		return 70, CategoryCode, "Source file"
	}
	return -1, "", ""
}

func scoreForCodeReview(lower, base string) (int, FileCategory, string) {
	if strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, ".test.ts") || strings.HasSuffix(lower, ".spec.ts") {
		return 20, CategoryCode, "Test file"
	}
	if strings.HasSuffix(lower, ".go") {
		return 30, CategoryCode, "Go source"
	}
	if strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") {
		return 35, CategoryCode, "TypeScript source"
	}
	return scoreForFeatureGap(lower, base)
}

func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExtensions[ext]
}

func isExcludedPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "node_modules/") ||
		strings.Contains(lower, "vendor/") ||
		strings.Contains(lower, "dist/") ||
		strings.Contains(lower, "build/") ||
		strings.Contains(lower, "coverage/") ||
		strings.HasPrefix(lower, ".git/")
}

func matchesAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// sortByPriority sorts in place, lower priority value first (insertion sort for stability on small slices).
func sortByPriority(files []SelectedFile) {
	n := len(files)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && files[j].Priority < files[j-1].Priority; j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
}
