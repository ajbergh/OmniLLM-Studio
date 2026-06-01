package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/filelibrary"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type VideoHandler struct {
	service       *video.Service
	projectRepo   *repository.VideoProjectRepo
	genRepo       *repository.VideoGenerationRepo
	assetRepo     *repository.VideoAssetRepo
	timelineRepo  *repository.VideoTimelineRepo
	renderJobRepo *repository.VideoRenderJobRepo
	convoRepo     *repository.ConversationRepo
	attachRepo    *repository.AttachmentRepo
	fileSvc       *filelibrary.LibraryService
	storageDir    string
}

func NewVideoHandler(
	service *video.Service,
	projectRepo *repository.VideoProjectRepo,
	genRepo *repository.VideoGenerationRepo,
	assetRepo *repository.VideoAssetRepo,
	timelineRepo *repository.VideoTimelineRepo,
	renderJobRepo *repository.VideoRenderJobRepo,
	convoRepo *repository.ConversationRepo,
	attachRepo *repository.AttachmentRepo,
	fileSvc *filelibrary.LibraryService,
	storageDir string,
) *VideoHandler {
	return &VideoHandler{
		service:       service,
		projectRepo:   projectRepo,
		genRepo:       genRepo,
		assetRepo:     assetRepo,
		timelineRepo:  timelineRepo,
		renderJobRepo: renderJobRepo,
		convoRepo:     convoRepo,
		attachRepo:    attachRepo,
		fileSvc:       fileSvc,
		storageDir:    storageDir,
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

func (h *VideoHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	timeline, doc, err := h.service.GetOrCreateTimeline(r.Context(), auth.UserIDFromContext(r.Context()), project.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"timeline": timeline,
		"document": doc,
	})
}

func (h *VideoHandler) SaveTimeline(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var doc video.TimelineDocument
	if err := decodeJSON(r, &doc); err != nil {
		respondError(w, http.StatusBadRequest, "invalid timeline document")
		return
	}
	timeline, saved, err := h.service.SaveTimeline(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, doc)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"timeline": timeline,
		"document": saved,
	})
}

func (h *VideoHandler) ImportAssetToTimeline(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var req video.TimelineImportAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	timeline, doc, err := h.service.ImportAssetToTimeline(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"timeline": timeline,
		"document": doc,
	})
}

// Generate starts a video generation asynchronously and returns 202 with the
// generation record immediately.  The frontend polls GET /video/generations/{id}
// until status transitions to "completed" or "failed".
func (h *VideoHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req video.GenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	project, generation, err := h.service.GenerateAsync(r.Context(), auth.UserIDFromContext(r.Context()), req)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, video.ErrCapabilityUnsupported) || errors.Is(err, video.ErrProviderUnavailable) {
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"generation_id": generation.ID,
		"project_id":    project.ID,
		"status":        generation.Status,
		"generation":    h.enrichGeneration(*generation),
	})
}

