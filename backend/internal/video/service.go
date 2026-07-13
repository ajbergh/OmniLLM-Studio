package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrCapabilityUnsupported = errors.New("video capability unsupported")
	ErrProviderUnavailable   = errors.New("video provider unavailable")
)

// llmCompleter is a narrow interface for making non-streaming LLM completions.
// Using an interface keeps the video package decoupled from llm.Service internals.
type llmCompleter interface {
	ChatComplete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type Service struct {
	projects         *repository.VideoProjectRepo
	generations      *repository.VideoGenerationRepo
	assets           *repository.VideoAssetRepo
	timelines        *repository.VideoTimelineRepo
	renderJobs       *repository.VideoRenderJobRepo
	providerProfiles *repository.ProviderRepo
	libraryFiles     *repository.LibraryFileRepo
	musicSessions    *repository.MusicSessionRepo
	musicAssets      *repository.MusicAssetRepo
	imageAssets      *repository.ImageNodeAssetRepo
	attachments      *repository.AttachmentRepo
	conversations    *repository.ConversationRepo
	attachmentsDir   string
	storage          *Storage
	registry         *ModelRegistry
	renderer         Renderer
	llm              llmCompleter // optional — nil = deterministic fallback

	// renderCancels maps running render job IDs to their context cancel funcs
	// so CancelRenderJob can actually stop the FFmpeg process, not just flip
	// the DB status.
	renderCancelsMu sync.Mutex
	renderCancels   map[string]context.CancelFunc
}

func NewService(
	projects *repository.VideoProjectRepo,
	generations *repository.VideoGenerationRepo,
	assets *repository.VideoAssetRepo,
	timelines *repository.VideoTimelineRepo,
	renderJobs *repository.VideoRenderJobRepo,
	providerProfiles *repository.ProviderRepo,
	attachmentsDir string,
	llmSvc llmCompleter,
) *Service {
	return &Service{
		projects:         projects,
		generations:      generations,
		assets:           assets,
		timelines:        timelines,
		renderJobs:       renderJobs,
		providerProfiles: providerProfiles,
		attachmentsDir:   attachmentsDir,
		storage:          NewStorage(attachmentsDir),
		registry:         NewModelRegistry(NewOpenRouterProvider("", ""), NewGeminiProvider("", ""), NewLumaProvider("", "")),
		renderer:         NewFFmpegRenderer(""),
		llm:              llmSvc,
		renderCancels:    map[string]context.CancelFunc{},
	}
}

func (s *Service) ConfigureExternalAssetSources(
	libraryFiles *repository.LibraryFileRepo,
	musicSessions *repository.MusicSessionRepo,
	musicAssets *repository.MusicAssetRepo,
	imageAssets *repository.ImageNodeAssetRepo,
	attachments *repository.AttachmentRepo,
	conversations *repository.ConversationRepo,
	attachmentsDir string,
) *Service {
	s.libraryFiles = libraryFiles
	s.musicSessions = musicSessions
	s.musicAssets = musicAssets
	s.imageAssets = imageAssets
	s.attachments = attachments
	s.conversations = conversations
	if strings.TrimSpace(attachmentsDir) != "" {
		s.attachmentsDir = attachmentsDir
	}
	return s
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
	var lumaBaseURL, lumaAPIKey string
	profiles, err := s.providerProfiles.List()
	if err == nil {
		for i := range profiles {
			profile := profiles[i]
			if !profile.Enabled {
				continue
			}
			providerType := NormalizeProvider(profile.Type)
			if providerType != ProviderOpenRouter && providerType != ProviderGemini && providerType != ProviderLuma {
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
			case ProviderLuma:
				if lumaAPIKey == "" {
					lumaBaseURL = baseURL
					lumaAPIKey = strings.TrimSpace(apiKey)
				}
			}
		}
	}
	return NewModelRegistry(
		NewOpenRouterProvider(openRouterBaseURL, openRouterAPIKey),
		NewGeminiProvider(geminiBaseURL, geminiAPIKey),
		NewLumaProvider(lumaBaseURL, lumaAPIKey),
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

func (s *Service) ValidateGeneration(ctx context.Context, req GenerateRequest) GenerateValidationResult {
	return s.providerRegistry().ValidateGenerateRequest(ctx, req)
}

func (s *Service) CreateProject(userID, title, provider, model string, width, height, fps int, aspectRatio string) (*models.VideoProject, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled Video Project"
	}
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = s.defaultProviderKey(context.Background())
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

func (s *Service) defaultProviderKey(ctx context.Context) string {
	registry := s.providerRegistry()
	for _, key := range []string{ProviderOpenRouter, ProviderGemini, ProviderLuma} {
		if provider, ok := registry.Provider(key); ok && provider.Configured() {
			return key
		}
	}
	for _, key := range []string{ProviderOpenRouter, ProviderGemini, ProviderLuma} {
		if _, ok := registry.Provider(key); ok {
			return key
		}
	}
	providers, err := registry.ListProviders(ctx)
	if err != nil || len(providers) == 0 {
		return ""
	}
	return providers[0].Key
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
	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = registry.DefaultModel(ctx, providerKey)
	}
	req.Provider = providerKey
	req.Model = modelID
	validation := registry.ValidateGenerateRequest(ctx, req)
	if !validation.Valid {
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrCapabilityUnsupported, validation.ErrorMessage())
	}
	req = validation.NormalizedRequest
	modelID = req.Model

	project, err := s.ensureProject(ctx, userID, req, providerKey, modelID)
	if err != nil {
		return nil, nil, nil, err
	}

	enhancedPrompt := strings.TrimSpace(req.EnhancedPrompt)
	if enhancedPrompt == "" && req.Enhance {
		inputMode := "text-to-video"
		switch {
		case req.SourceVideoAssetID != "":
			inputMode = "extend"
		case req.LastFrameAssetID != "":
			inputMode = "first_last_frame"
		case req.StartImageAssetID != "":
			inputMode = "image-to-video"
		case len(req.ReferenceAssetIDs) > 0:
			inputMode = "reference-images"
		}
		enhancedPrompt = EnhancePromptWithLLM(ctx, s.llm, EnhancePromptRequest{
			Prompt:          req.Prompt,
			AspectRatio:     req.AspectRatio,
			DurationSeconds: req.DurationSeconds,
			NegativePrompt:  req.NegativePrompt,
			InputMode:       inputMode,
			ProductionNotes: assembleCinematicNotes(req),
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

	// Build structured input_assets_json with roles.
	var inputAssets []InputAsset
	if req.StartImageAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.StartImageAssetID, Role: RoleStartFrame})
	}
	if req.LastFrameAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.LastFrameAssetID, Role: RoleLastFrame})
	}
	if req.SourceVideoAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.SourceVideoAssetID, Role: RoleSourceVideo})
	}
	for _, id := range req.ReferenceAssetIDs {
		inputAssets = append(inputAssets, InputAsset{AssetID: id, Role: RoleReference})
	}
	if inputAssets == nil {
		inputAssets = []InputAsset{}
	}
	inputAssetsJSONBytes, _ := json.Marshal(inputAssets)
	inputAssetsJSON := string(inputAssetsJSONBytes)

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
		InputAssetsJSON:   inputAssetsJSON,
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

	// Resolve reference asset IDs → absolute file paths so providers don't
	// need to know about the storage layer.
	if len(providerReq.ReferenceAssetIDs) > 0 {
		paths := make([]string, 0, len(providerReq.ReferenceAssetIDs))
		for _, assetID := range providerReq.ReferenceAssetIDs {
			asset, err := s.assets.GetByID(assetID)
			if err != nil || asset == nil {
				continue
			}
			absPath := filepath.Join(s.attachmentsDir, filepath.FromSlash(asset.FilePath))
			if _, statErr := os.Stat(absPath); statErr == nil {
				paths = append(paths, absPath)
			}
		}
		providerReq.ReferenceAssetPaths = paths
	}

	// Resolve start frame, last frame, and source video asset IDs to paths.
	if providerReq.StartImageAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.StartImageAssetID); err == nil {
			providerReq.StartImagePath = p
		}
	}
	if providerReq.LastFrameAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.LastFrameAssetID); err == nil {
			providerReq.LastFramePath = p
		}
	}
	if providerReq.SourceVideoAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.SourceVideoAssetID); err == nil {
			providerReq.SourceVideoPath = p
		}
	}

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
	s.attachAssetArtifacts(ctx, asset)
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

