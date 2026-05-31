package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/go-chi/chi/v5"
)

type VideoHandler struct {
	service     *video.Service
	projectRepo *repository.VideoProjectRepo
	genRepo     *repository.VideoGenerationRepo
	assetRepo   *repository.VideoAssetRepo
	storageDir  string
}

func NewVideoHandler(
	service *video.Service,
	projectRepo *repository.VideoProjectRepo,
	genRepo *repository.VideoGenerationRepo,
	assetRepo *repository.VideoAssetRepo,
	storageDir string,
) *VideoHandler {
	return &VideoHandler{
		service:     service,
		projectRepo: projectRepo,
		genRepo:     genRepo,
		assetRepo:   assetRepo,
		storageDir:  storageDir,
	}
}

func (h *VideoHandler) Providers(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListProviders(r.Context())
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, providers)
}

func (h *VideoHandler) Models(w http.ResponseWriter, r *http.Request) {
	models, err := h.service.ListModels(r.Context(), r.URL.Query().Get("provider"), false)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if models == nil {
		models = []video.Model{}
	}
	respondJSON(w, http.StatusOK, models)
}

func (h *VideoHandler) RefreshModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.service.ListModels(r.Context(), r.URL.Query().Get("provider"), true)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if models == nil {
		models = []video.Model{}
	}
	respondJSON(w, http.StatusOK, models)
}

func (h *VideoHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projectRepo.ListForUser(auth.UserIDFromContext(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if projects == nil {
		projects = []models.VideoProject{}
	}
	respondJSON(w, http.StatusOK, projects)
}

func (h *VideoHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title,omitempty"`
		Provider    string `json:"provider,omitempty"`
		Model       string `json:"model,omitempty"`
		Width       int    `json:"width,omitempty"`
		Height      int    `json:"height,omitempty"`
		FPS         int    `json:"fps,omitempty"`
		AspectRatio string `json:"aspect_ratio,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	project, err := h.service.CreateProject(auth.UserIDFromContext(r.Context()), req.Title, req.Provider, req.Model, req.Width, req.Height, req.FPS, req.AspectRatio)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, project)
}

func (h *VideoHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	generations, err := h.genRepo.ListByProject(project.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	assets, err := h.assetRepo.ListByProject(project.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"project":     project,
		"generations": h.enrichGenerations(generations),
		"assets":      assets,
	})
}

func (h *VideoHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var req struct {
		Title       string `json:"title,omitempty"`
		Provider    string `json:"provider,omitempty"`
		Model       string `json:"model,omitempty"`
		Width       int    `json:"width,omitempty"`
		Height      int    `json:"height,omitempty"`
		FPS         int    `json:"fps,omitempty"`
		AspectRatio string `json:"aspect_ratio,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.projectRepo.Update(project.ID, req.Title, req.Provider, req.Model, req.Width, req.Height, req.FPS, req.AspectRatio)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (h *VideoHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	assets, _ := h.assetRepo.ListByProject(project.ID)
	for _, asset := range assets {
		if fullPath, err := SafeJoin(h.storageDir, asset.FilePath); err == nil {
			_ = os.Remove(fullPath)
		}
	}
	if err := h.projectRepo.Delete(project.ID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *VideoHandler) ListGenerations(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	generations, err := h.genRepo.ListByProject(project.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, h.enrichGenerations(generations))
}

func (h *VideoHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req video.GenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	writeEvent := func(event string, payload interface{}) {
		data, _ := json.Marshal(payload)
		_, _ = w.Write([]byte("event: " + event + "\n"))
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		flusher.Flush()
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-done:
				return
			case <-ticker.C:
				writeEvent("video_generation_progress", map[string]string{"stage": "waiting", "message": "Still generating video"})
			}
		}
	}()

	project, generation, asset, err := h.service.Generate(r.Context(), auth.UserIDFromContext(r.Context()), req, func(progress video.GenerationProgress) {
		if progress.Stage == "started" {
			writeEvent("video_generation_started", progress)
			return
		}
		writeEvent("video_generation_progress", progress)
	})
	close(done)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, video.ErrCapabilityUnsupported) || errors.Is(err, video.ErrProviderUnavailable) {
			status = http.StatusBadRequest
		}
		writeEvent("video_generation_error", map[string]interface{}{
			"error":         err.Error(),
			"status":        status,
			"project_id":    videoProjectIDOrEmpty(project),
			"generation_id": videoGenerationIDOrEmpty(generation),
		})
		return
	}
	writeEvent("video_generation_done", map[string]interface{}{
		"project":    project,
		"generation": h.enrichGeneration(*generation),
		"asset":      asset,
	})
}

func (h *VideoHandler) GetGeneration(w http.ResponseWriter, r *http.Request) {
	generation, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, h.enrichGeneration(*generation))
}