// CancelGeneration cancels a pending or running generation.
func (h *VideoHandler) CancelGeneration(w http.ResponseWriter, r *http.Request) {
	generation, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	if err := h.service.CancelGeneration(generation.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"generation_id": generation.ID,
		"status":        "cancelled",
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
	timeline, doc, err := h.service.SendGenerationToTimeline(r.Context(), auth.UserIDFromContext(r.Context()), generation.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"project_id": generation.ProjectID,
		"asset_id":   generation.OutputAssetID,
		"timeline":   timeline,
		"document":   doc,
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

func (h *VideoHandler) ImportExternalAsset(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var req video.ExternalAssetImportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := h.service.ImportExternalAsset(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, asset)
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

// UploadAsset accepts a multipart file upload and creates a video asset in the
// project.  Intended for images used as first/last/reference frames.
func (h *VideoHandler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	const maxUploadSize = 50 << 20 // 50 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respondError(w, http.StatusBadRequest, "file too large (max 50 MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		if guessed := mime.TypeByExtension(filepath.Ext(header.Filename)); guessed != "" {
			mimeType = guessed
		} else {
			mimeType = "application/octet-stream"
		}
	}

	kind := "file"
	if strings.HasPrefix(mimeType, "image/") {
		kind = "image"
	} else if strings.HasPrefix(mimeType, "video/") {
		kind = "video"
	} else if strings.HasPrefix(mimeType, "audio/") {
		kind = "audio"
	}

	ext := filepath.Ext(header.Filename)
	storageFilename := uuid.New().String() + ext
	storagePath := filepath.Join(h.storageDir, storageFilename)

	if err := os.MkdirAll(h.storageDir, 0750); err != nil {
		respondInternalError(w, err)
		return
	}

	out, err := os.Create(storagePath)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		_ = os.Remove(storagePath)
		respondInternalError(w, err)
		return
	}

	asset := &models.VideoAsset{
		ProjectID:  &project.ID,
		SourceType: "upload",
		Kind:       kind,
		FileName:   header.Filename,
		FilePath:   storageFilename,
		MimeType:   mimeType,
		SizeBytes:  written,
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.assetRepo.Create(asset); err != nil {
		_ = os.Remove(storagePath)
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, asset)
}

func (h *VideoHandler) StartRender(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var settings video.ExportSettings
	if err := decodeJSON(r, &settings); err != nil {
		respondError(w, http.StatusBadRequest, "invalid render settings")
		return
	}
	job, err := h.service.StartRender(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, settings)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, job)
}

func (h *VideoHandler) GetRenderJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.GetRenderJob(auth.UserIDFromContext(r.Context()), chi.URLParam(r, "jobId"))
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, job)
}