// EnhancePrompt enhances a prompt using LLM if available, falling back to deterministic.
func (s *Service) EnhancePrompt(ctx context.Context, req EnhancePromptRequest) string {
	return EnhancePromptWithLLM(ctx, s.llm, req)
}

// DuplicateProject copies a project: same settings, every asset's file bytes
// copied into the new project's storage (artifacts regenerated), and the
// active timeline cloned with asset references remapped to the copies. Clip
// IDs are preserved so saved assistant plans still target them.
func (s *Service) DuplicateProject(ctx context.Context, userID, projectID string) (*models.VideoProject, error) {
	source, err := s.ensureProjectOwned(userID, projectID)
	if err != nil {
		return nil, err
	}
	provider := ""
	if source.DefaultProvider != nil {
		provider = *source.DefaultProvider
	}
	model := ""
	if source.DefaultModel != nil {
		model = *source.DefaultModel
	}
	copyProject, err := s.projects.Create(userID, source.Title+" copy", provider, model, source.Width, source.Height, source.FPS, source.AspectRatio)
	if err != nil {
		return nil, err
	}

	// Copy assets; assets whose files are missing on disk are skipped and
	// their clips keep a dangling reference (same as deleting the asset).
	assets, err := s.assets.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	idMap := map[string]string{}
	for i := range assets {
		src := assets[i]
		data, readErr := os.ReadFile(filepath.Join(s.attachmentsDir, filepath.FromSlash(src.FilePath)))
		if readErr != nil {
			continue
		}
		relPath, _, writeErr := s.storage.Write(copyProject.ID, "copy-"+src.ID, src.FileName, src.MimeType, data)
		if writeErr != nil {
			continue
		}
		projectRef := copyProject.ID
		dup := &models.VideoAsset{
			ProjectID:    &projectRef,
			SourceType:   src.SourceType,
			SourceStudio: src.SourceStudio,
			SourceID:     src.SourceID,
			Kind:         src.Kind,
			FileName:     src.FileName,
			FilePath:     relPath,
			MimeType:     src.MimeType,
			SizeBytes:    int64(len(data)),
			DurationMS:   src.DurationMS,
			Width:        src.Width,
			Height:       src.Height,
			FPS:          src.FPS,
			Provider:     src.Provider,
			Model:        src.Model,
			MetadataJSON: src.MetadataJSON,
		}
		s.attachAssetArtifacts(ctx, dup)
		if createErr := s.assets.Create(dup); createErr != nil {
			continue
		}
		idMap[src.ID] = dup.ID
	}

	_, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	for ti := range doc.Tracks {
		for ci := range doc.Tracks[ti].Clips {
			if mapped, ok := idMap[doc.Tracks[ti].Clips[ci].AssetID]; ok {
				doc.Tracks[ti].Clips[ci].AssetID = mapped
			}
		}
	}
	if _, _, err := s.SaveTimeline(ctx, userID, copyProject.ID, doc); err != nil {
		return nil, err
	}
	return copyProject, nil
}

// attachAssetArtifacts generates a thumbnail/waveform next to the asset file
// and fills the corresponding fields. Best-effort — silently skipped without
// FFmpeg.
func (s *Service) attachAssetArtifacts(ctx context.Context, asset *models.VideoAsset) {
	if asset == nil || asset.FilePath == "" {
		return
	}
	thumbRel, waveRel := GenerateAssetArtifacts(ctx, s.attachmentsDir, asset.FilePath, asset.MimeType)
	if thumbRel != "" {
		asset.ThumbnailPath = &thumbRel
	}
	if waveRel != "" {
		asset.WaveformPath = &waveRel
	}
}

// resolveAssetPath looks up a video asset by ID and returns its absolute file
// path under the attachments directory.
func (s *Service) resolveAssetPath(assetID string) (string, error) {
	asset, err := s.assets.GetByID(assetID)
	if err != nil {
		return "", err
	}
	if asset == nil {
		return "", fmt.Errorf("asset not found: %s", assetID)
	}
	absPath := filepath.Join(s.attachmentsDir, filepath.FromSlash(asset.FilePath))
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("asset file not found on disk: %s", absPath)
	}
	return absPath, nil
}