func (h *VideoHandler) BranchGeneration(w http.ResponseWriter, r *http.Request) {
	generation, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"parent_id":       generation.ID,
		"project_id":      generation.ProjectID,
		"prompt":          generation.Prompt,
		"enhanced_prompt": generation.EnhancedPrompt,
		"negative_prompt": generation.NegativePrompt,
		"provider":        generation.Provider,
		"model":           generation.Model,
		"settings_json":   generation.SettingsJSON,
	})
}

func (h *VideoHandler) SendGenerationToTimeline(w http.ResponseWriter, r *http.Request) {
	generation, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"project_id": generation.ProjectID,
		"asset_id":   generation.OutputAssetID,
		"queued":     false,
		"message":    "Timeline placement will be enabled in Phase 2.",
	})
}

func (h *VideoHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	assets, err := h.assetRepo.ListByProject(project.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if assets == nil {
		assets = []models.VideoAsset{}
	}
	respondJSON(w, http.StatusOK, assets)
}

func (h *VideoHandler) GetAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, asset)
}

func (h *VideoHandler) DownloadAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	fullPath, err := SafeJoin(h.storageDir, asset.FilePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid asset path")
		return
	}
	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(asset.FileName)+`"`)
	http.ServeFile(w, r, fullPath)
}

func (h *VideoHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	if fullPath, err := SafeJoin(h.storageDir, asset.FilePath); err == nil {
		_ = os.Remove(fullPath)
	}
	if err := h.assetRepo.Delete(asset.ID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *VideoHandler) EnhancePrompt(w http.ResponseWriter, r *http.Request) {
	var req video.EnhancePromptRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enhanced := video.EnhancePrompt(req)
	if enhanced == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	respondJSON(w, http.StatusOK, video.EnhancePromptResponse{Prompt: enhanced})
}

func (h *VideoHandler) loadOwnedProject(w http.ResponseWriter, r *http.Request, projectID string) (*models.VideoProject, bool) {
	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if project == nil {
		respondError(w, http.StatusNotFound, "video project not found")
		return nil, false
	}
	if userID := auth.UserIDFromContext(r.Context()); userID != "" && (project.UserID == nil || *project.UserID != userID) {
		respondError(w, http.StatusNotFound, "video project not found")
		return nil, false
	}
	return project, true
}

func (h *VideoHandler) loadOwnedGeneration(w http.ResponseWriter, r *http.Request, generationID string) (*models.VideoGeneration, bool) {
	generation, err := h.genRepo.GetByID(generationID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if generation == nil {
		respondError(w, http.StatusNotFound, "video generation not found")
		return nil, false
	}
	if _, ok := h.loadOwnedProject(w, r, generation.ProjectID); !ok {
		return nil, false
	}
	return generation, true
}

func (h *VideoHandler) loadOwnedAsset(w http.ResponseWriter, r *http.Request, assetID string) (*models.VideoAsset, bool) {
	asset, err := h.assetRepo.GetByID(assetID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if asset == nil || asset.ProjectID == nil {
		respondError(w, http.StatusNotFound, "video asset not found")
		return nil, false
	}
	if _, ok := h.loadOwnedProject(w, r, *asset.ProjectID); !ok {
		return nil, false
	}
	return asset, true
}

func (h *VideoHandler) enrichGenerations(generations []models.VideoGeneration) []video.GenerationDetail {
	out := make([]video.GenerationDetail, 0, len(generations))
	for _, generation := range generations {
		out = append(out, h.enrichGeneration(generation))
	}
	return out
}

func (h *VideoHandler) enrichGeneration(generation models.VideoGeneration) video.GenerationDetail {
	detail := video.GenerationDetail{
		ID:                generation.ID,
		ProjectID:         generation.ProjectID,
		ParentID:          generation.ParentID,
		Status:            generation.Status,
		Provider:          generation.Provider,
		Model:             generation.Model,
		Prompt:            generation.Prompt,
		EnhancedPrompt:    generation.EnhancedPrompt,
		NegativePrompt:    generation.NegativePrompt,
		SettingsJSON:      generation.SettingsJSON,
		InputAssetIDsJSON: generation.InputAssetIDsJSON,
		OutputAssetID:     generation.OutputAssetID,
		CostUSD:           generation.CostUSD,
		Error:             generation.Error,
		CreatedAt:         generation.CreatedAt,
		CompletedAt:       generation.CompletedAt,
	}
	if generation.OutputAssetID != nil {
		if asset, err := h.assetRepo.GetByID(*generation.OutputAssetID); err == nil && asset != nil {
			detail.AssetURL = "/v1/video/assets/" + asset.ID + "/download"
			detail.MimeType = asset.MimeType
		}
	}
	return detail
}

func videoProjectIDOrEmpty(project *models.VideoProject) string {
	if project == nil {
		return ""
	}
	return project.ID
}

func videoGenerationIDOrEmpty(generation *models.VideoGeneration) string {
	if generation == nil {
		return ""
	}
	return generation.ID
}
