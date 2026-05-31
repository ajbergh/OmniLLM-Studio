package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrCapabilityUnsupported = errors.New("video capability unsupported")
	ErrProviderUnavailable   = errors.New("video provider unavailable")
)

type Service struct {
	projects         *repository.VideoProjectRepo
	generations      *repository.VideoGenerationRepo
	assets           *repository.VideoAssetRepo
	timelines        *repository.VideoTimelineRepo
	renderJobs       *repository.VideoRenderJobRepo
	providerProfiles *repository.ProviderRepo
	storage          *Storage
	registry         *ModelRegistry
	renderer         Renderer
}

func NewService(
	projects *repository.VideoProjectRepo,
	generations *repository.VideoGenerationRepo,
	assets *repository.VideoAssetRepo,
	timelines *repository.VideoTimelineRepo,
	renderJobs *repository.VideoRenderJobRepo,
	providerProfiles *repository.ProviderRepo,
	attachmentsDir string,
) *Service {
	return &Service{
		projects:         projects,
		generations:      generations,
		assets:           assets,
		timelines:        timelines,
		renderJobs:       renderJobs,
		providerProfiles: providerProfiles,
		storage:          NewStorage(attachmentsDir),
		registry:         NewModelRegistry(NewMockProvider(), NewOpenRouterProvider("", ""), NewGeminiProvider("", "")),
		renderer:         NewMockRenderer(),
	}
}

func (s *Service) OutputDirectory() string {
	return s.storage.Root()
}

func (s *Service) providerRegistry() *ModelRegistry {
	if s.providerProfiles == nil {
		return s.registry
	}
	var openRouterBaseURL, openRouterAPIKey string
	var geminiBaseURL, geminiAPIKey string
	profiles, err := s.providerProfiles.List()
	if err == nil {
		for i := range profiles {
			profile := profiles[i]
			if !profile.Enabled {
				continue
			}
			providerType := NormalizeProvider(profile.Type)
			if providerType != ProviderOpenRouter && providerType != ProviderGemini {
				continue
			}
			apiKey, keyErr := s.providerProfiles.GetAPIKey(profile.ID)
			if keyErr != nil {
				apiKey = ""
			}
			baseURL := ""
			if profile.BaseURL != nil {
				baseURL = strings.TrimSpace(*profile.BaseURL)
			}
			switch providerType {
			case ProviderOpenRouter:
				if openRouterAPIKey == "" {
					openRouterBaseURL = baseURL
					openRouterAPIKey = strings.TrimSpace(apiKey)
				}
			case ProviderGemini:
				if geminiAPIKey == "" {
					geminiBaseURL = baseURL
					geminiAPIKey = strings.TrimSpace(apiKey)
				}
			}
		}
	}
	return NewModelRegistry(
		NewMockProvider(),
		NewOpenRouterProvider(openRouterBaseURL, openRouterAPIKey),
		NewGeminiProvider(geminiBaseURL, geminiAPIKey),
	)
}

func (s *Service) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	return s.providerRegistry().ListProviders(ctx)
}

func (s *Service) ListModels(ctx context.Context, provider string, refresh bool) ([]Model, error) {
	_ = refresh
	provider = NormalizeProvider(provider)
	if provider == "" {
		return nil, fmt.Errorf("%w: unsupported provider", ErrCapabilityUnsupported)
	}
	return s.providerRegistry().ListModels(ctx, provider)
}

func (s *Service) CreateProject(userID, title, provider, model string, width, height, fps int, aspectRatio string) (*models.VideoProject, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled Video Project"
	}
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = ProviderMock
	}
	registry := s.providerRegistry()
	if model == "" {
		model = registry.DefaultModel(context.Background(), provider)
	}
	if width <= 0 {
		width = DefaultProjectWidth
	}
	if height <= 0 {
		height = DefaultProjectHeight
	}
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	if strings.TrimSpace(aspectRatio) == "" {
		aspectRatio = DefaultAspectRatio
	}
	return s.projects.Create(userID, title, provider, model, width, height, fps, aspectRatio)
}

