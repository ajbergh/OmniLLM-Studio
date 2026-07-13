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
	"regexp"
	"strings"
	"time"
)

var geminiFileNamePattern = regexp.MustCompile(`files/([A-Za-z0-9_-]+)`)

type omniInteraction struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Steps  json.RawMessage `json:"steps"`
	Usage  json.RawMessage `json:"usage,omitempty"`
}

type omniVideoOutput struct {
	Data     string
	URI      string
	MimeType string
}

// GenerateOmni invokes Gemini Omni Flash through the Interactions API. Unlike
// Veo, an Omni interaction completes synchronously and returns either inline
// base64 or a Files API URI. The caller still runs this in the Video Studio
// background worker so the UI remains non-blocking.
func (p *GeminiProvider) GenerateOmni(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error) {
	if !p.Configured() {
		return nil, fmt.Errorf("%w: no enabled Gemini provider profile with an API key", ErrProviderUnavailable)
	}
	if !isGeminiOmniModel(req.Model) {
		return nil, fmt.Errorf("Gemini Omni interaction called with unsupported model %q", req.Model)
	}

	if progress != nil {
		progress(GenerationProgress{Stage: "preparing", Message: "Preparing Gemini Omni interaction", Progress: 0.08})
	}

	payload, cleanup, err := p.buildOmniPayload(ctx, req)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Gemini Omni request: %w", err)
	}

	if progress != nil {
		progress(GenerationProgress{Stage: "generating", Message: "Gemini Omni is generating video and audio", Progress: 0.2})
	}
	reqURL := p.baseURL + "/interactions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("create Gemini Omni interaction: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("read Gemini Omni response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini Omni returned %s: %s", resp.Status, responseSnippet(respBody))
	}

	var interaction omniInteraction
	if err := json.Unmarshal(respBody, &interaction); err != nil {
		return nil, fmt.Errorf("decode Gemini Omni response: %w", err)
	}
	output := extractOmniVideoOutput(respBody)
	if output.Data == "" && output.URI == "" {
		return nil, errors.New("Gemini Omni interaction completed without a video output")
	}

	var videoData []byte
	if output.Data != "" {
		videoData, err = base64.StdEncoding.DecodeString(output.Data)
		if err != nil {
			return nil, fmt.Errorf("decode Gemini Omni video: %w", err)
		}
	} else {
		if progress != nil {
			progress(GenerationProgress{Stage: "processing", Message: "Finalizing Gemini Omni video", Progress: 0.88})
		}
		if err := p.waitForGeminiFile(ctx, output.URI); err != nil {
			return nil, err
		}
		videoData, output.MimeType, err = p.downloadVideo(ctx, output.URI)
		if err != nil {
			return nil, err
		}
	}
	if output.MimeType == "" {
		output.MimeType = "video/mp4"
	}
	if len(videoData) == 0 {
		return nil, errors.New("Gemini Omni returned an empty video")
	}

	interactionID := strings.TrimSpace(interaction.ID)
	return &GenerationResult{
		MimeType:      output.MimeType,
		FileName:      "gemini-omni-flash" + extensionForMimeType(output.MimeType),
		Data:          videoData,
		UpstreamJobID: optionalStringPointer(interactionID),
		UsageJSON:     interaction.Usage,
		Metadata: map[string]any{
			"provider":        ProviderGemini,
			"model":           req.Model,
			"interaction_id":  interactionID,
			"generation_mode": omniTaskForRequest(req),
			"api":             "gemini_interactions",
			"native_audio":    true,
			"synthid":         true,
		},
	}, nil
}

