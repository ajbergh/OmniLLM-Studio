package sandbox

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var workspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

const workspaceSchema = `
CREATE TABLE IF NOT EXISTS sandbox_workspaces (
    id TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    root_path TEXT NOT NULL,
    root_identity TEXT,
    mode TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, owner_user_id)
);
CREATE INDEX IF NOT EXISTS idx_sandbox_workspaces_owner
    ON sandbox_workspaces(owner_user_id, id);

CREATE TABLE IF NOT EXISTS sandbox_workspace_changes (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    conversation_id TEXT,
    agent_run_id TEXT,
    task_id TEXT,
    sandbox_id TEXT,
    execution_id TEXT,
    relative_path TEXT NOT NULL,
    operation TEXT NOT NULL,
    before_exists INTEGER NOT NULL,
    before_sha256 TEXT,
    after_exists INTEGER NOT NULL,
    after_sha256 TEXT,
    before_content BLOB,
    before_mode INTEGER,
    revertable INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sandbox_workspace_changes_scope
    ON sandbox_workspace_changes(user_id, workspace_id, created_at DESC);
`

// FileWorkspace is an application-owned filesystem grant. RootPath and
// RootIdentity are internal host state and must not be returned through
// model-facing workspace tools.
type FileWorkspace struct {
	ID           string
	OwnerUserID  string
	RootPath     string
	RootIdentity string
	Mode         MountMode
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// WorkspaceRegistry persists explicit filesystem grants and resolves opaque IDs
// to canonical physical roots. It never accepts a physical path from a tool
// invocation; only trusted application/operator code may call Register.
type WorkspaceRegistry struct {
	db *sql.DB
}

// NewWorkspaceRegistry ensures the durable sandbox-workspace schema and returns
// a registry backed by the application's SQLite database.
func NewWorkspaceRegistry(db *sql.DB) (*WorkspaceRegistry, error) {
	if db == nil {
		return nil, fmt.Errorf("workspace registry database is required")
	}
	if _, err := db.Exec(workspaceSchema); err != nil {
		return nil, fmt.Errorf("ensure sandbox workspace schema: %w", err)
	}
	if err := ensureWorkspaceIdentityColumn(db); err != nil {
		return nil, err
	}
	return &WorkspaceRegistry{db: db}, nil
}

func ensureWorkspaceIdentityColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(sandbox_workspaces)`)
	if err != nil {
		return fmt.Errorf("inspect sandbox workspace schema: %w", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("inspect sandbox workspace schema: %w", err)
		}
		if name == "root_identity" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect sandbox workspace schema: %w", err)
	}
	if found {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE sandbox_workspaces ADD COLUMN root_identity TEXT`); err != nil {
		return fmt.Errorf("add sandbox workspace root identity: %w", err)
	}
	return nil
}

// Register creates or replaces one owner-scoped filesystem grant. The physical
// root is canonicalized and, on platforms with a proven identity primitive,
// bound to the filesystem object that occupies that path at registration time.
func (r *WorkspaceRegistry) Register(ownerUserID, id, root string, mode MountMode) (*FileWorkspace, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	id = strings.TrimSpace(id)
	if ownerUserID == "" {
		return nil, fmt.Errorf("workspace owner is required")
	}
	if !workspaceIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid workspace id")
	}
	if err := validateMountMode(mode); err != nil {
		return nil, err
	}
	canonical, err := canonicalWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	identity, err := captureWorkspaceRootIdentity(canonical)
	if err != nil {
		return nil, err
	}
	if workspaceRootIdentityRequired() && identity == "" {
		return nil, fmt.Errorf("workspace root identity is required on this platform")
	}
	now := time.Now().UTC()
	_, err = r.db.Exec(`
INSERT INTO sandbox_workspaces (id, owner_user_id, root_path, root_identity, mode, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id, owner_user_id) DO UPDATE SET
    root_path=excluded.root_path,
    root_identity=excluded.root_identity,
    mode=excluded.mode,
    updated_at=excluded.updated_at`, id, ownerUserID, canonical, nullableString(identity), string(mode), now, now)
	if err != nil {
		return nil, fmt.Errorf("register sandbox workspace: %w", err)
	}
	return r.Get(ownerUserID, id)
}

