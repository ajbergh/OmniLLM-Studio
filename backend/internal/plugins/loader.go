package plugins

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// Loader manages plugin discovery, loading, and lifecycle.
type Loader struct {
	pluginDir string
	repo      *repository.PluginRepo
	mu        sync.RWMutex
	processes map[string]*PluginProcess // name -> process
}

// NewLoader creates a plugin loader.
func NewLoader(pluginDir string, repo *repository.PluginRepo) *Loader {
	return &Loader{
		pluginDir: pluginDir,
		repo:      repo,
		processes: make(map[string]*PluginProcess),
	}
}

// DiscoverAndLoad scans the plugin directory and loads all discovered plugins.
// Plugins found on disk but not in the DB are auto-registered.
// Plugins in the DB but not on disk are left as-is (may be disabled).
func (l *Loader) DiscoverAndLoad() error {
	if l.pluginDir == "" {
		return nil
	}

	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no plugins directory yet
		}
		return fmt.Errorf("read plugin dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(l.pluginDir, entry.Name())
		manifest, err := LoadManifest(pluginPath)
		if err != nil {
			log.Printf("[plugins] skip %s: %v", entry.Name(), err)
			continue
		}

		// Check if already registered
		existing, err := l.repo.GetByName(manifest.Name)
		if err != nil {
			log.Printf("[plugins] error checking %s: %v", manifest.Name, err)
			continue
		}

		if existing == nil {
			// Auto-register discovered plugin
			manifestJSON, _ := json.Marshal(manifest)
			if err := l.repo.Create(models.InstalledPlugin{
				Name:     manifest.Name,
				Version:  manifest.Version,
				Manifest: string(manifestJSON),
				Enabled:  true,
			}); err != nil {
				log.Printf("[plugins] failed to register %s: %v", manifest.Name, err)
				continue
			}
			log.Printf("[plugins] discovered and registered: %s v%s", manifest.Name, manifest.Version)
		}

		// Only start enabled plugins
		if existing != nil && !existing.Enabled {
			log.Printf("[plugins] skipping disabled plugin: %s", manifest.Name)
			continue
		}

		// Start plugin process
		proc := NewPluginProcess(manifest, pluginPath)
		if proc == nil {
			log.Printf("[plugins] refusing to start %s: entrypoint validation failed", manifest.Name)
			continue
		}
		if err := proc.Start(); err != nil {
			log.Printf("[plugins] failed to start %s: %v", manifest.Name, err)
			continue
		}

		l.mu.Lock()
		l.processes[manifest.Name] = proc
		l.mu.Unlock()

		log.Printf("[plugins] started: %s v%s", manifest.Name, manifest.Version)
	}

	return nil
}

// GetProcess returns a running plugin process by name.
func (l *Loader) GetProcess(name string) (*PluginProcess, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.processes[name]
	return p, ok
}

// StopAll stops all running plugin processes.
func (l *Loader) StopAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for name, proc := range l.processes {
		if err := proc.Stop(); err != nil {
			log.Printf("[plugins] error stopping %s: %v", name, err)
		}
	}
	l.processes = make(map[string]*PluginProcess)
}

// StopPlugin stops a specific plugin process.
func (l *Loader) StopPlugin(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	proc, ok := l.processes[name]
	if !ok {
		return nil
	}
	delete(l.processes, name)
	return proc.Stop()
}

// ListRunning returns names of all running plugin processes.
func (l *Loader) ListRunning() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.processes))
	for name := range l.processes {
		names = append(names, name)
	}
	return names
}
