package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/plugins"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// PluginHandler handles plugin management API endpoints.
type PluginHandler struct {
	repo      *repository.PluginRepo
	loader    *plugins.Loader
	pluginDir string
}

// NewPluginHandler creates a new PluginHandler.
func NewPluginHandler(repo *repository.PluginRepo, loader *plugins.Loader, pluginDir string) *PluginHandler {
	return &PluginHandler{repo: repo, loader: loader, pluginDir: pluginDir}
}

// ListPlugins returns all installed plugins.
func (h *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if list == nil {
		list = []models.InstalledPlugin{}
	}

	// Enrich with running status
	type pluginResponse struct {
		models.InstalledPlugin
		Running bool `json:"running"`
	}

	result := make([]pluginResponse, len(list))
	for i, p := range list {
		_, running := h.loader.GetProcess(p.Name)
		result[i] = pluginResponse{
			InstalledPlugin: p,
			Running:         running,
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// installRequest is the body for the install endpoint.
type installRequest struct {
	Directory string `json:"directory"` // path to plugin directory
}

// InstallPlugin installs a plugin from a directory.
func (h *PluginHandler) InstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req installRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Directory == "" {
		respondError(w, http.StatusBadRequest, "directory is required")
		return
	}

	// Path containment: ensure directory is under the configured plugin root
	absDir, err := filepath.Abs(req.Directory)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid directory path")
		return
	}
	absRoot, _ := filepath.Abs(h.pluginDir)
	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		respondError(w, http.StatusBadRequest, "plugin must be installed in the configured plugin directory")
		return
	}

	// Load and validate manifest
	manifest, err := plugins.LoadManifest(absDir)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if already installed
	existing, err := h.repo.GetByName(manifest.Name)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if existing != nil {
		respondError(w, http.StatusConflict, "plugin already installed: "+manifest.Name)
		return
	}

	// Register in DB
	manifestJSON, _ := json.Marshal(manifest)
	plugin := models.InstalledPlugin{
		Name:     manifest.Name,
		Version:  manifest.Version,
		Manifest: string(manifestJSON),
		Enabled:  true,
	}
	if err := h.repo.Create(plugin); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, plugin)
}

// UninstallPlugin removes a plugin.
func (h *PluginHandler) UninstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Stop process if running
	h.loader.StopPlugin(name)

	if err := h.repo.Delete(name); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// updatePluginRequest is the body for enabling/disabling.
type updatePluginRequest struct {
	Enabled *bool `json:"enabled"`
}

// UpdatePlugin enables or disables a plugin.
func (h *PluginHandler) UpdatePlugin(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req updatePluginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Enabled == nil {
		respondError(w, http.StatusBadRequest, "enabled field is required")
		return
	}

	if err := h.repo.UpdateEnabled(name, *req.Enabled); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	// If disabling, stop the process
	if !*req.Enabled {
		h.loader.StopPlugin(name)
	}

	// Get updated record
	plugin, err := h.repo.GetByName(name)
	if err != nil || plugin == nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated plugin")
		return
	}

	respondJSON(w, http.StatusOK, plugin)
}
