package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// UserRepo handles CRUD operations for users.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// CountUsers returns the total number of registered users.
func (r *UserRepo) CountUsers() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetByID retrieves a user by ID.
func (r *UserRepo) GetByID(id string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, username, display_name, password_hash, role, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetByUsername retrieves a user by username.
func (r *UserRepo) GetByUsername(username string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, username, display_name, password_hash, role, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

// List returns all users (without password hashes in the returned models, but the caller should not serialize PasswordHash).
func (r *UserRepo) List() ([]models.User, error) {
	rows, err := r.db.Query(
		`SELECT id, username, display_name, password_hash, role, created_at, updated_at
		 FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateUserInput holds the parameters for creating a new user.
type CreateUserInput struct {
	Username     string
	DisplayName  string
	PasswordHash string
	Role         string // "admin", "member", "viewer"
}

// Create inserts a new user and returns it.
func (r *UserRepo) Create(input CreateUserInput) (*models.User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.db.Exec(
		`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, input.Username, input.DisplayName, input.PasswordHash, input.Role, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &models.User{
		ID:           id,
		Username:     input.Username,
		DisplayName:  input.DisplayName,
		PasswordHash: input.PasswordHash,
		Role:         input.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// UpdateUserInput holds the parameters for updating a user.
type UpdateUserInput struct {
	DisplayName  *string
	Role         *string
	PasswordHash *string
}

// Update modifies an existing user.
func (r *UserRepo) Update(id string, input UpdateUserInput) (*models.User, error) {
	u, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("user not found")
	}

	if input.DisplayName != nil {
		u.DisplayName = *input.DisplayName
	}
	if input.Role != nil {
		u.Role = *input.Role
	}
	if input.PasswordHash != nil {
		u.PasswordHash = *input.PasswordHash
	}

	now := time.Now().UTC()
	u.UpdatedAt = now

	_, err = r.db.Exec(
		`UPDATE users SET display_name = ?, role = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		u.DisplayName, u.Role, u.PasswordHash, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return u, nil
}

// Delete removes a user by ID.
func (r *UserRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}
