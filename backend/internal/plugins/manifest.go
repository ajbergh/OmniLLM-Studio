package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// LoadManifest reads and parses a plugin manifest.json from disk.
func LoadManifest(pluginDir string) (*models.PluginManifest, error) {
	path := filepath.Join(pluginDir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m models.PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := ValidateManifest(&m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &m, nil
}

// ValidateManifest checks required fields in a plugin manifest.
func ValidateManifest(m *models.PluginManifest) error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.Runtime == "" {
		return fmt.Errorf("runtime is required")
	}
	if m.Runtime != "executable" && m.Runtime != "wasm" {
		return fmt.Errorf("unsupported runtime %q (must be 'executable' or 'wasm')", m.Runtime)
	}
	if m.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("at least one capability is required")
	}
	for _, cap := range m.Capabilities {
		switch cap {
		case "tool", "provider", "processor":
			// ok
		default:
			return fmt.Errorf("unknown capability %q", cap)
		}
	}
	return nil
}
