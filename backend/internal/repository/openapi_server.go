package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// OpenAPIServerRepo persists governed OpenAPI tool-server definitions.
type OpenAPIServerRepo struct{ db *sql.DB }

func NewOpenAPIServerRepo(db *sql.DB) *OpenAPIServerRepo { return &OpenAPIServerRepo{db: db} }

func (r *OpenAPIServerRepo) List(ownerUserID string) ([]models.OpenAPIServer, error) {
	rows, err := r.db.Query(`SELECT id, owner_user_id, name, base_url, spec_json, enabled,
		allow_private_network, auth_header, auth_prefix, api_key_encrypted, created_at, updated_at
		FROM openapi_servers WHERE owner_user_id = ? ORDER BY name`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OpenAPIServer
	for rows.Next() {
		server, _, err := scanOpenAPIServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, server)
	}
	return out, rows.Err()
}

// ListRuntimeEnabled returns all enabled definitions with decrypted credentials.
func (r *OpenAPIServerRepo) ListRuntimeEnabled() ([]models.OpenAPIServerRuntime, error) {
	rows, err := r.db.Query(`SELECT id, owner_user_id, name, base_url, spec_json, enabled,
		allow_private_network, auth_header, auth_prefix, api_key_encrypted, created_at, updated_at
		FROM openapi_servers WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OpenAPIServerRuntime
	for rows.Next() {
		server, encrypted, err := scanOpenAPIServer(rows)
		if err != nil {
			return nil, err
		}
		key, err := crypto.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt OpenAPI credential for %s: %w", server.Name, err)
		}
		out = append(out, models.OpenAPIServerRuntime{OpenAPIServer: server, APIKey: key})
	}
	return out, rows.Err()
}

func (r *OpenAPIServerRepo) Get(ownerUserID, id string) (*models.OpenAPIServer, error) {
	row := r.db.QueryRow(`SELECT id, owner_user_id, name, base_url, spec_json, enabled,
		allow_private_network, auth_header, auth_prefix, api_key_encrypted, created_at, updated_at
		FROM openapi_servers WHERE owner_user_id = ? AND id = ?`, ownerUserID, id)
	server, _, err := scanOpenAPIServer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *OpenAPIServerRepo) GetRuntime(id string) (*models.OpenAPIServerRuntime, error) {
	row := r.db.QueryRow(`SELECT id, owner_user_id, name, base_url, spec_json, enabled,
		allow_private_network, auth_header, auth_prefix, api_key_encrypted, created_at, updated_at
		FROM openapi_servers WHERE id = ?`, id)
	server, encrypted, err := scanOpenAPIServer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := crypto.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	return &models.OpenAPIServerRuntime{OpenAPIServer: server, APIKey: key}, nil
}

func (r *OpenAPIServerRepo) Save(ownerUserID string, input models.OpenAPIServer, apiKey *string) (*models.OpenAPIServer, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.BaseURL) == "" || strings.TrimSpace(input.SpecJSON) == "" {
		return nil, fmt.Errorf("name, base_url, and spec_json are required")
	}
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	input.OwnerUserID = ownerUserID
	if input.AuthHeader == "" {
		input.AuthHeader = "Authorization"
	}
	if input.AuthPrefix == "" {
		input.AuthPrefix = "Bearer"
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	enabled := 0
	if input.Enabled {
		enabled = 1
	}
	privateNetwork := 0
	if input.AllowPrivateNetwork {
		privateNetwork = 1
	}
	currentEncrypted := ""
	_ = r.db.QueryRow(`SELECT api_key_encrypted FROM openapi_servers WHERE id = ? AND owner_user_id = ?`, input.ID, ownerUserID).Scan(&currentEncrypted)
	if apiKey != nil {
		encrypted, err := crypto.Encrypt(*apiKey)
		if err != nil {
			return nil, err
		}
		currentEncrypted = encrypted
	}
	_, err := r.db.Exec(`INSERT INTO openapi_servers (id, owner_user_id, name, base_url, spec_json, enabled, allow_private_network, auth_header, auth_prefix, api_key_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, base_url=excluded.base_url, spec_json=excluded.spec_json,
		enabled=excluded.enabled, allow_private_network=excluded.allow_private_network, auth_header=excluded.auth_header,
		auth_prefix=excluded.auth_prefix, api_key_encrypted=excluded.api_key_encrypted, updated_at=excluded.updated_at
		WHERE openapi_servers.owner_user_id=excluded.owner_user_id`, input.ID, ownerUserID, input.Name, input.BaseURL, input.SpecJSON, enabled, privateNetwork, input.AuthHeader, input.AuthPrefix, currentEncrypted, input.CreatedAt, input.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r.Get(ownerUserID, input.ID)
}

func (r *OpenAPIServerRepo) Delete(ownerUserID, id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM openapi_servers WHERE owner_user_id=? AND id=?`, ownerUserID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

type openAPIScanner interface{ Scan(dest ...any) error }

func scanOpenAPIServer(row openAPIScanner) (models.OpenAPIServer, string, error) {
	var s models.OpenAPIServer
	var enabled, privateNetwork int
	var encrypted string
	err := row.Scan(&s.ID, &s.OwnerUserID, &s.Name, &s.BaseURL, &s.SpecJSON, &enabled, &privateNetwork, &s.AuthHeader, &s.AuthPrefix, &encrypted, &s.CreatedAt, &s.UpdatedAt)
	s.Enabled = enabled != 0
	s.AllowPrivateNetwork = privateNetwork != 0
	s.HasAPIKey = encrypted != ""
	return s, encrypted, err
}
