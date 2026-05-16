package music

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

var (
	ErrCapabilityUnsupported = errors.New("music capability unsupported")
	ErrProviderUnavailable   = errors.New("music provider unavailable")
)

type Service struct {
	sessions      *repository.MusicSessionRepo
	generations   *repository.MusicGenerationRepo
	assets        *repository.MusicAssetRepo
	providers     *repository.ProviderRepo
	settings      *repository.SettingsRepo
	llmSvc        *llm.Service
	storage       *Storage
	modelRegistry *ModelRegistry
}

func NewService(
	sessions *repository.MusicSessionRepo,
	generations *repository.MusicGenerationRepo,
	assets *repository.MusicAssetRepo,
	providers *repository.ProviderRepo,
	settings *repository.SettingsRepo,
	llmSvc *llm.Service,
	attachmentsDir string,
) *Service {
	return &Service{
		sessions:      sessions,
		generations:   generations,
		assets:        assets,
		providers:     providers,
		settings:      settings,
		llmSvc:        llmSvc,
		storage:       NewStorage(attachmentsDir),
		modelRegistry: NewModelRegistry(),
	}
}

func (s *Service) OutputDirectory() string {
	return s.storage.Root()
}

func (s *Service) ListProviders() (ProvidersResponse, error) {
	providers, err := s.providers.List()
	if err != nil {
		return ProvidersResponse{}, err
	}
	var out ProvidersResponse
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		switch strings.ToLower(p.Type) {
		case ProviderOpenRouter:
			out.OpenRouter = true
		case ProviderGemini:
			out.Gemini = true
		case ProviderElevenLabs:
			out.ElevenLabs = true
		}
	}
	return out, nil
}

func (s *Service) ListModels(ctx context.Context, provider string, refresh bool) ([]Model, error) {
	profile, err := s.resolveMusicProvider(provider)
	if err != nil {
		if errors.Is(err, ErrProviderUnavailable) {
			// Baseline models are still useful for a settings dropdown even when a key is missing.
			return cloneModels(KnownLyriaModels[normalizeProvider(provider)]), nil
		}
		return nil, err
	}
	baseURL, apiKey, err := s.providerAPI(profile)
	if err != nil {
		return cloneModels(KnownLyriaModels[profile.Type]), err
	}
	settings, _ := s.settings.GetTyped()
	return s.modelRegistry.List(ctx, profile.Type, baseURL, apiKey, settings.CustomGeminiLyriaModel, refresh)
}

func (s *Service) CreateSession(userID, title, provider, model string) (*models.MusicSession, error) {
	if title == "" {
		title = "Untitled Track Session"
	}
	return s.sessions.Create(userID, title, provider, model)
}