func (p *GeminiProvider) buildOmniPayload(ctx context.Context, req GenerateRequest) (map[string]any, func(), error) {
	prompt := buildOmniPrompt(req)
	task := omniTaskForRequest(req)
	var input any = prompt
	var uploadedNames []string
	cleanupUploads := func() {
		for _, name := range uploadedNames {
			_ = p.deleteGeminiFile(context.Background(), name)
		}
	}

	if strings.TrimSpace(req.PreviousInteractionID) != "" {
		// The stored interaction carries forward the generated video state.
		input = prompt
	} else if strings.TrimSpace(req.SourceVideoPath) != "" {
		file, err := p.uploadGeminiFile(ctx, req.SourceVideoPath)
		if err != nil {
			return nil, nil, fmt.Errorf("upload source video to Gemini Files API: %w", err)
		}
		uploadedNames = append(uploadedNames, file.Name)
		input = []map[string]any{
			{"type": "document", "uri": file.URI},
			{"type": "text", "text": prompt},
		}
	} else if strings.TrimSpace(req.StartImagePath) != "" || len(req.ReferenceAssetPaths) > 0 {
		imagePaths := make([]string, 0, 1+len(req.ReferenceAssetPaths))
		if strings.TrimSpace(req.StartImagePath) != "" {
			imagePaths = append(imagePaths, req.StartImagePath)
		}
		imagePaths = append(imagePaths, req.ReferenceAssetPaths...)
		// Base64 adds roughly 33% overhead. Switch to Files API URIs before the
		// complete interaction approaches Google's 100 MB request limit.
		var imageBytes int64
		for _, path := range imagePaths {
			if info, statErr := os.Stat(path); statErr == nil {
				imageBytes += info.Size()
			}
		}
		useFilesAPI := imageBytes > 70*1024*1024
		parts := make([]map[string]any, 0, 1+len(imagePaths))
		for _, path := range imagePaths {
			var part map[string]any
			var err error
			if useFilesAPI {
				var file *geminiUploadedFile
				file, err = p.uploadGeminiFile(ctx, path)
				if err == nil {
					uploadedNames = append(uploadedNames, file.Name)
					part = map[string]any{"type": "image", "uri": file.URI, "mime_type": file.MimeType}
				}
			} else {
				part, err = p.omniImagePart(path)
			}
			if err != nil {
				cleanupUploads()
				return nil, nil, err
			}
			parts = append(parts, part)
		}
		parts = append(parts, map[string]any{"type": "text", "text": prompt})
		input = parts
	}

	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = DefaultAspectRatio
	}
	payload := map[string]any{
		"model": req.Model,
		"input": input,
		"response_format": map[string]any{
			"type":         "video",
			"aspect_ratio": aspectRatio,
			"delivery":     "uri",
		},
		"generation_config": map[string]any{
			"video_config": map[string]any{"task": task},
		},
		"background": false,
		"store":      true,
		"stream":     false,
	}
	if previous := strings.TrimSpace(req.PreviousInteractionID); previous != "" {
		payload["previous_interaction_id"] = previous
	}
	return payload, cleanupUploads, nil
}

func omniTaskForRequest(req GenerateRequest) string {
	if task := strings.ToLower(strings.TrimSpace(req.GenerationMode)); task != "" {
		return task
	}
	switch {
	case strings.TrimSpace(req.SourceVideoPath) != "" || strings.TrimSpace(req.PreviousInteractionID) != "":
		return "edit"
	case len(req.ReferenceAssetPaths) > 0:
		return "reference_to_video"
	case strings.TrimSpace(req.StartImagePath) != "":
		return "image_to_video"
	default:
		return "text_to_video"
	}
}

func buildOmniPrompt(req GenerateRequest) string {
	prompt := assembleProviderPrompt(req)
	if omniTaskForRequest(req) == "edit" {
		if !strings.Contains(strings.ToLower(prompt), "keep everything else") {
			prompt += " Keep everything else the same."
		}
		return strings.TrimSpace(prompt)
	}
	declarations := make([]string, 0, 2)
	imageNumber := 1
	if strings.TrimSpace(req.StartImagePath) != "" {
		declarations = append(declarations, fmt.Sprintf("[# Sources <FIRST_FRAME>@Image%d]", imageNumber))
		imageNumber++
	}
	if len(req.ReferenceAssetPaths) > 0 {
		refs := make([]string, 0, len(req.ReferenceAssetPaths))
		for i := range req.ReferenceAssetPaths {
			refs = append(refs, fmt.Sprintf("<IMAGE_REF_%d>@Image%d", i, imageNumber+i))
		}
		declarations = append(declarations, "[# References "+strings.Join(refs, " ")+"]")
	}
	if len(declarations) > 0 {
		prompt = strings.Join(declarations, " ") + " " + prompt
		if strings.TrimSpace(req.StartImagePath) != "" {
			prompt += " Use the source image as the starting frame."
		}
		if len(req.ReferenceAssetPaths) > 0 {
			prompt += " Use the reference images for identity, subject, product, and style consistency; do not treat them as literal initial frames."
		}
	}
	return strings.TrimSpace(prompt)
}

func (p *GeminiProvider) omniImagePart(path string) (map[string]any, error) {
	data, mimeType, err := p.readReferenceImage(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":      "image",
		"data":      base64.StdEncoding.EncodeToString(data),
		"mime_type": mimeType,
	}, nil
}

type geminiUploadedFile struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	State    string `json:"state"`
}