func (s *Service) Generate(ctx context.Context, userID string, req GenerateRequest, progress func(GenerationProgress)) (*models.VideoProject, *models.VideoGeneration, *models.VideoAsset, error) {
	providerKey := NormalizeProvider(req.Provider)
	if providerKey == "" {
		return nil, nil, nil, fmt.Errorf("%w: unsupported video provider", ErrCapabilityUnsupported)
	}
	registry := s.providerRegistry()
	provider, ok := registry.Provider(providerKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: no adapter registered for %s", ErrProviderUnavailable, providerKey)
	}
	if !provider.Configured() {
		return nil, nil, nil, fmt.Errorf("%w: no enabled %s provider profile with an API key", ErrProviderUnavailable, providerKey)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, nil, nil, fmt.Errorf("prompt is required")
	}

	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = registry.DefaultModel(ctx, providerKey)
	}
	if modelID == "" || !registry.ValidateModel(ctx, providerKey, modelID) {
		return nil, nil, nil, fmt.Errorf("%w: %s is not supported by %s", ErrCapabilityUnsupported, modelID, providerKey)
	}

	project, err := s.ensureProject(ctx, userID, req, providerKey, modelID)
	if err != nil {
		return nil, nil, nil, err
	}

	enhancedPrompt := strings.TrimSpace(req.EnhancedPrompt)
	if enhancedPrompt == "" && req.Enhance {
		enhancedPrompt = EnhancePrompt(EnhancePromptRequest{
			Prompt:          req.Prompt,
			AspectRatio:     req.AspectRatio,
			DurationSeconds: req.DurationSeconds,
			NegativePrompt:  req.NegativePrompt,
		})
	}
	var enhancedPtr *string
	if enhancedPrompt != "" {
		enhancedPtr = &enhancedPrompt
	}
	negativePrompt := strings.TrimSpace(req.NegativePrompt)
	var negativePtr *string
	if negativePrompt != "" {
		negativePtr = &negativePrompt
	}
	var parentID *string
	if strings.TrimSpace(req.ParentID) != "" {
		parentID = &req.ParentID
	}
	settingsJSON := buildSettingsJSON(req)
	inputIDsJSONBytes, _ := json.Marshal(req.ReferenceAssetIDs)
	inputIDsJSON := string(inputIDsJSONBytes)

	generation := &models.VideoGeneration{
		ProjectID:         project.ID,
		ParentID:          parentID,
		Status:            StatusPending,
		Provider:          providerKey,
		Model:             modelID,
		Prompt:            strings.TrimSpace(req.Prompt),
		EnhancedPrompt:    enhancedPtr,
		NegativePrompt:    negativePtr,
		SettingsJSON:      settingsJSON,
		InputAssetIDsJSON: inputIDsJSON,
	}
	if err := s.generations.Create(generation); err != nil {
		return project, nil, nil, err
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "started", Message: "Video generation started", ProjectID: project.ID, GenerationID: generation.ID})
	}
	_ = s.generations.MarkRunning(generation.ID)

	providerReq := req
	providerReq.Provider = providerKey
	providerReq.Model = modelID
	providerReq.ProjectID = project.ID
	providerReq.EnhancedPrompt = enhancedPrompt
	result, err := provider.Generate(ctx, providerReq, func(p GenerationProgress) {
		if progress == nil {
			return
		}
		p.ProjectID = project.ID
		p.GenerationID = generation.ID
		progress(p)
	})
	if err != nil {
		msg := err.Error()
		_ = s.generations.MarkFailed(generation.ID, msg)
		return project, generation, nil, err
	}
	if len(result.Data) == 0 {
		msg := "provider returned no video asset"
		_ = s.generations.MarkFailed(generation.ID, msg)
		return project, generation, nil, errors.New(msg)
	}
	if result.MimeType == "" {
		result.MimeType = "application/octet-stream"
	}
	relativePath, fileName, err := s.storage.Write(project.ID, generation.ID, result.FileName, result.MimeType, result.Data)
	if err != nil {
		_ = s.generations.MarkFailed(generation.ID, err.Error())
		return project, generation, nil, err
	}

	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["output_directory"] = s.storage.Root()
	metaJSONBytes, _ := json.Marshal(metadata)
	metaJSON := string(metaJSONBytes)
	projectRefID := project.ID
	asset := &models.VideoAsset{
		ProjectID:    &projectRefID,
		SourceType:   "generation",
		Kind:         "video",
		FileName:     fileName,
		FilePath:     relativePath,
		MimeType:     result.MimeType,
		SizeBytes:    int64(len(result.Data)),
		DurationMS:   result.DurationMS,
		Width:        result.Width,
		Height:       result.Height,
		FPS:          result.FPS,
		Provider:     &providerKey,
		Model:        &modelID,
		MetadataJSON: metaJSON,
	}
	if err := s.assets.Create(asset); err != nil {
		_ = s.generations.MarkFailed(generation.ID, err.Error())
		return project, generation, nil, err
	}

	var usagePtr *string
	if len(result.UsageJSON) > 0 && string(result.UsageJSON) != "null" {
		usage := string(result.UsageJSON)
		usagePtr = &usage
	}
	if err := s.generations.MarkCompleted(generation.ID, repository.VideoGenerationCompletion{
		OutputAssetID: asset.ID,
		UpstreamJobID: result.UpstreamJobID,
		UpstreamReqID: result.UpstreamReqID,
		UsageJSON:     usagePtr,
		CostUSD:       result.CostUSD,
	}); err != nil {
		return project, generation, asset, err
	}
	if refreshed, err := s.generations.GetByID(generation.ID); err == nil && refreshed != nil {
		generation = refreshed
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "done", Message: "Video generation complete", ProjectID: project.ID, GenerationID: generation.ID, Progress: 1})
	}
	if req.PlaceOnTimeline {
		_, _, _ = s.ImportAssetToTimeline(ctx, userID, project.ID, TimelineImportAssetRequest{AssetID: asset.ID})
	}
	return project, generation, asset, nil
}

