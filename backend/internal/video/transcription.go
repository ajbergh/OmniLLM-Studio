package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// TranscriptionAPIVersion identifies the persisted request/result contract.
const TranscriptionAPIVersion = "2026-07-20"

// TranscriptionRequest is provider-neutral. Provider-specific clients may
// ignore optional capabilities they do not support, but must never fabricate
// timestamps, speakers, confidence, language, translation, cost, or retention.
type TranscriptionRequest struct {
	AssetID               string `json:"asset_id"`
	ProviderProfileID     string `json:"provider_profile_id"`
	Model                 string `json:"model,omitempty"`
	Language              string `json:"language,omitempty"`
	TranslateTo           string `json:"translate_to,omitempty"`
	Prompt                string `json:"prompt,omitempty"`
	Diarization           bool   `json:"diarization,omitempty"`
	WordTimestamps        bool   `json:"word_timestamps,omitempty"`
	AllowRemoteProcessing bool   `json:"allow_remote_processing"`
	RetainProviderData    bool   `json:"retain_provider_data,omitempty"`
}

type TranscriptionProviderResult struct {
	Text               string
	Language           string
	TranslatedLanguage string
	Segments           []models.VideoTranscriptSegment
	Metadata           map[string]any
	CostUSD            *float64
}

type TranscriptionProvider interface {
	Transcribe(context.Context, string, TranscriptionRequest) (TranscriptionProviderResult, error)
}

// VideoTranscriptionService orchestrates provider-neutral, durable STT jobs.
type VideoTranscriptionService struct {
	transcripts    *repository.VideoTranscriptionRepo
	providers      *repository.ProviderRepo
	projects       *repository.VideoProjectRepo
	assets         *repository.VideoAssetRepo
	attachmentsDir string
	client         *http.Client
}

func NewVideoTranscriptionService(
	transcripts *repository.VideoTranscriptionRepo,
	providers *repository.ProviderRepo,
	projects *repository.VideoProjectRepo,
	assets *repository.VideoAssetRepo,
	attachmentsDir string,
) *VideoTranscriptionService {
	return &VideoTranscriptionService{
		transcripts:    transcripts,
		providers:      providers,
		projects:       projects,
		assets:         assets,
		attachmentsDir: attachmentsDir,
		client:         &http.Client{Timeout: 45 * time.Minute},
	}
}

func (s *VideoTranscriptionService) Start(
	ctx context.Context,
	userID,
	projectID string,
	request TranscriptionRequest,
) (*models.VideoTranscript, error) {
	if !request.AllowRemoteProcessing {
		return nil, fmt.Errorf("remote transcription requires explicit allow_remote_processing consent")
	}
	request.AssetID = strings.TrimSpace(request.AssetID)
	request.ProviderProfileID = strings.TrimSpace(request.ProviderProfileID)
	request.Language = strings.TrimSpace(request.Language)
	request.TranslateTo = strings.ToLower(strings.TrimSpace(request.TranslateTo))
	if request.AssetID == "" || request.ProviderProfileID == "" {
		return nil, fmt.Errorf("asset_id and provider_profile_id are required")
	}
	if request.TranslateTo != "" && request.TranslateTo != "en" {
		return nil, fmt.Errorf("the configured OpenAI-compatible transcription contract currently supports translation to English only")
	}

	project, err := s.projects.GetByID(projectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("video project not found")
	}
	owned, err := s.projects.BelongsToUser(projectID, userID)
	if err != nil || !owned {
		return nil, fmt.Errorf("video project not found")
	}
	asset, err := s.assets.GetByID(request.AssetID)
	if err != nil || asset == nil || asset.ProjectID == nil || *asset.ProjectID != projectID {
		return nil, fmt.Errorf("video asset not found")
	}
	if !strings.HasPrefix(asset.MimeType, "audio/") && !strings.HasPrefix(asset.MimeType, "video/") {
		return nil, fmt.Errorf("only audio and video assets can be transcribed")
	}
	profile, err := s.providers.GetByID(request.ProviderProfileID)
	if err != nil || profile == nil || !profile.Enabled {
		return nil, fmt.Errorf("transcription provider profile is unavailable")
	}
	providerType := strings.ToLower(strings.TrimSpace(profile.Type))
	if providerType != "openai" && providerType != "custom" {
		return nil, fmt.Errorf("provider %q does not expose a supported transcription API", providerType)
	}
	model := strings.TrimSpace(request.Model)
	if model == "" && profile.DefaultModel != nil {
		model = strings.TrimSpace(*profile.DefaultModel)
	}
	if model == "" {
		model = "gpt-4o-mini-transcribe"
	}

	privacy, _ := json.Marshal(map[string]any{
		"remote_processing":          true,
		"retain_provider_data":       request.RetainProviderData,
		"requested_word_timestamps": request.WordTimestamps,
		"requested_diarization":      request.Diarization,
	})
	metadata, _ := json.Marshal(map[string]any{
		"api_version":  TranscriptionAPIVersion,
		"request_model": model,
		"translate_to": request.TranslateTo,
	})
	uid := userID
	item := &models.VideoTranscript{
		ProjectID:          projectID,
		AssetID:            asset.ID,
		UserID:             &uid,
		ProviderProfileID:  profile.ID,
		Provider:           providerType,
		Model:              model,
		Status:             "queued",
		Language:           request.Language,
		TranslatedLanguage: request.TranslateTo,
		PrivacyJSON:        string(privacy),
		MetadataJSON:       string(metadata),
	}
	if err := s.transcripts.Create(item); err != nil {
		return nil, err
	}
	request.Model = model
	go s.run(context.Background(), item.ID, *profile, *asset, request)
	return item, nil
}

