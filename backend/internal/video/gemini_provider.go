package video

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultGeminiVideoBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type GeminiProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewGeminiProvider(baseURL, apiKey string) *GeminiProvider {
	baseURL = normalizeGeminiVideoBaseURL(baseURL)
	if baseURL == "" {
		baseURL = defaultGeminiVideoBaseURL
	}
	return &GeminiProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (p *GeminiProvider) Key() string {
	return ProviderGemini
}

func (p *GeminiProvider) DisplayName() string {
	return "Google Video (Omni + Veo)"
}

func (p *GeminiProvider) Configured() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

func (p *GeminiProvider) ListModels(ctx context.Context) ([]Model, error) {
	if !p.Configured() {
		return KnownGeminiVideoModels(), nil
	}
	live, err := p.fetchLiveModels(ctx)
	if err != nil {
		// Fall back to static list on any error.
		return KnownGeminiVideoModels(), nil
	}
	// The Interactions API preview model is not consistently returned by
	// models.list. Keep the documented Omni entry and merge live Veo models.
	return mergeGeminiVideoModels(KnownGeminiVideoModels(), live), nil
}

// fetchLiveModels calls the Gemini models.list API and returns Veo model entries.
func (p *GeminiProvider) fetchLiveModels(ctx context.Context) ([]Model, error) {
	type geminiModel struct {
		Name                       string   `json:"name"`
		DisplayName                string   `json:"displayName"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	}
	type listResp struct {
		Models []geminiModel `json:"models"`
	}

	reqURL := p.baseURL + "/models?pageSize=100"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-goog-api-key", p.apiKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list Gemini models: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("read Gemini models list: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini models list returned %s", resp.Status)
	}

	var data listResp
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode Gemini models list: %w", err)
	}

	var out []Model
	for _, m := range data.Models {
		// name is like "models/veo-3.1-generate-preview"
		modelID := strings.TrimPrefix(m.Name, "models/")
		if !strings.Contains(strings.ToLower(modelID), "veo") {
			continue
		}
		// Check that predictLongRunning is available.
		supportsPredict := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "predictLongRunning" || method == "predict" {
				supportsPredict = true
				break
			}
		}
		if !supportsPredict {
			continue
		}
		displayName := strings.TrimSpace(m.DisplayName)
		if displayName == "" {
			displayName = modelID
		}
		out = append(out, geminiVeoKnownModel(modelID, displayName, []string{"720p", "1080p"}, "Discovered via Gemini models API."))
	}
	return out, nil
}

func (p *GeminiProvider) Capabilities(model string) []Capability {
	if isGeminiOmniModel(model) {
		return geminiOmniCapabilities()
	}
	return geminiVeoCapabilitiesForID(model)
}

// Submit starts a Veo predictLongRunning operation and returns the operation name immediately.
// It does NOT poll or download — that is done by PollOperation + DownloadVideo.
func (p *GeminiProvider) Submit(ctx context.Context, req GenerateRequest) (operationName string, err error) {
	if !p.Configured() {
		return "", fmt.Errorf("%w: no enabled Gemini provider profile with an API key", ErrProviderUnavailable)
	}
	payload, err := p.buildPayload(req)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(payload)
	submitURL := p.baseURL + "/models/" + strings.TrimSpace(req.Model) + ":predictLongRunning"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("submit Gemini Veo operation: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return "", fmt.Errorf("read Gemini submit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini returned %s: %s", resp.Status, responseSnippet(respBody))
	}
	var op geminiOperation
	if err := json.Unmarshal(respBody, &op); err != nil {
		return "", fmt.Errorf("decode Gemini submit response: %w", err)
	}
	if strings.TrimSpace(op.Name) == "" {
		return "", errors.New("Gemini submit response did not include an operation name")
	}
	return op.Name, nil
}

// PollOnce checks the current state of an operation without blocking.
// Returns (done, videoURI, rawResponse, error).
func (p *GeminiProvider) PollOnce(ctx context.Context, operationName string) (done bool, videoURI string, raw json.RawMessage, err error) {
	operationURL := p.operationURL(operationName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, operationURL, nil)
	if err != nil {
		return false, "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)
	resp, pollErr := p.client.Do(req)
	if pollErr != nil {
		return false, "", nil, fmt.Errorf("poll Gemini Veo operation: %w", pollErr)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	_ = resp.Body.Close()
	if readErr != nil {
		return false, "", nil, fmt.Errorf("read Gemini poll response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", nil, fmt.Errorf("Gemini poll returned %s: %s", resp.Status, responseSnippet(body))
	}
	var op geminiOperation
	if err := json.Unmarshal(body, &op); err != nil {
		return false, "", nil, fmt.Errorf("decode Gemini poll response: %w", err)
	}
	if op.Error != nil {
		return false, "", nil, fmt.Errorf("Gemini Veo operation failed: %s", op.Error.Message)
	}
	if op.Done {
		uri := extractGeminiVideoURI(op.Response)
		return true, uri, op.Response, nil
	}
	return false, "", nil, nil
}

// DownloadVideo downloads a generated MP4 from the given videoURI.
func (p *GeminiProvider) DownloadVideo(ctx context.Context, videoURI string) ([]byte, string, error) {
	return p.downloadVideo(ctx, videoURI)
}

// buildPayload constructs the Gemini Veo request payload from a GenerateRequest.
func (p *GeminiProvider) buildPayload(req GenerateRequest) (map[string]any, error) {
	duration := defaultInt(req.DurationSeconds, 8)
	if duration > 8 {
		duration = 8
	}
	if duration < 4 {
		duration = 4
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "720p"
	}
	// 1080p / 4k require duration=8.
	if (strings.EqualFold(resolution, "1080p") || strings.EqualFold(resolution, "4k")) && duration != 8 {
		duration = 8
	}
	// Extension and reference images also require duration=8.
	hasStartImage := strings.TrimSpace(req.StartImagePath) != ""
	hasLastFrame := strings.TrimSpace(req.LastFramePath) != ""
	hasSourceVideo := strings.TrimSpace(req.SourceVideoPath) != ""
	hasReferenceImages := len(req.ReferenceAssetPaths) > 0
	if (hasLastFrame || hasReferenceImages || hasSourceVideo) && duration != 8 {
		duration = 8
	}

	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = DefaultAspectRatio
	}
	prompt := assembleProviderPrompt(req)

	instance := map[string]any{"prompt": prompt}

	// --- Image-to-video: first frame ---
	if hasStartImage {
		imgBytes, imgMime, readErr := p.readReferenceImage(req.StartImagePath)
		if readErr != nil {
			return nil, fmt.Errorf("read start frame image: %w", readErr)
		}
		instance["image"] = map[string]any{
			"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imgBytes),
			"mimeType":           imgMime,
		}
	}

	// --- First/last-frame interpolation: last frame ---
	if hasLastFrame {
		if !hasStartImage {
			return nil, errors.New("last_frame requires a start_image (first frame) to be provided")
		}
		imgBytes, imgMime, readErr := p.readReferenceImage(req.LastFramePath)
		if readErr != nil {
			return nil, fmt.Errorf("read last frame image: %w", readErr)
		}
		instance["lastFrame"] = map[string]any{
			"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imgBytes),
			"mimeType":           imgMime,
		}
	}

	// --- Video extension: source video ---
	if hasSourceVideo {
		vidBytes, vidMime, readErr := p.readVideoFile(req.SourceVideoPath)
		if readErr != nil {
			return nil, fmt.Errorf("read source video: %w", readErr)
		}
		// Extension is 720p only per Veo docs.
		resolution = "720p"
		instance["video"] = map[string]any{
			"bytesBase64Encoded": base64.StdEncoding.EncodeToString(vidBytes),
			"mimeType":           vidMime,
		}
	}

	// --- Reference images (up to 3) ---
	if hasReferenceImages {
		maxRef := 3
		if len(req.ReferenceAssetPaths) < maxRef {
			maxRef = len(req.ReferenceAssetPaths)
		}
		refImages := make([]map[string]any, 0, maxRef)
		for i := 0; i < maxRef; i++ {
			imgBytes, imgMime, readErr := p.readReferenceImage(req.ReferenceAssetPaths[i])
			if readErr != nil {
				continue
			}
			refImages = append(refImages, map[string]any{
				"referenceType": "asset",
				"referenceId":   i + 1,
				"assetImage": map[string]any{
					"image": map[string]any{
						"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imgBytes),
						"mimeType":           imgMime,
					},
				},
			})
		}
		if len(refImages) > 0 {
			instance["referenceImages"] = refImages
		}
	}

	parameters := map[string]any{
		"aspectRatio":     aspectRatio,
		"durationSeconds": duration,
		"resolution":      resolution,
	}
	if req.Seed != nil {
		parameters["seed"] = *req.Seed
	}
	if pg := strings.TrimSpace(req.PersonGeneration); pg != "" {
		parameters["personGeneration"] = pg
	}
	if negativePrompt := strings.TrimSpace(req.NegativePrompt); negativePrompt != "" {
		parameters["negativePrompt"] = negativePrompt
	}
	mergeAllowedVideoSettings(parameters, req.Settings, map[string]bool{
		"aspectRatio":      true,
		"durationSeconds":  true,
		"negativePrompt":   true,
		"numberOfVideos":   true,
		"personGeneration": true,
		"resolution":       true,
		"sampleCount":      true,
		"seed":             true,
	})

	return map[string]any{
		"instances":  []map[string]any{instance},
		"parameters": parameters,
	}, nil
}

// Generate is the synchronous (blocking) implementation retained for backward compat.
// Prefer the async Submit → PollOnce → DownloadVideo flow for new callers.
func (p *GeminiProvider) Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error) {
	if isGeminiOmniModel(req.Model) {
		return p.GenerateOmni(ctx, req, progress)
	}
	if !p.Configured() {
		return nil, fmt.Errorf("%w: no enabled Gemini provider profile with an API key", ErrProviderUnavailable)
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "submitting", Message: "Submitting Gemini Veo long-running operation", Progress: 0.08})
	}

	// Resolve resolution/duration for result metadata.
	duration := defaultInt(req.DurationSeconds, 8)
	if duration < 4 {
		duration = 4
	}
	if duration > 8 {
		duration = 8
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "720p"
	}
	if (strings.EqualFold(resolution, "1080p") || strings.EqualFold(resolution, "4k")) && duration != 8 {
		duration = 8
	}
	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = DefaultAspectRatio
	}

	payload, err := p.buildPayload(req)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(payload)
	submitURL := p.baseURL + "/models/" + strings.TrimSpace(req.Model) + ":predictLongRunning"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit Gemini Veo operation: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("read Gemini submit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini returned %s: %s", resp.Status, responseSnippet(respBody))
	}

	var op geminiOperation
	if err := json.Unmarshal(respBody, &op); err != nil {
		return nil, fmt.Errorf("decode Gemini submit response: %w", err)
	}
	if strings.TrimSpace(op.Name) == "" {
		return nil, errors.New("Gemini submit response did not include an operation name")
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "queued", Message: "Gemini Veo operation accepted", Progress: 0.15})
	}
	completed, err := p.pollOperation(ctx, op.Name, progress)
	if err != nil {
		return nil, err
	}
	videoURI := extractGeminiVideoURI(completed.Response)
	if videoURI == "" {
		return nil, errors.New("Gemini Veo operation completed without a video URI")
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "downloading", Message: "Downloading Gemini Veo video output", Progress: 0.95})
	}
	data, mimeType, err := p.downloadVideo(ctx, videoURI)
	if err != nil {
		return nil, err
	}
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	width, height := dimensionsForResolution(resolution, aspectRatio)
	fps := float64(24)
	durationMS := int64(duration * 1000)
	operationName := completed.Name
	fileName := "gemini-" + sanitizePathSegment(req.Model) + extensionForMimeType(mimeType)
	return &GenerationResult{
		MimeType:      mimeType,
		FileName:      fileName,
		Data:          data,
		DurationMS:    &durationMS,
		Width:         &width,
		Height:        &height,
		FPS:           &fps,
		UpstreamJobID: &operationName,
		Metadata: map[string]any{
			"provider":       ProviderGemini,
			"model":          req.Model,
			"operation_name": operationName,
			"api":            "gemini_predict_long_running",
		},
	}, nil
}

func (p *GeminiProvider) pollOperation(ctx context.Context, operationName string, progress func(GenerationProgress)) (*geminiOperation, error) {
	operationURL := p.operationURL(operationName)
	for attempt := 0; attempt < 120; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, operationURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-goog-api-key", p.apiKey)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("poll Gemini Veo operation: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Gemini poll response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Gemini poll returned %s: %s", resp.Status, responseSnippet(body))
		}
		var op geminiOperation
		if err := json.Unmarshal(body, &op); err != nil {
			return nil, fmt.Errorf("decode Gemini poll response: %w", err)
		}
		if op.Name == "" {
			op.Name = operationName
		}
		if op.Error != nil {
			return nil, fmt.Errorf("Gemini Veo operation failed: %s", op.Error.Message)
		}
		if progress != nil {
			progress(GenerationProgress{
				Stage:    "polling",
				Message:  "Gemini Veo operation in progress",
				Progress: minFloat(0.9, 0.2+(float64(attempt)*0.03)),
			})
		}
		if op.Done {
			return &op, nil
		}
	}
	return nil, errors.New("Gemini Veo operation timed out while polling")
}

func (p *GeminiProvider) operationURL(operationName string) string {
	operationName = strings.TrimSpace(operationName)
	if strings.HasPrefix(operationName, "http://") || strings.HasPrefix(operationName, "https://") {
		return operationName
	}
	return p.baseURL + "/" + strings.TrimLeft(operationName, "/")
}

func (p *GeminiProvider) downloadVideo(ctx context.Context, videoURI string) ([]byte, string, error) {
	return downloadWithRetry(ctx, p.client, p.mediaURL(videoURI), "Gemini", map[string]string{
		"x-goog-api-key": p.apiKey,
	})
}

func (p *GeminiProvider) mediaURL(videoURI string) string {
	videoURI = strings.TrimSpace(videoURI)
	if strings.HasPrefix(videoURI, "http://") || strings.HasPrefix(videoURI, "https://") {
		return videoURI
	}
	videoURI = strings.TrimLeft(videoURI, "/")
	if strings.HasSuffix(videoURI, ":download") || strings.Contains(videoURI, ":download?") {
		return p.baseURL + "/" + videoURI
	}
	if strings.HasPrefix(videoURI, "files/") {
		return p.baseURL + "/" + videoURI + ":download?alt=media"
	}
	return p.baseURL + "/" + videoURI
}

// readReferenceImage reads an image file and returns its bytes and MIME type.
// Only image MIME types are accepted; non-image files return an error.
func (p *GeminiProvider) readReferenceImage(path string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mimeByExt := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		".gif":  "image/gif",
	}
	mimeType, ok := mimeByExt[ext]
	if !ok {
		return nil, "", fmt.Errorf("unsupported reference image type: %s", ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read reference image: %w", err)
	}
	return data, mimeType, nil
}

// readVideoFile reads a video file and returns its bytes and MIME type.
func (p *GeminiProvider) readVideoFile(path string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mimeByExt := map[string]string{
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".webm": "video/webm",
		".mkv":  "video/x-matroska",
	}
	mimeType, ok := mimeByExt[ext]
	if !ok {
		mimeType = "video/mp4" // default
	}
	data, err := os.ReadFile(path) // #nosec G304 — path comes from internal DB asset lookup, not user input
	if err != nil {
		return nil, "", fmt.Errorf("read video file: %w", err)
	}
	return data, mimeType, nil
}

func normalizeGeminiVideoBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	baseURL = strings.TrimSuffix(baseURL, "/openai")
	return strings.TrimRight(baseURL, "/")
}

type geminiOperation struct {
	Name     string          `json:"name"`
	Done     bool            `json:"done"`
	Error    *geminiAPIError `json:"error,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

type geminiAPIError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

func KnownGeminiVeoModels() []Model {
	return []Model{
		geminiVeoKnownModel("veo-3.1-generate-preview", "Veo 3.1", []string{"720p", "1080p", "4k"}, "Full Veo 3.1 suite: image-to-video, first/last-frame, reference images, extension, 4K, person_generation."),
		geminiVeoKnownModel("veo-3.1-fast-generate-preview", "Veo 3.1 Fast", []string{"720p", "1080p"}, "Veo 3.1 Fast: image-to-video, first/last-frame, reference images, extension (720p), no 4K."),
		geminiVeoKnownModel("veo-3.1-lite-generate-preview", "Veo 3.1 Lite", []string{"720p", "1080p"}, "Veo 3.1 Lite: image-to-video, first/last-frame, reference images. No 4K, no extension."),
	}
}

func KnownGeminiVideoModels() []Model {
	return append([]Model{geminiOmniKnownModel()}, KnownGeminiVeoModels()...)
}

func geminiOmniKnownModel() Model {
	return Model{
		ID:                 "gemini-omni-flash-preview",
		Provider:           ProviderGemini,
		Name:               "Gemini Omni Flash (Preview)",
		Capabilities:       geminiOmniCapabilities(),
		AspectRatios:       []string{"16:9", "9:16"},
		MaxPromptChars:     8000,
		MaxReferenceImages: 6,
		Notes:              "Interactions API: text, image, multi-reference, and conversational video editing with native audio.",
	}
}

func geminiOmniCapabilities() []Capability {
	return []Capability{
		CapabilityTextToVideo,
		CapabilityImageToVideo,
		CapabilityReferenceImages,
		CapabilityVideoToVideo,
		CapabilityCameraMotion,
		CapabilityAudioGeneration,
	}
}

func isGeminiOmniModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "gemini-omni-flash-preview")
}

func mergeGeminiVideoModels(static, live []Model) []Model {
	out := append([]Model(nil), static...)
	seen := make(map[string]bool, len(out))
	for _, model := range out {
		seen[strings.ToLower(model.ID)] = true
	}
	for _, model := range live {
		if !seen[strings.ToLower(model.ID)] {
			out = append(out, model)
		}
	}
	return out
}

func geminiVeoKnownModel(id, name string, resolutions []string, notes string) Model {
	return Model{
		ID:                 id,
		Provider:           ProviderGemini,
		Name:               name,
		Capabilities:       geminiVeoCapabilitiesForID(id),
		AspectRatios:       []string{"16:9", "9:16"},
		Resolutions:        cloneStrings(resolutions),
		DurationMinSeconds: 4,
		DurationMaxSeconds: 8,
		FPSOptions:         []int{24},
		MaxPromptChars:     4000,
		Notes:              notes,
	}
}

func geminiVeoCapabilitiesForID(model string) []Capability {
	base := []Capability{
		CapabilityTextToVideo,
		CapabilityImageToVideo,
		CapabilityFirstLastFrame,
		CapabilityReferenceImages,
		CapabilityNegativePrompt,
		CapabilityPersonGeneration,
		CapabilitySeed,
		CapabilityCameraMotion,
		CapabilityAudioGeneration,
	}
	switch model {
	case "veo-3.1-generate-preview":
		// Full suite — also supports extend and 4K.
		return append(base, CapabilityExtendVideo)
	case "veo-3.1-fast-generate-preview":
		// Fast — supports extend (720p only), no 4K.
		return append(base, CapabilityExtendVideo)
	case "veo-3.1-lite-generate-preview":
		// Lite — no extension, no 4K.
		return base
	default:
		// Unknown/future Veo model — return base set.
		return base
	}
}

func extractGeminiVideoURI(raw json.RawMessage) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	return findGeminiVideoURI(decoded, false)
}

func findGeminiVideoURI(value any, insideVideo bool) string {
	switch typed := value.(type) {
	case map[string]any:
		if insideVideo {
			if uri, ok := typed["uri"].(string); ok && strings.TrimSpace(uri) != "" {
				return strings.TrimSpace(uri)
			}
		}
		for key, child := range typed {
			if strings.EqualFold(key, "video") {
				if found := findGeminiVideoURI(child, true); found != "" {
					return found
				}
			}
		}
		for key, child := range typed {
			if strings.EqualFold(key, "uri") {
				if uri, ok := child.(string); ok && looksLikeVideoURI(uri) {
					return strings.TrimSpace(uri)
				}
			}
			if found := findGeminiVideoURI(child, insideVideo); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findGeminiVideoURI(child, insideVideo); found != "" {
				return found
			}
		}
	}
	return ""
}

func looksLikeVideoURI(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.Contains(value, "/files/") ||
		strings.Contains(value, "video")
}