func (h *VideoHandler) CancelRenderJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.CancelRenderJob(auth.UserIDFromContext(r.Context()), chi.URLParam(r, "jobId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, job)
}

func (h *VideoHandler) EnhancePrompt(w http.ResponseWriter, r *http.Request) {
	var req video.EnhancePromptRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enhanced := h.service.EnhancePrompt(r.Context(), req)
	if enhanced == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	respondJSON(w, http.StatusOK, video.EnhancePromptResponse{Prompt: enhanced})
}

func (h *VideoHandler) AssistantStoryboard(w http.ResponseWriter, r *http.Request) {
	project, req, ok := h.decodeAssistantRequest(w, r)
	if !ok {
		return
	}
	resp, err := h.service.CreateStoryboard(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *VideoHandler) AssistantTimelinePlan(w http.ResponseWriter, r *http.Request) {
	project, req, ok := h.decodeAssistantRequest(w, r)
	if !ok {
		return
	}
	resp, err := h.service.CreateTimelinePlan(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *VideoHandler) AssistantEditPlan(w http.ResponseWriter, r *http.Request) {
	project, req, ok := h.decodeAssistantRequest(w, r)
	if !ok {
		return
	}
	resp, err := h.service.CreateEditPlan(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *VideoHandler) AssistantApplyEditPlan(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	var plan video.EditPlan
	if err := decodeJSON(r, &plan); err != nil {
		respondError(w, http.StatusBadRequest, "invalid edit plan")
		return
	}
	timeline, doc, err := h.service.ApplyEditPlan(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, plan)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"timeline": timeline,
		"document": doc,
	})
}

func (h *VideoHandler) AssistantSocialVariants(w http.ResponseWriter, r *http.Request) {
	project, req, ok := h.decodeAssistantRequest(w, r)
	if !ok {
		return
	}
	resp, err := h.service.CreateSocialVariants(r.Context(), auth.UserIDFromContext(r.Context()), project.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *VideoHandler) decodeAssistantRequest(w http.ResponseWriter, r *http.Request) (*models.VideoProject, video.AssistantRequest, bool) {
	project, ok := h.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return nil, video.AssistantRequest{}, false
	}
	var req video.AssistantRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return nil, video.AssistantRequest{}, false
	}
	return project, req, true
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

// AttachToConversation copies a video asset into a conversation attachment.
// POST /v1/video/assets/{assetId}/attach-to-conversation
func (h *VideoHandler) AttachToConversation(w http.ResponseWriter, r *http.Request) {
	if h.attachRepo == nil || h.convoRepo == nil {
		respondError(w, http.StatusNotImplemented, "attach-to-conversation not configured")
		return
	}
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	var req struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConversationID == "" {
		respondError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, req.ConversationID) {
		return
	}

	srcPath, err := SafeJoin(h.storageDir, asset.FilePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid asset path")
		return
	}
	srcFile, err := os.Open(srcPath) // #nosec G304 — srcPath validated by SafeJoin
	if err != nil {
		respondError(w, http.StatusNotFound, "asset file not found on disk")
		return
	}
	defer srcFile.Close()

	newFileName := uuid.New().String() + filepath.Ext(asset.FileName)
	dstPath := filepath.Join(h.storageDir, newFileName)
	dstFile, err := os.Create(dstPath) // #nosec G304
	if err != nil {
		respondInternalError(w, err)
		return
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = os.Remove(dstPath)
		respondInternalError(w, err)
		return
	}
	_ = dstFile.Close()

	attachType := "file"
	if len(asset.MimeType) >= 6 && asset.MimeType[:6] == "image/" {
		attachType = "image"
	}
	attachment := &models.Attachment{
		ID:             uuid.New().String(),
		ConversationID: req.ConversationID,
		Type:           attachType,
		MimeType:       asset.MimeType,
		StoragePath:    newFileName,
		Bytes:          asset.SizeBytes,
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.attachRepo.Create(attachment); err != nil {
		_ = os.Remove(dstPath)
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, attachment)
}

// RegisterInLibrary ingests a video asset into the File Library at global scope.
// POST /v1/video/assets/{assetId}/register-in-library
func (h *VideoHandler) RegisterInLibrary(w http.ResponseWriter, r *http.Request) {
	if h.fileSvc == nil || h.attachRepo == nil {
		respondError(w, http.StatusNotImplemented, "file library not configured")
		return
	}
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}

	// First create a temporary attachment record so IngestFile can locate the file.
	attachID := uuid.New().String()
	newFileName := uuid.New().String() + filepath.Ext(asset.FileName)
	srcPath, err := SafeJoin(h.storageDir, asset.FilePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid asset path")
		return
	}
	srcBytes, err := os.ReadFile(srcPath) // #nosec G304
	if err != nil {
		respondError(w, http.StatusNotFound, "asset file not found on disk")
		return
	}
	dstPath := filepath.Join(h.storageDir, newFileName)
	if err := os.WriteFile(dstPath, srcBytes, 0o644); err != nil { // #nosec G306
		respondInternalError(w, err)
		return
	}

	userID := auth.UserIDFromContext(r.Context())

	// File Library requires an attachment tied to a conversation (schema constraint).
	// Create a staging conversation to host this attachment.
	stagingConvo, err := h.convoRepo.CreateWithKind(userID, "Video Library: "+asset.FileName, "video_library", nil, nil, nil)
	if err != nil {
		_ = os.Remove(dstPath)
		respondInternalError(w, err)
		return
	}

	attachType := "file"
	if len(asset.MimeType) >= 6 && asset.MimeType[:6] == "image/" {
		attachType = "image"
	}
	tempAttach := &models.Attachment{
		ID:             attachID,
		ConversationID: stagingConvo.ID,
		Type:           attachType,
		MimeType:       asset.MimeType,
		StoragePath:    newFileName,
		Bytes:          asset.SizeBytes,
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.attachRepo.Create(tempAttach); err != nil {
		_ = os.Remove(dstPath)
		respondInternalError(w, err)
		return
	}

	displayName := asset.FileName
	file, err := h.fileSvc.IngestFile(r.Context(), filelibrary.IngestFileRequest{
		OwnerUserID:  userID,
		AttachmentID: attachID,
		Scope:        "global",
		DisplayName:  displayName,
		Metadata: map[string]interface{}{
			"source":         "video_studio",
			"video_asset_id": asset.ID,
			"mime_type":      asset.MimeType,
		},
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, file)
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