func (s *Service) Generate(ctx context.Context, userID string, req GenerateRequest, progress func(GenerationProgress)) (*models.MusicSession, *models.MusicGeneration, *models.MusicAsset, error) {
	providerKey := normalizeProvider(req.Provider)
	if providerKey == "" {
		return nil, nil, nil, fmt.Errorf("%w: provider must be openrouter, gemini, or elevenlabs", ErrCapabilityUnsupported)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, nil, nil, fmt.Errorf("prompt is required")
	}
	profile, err := s.resolveMusicProvider(providerKey)
	if err != nil {
		return nil, nil, nil, err
	}

	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = DefaultModel(providerKey)
	}
	availableModels, modelErr := s.ListModels(ctx, providerKey, false)
	if modelErr != nil {
		availableModels = cloneModels(KnownLyriaModels[providerKey])
	}
	if !ValidateModel(providerKey, modelID, availableModels) {
		return nil, nil, nil, fmt.Errorf("%w: %s is not a supported Lyria model for %s", ErrCapabilityUnsupported, modelID, providerKey)
	}

	session, err := s.ensureSession(userID, req, providerKey, modelID)
	if err != nil {
		return nil, nil, nil, err
	}

	assembledPrompt := AssemblePrompt(req)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = DeriveTitle(req.Prompt)
	}
	var parentID *string
	if strings.TrimSpace(req.ParentID) != "" {
		parentID = &req.ParentID
	}
	generation := &models.MusicGeneration{
		SessionID:       session.ID,
		ParentID:        parentID,
		Title:           title,
		Status:          "pending",
		Provider:        providerKey,
		Model:           modelID,
		Prompt:          strings.TrimSpace(req.Prompt),
		AssembledPrompt: assembledPrompt,
		InputChars:      len([]rune(assembledPrompt)),
		MetadataJSON:    "{}",
	}
	if err := s.generations.Create(generation); err != nil {
		return nil, nil, nil, err
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "started", Message: "Music generation started", SessionID: session.ID, GenerationID: generation.ID})
	}
	_ = s.generations.MarkRunning(generation.ID)

	llmOptions := llm.MusicOptions{
		Seed:        req.Options.Seed,
		Temperature: req.Options.Temperature,
	}
	if ms := parseDurationToMS(req.Options.Duration); ms != nil {
		llmOptions.DurationMS = ms
	}
	if req.Instrumental || strings.EqualFold(strings.TrimSpace(req.VocalMode), "instrumental") {
		instr := true
		llmOptions.ForceInstrumental = &instr
	}
	result, err := s.llmSvc.GenerateMusic(ctx, llm.MusicRequest{
		Provider: profile.ID,
		Model:    modelID,
		Prompt:   assembledPrompt,
		Options:  llmOptions,
	})
	if err != nil {
		msg := err.Error()
		_ = s.generations.MarkFailed(generation.ID, msg)
		return session, generation, nil, err
	}
	if len(result.AudioBytes) == 0 {
		msg := "upstream returned no audio"
		_ = s.generations.MarkFailed(generation.ID, msg)
		return session, generation, nil, errors.New(msg)
	}

	relativePath, fileName, err := s.storage.Write(session.ID, generation.ID, result.MimeType, result.AudioBytes)
	if err != nil {
		_ = s.generations.MarkFailed(generation.ID, err.Error())
		return session, generation, nil, err
	}

	meta := result.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	meta["provider_profile_id"] = profile.ID
	meta["output_directory"] = s.storage.Root()
	metaJSONBytes, _ := json.Marshal(meta)
	metaJSON := string(metaJSONBytes)
	usageJSON := string(result.UsageJSON)
	var usagePtr *string
	if usageJSON != "" && usageJSON != "null" {
		usagePtr = &usageJSON
	}
	var reqIDPtr *string
	if strings.TrimSpace(result.UpstreamReqID) != "" {
		reqIDPtr = &result.UpstreamReqID
	}
	if result.MimeType == "" {
		result.MimeType = "audio/mpeg"
	}
	asset := &models.MusicAsset{
		SessionID:    session.ID,
		GenerationID: generation.ID,
		Kind:         "music",
		FileName:     fileName,
		FilePath:     relativePath,
		MimeType:     result.MimeType,
		SizeBytes:    int64(len(result.AudioBytes)),
		Provider:     providerKey,
		Model:        modelID,
		MetadataJSON: metaJSON,
	}
	if err := s.assets.Create(asset); err != nil {
		_ = s.generations.MarkFailed(generation.ID, err.Error())
		return session, generation, nil, err
	}
	if err := s.generations.MarkCompleted(generation.ID, repository.MusicGenerationCompletion{
		Lyrics:        result.Lyrics,
		Structure:     result.Structure,
		UpstreamReqID: reqIDPtr,
		UsageJSON:     usagePtr,
		CostUSD:       result.CostUSD,
		OutputBytes:   int64(len(result.AudioBytes)),
		MetadataJSON:  metaJSON,
	}); err != nil {
		return session, generation, asset, err
	}
	_ = s.sessions.UpdateActiveGeneration(session.ID, generation.ID)
	generation, _ = s.generations.GetByID(generation.ID)
	if progress != nil {
		progress(GenerationProgress{Stage: "done", Message: "Music generation complete", SessionID: session.ID, GenerationID: generation.ID})
	}
	return session, generation, asset, nil
}

func (s *Service) ensureSession(userID string, req GenerateRequest, provider, model string) (*models.MusicSession, error) {
	if strings.TrimSpace(req.SessionID) != "" {
		session, err := s.sessions.GetByID(req.SessionID)
		if err != nil {
			return nil, err
		}
		if session == nil {
			return nil, fmt.Errorf("music session not found")
		}
		ok, err := s.sessions.BelongsToUser(session.ID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("music session not found")
		}
		_, _ = s.sessions.Update(session.ID, "", provider, model)
		return session, nil
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = DeriveTitle(req.Prompt)
	}
	return s.sessions.Create(userID, title, provider, model)
}

func (s *Service) resolveMusicProvider(provider string) (*models.ProviderProfile, error) {
	provider = normalizeProvider(provider)
	if provider == "" {
		return nil, fmt.Errorf("%w: provider must be openrouter, gemini, or elevenlabs", ErrCapabilityUnsupported)
	}
	profiles, err := s.providers.List()
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].Enabled && strings.EqualFold(profiles[i].Type, provider) {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("%w: no enabled %s provider profile", ErrProviderUnavailable, provider)
}

func (s *Service) providerAPI(profile *models.ProviderProfile) (string, string, error) {
	key, err := s.providers.GetAPIKey(profile.ID)
	if err != nil {
		return "", "", err
	}
	baseURL := ""
	if profile.BaseURL != nil {
		baseURL = strings.TrimSpace(*profile.BaseURL)
	}
	switch strings.ToLower(profile.Type) {
	case ProviderOpenRouter:
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
	case ProviderGemini:
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		}
	case ProviderElevenLabs:
		if baseURL == "" {
			baseURL = "https://api.elevenlabs.io"
		}
	}
	return baseURL, key, nil
}