// GenerateAsync creates the generation record, submits the provider operation in
// the background, and returns immediately.  The caller should poll
// GET /video/generations/{id} until status is "completed" or "failed".
func (s *Service) GenerateAsync(ctx context.Context, userID string, req GenerateRequest) (*models.VideoProject, *models.VideoGeneration, error) {
	providerKey := NormalizeProvider(req.Provider)
	if providerKey == "" {
		return nil, nil, fmt.Errorf("%w: unsupported video provider", ErrCapabilityUnsupported)
	}
	registry := s.providerRegistry()
	provider, ok := registry.Provider(providerKey)
	if !ok {
		return nil, nil, fmt.Errorf("%w: no adapter registered for %s", ErrProviderUnavailable, providerKey)
	}
	if !provider.Configured() {
		return nil, nil, fmt.Errorf("%w: no enabled %s provider profile with an API key", ErrProviderUnavailable, providerKey)
	}

	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = registry.DefaultModel(ctx, providerKey)
	}
	req.Provider = providerKey
	req.Model = modelID
	validation := registry.ValidateGenerateRequest(ctx, req)
	if !validation.Valid {
		return nil, nil, fmt.Errorf("%w: %s", ErrCapabilityUnsupported, validation.ErrorMessage())
	}
	req = validation.NormalizedRequest
	modelID = req.Model

	project, err := s.ensureProject(ctx, userID, req, providerKey, modelID)
	if err != nil {
		return nil, nil, err
	}

	// Build enhanced prompt (use LLM if available).
	enhancedPrompt := strings.TrimSpace(req.EnhancedPrompt)
	if enhancedPrompt == "" && req.Enhance {
		inputMode := "text-to-video"
		switch {
		case req.SourceVideoAssetID != "":
			inputMode = "extend"
		case req.LastFrameAssetID != "":
			inputMode = "first_last_frame"
		case req.StartImageAssetID != "":
			inputMode = "image-to-video"
		case len(req.ReferenceAssetIDs) > 0:
			inputMode = "reference-images"
		}
		enhancedPrompt = EnhancePromptWithLLM(ctx, s.llm, EnhancePromptRequest{
			Prompt:          req.Prompt,
			AspectRatio:     req.AspectRatio,
			DurationSeconds: req.DurationSeconds,
			NegativePrompt:  req.NegativePrompt,
			InputMode:       inputMode,
			ProductionNotes: assembleCinematicNotes(req),
		})
	}

	var enhancedPtr, negativePtr, parentID *string
	if enhancedPrompt != "" {
		enhancedPtr = &enhancedPrompt
	}
	if neg := strings.TrimSpace(req.NegativePrompt); neg != "" {
		negativePtr = &neg
	}
	if strings.TrimSpace(req.ParentID) != "" {
		parentID = &req.ParentID
	}

	settingsJSON := buildSettingsJSON(req)
	inputIDsJSONBytes, _ := json.Marshal(req.ReferenceAssetIDs)

	var inputAssets []InputAsset
	if req.StartImageAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.StartImageAssetID, Role: RoleStartFrame})
	}
	if req.LastFrameAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.LastFrameAssetID, Role: RoleLastFrame})
	}
	if req.SourceVideoAssetID != "" {
		inputAssets = append(inputAssets, InputAsset{AssetID: req.SourceVideoAssetID, Role: RoleSourceVideo})
	}
	for _, id := range req.ReferenceAssetIDs {
		inputAssets = append(inputAssets, InputAsset{AssetID: id, Role: RoleReference})
	}
	if inputAssets == nil {
		inputAssets = []InputAsset{}
	}
	inputAssetsJSONBytes, _ := json.Marshal(inputAssets)

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
		InputAssetIDsJSON: string(inputIDsJSONBytes),
		InputAssetsJSON:   string(inputAssetsJSONBytes),
	}
	if err := s.generations.Create(generation); err != nil {
		return project, nil, err
	}

	// Start background generation goroutine.
	go s.runGenerationBackground(context.Background(), userID, project.ID, generation.ID, modelID, providerKey, req, enhancedPrompt)

	return project, generation, nil
}

// runGenerationBackground performs the provider submit + poll + download cycle
// in a background goroutine.  It never panics — all errors are written to the
// generation record.
func (s *Service) runGenerationBackground(
	ctx context.Context,
	userID, projectID, generationID, modelID, providerKey string,
	req GenerateRequest,
	enhancedPrompt string,
) {
	fail := func(msg string) {
		_ = s.generations.MarkFailed(generationID, msg)
	}

	registry := s.providerRegistry()
	prov, ok := registry.Provider(providerKey)
	if !ok {
		fail("provider not available: " + providerKey)
		return
	}
	gemProv, isGemini := prov.(*GeminiProvider)
	if !isGemini {
		// Fall back to synchronous Generate for non-Gemini providers, under
		// the requesting user (not ""). Generate creates and completes its own
		// generation record, so retire the pending async row instead of
		// leaving it stuck forever.
		_, _, _, err := s.Generate(ctx, userID, req, nil)
		if err != nil {
			fail(err.Error())
			return
		}
		_ = s.generations.MarkCancelled(generationID)
		return
	}

	_ = s.generations.MarkRunning(generationID)

	// Resolve paths.
	providerReq := req
	providerReq.EnhancedPrompt = enhancedPrompt
	providerReq.Provider = providerKey
	providerReq.Model = modelID
	providerReq.ProjectID = projectID
	if len(providerReq.ReferenceAssetIDs) > 0 {
		paths := make([]string, 0, len(providerReq.ReferenceAssetIDs))
		for _, id := range providerReq.ReferenceAssetIDs {
			if p, err := s.resolveAssetPath(id); err == nil {
				paths = append(paths, p)
			}
		}
		providerReq.ReferenceAssetPaths = paths
	}
	if providerReq.StartImageAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.StartImageAssetID); err == nil {
			providerReq.StartImagePath = p
		}
	}
	if providerReq.LastFrameAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.LastFrameAssetID); err == nil {
			providerReq.LastFramePath = p
		}
	}
	if providerReq.SourceVideoAssetID != "" {
		if p, err := s.resolveAssetPath(providerReq.SourceVideoAssetID); err == nil {
			providerReq.SourceVideoPath = p
		}
	}

	if isGeminiOmniModel(modelID) {
		if providerReq.ParentID != "" {
			parent, err := s.generations.GetByID(providerReq.ParentID)
			if err != nil || parent == nil {
				fail("previous Omni generation was not found")
				return
			}
			if parent.ProjectID != projectID || parent.Provider != ProviderGemini || !isGeminiOmniModel(parent.Model) {
				fail("previous interaction must be a Gemini Omni generation in this project")
				return
			}
			if parent.Status != StatusCompleted || parent.UpstreamJobID == nil || strings.TrimSpace(*parent.UpstreamJobID) == "" {
				fail("previous Gemini Omni interaction is not available for editing")
				return
			}
			providerReq.PreviousInteractionID = strings.TrimSpace(*parent.UpstreamJobID)
		}
		result, err := gemProv.GenerateOmni(ctx, providerReq, nil)
		if err != nil {
			fail(err.Error())
			return
		}
		if err := s.finalizeProviderGeneration(ctx, userID, projectID, generationID, modelID, providerKey, result, req); err != nil {
			fail(err.Error())
		}
		return
	}

	opName, err := gemProv.Submit(ctx, providerReq)
	if err != nil {
		fail(err.Error())
		return
	}
	_ = s.generations.SetUpstreamJobID(generationID, opName)

	s.pollAndFinalize(ctx, projectID, generationID, modelID, providerKey, gemProv, opName, req)
}

