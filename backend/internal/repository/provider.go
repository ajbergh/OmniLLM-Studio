package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ProviderRepo handles provider profile CRUD.
type ProviderRepo struct {
	db *sql.DB
}

// NewProviderRepo creates a new ProviderRepo.
func NewProviderRepo(db *sql.DB) *ProviderRepo {
	return &ProviderRepo{db: db}
}

// List returns all provider profiles.
func (r *ProviderRepo) List() ([]models.ProviderProfile, error) {
	rows, err := r.db.Query(`
		SELECT id, name, type, base_url, default_model, default_image_model, enabled, created_at, updated_at, metadata_json
		FROM provider_profiles ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []models.ProviderProfile
	for rows.Next() {
		var p models.ProviderProfile
		var enabled int
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.DefaultModel, &p.DefaultImageModel,
			&enabled, &p.CreatedAt, &p.UpdatedAt, &p.MetadataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		p.Enabled = enabled != 0
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// GetByID retrieves a provider profile by ID.
func (r *ProviderRepo) GetByID(id string) (*models.ProviderProfile, error) {
	var p models.ProviderProfile
	var enabled int
	err := r.db.QueryRow(`
		SELECT id, name, type, base_url, default_model, default_image_model, enabled, created_at, updated_at, metadata_json
		FROM provider_profiles WHERE id = ?
	`, id).Scan(
		&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.DefaultModel, &p.DefaultImageModel,
		&enabled, &p.CreatedAt, &p.UpdatedAt, &p.MetadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	p.Enabled = enabled != 0
	return &p, nil
}

// CreateInput holds data for creating a new provider profile.
type CreateProviderInput struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	BaseURL           *string `json:"base_url,omitempty"`
	DefaultModel      *string `json:"default_model,omitempty"`
	DefaultImageModel *string `json:"default_image_model,omitempty"`
	APIKey            string  `json:"api_key,omitempty"`
}

// Create inserts a new provider profile and optionally stores an API key.
func (r *ProviderRepo) Create(input CreateProviderInput) (*models.ProviderProfile, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO provider_profiles (id, name, type, base_url, default_model, default_image_model, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)
	`, id, input.Name, input.Type, input.BaseURL, input.DefaultModel, input.DefaultImageModel, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert provider: %w", err)
	}

	if input.APIKey != "" {
		encryptedKey, err := crypto.Encrypt(input.APIKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
		_, err = tx.Exec(`
			INSERT INTO secrets (provider_profile_id, api_key_encrypted, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`, id, encryptedKey, now, now)
		if err != nil {
			return nil, fmt.Errorf("insert secret: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetByID(id)
}

// ProviderUpdate holds update data.
type ProviderUpdate struct {
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty"`
	BaseURL           *string `json:"base_url,omitempty"`
	DefaultModel      *string `json:"default_model,omitempty"`
	DefaultImageModel *string `json:"default_image_model,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	APIKey            *string `json:"api_key,omitempty"`
}

// Update modifies an existing provider profile.
func (r *ProviderRepo) Update(id string, upd ProviderUpdate) (*models.ProviderProfile, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	sets := []string{}
	args := []interface{}{}

	if upd.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *upd.Name)
	}
	if upd.Type != nil {
		sets = append(sets, "type = ?")
		args = append(args, *upd.Type)
	}
	if upd.BaseURL != nil {
		sets = append(sets, "base_url = ?")
		args = append(args, *upd.BaseURL)
	}
	if upd.DefaultModel != nil {
		sets = append(sets, "default_model = ?")
		args = append(args, *upd.DefaultModel)
	}
	if upd.DefaultImageModel != nil {
		sets = append(sets, "default_image_model = ?")
		args = append(args, *upd.DefaultImageModel)
	}
	if upd.Enabled != nil {
		sets = append(sets, "enabled = ?")
		if *upd.Enabled {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = ?")
		args = append(args, time.Now().UTC())
		args = append(args, id)

		query := "UPDATE provider_profiles SET "
		for i, s := range sets {
			if i > 0 {
				query += ", "
			}
			query += s
		}
		query += " WHERE id = ?"

		if _, err := tx.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update provider: %w", err)
		}
	}

	if upd.APIKey != nil {
		now := time.Now().UTC()
		encryptedKey, encErr := crypto.Encrypt(*upd.APIKey)
		if encErr != nil {
			return nil, fmt.Errorf("encrypt api key: %w", encErr)
		}
		_, err := tx.Exec(`
			INSERT INTO secrets (provider_profile_id, api_key_encrypted, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(provider_profile_id) DO UPDATE SET
				api_key_encrypted = excluded.api_key_encrypted,
				updated_at = excluded.updated_at
		`, id, encryptedKey, now, now)
		if err != nil {
			return nil, fmt.Errorf("upsert secret: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetByID(id)
}

// Delete removes a provider profile and its secrets.
func (r *ProviderRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM provider_profiles WHERE id = ?", id)
	return err
}

// GetAPIKey retrieves and decrypts the API key for a provider.
func (r *ProviderRepo) GetAPIKey(providerID string) (string, error) {
	var key string
	err := r.db.QueryRow("SELECT api_key_encrypted FROM secrets WHERE provider_profile_id = ?", providerID).Scan(&key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get api key: %w", err)
	}
	decryptedKey, err := crypto.Decrypt(key)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	return decryptedKey, nil
}
