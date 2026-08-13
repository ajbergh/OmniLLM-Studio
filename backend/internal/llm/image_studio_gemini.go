package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// geminiStudioImageGenerate uses the current Gemini Developer API contracts for
// Image Studio. Gemini image models use generateContent; Imagen models use the
// predict endpoint and a different request/response envelope.
func (s *Service) geminiStudioImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "imagen-") {
		return s.imagenStudioImageGenerate(ctx, baseURL, apiKey, model, req, geometry)
	}
	return s.geminiGenerateContentStudioImage(ctx, baseURL, apiKey, model, req, geometry)
}

func buildGeminiStudioImageBody(req ImageStudioRequest, geometry imageStudioGeometryResolution) map[string]interface{} {
	parts := []map[string]interface{}{{"text": req.Prompt}}
	appendImage := func(ref ReferenceImage) {
		if strings.TrimSpace(ref.Data) == "" {
			return
		}
		mimeType := strings.TrimSpace(ref.MimeType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		parts = append(parts, map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": mimeType,
				"data":     ref.Data,
			},
		})
	}

	if req.ReferenceImage != nil {
		appendImage(*req.ReferenceImage)
	}
	for _, ref := range req.ReferenceImages {
		appendImage(ref)
	}
	if req.MaskImage != nil && strings.TrimSpace(req.MaskImage.Data) != "" {
		parts = append(parts, map[string]interface{}{"text": "Use this mask to identify the region to edit:"})
		appendImage(*req.MaskImage)
	}
	for _, ref := range req.StyleReferenceImages {
		if strings.TrimSpace(ref.Data) == "" {
			continue
		}
		parts = append(parts, map[string]interface{}{"text": "Use this as a style reference:"})
		appendImage(ref)
	}

	generationConfig := map[string]interface{}{
		"responseModalities": []string{"TEXT", "IMAGE"},
	}
	if geometry.Mode == ImageGeometryExplicit && geometry.AspectRatio != "" {
		// The current REST contract puts image geometry under
		// generationConfig.responseFormat.image, not imageConfig.
		generationConfig["responseFormat"] = map[string]interface{}{
			"image": map[string]interface{}{
				"aspectRatio": geometry.AspectRatio,
			},
		}
	}

	return map[string]interface{}{
		"contents": []map[string]interface{}{{"parts": parts}},
		"generationConfig": generationConfig,
	}
}

func (s *Service) geminiGenerateContentStudioImage(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", strings.TrimRight(geminiNativeBaseURL(baseURL), "/"), model)
	body := buildGeminiStudioImageBody(req, geometry)
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini image request: %w", err)
	}

	respBody, status, err := s.doGeminiStudioImageRequest(ctx, endpoint, apiKey, jsonBody)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Gemini image API returned status %d: %s", status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode Gemini image response: %w", err)
	}

	images := make([]ImageResult, 0, 1)
	for _, candidate := range result.Candidates {
		revisedPrompt := ""
		for _, part := range candidate.Content.Parts {
			if revisedPrompt == "" && strings.TrimSpace(part.Text) != "" {
				revisedPrompt = part.Text
			}
			if part.InlineData != nil && strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "image/") && part.InlineData.Data != "" {
				images = append(images, ImageResult{B64JSON: part.InlineData.Data, RevisedPrompt: revisedPrompt})
			}
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by Gemini")
	}
	return &ImageResponse{Images: images, Provider: "gemini", Model: model}, nil
}

func buildImagenStudioImageBody(req ImageStudioRequest, geometry imageStudioGeometryResolution) map[string]interface{} {
	n := req.N
	if n <= 0 {
		n = 1
	}
	parameters := map[string]interface{}{"sampleCount": n}
	if geometry.Mode == ImageGeometryExplicit && geometry.AspectRatio != "" && imagenSupportsAspectRatio(geometry.AspectRatio) {
		parameters["aspectRatio"] = geometry.AspectRatio
	}
	return map[string]interface{}{
		"instances":  []map[string]interface{}{{"prompt": req.Prompt}},
		"parameters": parameters,
	}
}

func imagenSupportsAspectRatio(ratio string) bool {
	switch ratio {
	case "1:1", "3:4", "4:3", "9:16", "16:9":
		return true
	default:
		return false
	}
}

func (s *Service) imagenStudioImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	if req.OperationType == "edit" || req.ReferenceImage != nil || len(req.ReferenceImages) > 0 || len(req.StyleReferenceImages) > 0 || req.MaskImage != nil {
		return nil, fmt.Errorf("Imagen 4 is generation-only in Image Studio")
	}
	endpoint := fmt.Sprintf("%s/models/%s:predict", strings.TrimRight(geminiNativeBaseURL(baseURL), "/"), model)
	body := buildImagenStudioImageBody(req, geometry)
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal Imagen request: %w", err)
	}

	respBody, status, err := s.doGeminiStudioImageRequest(ctx, endpoint, apiKey, jsonBody)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Imagen API returned status %d: %s", status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Predictions []map[string]interface{} `json:"predictions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode Imagen response: %w", err)
	}
	images := make([]ImageResult, 0, len(result.Predictions))
	for _, prediction := range result.Predictions {
		var data string
		for _, key := range []string{"bytesBase64Encoded", "bytes_base64_encoded", "b64_json"} {
			if value, ok := prediction[key].(string); ok && value != "" {
				data = value
				break
			}
		}
		if data != "" {
			images = append(images, ImageResult{B64JSON: data})
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by Imagen")
	}
	return &ImageResponse{Images: images, Provider: "gemini", Model: model}, nil
}

func (s *Service) doGeminiStudioImageRequest(ctx context.Context, endpoint, apiKey string, jsonBody []byte) ([]byte, int, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create Gemini image request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	}
	client := &http.Client{Timeout: 180 * time.Second, Transport: s.httpClient.Transport}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("Gemini image request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read Gemini image response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}
