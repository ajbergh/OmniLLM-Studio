package repository

import (
	"database/sql"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

// SettingsRepo handles application settings persistence.
type SettingsRepo struct {
	db *sql.DB
}

// NewSettingsRepo creates a new SettingsRepo.
func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// GetAll returns all settings as a map.
func (r *SettingsRepo) GetAll() (map[string]string, error) {
	rows, err := r.db.Query("SELECT key, value_json FROM settings")
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var s models.Setting
		if err := rows.Scan(&s.Key, &s.ValueJSON); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings[s.Key] = s.ValueJSON
	}
	return settings, rows.Err()
}

// Get returns a single setting value.
func (r *SettingsRepo) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow("SELECT value_json FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting: %w", err)
	}
	return value, nil
}

// Set upserts a setting.
func (r *SettingsRepo) Set(key, valueJSON string) error {
	_, err := r.db.Exec(`
		INSERT INTO settings (key, value_json) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json
	`, key, valueJSON)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

// SetMany upserts multiple settings.
func (r *SettingsRepo) SetMany(settings map[string]string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO settings (key, value_json) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json
	`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for k, v := range settings {
		if _, err := stmt.Exec(k, v); err != nil {
			return fmt.Errorf("set %s: %w", k, err)
		}
	}

	return tx.Commit()
}

// Delete removes a setting.
func (r *SettingsRepo) Delete(key string) error {
	_, err := r.db.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

// GetTyped returns all settings as a typed AppSettings struct.
// Sensitive fields (e.g. brave_api_key) are decrypted transparently.
func (r *SettingsRepo) GetTyped() (models.AppSettings, error) {
	m, err := r.GetAll()
	if err != nil {
		return models.AppSettings{}, err
	}
	s := models.AppSettingsFromMap(m)

	// Decrypt sensitive fields
	if s.BraveAPIKey != "" {
		decrypted, err := crypto.Decrypt(s.BraveAPIKey)
		if err != nil {
			return models.AppSettings{}, fmt.Errorf("decrypt brave_api_key: %w", err)
		}
		s.BraveAPIKey = decrypted
	}

	return s, nil
}

// GetTypedMasked returns typed settings with sensitive fields masked.
func (r *SettingsRepo) GetTypedMasked() (models.AppSettings, error) {
	s, err := r.GetTyped()
	if err != nil {
		return s, err
	}
	if s.BraveAPIKey != "" {
		s.BraveAPIKey = "••••••••"
	}
	return s, nil
}

// SetTyped persists a typed AppSettings struct.
// Sensitive fields (e.g. brave_api_key) are encrypted before storage.
func (r *SettingsRepo) SetTyped(s models.AppSettings) error {
	if s.BraveAPIKey != "" {
		encrypted, err := crypto.Encrypt(s.BraveAPIKey)
		if err != nil {
			return fmt.Errorf("encrypt brave_api_key: %w", err)
		}
		s.BraveAPIKey = encrypted
	}
	return r.SetMany(s.ToMap())
}