func (s *Service) GetOrCreateTimeline(ctx context.Context, userID, projectID string) (*models.VideoTimeline, TimelineDocument, error) {
	project, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline, err := s.timelines.GetActiveByProject(project.ID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	fallback := NewEmptyTimeline(project.Width, project.Height, project.FPS)
	if timeline != nil {
		doc, err := TimelineFromJSON(timeline.TimelineJSON, fallback)
		if err != nil {
			return nil, TimelineDocument{}, err
		}
		return timeline, doc, nil
	}
	_ = ctx
	doc, err := ValidateTimelineDocument(fallback)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	raw, err := TimelineToJSON(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline = &models.VideoTimeline{
		ProjectID:    project.ID,
		Name:         "Main Timeline",
		Active:       true,
		TimelineJSON: raw,
		DurationMS:   doc.DurationMS,
	}
	if err := s.timelines.Create(timeline); err != nil {
		return nil, TimelineDocument{}, err
	}
	return timeline, doc, nil
}

func (s *Service) SaveTimeline(ctx context.Context, userID, projectID string, doc TimelineDocument) (*models.VideoTimeline, TimelineDocument, error) {
	project, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	doc, err = ValidateTimelineDocument(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	raw, err := TimelineToJSON(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline, _, err := s.GetOrCreateTimeline(ctx, userID, project.ID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline.TimelineJSON = raw
	timeline.DurationMS = doc.DurationMS
	timeline.Active = true
	if err := s.timelines.Save(timeline); err != nil {
		return nil, TimelineDocument{}, err
	}
	refreshed, err := s.timelines.GetByID(timeline.ID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if refreshed != nil {
		timeline = refreshed
	}
	return timeline, doc, nil
}

func (s *Service) ImportAssetToTimeline(ctx context.Context, userID, projectID string, req TimelineImportAssetRequest) (*models.VideoTimeline, TimelineDocument, error) {
	if strings.TrimSpace(req.AssetID) == "" {
		return nil, TimelineDocument{}, fmt.Errorf("asset_id is required")
	}
	project, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	asset, err := s.assets.GetByID(req.AssetID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if asset == nil || asset.ProjectID == nil || *asset.ProjectID != project.ID {
		return nil, TimelineDocument{}, fmt.Errorf("video asset not found")
	}
	timeline, doc, err := s.GetOrCreateTimeline(ctx, userID, project.ID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	doc, _, err = AddAssetToTimeline(doc, *asset, req)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	raw, err := TimelineToJSON(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline.TimelineJSON = raw
	timeline.DurationMS = doc.DurationMS
	timeline.Active = true
	if err := s.timelines.Save(timeline); err != nil {
		return nil, TimelineDocument{}, err
	}
	refreshed, err := s.timelines.GetByID(timeline.ID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if refreshed != nil {
		timeline = refreshed
	}
	return timeline, doc, nil
}

func (s *Service) SendGenerationToTimeline(ctx context.Context, userID, generationID string) (*models.VideoTimeline, TimelineDocument, error) {
	generation, err := s.generations.GetByID(generationID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if generation == nil {
		return nil, TimelineDocument{}, fmt.Errorf("video generation not found")
	}
	if _, err := s.ensureProjectOwned(userID, generation.ProjectID); err != nil {
		return nil, TimelineDocument{}, err
	}
	if generation.OutputAssetID == nil || *generation.OutputAssetID == "" {
		return nil, TimelineDocument{}, fmt.Errorf("generation has no output asset")
	}
	return s.ImportAssetToTimeline(ctx, userID, generation.ProjectID, TimelineImportAssetRequest{AssetID: *generation.OutputAssetID})
}

func (s *Service) ImportExternalAsset(ctx context.Context, userID, projectID string, req ExternalAssetImportRequest) (*models.VideoAsset, error) {
	project, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, err
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if trackTypeForAssetKind(kind) == "" && kind != "export" && kind != "other" {
		return nil, fmt.Errorf("unsupported media type")
	}
	sourceStudio := strings.TrimSpace(req.SourceStudio)
	if sourceStudio == "" {
		sourceStudio = "file_library"
	}
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = "import"
	}
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = sourceStudio + "-" + sourceID + extensionForMimeType(req.MimeType)
	}
	mimeType := strings.TrimSpace(req.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	importID := "import-" + uuid.New().String()
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["source_studio"] = sourceStudio
	metadata["source_id"] = sourceID
	payload, _ := json.MarshalIndent(metadata, "", "  ")
	relativePath, storedName, err := s.storage.Write(project.ID, importID, fileName+".ref.txt", "text/plain", payload)
	if err != nil {
		return nil, err
	}
	metaJSONBytes, _ := json.Marshal(metadata)
	metaJSON := string(metaJSONBytes)
	projectRefID := project.ID
	asset := &models.VideoAsset{
		ProjectID:    &projectRefID,
		SourceType:   sourceType,
		SourceStudio: &sourceStudio,
		SourceID:     &sourceID,
		Kind:         kind,
		FileName:     storedName,
		FilePath:     relativePath,
		MimeType:     "text/plain",
		SizeBytes:    int64(len(payload)),
		DurationMS:   req.DurationMS,
		Width:        req.Width,
		Height:       req.Height,
		FPS:          req.FPS,
		MetadataJSON: metaJSON,
	}
	if asset.SizeBytes == 0 {
		asset.SizeBytes = req.SizeBytes
	}
	if err := s.assets.Create(asset); err != nil {
		return nil, err
	}
	_ = ctx
	return asset, nil
}

func (s *Service) StartRender(ctx context.Context, userID, projectID string, settings ExportSettings) (*models.VideoRenderJob, error) {
	project, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, err
	}
	settings, err = validateExportSettings(settings, *project)
	if err != nil {
		return nil, err
	}
	timeline, _, err := s.GetOrCreateTimeline(ctx, userID, project.ID)
	if err != nil {
		return nil, err
	}
	settingsJSON, _ := json.Marshal(settings)
	job := &models.VideoRenderJob{
		ProjectID:    project.ID,
		TimelineID:   timeline.ID,
		Status:       "queued",
		Progress:     0,
		SettingsJSON: string(settingsJSON),
	}
	if err := s.renderJobs.Create(job); err != nil {
		return nil, err
	}
	go s.runRenderJob(context.Background(), job.ID)
	return job, nil
}

func (s *Service) GetRenderJob(userID, jobID string) (*models.VideoRenderJob, error) {
	job, err := s.renderJobs.GetByID(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("render job not found")
	}
	if _, err := s.ensureProjectOwned(userID, job.ProjectID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Service) CancelRenderJob(userID, jobID string) (*models.VideoRenderJob, error) {
	job, err := s.GetRenderJob(userID, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		return job, nil
	}
	if err := s.renderJobs.MarkCancelled(job.ID); err != nil {
		return nil, err
	}
	return s.renderJobs.GetByID(job.ID)
}

func (s *Service) runRenderJob(ctx context.Context, jobID string) {
	job, err := s.renderJobs.GetByID(jobID)
	if err != nil || job == nil {
		return
	}
	if err := s.renderJobs.MarkRunning(job.ID); err != nil {
		return
	}
	project, err := s.projects.GetByID(job.ProjectID)
	if err != nil || project == nil {
		_ = s.renderJobs.MarkFailed(job.ID, "video project not found")
		return
	}
	timeline, err := s.timelines.GetByID(job.TimelineID)
	if err != nil || timeline == nil {
		_ = s.renderJobs.MarkFailed(job.ID, "timeline not found")
		return
	}
	doc, err := TimelineFromJSON(timeline.TimelineJSON, NewEmptyTimeline(project.Width, project.Height, project.FPS))
	if err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	var settings ExportSettings
	_ = json.Unmarshal([]byte(job.SettingsJSON), &settings)
	settings, _ = validateExportSettings(settings, *project)
	result, err := s.renderer.Render(ctx, RenderRequest{Project: *project, Timeline: doc, Settings: settings}, func(p RenderProgress) {
		_ = s.renderJobs.UpdateProgress(job.ID, p.Progress)
	})
	if err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	relativePath, fileName, err := s.storage.Write(project.ID, job.ID, result.FileName, result.MimeType, result.Data)
	if err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	meta := result.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	meta["render_job_id"] = job.ID
	meta["timeline_id"] = timeline.ID
	metaJSONBytes, _ := json.Marshal(meta)
	metaJSON := string(metaJSONBytes)
	projectID := project.ID
	asset := &models.VideoAsset{
		ProjectID:    &projectID,
		SourceType:   "render",
		SourceID:     &job.ID,
		Kind:         "export",
		FileName:     fileName,
		FilePath:     relativePath,
		MimeType:     result.MimeType,
		SizeBytes:    int64(len(result.Data)),
		DurationMS:   &result.DurationMS,
		Width:        &result.Width,
		Height:       &result.Height,
		FPS:          &result.FPS,
		MetadataJSON: metaJSON,
	}
	if err := s.assets.Create(asset); err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	_ = s.renderJobs.MarkCompleted(job.ID, asset.ID)
}

func (s *Service) ensureProject(ctx context.Context, userID string, req GenerateRequest, provider, model string) (*models.VideoProject, error) {
	_ = ctx
	if strings.TrimSpace(req.ProjectID) != "" {
		project, err := s.projects.GetByID(req.ProjectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, fmt.Errorf("video project not found")
		}
		ok, err := s.projects.BelongsToUser(project.ID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("video project not found")
		}
		if _, err := s.projects.Update(project.ID, "", provider, model, 0, 0, 0, ""); err != nil {
			return nil, err
		}
		return project, nil
	}
	width, height := dimensionsForResolution(req.Resolution, req.AspectRatio)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = DeriveTitle(req.Prompt)
	}
	return s.CreateProject(userID, title, provider, model, width, height, defaultInt(req.FPS, DefaultProjectFPS), req.AspectRatio)
}

func (s *Service) ensureProjectOwned(userID, projectID string) (*models.VideoProject, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	project, err := s.projects.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("video project not found")
	}
	ok, err := s.projects.BelongsToUser(project.ID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("video project not found")
	}
	return project, nil
}

func validateExportSettings(settings ExportSettings, project models.VideoProject) (ExportSettings, error) {
	settings.Format = strings.ToLower(strings.TrimSpace(settings.Format))
	if settings.Format == "" {
		settings.Format = "mp4"
	}
	if settings.Format != "mp4" && settings.Format != "webm" {
		return ExportSettings{}, fmt.Errorf("unsupported export format")
	}
	settings.Codec = strings.ToLower(strings.TrimSpace(settings.Codec))
	settings.Resolution = strings.ToLower(strings.TrimSpace(settings.Resolution))
	if settings.Resolution == "" {
		settings.Resolution = "project"
	}
	switch settings.Resolution {
	case "project", "720p", "1080p":
	default:
		return ExportSettings{}, fmt.Errorf("unsupported export resolution")
	}
	if settings.FPS <= 0 {
		settings.FPS = project.FPS
	}
	if settings.FPS <= 0 {
		settings.FPS = DefaultProjectFPS
	}
	if settings.FPS > 120 {
		return ExportSettings{}, fmt.Errorf("fps must be 120 or lower")
	}
	settings.Quality = strings.ToLower(strings.TrimSpace(settings.Quality))
	if settings.Quality == "" {
		settings.Quality = "standard"
	}
	switch settings.Quality {
	case "draft", "standard", "high":
	default:
		return ExportSettings{}, fmt.Errorf("unsupported export quality")
	}
	return settings, nil
}

func buildSettingsJSON(req GenerateRequest) string {
	if len(req.Settings) > 0 && string(req.Settings) != "null" {
		return string(req.Settings)
	}
	settings := map[string]any{
		"aspect_ratio":      req.AspectRatio,
		"duration_seconds":  req.DurationSeconds,
		"resolution":        req.Resolution,
		"fps":               req.FPS,
		"camera_motion":     req.CameraMotion,
		"shot_type":         req.ShotType,
		"style_preset":      req.StylePreset,
		"production_notes":  req.ProductionNotes,
		"place_on_timeline": req.PlaceOnTimeline,
	}
	if req.Seed != nil {
		settings["seed"] = *req.Seed
	}
	data, _ := json.Marshal(settings)
	return string(data)
}
