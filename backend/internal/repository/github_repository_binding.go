package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbschema "github.com/ajbergh/omnillm-studio/internal/db"
)

// GitHubRepositoryBinding is the persistence model for an explicit association
// between an owner-scoped GitHub repository and one startup-allowlisted local
// repository ID. It intentionally contains neither filesystem paths nor tokens.
type GitHubRepositoryBinding struct {
	OwnerID            string    `json:"-"`
	LocalRepositoryID  string    `json:"local_repository_id"`
	GitHubUserID       int64     `json:"github_user_id"`
	GitHubRepositoryID int64     `json:"github_repository_id"`
	GitHubFullName     string    `json:"github_full_name"`
	DefaultBranch      string    `json:"default_branch"`
	Private            bool      `json:"private"`
	Fork               bool      `json:"fork"`
	Archived           bool      `json:"archived"`
	Disabled           bool      `json:"disabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type GitHubRepositoryBindingRepo struct {
	db      *sql.DB
	initErr error
}

func NewGitHubRepositoryBindingRepo(database *sql.DB) *GitHubRepositoryBindingRepo {
	repo := &GitHubRepositoryBindingRepo{db: database}
	repo.initErr = dbschema.EnsureGitHubRepositoryBindingsSchema(database)
	return repo
}

func (r *GitHubRepositoryBindingRepo) ready() error {
	if r == nil || r.db == nil {
		return fmt.Errorf("GitHub repository binding repository is unavailable")
	}
	if r.initErr != nil {
		return r.initErr
	}
	return nil
}

// List returns all bindings for one OmniLLM owner ordered by local repository ID.
func (r *GitHubRepositoryBindingRepo) List(ownerID string) ([]GitHubRepositoryBinding, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner ID is required")
	}
	rows, err := r.db.Query(`
		SELECT owner_id, local_repository_id, github_user_id, github_repository_id,
			github_full_name, default_branch, private, fork, archived, disabled,
			created_at, updated_at
		FROM github_repository_bindings
		WHERE owner_id = ?
		ORDER BY local_repository_id ASC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list GitHub repository bindings: %w", err)
	}
	defer rows.Close()

	bindings := make([]GitHubRepositoryBinding, 0)
	for rows.Next() {
		binding, err := scanGitHubRepositoryBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list GitHub repository bindings: %w", err)
	}
	return bindings, nil
}

// Get returns one owner/local-repository binding or nil when it does not exist.
func (r *GitHubRepositoryBindingRepo) Get(ownerID, localRepositoryID string) (*GitHubRepositoryBinding, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	ownerID = strings.TrimSpace(ownerID)
	localRepositoryID = strings.TrimSpace(localRepositoryID)
	if ownerID == "" || localRepositoryID == "" {
		return nil, fmt.Errorf("owner ID and local repository ID are required")
	}
	row := r.db.QueryRow(`
		SELECT owner_id, local_repository_id, github_user_id, github_repository_id,
			github_full_name, default_branch, private, fork, archived, disabled,
			created_at, updated_at
		FROM github_repository_bindings
		WHERE owner_id = ? AND local_repository_id = ?
	`, ownerID, localRepositoryID)
	binding, err := scanGitHubRepositoryBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// Upsert replaces the selected local repository's GitHub association for one owner.
func (r *GitHubRepositoryBindingRepo) Upsert(ownerID string, binding GitHubRepositoryBinding) error {
	if err := r.ready(); err != nil {
		return err
	}
	ownerID = strings.TrimSpace(ownerID)
	binding.LocalRepositoryID = strings.TrimSpace(binding.LocalRepositoryID)
	binding.GitHubFullName = strings.TrimSpace(binding.GitHubFullName)
	binding.DefaultBranch = strings.TrimSpace(binding.DefaultBranch)
	if ownerID == "" || binding.LocalRepositoryID == "" {
		return fmt.Errorf("owner ID and local repository ID are required")
	}
	if binding.GitHubUserID <= 0 || binding.GitHubRepositoryID <= 0 || binding.GitHubFullName == "" {
		return fmt.Errorf("GitHub repository identity is required")
	}
	_, err := r.db.Exec(`
		INSERT INTO github_repository_bindings (
			owner_id, local_repository_id, github_user_id, github_repository_id,
			github_full_name, default_branch, private, fork, archived, disabled,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(owner_id, local_repository_id) DO UPDATE SET
			github_user_id = excluded.github_user_id,
			github_repository_id = excluded.github_repository_id,
			github_full_name = excluded.github_full_name,
			default_branch = excluded.default_branch,
			private = excluded.private,
			fork = excluded.fork,
			archived = excluded.archived,
			disabled = excluded.disabled,
			updated_at = CURRENT_TIMESTAMP
	`, ownerID, binding.LocalRepositoryID, binding.GitHubUserID, binding.GitHubRepositoryID,
		binding.GitHubFullName, binding.DefaultBranch, binding.Private, binding.Fork, binding.Archived, binding.Disabled)
	if err != nil {
		return fmt.Errorf("save GitHub repository binding: %w", err)
	}
	return nil
}

// Delete removes only the selected owner's binding for one local repository ID.
func (r *GitHubRepositoryBindingRepo) Delete(ownerID, localRepositoryID string) error {
	if err := r.ready(); err != nil {
		return err
	}
	ownerID = strings.TrimSpace(ownerID)
	localRepositoryID = strings.TrimSpace(localRepositoryID)
	if ownerID == "" || localRepositoryID == "" {
		return fmt.Errorf("owner ID and local repository ID are required")
	}
	if _, err := r.db.Exec(`
		DELETE FROM github_repository_bindings
		WHERE owner_id = ? AND local_repository_id = ?
	`, ownerID, localRepositoryID); err != nil {
		return fmt.Errorf("delete GitHub repository binding: %w", err)
	}
	return nil
}

type githubRepositoryBindingScanner interface {
	Scan(dest ...any) error
}

func scanGitHubRepositoryBinding(scanner githubRepositoryBindingScanner) (GitHubRepositoryBinding, error) {
	var binding GitHubRepositoryBinding
	if err := scanner.Scan(
		&binding.OwnerID,
		&binding.LocalRepositoryID,
		&binding.GitHubUserID,
		&binding.GitHubRepositoryID,
		&binding.GitHubFullName,
		&binding.DefaultBranch,
		&binding.Private,
		&binding.Fork,
		&binding.Archived,
		&binding.Disabled,
		&binding.CreatedAt,
		&binding.UpdatedAt,
	); err != nil {
		return GitHubRepositoryBinding{}, err
	}
	return binding, nil
}