// finalizeProviderGeneration persists a provider result for adapters (such as
// Gemini Omni) that return completed bytes rather than a pollable operation.
func (s *Service) finalizeProviderGeneration(
	ctx context.Context,
	userID, projectID, generationID, modelID, providerKey string,
	result *GenerationResult,
	req GenerateRequest,
) error {
	if result == nil || len(result.Data) == 0 {
		return errors.New("provider returned no video asset")
	}
	if result.MimeType == "" {
		result.MimeType = "video/mp4"
	}
	if result.FileName == "" {
		result.FileName = "generated-video" + extensionForMimeType(result.MimeType)
	}
	relativePath, fileName, err := s.storage.Write(projectID, generationID, result.FileName, result.MimeType, result.Data)
	if err != nil {
		return err
	}
	if result.DurationMS == nil || result.Width == nil || result.Height == nil || result.FPS == nil {
		if probe, probeErr := ProbeMedia(ctx, filepath.Join(s.attachmentsDir, filepath.FromSlash(relativePath))); probeErr == nil && probe != nil {
			if result.DurationMS == nil && probe.DurationMS > 0 {
				result.DurationMS = &probe.DurationMS
			}
			if result.Width == nil && probe.Width > 0 {
				result.Width = &probe.Width
			}
			if result.Height == nil && probe.Height > 0 {
				result.Height = &probe.Height
			}
			if result.FPS == nil && probe.FPS > 0 {
				result.FPS = &probe.FPS
			}
		}
	}
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["output_directory"] = s.storage.Root()
	metaJSON, _ := json.Marshal(metadata)
	projectRef := projectID
	asset := &models.VideoAsset{
		ProjectID:    &projectRef,
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
		MetadataJSON: string(metaJSON),
	}
	s.attachAssetArtifacts(ctx, asset)
	if err := s.assets.Create(asset); err != nil {
		return err
	}
	var usage *string
	if len(result.UsageJSON) > 0 && string(result.UsageJSON) != "null" {
		value := string(result.UsageJSON)
		usage = &value
	}
	if err := s.generations.MarkCompleted(generationID, repository.VideoGenerationCompletion{
		OutputAssetID: asset.ID,
		UpstreamJobID: result.UpstreamJobID,
		UpstreamReqID: result.UpstreamReqID,
		UsageJSON:     usage,
		CostUSD:       result.CostUSD,
	}); err != nil {
		return err
	}
	if req.PlaceOnTimeline {
		_, _, _ = s.ImportAssetToTimeline(ctx, userID, projectID, TimelineImportAssetRequest{AssetID: asset.ID})
	}
	return nil
}

// pollAndFinalize polls a Gemini operation until done then stores the result.
func (s *Service) pollAndFinalize(
	ctx context.Context,
	projectID, generationID, modelID, providerKey string,
	gemProv *GeminiProvider,
	opName string,
	req GenerateRequest,
) {
	fail := func(msg string) {
		_ = s.generations.MarkFailed(generationID, msg)
	}

	const maxAttempts = 120
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				fail("generation cancelled")
				return
			case <-asyncPollTimer(10):
			}
		}
		// CancelGeneration flips the DB status (the poll context is
		// Background and never fires Done): stop polling and keep the
		// cancelled status rather than clobbering it later.
		if gen, err := s.generations.GetByID(generationID); err == nil && gen != nil && gen.Status == StatusCancelled {
			return
		}
		done, videoURI, _, err := gemProv.PollOnce(ctx, opName)
		if err != nil {
			fail(err.Error())
			return
		}
		if done {
			if videoURI == "" {
				fail("Gemini Veo completed without a video URI")
				return
			}
			data, mimeType, err := gemProv.DownloadVideo(ctx, videoURI)
			if err != nil {
				fail(err.Error())
				return
			}
			if mimeType == "" {
				mimeType = "video/mp4"
			}
			s.finalizeGeneration(projectID, generationID, modelID, providerKey, opName, mimeType, data, req)
			return
		}
	}
	fail("Gemini Veo operation timed out")
}

// asyncPollTimer returns a channel that fires after d seconds.
func asyncPollTimer(seconds int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		close(ch)
	}()
	return ch
}