func (s *VideoTranscriptionService) run(
	ctx context.Context,
	id string,
	profile models.ProviderProfile,
	asset models.VideoAsset,
	request TranscriptionRequest,
) {
	if err := s.transcripts.MarkRunning(id); err != nil {
		return
	}
	key, err := s.providers.GetAPIKey(profile.ID)
	if err != nil || strings.TrimSpace(key) == "" {
		_ = s.transcripts.MarkFailed(id, "transcription provider API key is unavailable")
		return
	}
	path := asset.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.attachmentsDir, path)
	}
	endpoint := "https://api.openai.com/v1"
	if profile.BaseURL != nil && strings.TrimSpace(*profile.BaseURL) != "" {
		endpoint = strings.TrimRight(strings.TrimSpace(*profile.BaseURL), "/")
	}
	provider := &OpenAICompatibleTranscriber{
		baseURL: endpoint,
		apiKey:  key,
		client:  s.client,
	}
	result, err := provider.Transcribe(ctx, path, request)
	if err != nil {
		_ = s.transcripts.MarkFailed(id, err.Error())
		return
	}
	item, err := s.transcripts.GetByID(id)
	if err != nil || item == nil {
		return
	}
	item.Text = result.Text
	item.Language = result.Language
	item.TranslatedLanguage = result.TranslatedLanguage
	item.CostUSD = result.CostUSD
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["api_version"] = TranscriptionAPIVersion
	metadata["provider_profile_id"] = profile.ID
	metadata["requested_diarization"] = request.Diarization
	metadata["diarization_returned"] = transcriptHasSpeakers(result.Segments)
	metadataBytes, _ := json.Marshal(metadata)
	item.MetadataJSON = string(metadataBytes)
	if err := s.transcripts.Complete(item, result.Segments); err != nil {
		_ = s.transcripts.MarkFailed(id, err.Error())
	}
}

func transcriptHasSpeakers(segments []models.VideoTranscriptSegment) bool {
	for _, segment := range segments {
		if strings.TrimSpace(segment.Speaker) != "" {
			return true
		}
	}
	return false
}

func (s *VideoTranscriptionService) Get(userID, id string) (*models.VideoTranscript, error) {
	item, err := s.transcripts.GetByID(id)
	if err != nil || item == nil {
		return item, err
	}
	ok, err := s.projects.BelongsToUser(item.ProjectID, userID)
	if err != nil || !ok {
		return nil, fmt.Errorf("transcript not found")
	}
	return item, nil
}

func (s *VideoTranscriptionService) List(userID, projectID string) ([]models.VideoTranscript, error) {
	ok, err := s.projects.BelongsToUser(projectID, userID)
	if err != nil || !ok {
		return nil, fmt.Errorf("video project not found")
	}
	return s.transcripts.ListByProject(projectID)
}

