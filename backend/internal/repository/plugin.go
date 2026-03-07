package repository

import (
	"database/sql"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// PluginRepo handles CRUD operations for installed plugins.
type PluginRepo struct {
	db *sql.DB
}

// NewPluginRepo creates a new PluginRepo.
func NewPluginRepo(db *sql.DB) *PluginRepo {
	return &PluginRepo{db: db}
}

// List returns all installed plugins.
func (r *PluginRepo) List() ([]models.InstalledPlugin, error) {
	rows, err := r.db.Query(
		"SELECT name, version, manifest, enabled, installed_at FROM installed_plugins ORDER BY installed_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()

	var plugins []models.InstalledPlugin
	for rows.Next() {
		var p models.InstalledPlugin
		if err := rows.Scan(&p.Name, &p.Version, &p.Manifest, &p.Enabled, &p.InstalledAt); err != nil {
			return nil, fmt.Errorf("scan plugin: %w", err)
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// GetByName returns a plugin by name, or nil if not found.
func (r *PluginRepo) GetByName(name string) (*models.InstalledPlugin, error) {
	var p models.InstalledPlugin
	err := r.db.QueryRow(
		"SELECT name, version, manifest, enabled, installed_at FROM installed_plugins WHERE name = ?",
		name,
	).Scan(&p.Name, &p.Version, &p.Manifest, &p.Enabled, &p.InstalledAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get plugin %q: %w", name, err)
	}
	return &p, nil
}

// Create inserts a new plugin record.
func (r *PluginRepo) Create(p models.InstalledPlugin) error {
	_, err := r.db.Exec(
		"INSERT INTO installed_plugins (name, version, manifest, enabled) VALUES (?, ?, ?, ?)",
		p.Name, p.Version, p.Manifest, p.Enabled,
	)
	if err != nil {
		return fmt.Errorf("create plugin %q: %w", p.Name, err)
	}
	return nil
}

// UpdateEnabled sets the enabled state of a plugin.
func (r *PluginRepo) UpdateEnabled(name string, enabled bool) error {
	res, err := r.db.Exec(
		"UPDATE installed_plugins SET enabled = ? WHERE name = ?",
		enabled, name,
	)
	if err != nil {
		return fmt.Errorf("update plugin %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("plugin %q not found", name)
	}
	return nil
}

// Delete removes a plugin record.
func (r *PluginRepo) Delete(name string) error {
	res, err := r.db.Exec("DELETE FROM installed_plugins WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete plugin %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("plugin %q not found", name)
	}
	return nil
}