// finalizeGeneration stores the downloaded video bytes as an asset and marks
// the generation completed.
func (s *Service) finalizeGeneration(
	projectID, generationID, modelID, providerKey, opName, mimeType string,
	data []byte,
	req GenerateRequest,
) {
	fail := func(msg string) {
		_ = s.generations.MarkFailed(generationID, msg)
	}

	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "720p"
	}
	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = DefaultAspectRatio
	}
	duration := defaultInt(req.DurationSeconds, 8)

	fileName := "gemini-" + sanitizePathSegment(req.Model) + extensionForMimeType(mimeType)
	relativePath, savedFileName, err := s.storage.Write(projectID, generationID, fileName, mimeType, data)
	if err != nil {
		fail(err.Error())
		return
	}

	width, height := dimensionsForResolution(resolution, aspectRatio)
	fps := float64(24)
	durationMS := int64(duration * 1000)

	metadata := map[string]any{
		"provider":       providerKey,
		"model":          modelID,
		"operation_name": opName,
		"api":            "gemini_predict_long_running",
	}
	metaJSONBytes, _ := json.Marshal(metadata)

	asset := &models.VideoAsset{
		ProjectID:    &projectID,
		SourceType:   "generation",
		Kind:         "video",
		FileName:     savedFileName,
		FilePath:     relativePath,
		MimeType:     mimeType,
		SizeBytes:    int64(len(data)),
		DurationMS:   &durationMS,
		Width:        &width,
		Height:       &height,
		FPS:          &fps,
		Provider:     &providerKey,
		Model:        &modelID,
		MetadataJSON: string(metaJSONBytes),
	}
	s.attachAssetArtifacts(context.Background(), asset)
	if err := s.assets.Create(asset); err != nil {
		fail(err.Error())
		return
	}

	jobStr := opName
	_ = s.generations.MarkCompleted(generationID, repository.VideoGenerationCompletion{
		OutputAssetID: asset.ID,
		UpstreamJobID: &jobStr,
	})

	if req.PlaceOnTimeline {
		_, _, _ = s.ImportAssetToTimeline(context.Background(), "", projectID, TimelineImportAssetRequest{AssetID: asset.ID})
	}
}

// CancelGeneration marks a generation as cancelled.  If the generation is
// still in pending/running state, it will not be able to proceed.
func (s *Service) CancelGeneration(generationID string) error {
	gen, err := s.generations.GetByID(generationID)
	if err != nil {
		return err
	}
	if gen == nil {
		return fmt.Errorf("generation not found: %s", generationID)
	}
	if gen.Status == StatusCompleted || gen.Status == StatusFailed || gen.Status == StatusCancelled {
		return nil // already terminal, no-op
	}
	return s.generations.MarkCancelled(generationID)
}

