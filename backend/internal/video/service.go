package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

var (
	ErrCapabilityUnsupported = errors.New("video capability unsupported")
	ErrProviderUnavailable   = errors.New("video provider unavailable")
)

type Service struct {
	projects    *repository.VideoProjectRepo
	generations *repository.VideoGenerationRepo
	assets      *repository.VideoAssetRepo
	storage     *Storage
	registry    *ModelRegistry
}

func NewService(
	projects *repository.VideoProjectRepo,
	generations *repository.VideoGenerationRepo,
	assets *repository.VideoAssetRepo,
	attachmentsDir string,
) *Service {
	return &Service{
		projects:    projects,
		generations: generations,
		assets:      assets,
		storage:     NewStorage(attachmentsDir),
		registry:    NewModelRegistry(NewMockProvider()),
	}
}

func (s *Service) OutputDirectory() string {
	return s.storage.Root()
}

func (s *Service) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	return s.registry.ListProviders(ctx)
}

func (s *Service) ListModels(ctx context.Context, provider string, refresh bool) ([]Model, error) {
	_ = refresh
	provider = NormalizeProvider(provider)
	if provider == "" {
		return nil, fmt.Errorf("%w: unsupported provider", ErrCapabilityUnsupported)
	}
	return s.registry.ListModels(ctx, provider)
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
	if model == "" {
		model = s.registry.DefaultModel(context.Background(), provider)
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
	provider, ok := s.registry.Provider(providerKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: no adapter registered for %s", ErrProviderUnavailable, providerKey)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, nil, nil, fmt.Errorf("prompt is required")
	}

	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = s.registry.DefaultModel(ctx, providerKey)
	}
	if modelID == "" || !s.registry.ValidateModel(ctx, providerKey, modelID) {
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
	projectID := project.ID
	asset := &models.VideoAsset{
		ProjectID:    &projectID,
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
	return project, generation, asset, nil
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
