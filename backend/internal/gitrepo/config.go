package gitrepo

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	// RepositoriesEnv maps stable repository IDs to local worktrees. Entries are
	// separated by semicolons, for example: omni=C:\\src\\OmniLLM-Studio;twynn=C:\\src\\Twynn.
	RepositoriesEnv = "OMNILLM_GIT_REPOSITORIES"
	// WriteEnabledEnv is an operator-controlled second gate for local Git
	// mutations. Read-only tools remain available without it.
	WriteEnabledEnv = "OMNILLM_GIT_WRITE_ENABLED"
)

var repositoryIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Service exposes controlled Git operations against an immutable allowlist of
// repository IDs. Callers never provide repository filesystem roots directly.
// Mutations are serialized and remain disabled unless explicitly enabled.
type Service struct {
	repositories map[string]string
	ids          []string
	writeEnabled bool
	writeMu      sync.Mutex
}

// NewService builds a read-only repository service from ID-to-path
// configuration. Invalid IDs and empty paths are ignored; filesystem paths are
// canonicalized once and never returned to callers.
func NewService(configured map[string]string) *Service {
	return newService(configured, false)
}

// NewServiceWithWriteAccess builds a service with an explicit mutation gate.
// This is primarily useful for composition and tests; normal application startup
// should use NewServiceFromEnvironment.
func NewServiceWithWriteAccess(configured map[string]string, writeEnabled bool) *Service {
	return newService(configured, writeEnabled)
}

func newService(configured map[string]string, writeEnabled bool) *Service {
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
	return &Service{repositories: repositories, ids: ids, writeEnabled: writeEnabled}
}

// NewServiceFromEnvironment parses local Git configuration and constructs a
// service. Write access is enabled only when OMNILLM_GIT_WRITE_ENABLED parses as
// a true boolean value.
func NewServiceFromEnvironment() *Service {
	writeEnabled, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(WriteEnabledEnv)))
	return NewServiceWithWriteAccess(ParseRepositoryConfig(os.Getenv(RepositoriesEnv)), writeEnabled)
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

// WriteEnabled reports whether this service instance permits mutations.
func (s *Service) WriteEnabled() bool {
	return s != nil && s.writeEnabled
}

func (s *Service) requireWriteEnabled() error {
	if !s.WriteEnabled() {
		return errWriteDisabled
	}
	return nil
}