// RecoverPendingGenerations scans for running/pending generations that have an
// upstream_job_id and re-spawns their poll goroutines.  Call once at startup.
func (s *Service) RecoverPendingGenerations() {
	active, err := s.generations.ListActiveWithUpstreamJob()
	if err != nil || len(active) == 0 {
		return
	}
	for i := range active {
		gen := active[i]
		if gen.UpstreamJobID == nil || *gen.UpstreamJobID == "" {
			continue
		}
		opName := *gen.UpstreamJobID
		providerKey := gen.Provider
		modelID := gen.Model
		projectID := gen.ProjectID
		generationID := gen.ID

		// Rebuild a minimal request for finalize (duration/resolution from settings).
		req := GenerateRequest{
			Provider:        providerKey,
			Model:           modelID,
			DurationSeconds: 8,
			Resolution:      "720p",
		}
		registry := s.providerRegistry()
		prov, ok := registry.Provider(NormalizeProvider(providerKey))
		if !ok {
			continue
		}
		gemProv, isGemini := prov.(*GeminiProvider)
		if !isGemini {
			continue
		}
		go s.pollAndFinalize(context.Background(), projectID, generationID, modelID, providerKey, gemProv, opName, req)
	}
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
	sourceStudio := normalizeExternalSourceStudio(req.SourceStudio)
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

	resolved, err := s.resolveExternalAssetData(ctx, userID, sourceStudio, sourceID, req)
	if err != nil {
		return nil, err
	}
	kind := strings.ToLower(strings.TrimSpace(resolved.kind))
	if kind == "" {
		kind = kindForMimeType(resolved.mimeType)
	}
	if trackTypeForAssetKind(kind) == "" && kind != "export" && kind != "other" {
		return nil, fmt.Errorf("unsupported media type")
	}
	fileName := strings.TrimSpace(resolved.fileName)
	if fileName == "" {
		fileName = sourceStudio + "-" + sourceID + extensionForMimeType(resolved.mimeType)
	}
	mimeType := strings.TrimSpace(resolved.mimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	importID := "import-" + uuid.New().String()
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	for key, value := range resolved.metadata {
		if _, exists := metadata[key]; !exists {
			metadata[key] = value
		}
	}
	metadata["source_studio"] = sourceStudio
	metadata["source_id"] = sourceID
	relativePath, storedName, err := s.storage.Write(project.ID, importID, fileName, mimeType, resolved.data)
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
		MimeType:     mimeType,
		SizeBytes:    int64(len(resolved.data)),
		DurationMS:   resolved.durationMS,
		Width:        resolved.width,
		Height:       resolved.height,
		FPS:          resolved.fps,
		MetadataJSON: metaJSON,
	}
	if asset.SizeBytes == 0 {
		asset.SizeBytes = resolved.sizeBytes
	}
	s.attachAssetArtifacts(ctx, asset)
	if err := s.assets.Create(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

type externalAssetData struct {
	data       []byte
	fileName   string
	mimeType   string
	sizeBytes  int64
	kind       string
	durationMS *int64
	width      *int
	height     *int
	fps        *float64
	metadata   map[string]any
}

func (s *Service) resolveExternalAssetData(ctx context.Context, userID, sourceStudio, sourceID string, req ExternalAssetImportRequest) (externalAssetData, error) {
	_ = ctx
	switch sourceStudio {
	case "file_library":
		return s.resolveFileLibraryAsset(userID, sourceID, req)
	case "music":
		return s.resolveMusicAsset(userID, sourceID, req)
	case "image":
		return s.resolveImageAsset(userID, sourceID, req)
	case "attachment":
		return s.resolveAttachmentAsset(userID, sourceID, req, map[string]any{})
	default:
		return externalAssetData{}, fmt.Errorf("unsupported source_studio %q", sourceStudio)
	}
}

func (s *Service) resolveFileLibraryAsset(userID, sourceID string, req ExternalAssetImportRequest) (externalAssetData, error) {
	if s.libraryFiles == nil {
		return externalAssetData{}, fmt.Errorf("file library import is not configured")
	}
	file, err := s.libraryFiles.GetByID(sourceID)
	if err != nil {
		return externalAssetData{}, err
	}
	if file == nil || !libraryFileVisibleToUser(file.OwnerUserID, userID) || strings.EqualFold(file.Status, "deleted") {
		return externalAssetData{}, fmt.Errorf("file library asset not found")
	}
	if file.StoragePath == nil || strings.TrimSpace(*file.StoragePath) == "" {
		return externalAssetData{}, fmt.Errorf("file library asset has no stored file")
	}
	path, err := safeJoin(s.attachmentsDir, *file.StoragePath)
	if err != nil {
		return externalAssetData{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return externalAssetData{}, fmt.Errorf("read file library asset: %w", err)
	}
	mimeType := firstNonEmpty(req.MimeType, ptrValue(file.MimeType), mime.TypeByExtension(filepath.Ext(path)), "application/octet-stream")
	fileName := firstNonEmpty(req.FileName, ptrValue(file.OriginalFilename), file.DisplayName, filepath.Base(*file.StoragePath))
	kind := firstNonEmpty(req.Kind, kindForMimeType(mimeType))
	return externalAssetData{
		data:       data,
		fileName:   fileName,
		mimeType:   mimeType,
		sizeBytes:  firstPositiveInt64(file.SizeBytes, int64(len(data))),
		kind:       kind,
		durationMS: req.DurationMS,
		width:      req.Width,
		height:     req.Height,
		fps:        req.FPS,
		metadata: map[string]any{
			"library_scope":        file.Scope,
			"library_status":       file.Status,
			"library_display_name": file.DisplayName,
		},
	}, nil
}

func (s *Service) resolveMusicAsset(userID, sourceID string, req ExternalAssetImportRequest) (externalAssetData, error) {
	if s.musicAssets == nil || s.musicSessions == nil {
		return externalAssetData{}, fmt.Errorf("music studio import is not configured")
	}
	asset, err := s.musicAssets.GetByID(sourceID)
	if err != nil {
		return externalAssetData{}, err
	}
	if asset == nil {
		asset, err = s.musicAssets.GetByGeneration(sourceID)
		if err != nil {
			return externalAssetData{}, err
		}
	}
	if asset == nil {
		return externalAssetData{}, fmt.Errorf("music asset not found")
	}
	ok, err := s.musicSessions.BelongsToUser(asset.SessionID, userID)
	if err != nil {
		return externalAssetData{}, err
	}
	if !ok {
		return externalAssetData{}, fmt.Errorf("music asset not found")
	}
	path, err := safeJoin(s.attachmentsDir, asset.FilePath)
	if err != nil {
		return externalAssetData{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return externalAssetData{}, fmt.Errorf("read music asset: %w", err)
	}
	var durationMS *int64
	if asset.DurationMS > 0 {
		duration := asset.DurationMS
		durationMS = &duration
	}
	metadata := map[string]any{
		"music_session_id":    asset.SessionID,
		"music_generation_id": asset.GenerationID,
		"music_provider":      asset.Provider,
		"music_model":         asset.Model,
	}
	if strings.TrimSpace(asset.MetadataJSON) != "" && asset.MetadataJSON != "{}" {
		metadata["music_metadata_json"] = asset.MetadataJSON
	}
	return externalAssetData{
		data:       data,
		fileName:   firstNonEmpty(req.FileName, asset.FileName, filepath.Base(asset.FilePath)),
		mimeType:   firstNonEmpty(req.MimeType, asset.MimeType, mime.TypeByExtension(filepath.Ext(asset.FilePath)), "application/octet-stream"),
		sizeBytes:  firstPositiveInt64(asset.SizeBytes, int64(len(data))),
		kind:       firstNonEmpty(req.Kind, asset.Kind, "music"),
		durationMS: firstNonNilInt64(req.DurationMS, durationMS),
		width:      req.Width,
		height:     req.Height,
		fps:        req.FPS,
		metadata:   metadata,
	}, nil
}

func (s *Service) resolveImageAsset(userID, sourceID string, req ExternalAssetImportRequest) (externalAssetData, error) {
	if s.imageAssets == nil {
		return s.resolveAttachmentAsset(userID, sourceID, req, map[string]any{})
	}
	imageAsset, err := s.imageAssets.GetByID(sourceID)
	if err != nil {
		return externalAssetData{}, err
	}
	if imageAsset == nil {
		return s.resolveAttachmentAsset(userID, sourceID, req, map[string]any{})
	}
	metadata := map[string]any{
		"image_node_id":  imageAsset.NodeID,
		"variant_index":  imageAsset.VariantIndex,
		"image_asset_id": imageAsset.ID,
		"image_selected": imageAsset.IsSelected,
		"attachment_id":  imageAsset.AttachmentID,
	}
	return s.resolveAttachmentAsset(userID, imageAsset.AttachmentID, req, metadata)
}

func (s *Service) resolveAttachmentAsset(userID, sourceID string, req ExternalAssetImportRequest, metadata map[string]any) (externalAssetData, error) {
	if s.attachments == nil {
		return externalAssetData{}, fmt.Errorf("attachment import is not configured")
	}
	attachment, err := s.attachments.GetByID(sourceID)
	if err != nil {
		return externalAssetData{}, err
	}
	if attachment == nil {
		return externalAssetData{}, fmt.Errorf("attachment asset not found")
	}
	if userID != "" && s.conversations != nil {
		conversation, err := s.conversations.GetByIDForUser(attachment.ConversationID, userID)
		if err != nil {
			return externalAssetData{}, err
		}
		if conversation == nil {
			return externalAssetData{}, fmt.Errorf("attachment asset not found")
		}
	}
	path, err := safeJoin(s.attachmentsDir, attachment.StoragePath)
	if err != nil {
		return externalAssetData{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return externalAssetData{}, fmt.Errorf("read attachment asset: %w", err)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["attachment_id"] = attachment.ID
	metadata["attachment_conversation_id"] = attachment.ConversationID
	fileName := firstNonEmpty(req.FileName, attachmentOriginalName(attachment), filepath.Base(attachment.StoragePath))
	mimeType := firstNonEmpty(req.MimeType, attachment.MimeType, mime.TypeByExtension(filepath.Ext(path)), "application/octet-stream")
	return externalAssetData{
		data:       data,
		fileName:   fileName,
		mimeType:   mimeType,
		sizeBytes:  firstPositiveInt64(attachment.Bytes, int64(len(data))),
		kind:       firstNonEmpty(req.Kind, kindForMimeType(mimeType), "image"),
		durationMS: req.DurationMS,
		width:      firstNonNilInt(req.Width, attachment.Width),
		height:     firstNonNilInt(req.Height, attachment.Height),
		fps:        req.FPS,
		metadata:   metadata,
	}, nil
}

// StartRender validates the export settings against the project, persists a
// queued render job, and launches the render in a background goroutine that
// outlives the HTTP request. The returned job is immediately pollable.
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
	// A cancellable context lets CancelRenderJob kill the FFmpeg process; the
	// job still survives request lifetimes (parent is Background, not the
	// HTTP request context).
	renderCtx, cancel := context.WithCancel(context.Background())
	s.renderCancelsMu.Lock()
	s.renderCancels[job.ID] = cancel
	s.renderCancelsMu.Unlock()
	go func() {
		defer func() {
			s.renderCancelsMu.Lock()
			delete(s.renderCancels, job.ID)
			s.renderCancelsMu.Unlock()
			cancel()
		}()
		s.runRenderJob(renderCtx, job.ID)
	}()
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
	s.renderCancelsMu.Lock()
	cancel := s.renderCancels[job.ID]
	s.renderCancelsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return s.renderJobs.GetByID(job.ID)
}

// DeleteRenderJob removes a terminal render job record after an ownership
// check. Active jobs must be cancelled first; the job's output asset (if any)
// is an independent record and is not deleted.
func (s *Service) DeleteRenderJob(userID, jobID string) error {
	job, err := s.GetRenderJob(userID, jobID)
	if err != nil {
		return err
	}
	if job.Status == "queued" || job.Status == "running" {
		return fmt.Errorf("cancel the render job before deleting it")
	}
	return s.renderJobs.Delete(job.ID)
}

// runRenderJob executes one render job end-to-end: load the project/timeline/
// settings, invoke the renderer (progress updates stream into the job row),
// persist the output as an export asset, optionally write an SRT/VTT caption
// sidecar asset, and mark the job completed. Failures keep FFmpeg diagnostics
// in the job metadata; a cancelled job keeps its cancelled status rather than
// being overwritten as failed.
func (s *Service) runRenderJob(ctx context.Context, jobID string) {
	job, err := s.renderJobs.GetByID(jobID)
	if err != nil || job == nil {
		return
	}
	if err := s.renderJobs.MarkRunning(job.ID); err != nil {
		return
	}
	if s.renderJobCancelled(job.ID) {
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
	if err := json.Unmarshal([]byte(job.SettingsJSON), &settings); err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, "invalid render settings: "+err.Error())
		return
	}
	settings, _ = validateExportSettings(settings, *project)

	// Resolve asset map for media compositing.
	assetList, _ := s.assets.ListByProject(project.ID)
	assetMap := make(map[string]models.VideoAsset, len(assetList))
	for _, a := range assetList {
		assetMap[a.ID] = a
	}
	attachmentsDir := filepath.Dir(s.storage.Root())

	result, err := s.renderer.Render(ctx, RenderRequest{
		Project:        *project,
		Timeline:       doc,
		Settings:       settings,
		AttachmentsDir: attachmentsDir,
		Assets:         assetMap,
	}, func(p RenderProgress) {
		_ = s.renderJobs.UpdateProgress(job.ID, p.Progress)
	})
	if err != nil {
		// A cancelled job errors out of the renderer (killed FFmpeg) — keep
		// the cancelled status instead of overwriting it with failed.
		if ctx.Err() != nil || s.renderJobCancelled(job.ID) {
			return
		}
		var renderErr *RenderError
		if errors.As(err, &renderErr) {
			diag := map[string]any{
				"ffmpeg_command": renderErr.Command,
				"ffmpeg_stderr":  truncateForMetadata(renderErr.Stderr, 8192),
			}
			if diagJSON, marshalErr := json.Marshal(diag); marshalErr == nil {
				_ = s.renderJobs.SetMetadata(job.ID, string(diagJSON))
			}
		}
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	if s.renderJobCancelled(job.ID) {
		return
	}
	jobMeta := map[string]any{}
	for k, v := range result.Metadata {
		jobMeta[k] = v
	}
	jobMeta["output_duration_ms"] = result.DurationMS
	jobMeta["output_width"] = result.Width
	jobMeta["output_height"] = result.Height
	jobMeta["output_fps"] = result.FPS
	if settings.EstimatedDurationMS > 0 {
		jobMeta["estimated_duration_ms"] = settings.EstimatedDurationMS
		diff := result.DurationMS - settings.EstimatedDurationMS
		if diff < 0 {
			diff = -diff
		}
		jobMeta["duration_matches_estimate"] = diff <= 1500
	}
	if jobMetaJSON, marshalErr := json.Marshal(jobMeta); marshalErr == nil {
		_ = s.renderJobs.SetMetadata(job.ID, string(jobMetaJSON))
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
	s.attachAssetArtifacts(context.Background(), asset)
	if err := s.assets.Create(asset); err != nil {
		_ = s.renderJobs.MarkFailed(job.ID, err.Error())
		return
	}
	// Sidecar caption files ship as a sibling asset. Range exports slice the
	// document the same way the renderer did so cue times match the output.
	if format := settings.SidecarCaptions; format == "srt" || format == "vtt" {
		sidecarDoc := doc
		if settings.RangeEndMS > settings.RangeStartMS && settings.RangeStartMS >= 0 {
			sidecarDoc = SliceTimelineRange(doc, settings.RangeStartMS, settings.RangeEndMS)
		}
		content := SerializeCaptions(CaptionCuesFromTimeline(sidecarDoc), format)
		if content == "" {
			jobMeta["captions_sidecar"] = "skipped — no caption clips"
		} else {
			mime := "application/x-subrip"
			if format == "vtt" {
				mime = "text/vtt"
			}
			sidecarPath, sidecarName, sidecarErr := s.storage.Write(project.ID, job.ID+"-captions", "captions."+format, mime, []byte(content))
			if sidecarErr == nil {
				sidecar := &models.VideoAsset{
					ProjectID:    &projectID,
					SourceType:   "render",
					SourceID:     &job.ID,
					Kind:         "caption",
					FileName:     sidecarName,
					FilePath:     sidecarPath,
					MimeType:     mime,
					SizeBytes:    int64(len(content)),
					MetadataJSON: fmt.Sprintf(`{"render_job_id":%q,"sidecar_format":%q}`, job.ID, format),
				}
				if createErr := s.assets.Create(sidecar); createErr == nil {
					jobMeta["captions_sidecar_asset_id"] = sidecar.ID
				} else {
					jobMeta["captions_sidecar"] = "failed: " + createErr.Error()
				}
			} else {
				jobMeta["captions_sidecar"] = "failed: " + sidecarErr.Error()
			}
		}
		if jobMetaJSON, marshalErr := json.Marshal(jobMeta); marshalErr == nil {
			_ = s.renderJobs.SetMetadata(job.ID, string(jobMetaJSON))
		}
	}
	_ = s.renderJobs.MarkCompleted(job.ID, asset.ID)
}

func (s *Service) renderJobCancelled(jobID string) bool {
	job, err := s.renderJobs.GetByID(jobID)
	return err == nil && job != nil && job.Status == "cancelled"
}

// truncateForMetadata caps diagnostic strings persisted in job metadata.
func truncateForMetadata(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "… (truncated)"
}

// RendererCapabilities reports which timeline features the active renderer
// honors at export time. The frontend derives export-fidelity warnings from it.
func (s *Service) RendererCapabilities() RendererCapabilities {
	return FFmpegRendererCapabilities()
}

// RecoverInterruptedRenderJobs marks render jobs that were queued or running
// when the process exited as failed, so they don't appear stuck forever.
// Call once at startup.
func (s *Service) RecoverInterruptedRenderJobs() {
	jobs, err := s.renderJobs.ListActive()
	if err != nil {
		return
	}
	for _, job := range jobs {
		_ = s.renderJobs.MarkFailed(job.ID, "render interrupted by application restart — start a new export")
	}
}

// firstEnabledChatProvider returns the name of the first enabled LLM provider, or "".
func (s *Service) firstEnabledChatProvider() string {
	providers, err := s.providerProfiles.List()
	if err != nil {
		return ""
	}
	for _, p := range providers {
		if p.Enabled {
			return p.Name
		}
	}
	return ""
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

// validateExportSettings normalizes and bounds-checks user-supplied export
// settings (format/codec pairing, resolution, FPS, quality, caption sidecar
// format, export range ordering, audio bitrate) before a job is persisted, so
// invalid combinations fail the request instead of the render.
func validateExportSettings(settings ExportSettings, project models.VideoProject) (ExportSettings, error) {
	settings.Format = strings.ToLower(strings.TrimSpace(settings.Format))
	if settings.Format == "" {
		settings.Format = "mp4"
	}
	if settings.Format != "mp4" && settings.Format != "webm" {
		return ExportSettings{}, fmt.Errorf("unsupported export format")
	}
	settings.Codec = strings.ToLower(strings.TrimSpace(settings.Codec))
	settings.Preset = strings.ToLower(strings.TrimSpace(settings.Preset))
	settings.Resolution = strings.ToLower(strings.TrimSpace(settings.Resolution))
	if settings.Resolution == "" {
		settings.Resolution = "project"
	}
	switch settings.Resolution {
	case "project", "720p", "1080p":
	case "custom":
		if settings.Width <= 0 || settings.Height <= 0 {
			return ExportSettings{}, fmt.Errorf("custom export resolution requires width and height")
		}
	default:
		return ExportSettings{}, fmt.Errorf("unsupported export resolution")
	}
	if settings.Width != 0 || settings.Height != 0 {
		if settings.Width < 16 || settings.Height < 16 || settings.Width > 7680 || settings.Height > 7680 {
			return ExportSettings{}, fmt.Errorf("export width/height must be between 16 and 7680 pixels")
		}
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
	switch settings.Codec {
	case "", "h264", "vp9":
	case "h265":
		if settings.Format != "mp4" {
			return ExportSettings{}, fmt.Errorf("h265 requires the mp4 format")
		}
	default:
		return ExportSettings{}, fmt.Errorf("unsupported export codec")
	}
	settings.SidecarCaptions = strings.ToLower(strings.TrimSpace(settings.SidecarCaptions))
	switch settings.SidecarCaptions {
	case "", "srt", "vtt":
	default:
		return ExportSettings{}, fmt.Errorf("unsupported sidecar caption format")
	}
	if settings.RangeStartMS < 0 || settings.RangeEndMS < 0 {
		return ExportSettings{}, fmt.Errorf("export range must be non-negative")
	}
	if settings.RangeEndMS > 0 && settings.RangeEndMS <= settings.RangeStartMS {
		return ExportSettings{}, fmt.Errorf("export range end must be after its start")
	}
	if settings.AudioBitrateKbps != 0 && (settings.AudioBitrateKbps < 32 || settings.AudioBitrateKbps > 512) {
		return ExportSettings{}, fmt.Errorf("audio bitrate must be between 32 and 512 kbps")
	}
	return settings, nil
}

func buildSettingsJSON(req GenerateRequest) string {
	if len(req.Settings) > 0 && string(req.Settings) != "null" {
		return string(req.Settings)
	}
	settings := map[string]any{
		"generation_mode":   req.GenerationMode,
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

func libraryFileVisibleToUser(ownerUserID *string, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ownerUserID == nil || strings.TrimSpace(*ownerUserID) == ""
	}
	return ownerUserID != nil && *ownerUserID == userID
}

func normalizeExternalSourceStudio(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "", "file", "filelibrary", "file_library":
		return "file_library"
	case "music", "music_studio":
		return "music"
	case "image", "image_studio":
		return "image"
	case "attachment", "attachments":
		return "attachment"
	default:
		return value
	}
}

func kindForMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch {
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case mimeType == "application/pdf", mimeType == "application/json", mimeType == "application/markdown":
		return "text"
	default:
		return "other"
	}
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonNilInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonNilInt(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func attachmentOriginalName(attachment *models.Attachment) string {
	if attachment == nil || strings.TrimSpace(attachment.MetadataJSON) == "" {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(attachment.MetadataJSON), &metadata); err != nil {
		return ""
	}
	if value, ok := metadata["original_name"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
