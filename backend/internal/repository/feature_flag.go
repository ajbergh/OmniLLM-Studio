package repository

import (
	"database/sql"
	"fmt"
)

// FeatureFlag represents a feature flag record.
type FeatureFlag struct {
	Key      string `json:"key"`
	Enabled  bool   `json:"enabled"`
	Metadata string `json:"metadata,omitempty"`
}

// FeatureFlagRepo handles feature flag persistence.
type FeatureFlagRepo struct {
	db *sql.DB
}

// NewFeatureFlagRepo creates a new FeatureFlagRepo.
func NewFeatureFlagRepo(db *sql.DB) *FeatureFlagRepo {
	return &FeatureFlagRepo{db: db}
}

// List returns all feature flags.
func (r *FeatureFlagRepo) List() ([]FeatureFlag, error) {
	rows, err := r.db.Query("SELECT key, enabled, metadata FROM feature_flags ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}
	defer rows.Close()

	var flags []FeatureFlag
	for rows.Next() {
		var f FeatureFlag
		var enabled int
		if err := rows.Scan(&f.Key, &enabled, &f.Metadata); err != nil {
			return nil, fmt.Errorf("scan feature flag: %w", err)
		}
		f.Enabled = enabled != 0
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

// IsEnabled returns whether a feature flag is enabled. Returns false if not found.
func (r *FeatureFlagRepo) IsEnabled(key string) bool {
	var enabled int
	err := r.db.QueryRow("SELECT enabled FROM feature_flags WHERE key = ?", key).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled != 0
}

// Set creates or updates a feature flag.
func (r *FeatureFlagRepo) Set(key string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := r.db.Exec(`
		INSERT INTO feature_flags (key, enabled) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET enabled = excluded.enabled`,
		key, val,
	)
	if err != nil {
		return fmt.Errorf("set feature flag: %w", err)
	}
	return nil
}

// SetWithMetadata creates or updates a feature flag with metadata.
func (r *FeatureFlagRepo) SetWithMetadata(key string, enabled bool, metadata string) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := r.db.Exec(`
		INSERT INTO feature_flags (key, enabled, metadata) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET enabled = excluded.enabled, metadata = excluded.metadata`,
		key, val, metadata,
	)
	if err != nil {
		return fmt.Errorf("set feature flag: %w", err)
	}
	return nil
}

// Delete removes a feature flag.
func (r *FeatureFlagRepo) Delete(key string) error {
	_, err := r.db.Exec("DELETE FROM feature_flags WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete feature flag: %w", err)
	}
	return nil
}

// AsMap returns all feature flags as a map of key -> enabled.
func (r *FeatureFlagRepo) AsMap() (map[string]bool, error) {
	flags, err := r.List()
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(flags))
	for _, f := range flags {
		m[f.Key] = f.Enabled
	}
	return m, nil
}