func (p *GeminiProvider) uploadGeminiFile(ctx context.Context, path string) (*geminiUploadedFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mimeType, err := mimeTypeForGeminiFile(path)
	if err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(map[string]any{"file": map[string]any{"display_name": filepath.Base(path)}})
	startURL := strings.TrimSuffix(p.baseURL, "/v1beta") + "/upload/v1beta/files"
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewReader(meta))
	if err != nil {
		return nil, err
	}
	startReq.Header.Set("x-goog-api-key", p.apiKey)
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("X-Goog-Upload-Protocol", "resumable")
	startReq.Header.Set("X-Goog-Upload-Command", "start")
	startReq.Header.Set("X-Goog-Upload-Header-Content-Length", fmt.Sprintf("%d", info.Size()))
	startReq.Header.Set("X-Goog-Upload-Header-Content-Type", mimeType)
	startResp, err := p.client.Do(startReq)
	if err != nil {
		return nil, err
	}
	defer startResp.Body.Close()
	if startResp.StatusCode < 200 || startResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(startResp.Body, maxProviderJSONBytes))
		return nil, fmt.Errorf("start file upload returned %s: %s", startResp.Status, responseSnippet(body))
	}
	uploadURL := strings.TrimSpace(startResp.Header.Get("X-Goog-Upload-URL"))
	if uploadURL == "" {
		return nil, errors.New("Gemini Files API did not return an upload URL")
	}

	file, err := os.Open(path) // #nosec G304 -- internal asset path resolved by the service
	if err != nil {
		return nil, err
	}
	defer file.Close()
	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, file)
	if err != nil {
		return nil, err
	}
	uploadReq.ContentLength = info.Size()
	uploadReq.Header.Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	uploadReq.Header.Set("X-Goog-Upload-Offset", "0")
	uploadReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")
	uploadResp, err := p.client.Do(uploadReq)
	if err != nil {
		return nil, err
	}
	defer uploadResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(uploadResp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, err
	}
	if uploadResp.StatusCode < 200 || uploadResp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload file returned %s: %s", uploadResp.Status, responseSnippet(body))
	}
	var result struct {
		File geminiUploadedFile `json:"file"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.File.URI == "" {
		return nil, errors.New("Gemini Files API upload completed without a file URI")
	}
	if err := p.waitForGeminiFile(ctx, result.File.Name); err != nil {
		return nil, err
	}
	return &result.File, nil
}

func mimeTypeForGeminiFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif",
		".mp4": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm", ".avi": "video/x-msvideo", ".mkv": "video/x-matroska",
	}[ext]
	if mimeType == "" {
		return "", fmt.Errorf("unsupported Gemini media file type: %s", ext)
	}
	return mimeType, nil
}

func (p *GeminiProvider) waitForGeminiFile(ctx context.Context, nameOrURI string) error {
	match := geminiFileNamePattern.FindStringSubmatch(nameOrURI)
	if len(match) < 2 {
		// Inline output and non-Files URLs do not need polling.
		return nil
	}
	name := "files/" + match[1]
	for attempt := 0; attempt < 120; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/"+name, nil)
		if err != nil {
			return err
		}
		req.Header.Set("x-goog-api-key", p.apiKey)
		resp, err := p.client.Do(req)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("Gemini file status returned %s: %s", resp.Status, responseSnippet(body))
		}
		var file geminiUploadedFile
		if err := json.Unmarshal(body, &file); err != nil {
			return err
		}
		switch strings.ToUpper(file.State) {
		case "ACTIVE", "STATE_UNSPECIFIED", "":
			return nil
		case "FAILED":
			return errors.New("Gemini file processing failed")
		}
	}
	return errors.New("Gemini file processing timed out")
}

func (p *GeminiProvider) deleteGeminiFile(ctx context.Context, name string) error {
	name = strings.TrimLeft(strings.TrimSpace(name), "/")
	if name == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.baseURL+"/"+name, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-goog-api-key", p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func extractOmniVideoOutput(raw []byte) omniVideoOutput {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return omniVideoOutput{}
	}
	return findOmniVideoOutput(value)
}

func findOmniVideoOutput(value any) omniVideoOutput {
	switch typed := value.(type) {
	case map[string]any:
		if strings.EqualFold(stringValue(typed["type"]), "video") || typed["output_video"] != nil {
			candidate := typed
			if nested, ok := typed["output_video"].(map[string]any); ok {
				candidate = nested
			}
			out := omniVideoOutput{
				Data:     stringValue(candidate["data"]),
				URI:      stringValue(candidate["uri"]),
				MimeType: firstNonEmptyString(stringValue(candidate["mime_type"]), stringValue(candidate["mimeType"])),
			}
			if out.Data != "" || out.URI != "" {
				return out
			}
		}
		for _, child := range typed {
			if found := findOmniVideoOutput(child); found.Data != "" || found.URI != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findOmniVideoOutput(child); found.Data != "" || found.URI != "" {
				return found
			}
		}
	}
	return omniVideoOutput{}
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func optionalStringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	value = strings.TrimSpace(value)
	return &value
}