// CaptionClips regenerates ordinary timeline caption clips from the durable
// transcript without retranscribing or contacting the provider.
func (s *VideoTranscriptionService) CaptionClips(userID, id string) ([]TimelineClip, error) {
	item, err := s.Get(userID, id)
	if err != nil {
		return nil, err
	}
	if item.Status != "completed" {
		return nil, fmt.Errorf("transcript is not complete")
	}
	clips := make([]TimelineClip, 0, len(item.Segments))
	for _, segment := range item.Segments {
		duration := segment.EndMS - segment.StartMS
		if duration < 100 {
			duration = 100
		}
		clips = append(clips, TimelineClip{
			ID:         "caption-" + segment.ID,
			StartMS:    segment.StartMS,
			DurationMS: duration,
			TrimInMS:   0,
			TrimOutMS:  duration,
			Transform: map[string]any{
				"x": 0.0, "y": 420.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
			},
			Text: &TimelineText{
				Text: segment.Text, FontSize: 48, Color: "#ffffff",
				Background: "#000000", TextAlign: "center", Shadow: true,
			},
			Effects:     []TimelineEffect{},
			Keyframes:   []TimelineKeyframe{},
			Transitions: []TimelineTransition{},
		})
	}
	return clips, nil
}

type OpenAICompatibleTranscriber struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (p *OpenAICompatibleTranscriber) Transcribe(
	ctx context.Context,
	path string,
	request TranscriptionRequest,
) (TranscriptionProviderResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return TranscriptionProviderResult{}, fmt.Errorf("open transcription media: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return TranscriptionProviderResult{}, err
	}
	if _, err = io.Copy(part, file); err != nil {
		return TranscriptionProviderResult{}, err
	}
	if err := writer.WriteField("model", request.Model); err != nil {
		return TranscriptionProviderResult{}, err
	}
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return TranscriptionProviderResult{}, err
	}
	if request.Language != "" {
		if err := writer.WriteField("language", request.Language); err != nil {
			return TranscriptionProviderResult{}, err
		}
	}
	if request.Prompt != "" {
		if err := writer.WriteField("prompt", request.Prompt); err != nil {
			return TranscriptionProviderResult{}, err
		}
	}
	if request.WordTimestamps {
		if err := writer.WriteField("timestamp_granularities[]", "word"); err != nil {
			return TranscriptionProviderResult{}, err
		}
		if err := writer.WriteField("timestamp_granularities[]", "segment"); err != nil {
			return TranscriptionProviderResult{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return TranscriptionProviderResult{}, err
	}

	route := "audio/transcriptions"
	if strings.EqualFold(request.TranslateTo, "en") {
		route = "audio/translations"
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.baseURL, "/")+"/"+route,
		&body,
	)
	if err != nil {
		return TranscriptionProviderResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := p.client.Do(httpRequest)
	if err != nil {
		return TranscriptionProviderResult{}, fmt.Errorf("transcription request: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TranscriptionProviderResult{}, fmt.Errorf(
			"transcription provider returned %d: %s",
			response.StatusCode,
			responseSnippet(data),
		)
	}

	var decoded struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
		Segments []struct {
			Start      float64          `json:"start"`
			End        float64          `json:"end"`
			Text       string           `json:"text"`
			Speaker    string           `json:"speaker"`
			AvgLogProb *float64         `json:"avg_logprob"`
			Words      []map[string]any `json:"words"`
		} `json:"segments"`
		Words []map[string]any `json:"words"`
		Usage map[string]any   `json:"usage"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return TranscriptionProviderResult{}, fmt.Errorf("decode transcription response: %w", err)
	}

	segments := make([]models.VideoTranscriptSegment, 0, len(decoded.Segments))
	for _, segment := range decoded.Segments {
		words, _ := json.Marshal(segment.Words)
		segments = append(segments, models.VideoTranscriptSegment{
			StartMS:    int64(segment.Start*1000 + 0.5),
			EndMS:      int64(segment.End*1000 + 0.5),
			Text:       strings.TrimSpace(segment.Text),
			Speaker:    strings.TrimSpace(segment.Speaker),
			Confidence: segment.AvgLogProb,
			WordsJSON:  string(words),
		})
	}
	if len(segments) == 0 && strings.TrimSpace(decoded.Text) != "" {
		endMS := int64(decoded.Duration*1000 + 0.5)
		if endMS < 100 {
			endMS = 100
		}
		segments = append(segments, models.VideoTranscriptSegment{
			StartMS: 0, EndMS: endMS, Text: strings.TrimSpace(decoded.Text), WordsJSON: "[]",
		})
	}
	translated := ""
	if route == "audio/translations" {
		translated = "en"
	}
	return TranscriptionProviderResult{
		Text:               strings.TrimSpace(decoded.Text),
		Language:           strings.TrimSpace(decoded.Language),
		TranslatedLanguage: translated,
		Segments:           segments,
		Metadata: map[string]any{
			"usage":            decoded.Usage,
			"word_count":       len(decoded.Words),
			"duration_seconds": decoded.Duration,
		},
	}, nil
}
