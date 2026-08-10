package gitrepo

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// RepositoriesEnv maps stable repository IDs to local worktrees. Entries are
	// separated by semicolons, for example: omni=C:\\src\\OmniLLM-Studio;twynn=C:\\src\\Twynn.
	RepositoriesEnv = "OMNILLM_GIT_REPOSITORIES"
)

var repositoryIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Service exposes read-only Git operations against an immutable allowlist of
// repository IDs. Callers never provide filesystem paths directly.
type Service struct {
	repositories map[string]string
	ids          []string
}

// NewService builds a repository service from ID-to-path configuration. Invalid
// IDs and empty paths are ignored; filesystem paths are canonicalized once and
// never returned to callers.
func NewService(configured map[string]string) *Service {
	repositories := make(map[string]string, len(configured))
	ids := make([]string, 0, len(configured))
	for id, rawPath := range configured {
		id = strings.TrimSpace(id)
		rawPath = strings.TrimSpace(rawPath)
		if !ValidRepositoryID(id) || rawPath == "" {
			continue
		}
		absolute, err := filepath.Abs(rawPath)
		if err != nil {
			continue
		}
		absolute = filepath.Clean(absolute)
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
			absolute = filepath.Clean(resolved)
		}
		repositories[id] = absolute
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return &Service{repositories: repositories, ids: ids}
}

// NewServiceFromEnvironment parses OMNILLM_GIT_REPOSITORIES and constructs a service.
func NewServiceFromEnvironment() *Service {
	return NewService(ParseRepositoryConfig(os.Getenv(RepositoriesEnv)))
}

// ParseRepositoryConfig parses semicolon-separated ID=path entries. Malformed
// entries are ignored so a single bad configuration cannot expose arbitrary paths.
func ParseRepositoryConfig(raw string) map[string]string {
	result := map[string]string{}
	for _, entry := range strings.Split(raw, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		id := strings.TrimSpace(parts[0])
		repoPath := strings.TrimSpace(parts[1])
		if !ValidRepositoryID(id) || repoPath == "" {
			continue
		}
		result[id] = repoPath
	}
	return result
}

// ValidRepositoryID reports whether an ID is safe for tool-facing use.
func ValidRepositoryID(id string) bool {
	return id != "" && len(id) <= maxRepositoryIDBytes && repositoryIDPattern.MatchString(id)
}

// Configured reports whether at least one repository ID is registered.
func (s *Service) Configured() bool {
	return s != nil && len(s.ids) > 0
}
