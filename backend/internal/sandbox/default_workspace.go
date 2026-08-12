package sandbox

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ajbergh/omnillm-studio/internal/db"
)

var defaultWorkspaceState struct {
	mu       sync.Mutex
	registry *WorkspaceRegistry
	databaseCloser func() error
	err      error
}

// SetDefaultWorkspaceRegistry installs an explicitly composed registry. It is
// primarily used by application composition/tests; nil clears the cached value.
func SetDefaultWorkspaceRegistry(registry *WorkspaceRegistry) {
	defaultWorkspaceState.mu.Lock()
	defer defaultWorkspaceState.mu.Unlock()
	defaultWorkspaceState.registry = registry
	defaultWorkspaceState.err = nil
}

// DefaultWorkspaceRegistry lazily opens the same application SQLite path when
// explicit router injection is not yet available. This is a transitional
// composition seam for the sandbox program, not a second persistence model.
func DefaultWorkspaceRegistry() (*WorkspaceRegistry, error) {
	defaultWorkspaceState.mu.Lock()
	defer defaultWorkspaceState.mu.Unlock()
	if defaultWorkspaceState.registry != nil || defaultWorkspaceState.err != nil {
		return defaultWorkspaceState.registry, defaultWorkspaceState.err
	}
	path := strings.TrimSpace(os.Getenv("OMNILLM_DB_PATH"))
	if path == "" {
		path = "omnillm-studio.db"
	}
	database, err := db.Open(path)
	if err != nil {
		defaultWorkspaceState.err = fmt.Errorf("open workspace registry database: %w", err)
		return nil, defaultWorkspaceState.err
	}
	registry, err := NewWorkspaceRegistry(database)
	if err != nil {
		_ = db.Close(database)
		defaultWorkspaceState.err = err
		return nil, err
	}
	defaultWorkspaceState.databaseCloser = func() error { return db.Close(database) }
	defaultWorkspaceState.registry = registry
	return registry, nil
}

// seedConfiguredWorkspaces imports trusted process configuration for the current
// owner. Syntax: semicolon-separated `owner:id=mode,path`. The model never sees
// or supplies the path; entries for other owners are ignored.
func seedConfiguredWorkspaces(ownerUserID string, registry *WorkspaceRegistry) error {
	raw := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_WORKSPACES"))
	if raw == "" || registry == nil {
		return nil
	}
	for _, entry := range strings.Split(raw, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		ownerAndRest := strings.SplitN(entry, ":", 2)
		if len(ownerAndRest) != 2 || strings.TrimSpace(ownerAndRest[0]) != ownerUserID {
			continue
		}
		idAndSpec := strings.SplitN(ownerAndRest[1], "=", 2)
		if len(idAndSpec) != 2 {
			return fmt.Errorf("invalid OMNILLM_SANDBOX_WORKSPACES entry")
		}
		modeAndPath := strings.SplitN(idAndSpec[1], ",", 2)
		if len(modeAndPath) != 2 {
			return fmt.Errorf("invalid OMNILLM_SANDBOX_WORKSPACES entry")
		}
		if _, err := registry.Register(ownerUserID, strings.TrimSpace(idAndSpec[0]), strings.TrimSpace(modeAndPath[1]), MountMode(strings.TrimSpace(modeAndPath[0]))); err != nil {
			return fmt.Errorf("configure sandbox workspace %q: %w", strings.TrimSpace(idAndSpec[0]), err)
		}
	}
	return nil
}

// WorkspaceFSForOwner resolves the default registry, imports trusted process
// grants for this owner, and returns the containment-checked filesystem service.
func WorkspaceFSForOwner(ownerUserID string) (*WorkspaceFS, *WorkspaceRegistry, error) {
	registry, err := DefaultWorkspaceRegistry()
	if err != nil {
		return nil, nil, err
	}
	if err := seedConfiguredWorkspaces(strings.TrimSpace(ownerUserID), registry); err != nil {
		return nil, nil, err
	}
	fs, err := NewWorkspaceFS(registry)
	return fs, registry, err
}