// Get resolves one workspace only within the exact owner scope and, where the
// platform supports durable root identity, verifies that the registered path
// still names the same filesystem object before returning the grant.
func (r *WorkspaceRegistry) Get(ownerUserID, id string) (*FileWorkspace, error) {
	var workspace FileWorkspace
	var mode string
	var rootIdentity sql.NullString
	err := r.db.QueryRow(`SELECT id, owner_user_id, root_path, root_identity, mode, created_at, updated_at
FROM sandbox_workspaces WHERE id=? AND owner_user_id=?`, strings.TrimSpace(id), strings.TrimSpace(ownerUserID)).
		Scan(&workspace.ID, &workspace.OwnerUserID, &workspace.RootPath, &rootIdentity, &mode, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sandbox workspace not found")
	}
	if err != nil {
		return nil, fmt.Errorf("read sandbox workspace: %w", err)
	}
	workspace.RootIdentity = strings.TrimSpace(rootIdentity.String)
	workspace.Mode = MountMode(mode)
	if err := validateMountMode(workspace.Mode); err != nil {
		return nil, fmt.Errorf("stored sandbox workspace mode is invalid")
	}
	if err := verifyWorkspaceRootIdentity(workspace.RootPath, workspace.RootIdentity); err != nil {
		return nil, err
	}
	return &workspace, nil
}

func verifyWorkspaceRootIdentity(rootPath, expectedIdentity string) error {
	if !workspaceRootIdentityRequired() {
		return nil
	}
	expectedIdentity = strings.TrimSpace(expectedIdentity)
	if expectedIdentity == "" {
		return fmt.Errorf("sandbox workspace grant predates durable root identity; re-register the workspace")
	}
	canonical, err := canonicalWorkspaceRoot(rootPath)
	if err != nil || canonical != rootPath {
		return fmt.Errorf("workspace root is no longer safely canonical")
	}
	currentIdentity, err := captureWorkspaceRootIdentity(rootPath)
	if err != nil {
		return err
	}
	if currentIdentity != expectedIdentity {
		return fmt.Errorf("workspace root identity changed; re-register the workspace")
	}
	return nil
}

// List returns owner-scoped grants in deterministic ID order. On platforms with
// durable root identity, stale/legacy grants are omitted rather than surfaced as
// usable grants; callers must re-register them through a trusted path.
func (r *WorkspaceRegistry) List(ownerUserID string) ([]FileWorkspace, error) {
	rows, err := r.db.Query(`SELECT id, owner_user_id, root_path, root_identity, mode, created_at, updated_at
FROM sandbox_workspaces WHERE owner_user_id=? ORDER BY id`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, fmt.Errorf("list sandbox workspaces: %w", err)
	}
	defer rows.Close()
	var out []FileWorkspace
	for rows.Next() {
		var workspace FileWorkspace
		var mode string
		var rootIdentity sql.NullString
		if err := rows.Scan(&workspace.ID, &workspace.OwnerUserID, &workspace.RootPath, &rootIdentity, &mode, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
			return nil, err
		}
		workspace.RootIdentity = strings.TrimSpace(rootIdentity.String)
		workspace.Mode = MountMode(mode)
		if err := validateMountMode(workspace.Mode); err != nil {
			return nil, fmt.Errorf("stored sandbox workspace mode is invalid")
		}
		if err := verifyWorkspaceRootIdentity(workspace.RootPath, workspace.RootIdentity); err != nil {
			continue
		}
		out = append(out, workspace)
	}
	return out, rows.Err()
}

// Remove revokes an exact owner-scoped workspace grant. It does not delete any
// filesystem content.
func (r *WorkspaceRegistry) Remove(ownerUserID, id string) error {
	result, err := r.db.Exec(`DELETE FROM sandbox_workspaces WHERE id=? AND owner_user_id=?`, strings.TrimSpace(id), strings.TrimSpace(ownerUserID))
	if err != nil {
		return fmt.Errorf("remove sandbox workspace: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("sandbox workspace not found")
	}
	return nil
}

// InternalMount resolves an owner-scoped opaque workspace ID into the mount
// descriptor and canonical host path used only across the trusted Broker/runtime
// boundary.
func (r *WorkspaceRegistry) InternalMount(ownerUserID, id string) (*FileWorkspace, WorkspaceMount, error) {
	workspace, err := r.Get(ownerUserID, id)
	if err != nil {
		return nil, WorkspaceMount{}, err
	}
	return workspace, WorkspaceMount{WorkspaceID: workspace.ID, Mode: workspace.Mode}, nil
}

func validateMountMode(mode MountMode) error {
	switch mode {
	case MountReadOnly, MountReadWriteNoDelete, MountReadWrite:
		return nil
	default:
		return fmt.Errorf("unsupported workspace mode %q", mode)
	}
}

func canonicalWorkspaceRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("workspace root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root is not a directory")
	}
	return filepath.Clean(resolved), nil
}

// workspaceIDs is useful for deterministic policy/error reporting without host
// path disclosure.
func workspaceIDs(workspaces []FileWorkspace) []string {
	ids := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		ids = append(ids, workspace.ID)
	}
	sort.Strings(ids)
	return ids
}
