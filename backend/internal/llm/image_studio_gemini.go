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

// buildGeminiStudioImageBody serializes the current Gemini GenerateContent
// contract used by Image Studio. Pixel dimensions selected in the UI map to an
// aspect ratio here; they are not treated as Gemini imageSize resolution tiers.
func buildGeminiStudioImageBody(req ImageStudioRequest, geometry imageStudioGeometryResolution) map[string]interface{} {
	generationConfig := map[string]interface{}{
		"responseModalities": []string{"IMAGE", "TEXT"},
	}
	if geometry.Mode == ImageGeometryExplicit {
		aspectRatio := geometry.AspectRatio
		if strings.TrimSpace(geometry.Size) != "" {
			aspectRatio = sizeToGeminiAspectRatio(geometry.Size)
		}
		if aspectRatio != "" {
			generationConfig["responseFormat"] = map[string]interface{}{
				"image": map[string]interface{}{
					"aspectRatio": aspectRatio,
				},
			}
		}
	}

	parts := []map[string]interface{}{{"text": req.Prompt}}
	appendInlineImage := func(ref ReferenceImage) {
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
		appendInlineImage(*req.ReferenceImage)
	}
	for _, ref := range req.ReferenceImages {
		appendInlineImage(ref)
	}
	if req.MaskImage != nil && strings.TrimSpace(req.MaskImage.Data) != "" {
		parts = append(parts, map[string]interface{}{"text": "Use this mask to identify the region to edit:"})
		appendInlineImage(*req.MaskImage)
	}
	for _, ref := range req.StyleReferenceImages {
		if strings.TrimSpace(ref.Data) == "" {
			continue
		}
		parts = append(parts, map[string]interface{}{"text": "Use this as a style reference:"})
		appendInlineImage(ref)
	}

	return map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": parts},
		},
		"generationConfig": generationConfig,
	}
}

// geminiStudioImageGenerate fans out Image Studio variants as independent
// Gemini requests. Gemini image models do not expose a portable multi-candidate
// image contract, so Image Studio never sends candidateCount.
func (s *Service) geminiStudioImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	variantCount := req.N
	if variantCount <= 0 {
		variantCount = 1
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent", geminiNativeBaseURL(baseURL), model)
	body := buildGeminiStudioImageBody(req, geometry)
	images := make([]ImageResult, 0, variantCount)

	for variant := 0; variant < variantCount; variant++ {
		respBody, status, err := s.doGeminiStudioImageRequest(ctx, endpoint, apiKey, body)
		if err != nil {
			return nil, fmt.Errorf("Gemini Image Studio request %d/%d: %w", variant+1, variantCount, err)
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("Gemini image API returned status %d: %s", status, strings.TrimSpace(string(respBody)))
		}
		variantImages, err := parseGeminiStudioImageResponse(respBody)
		if err != nil {
			return nil, fmt.Errorf("Gemini Image Studio response %d/%d: %w", variant+1, variantCount, err)
		}
		images = append(images, variantImages...)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by Gemini")
	}
	return &ImageResponse{Images: images, Provider: "gemini", Model: model}, nil
}

func (s *Service) doGeminiStudioImageRequest(ctx context.Context, endpoint, apiKey string, body map[string]interface{}) ([]byte, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal Gemini image request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create Gemini image request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	if s.httpClient != nil {
		client.Transport = s.httpClient.Transport
	}
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

func parseGeminiStudioImageResponse(respBody []byte) ([]ImageResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("decode Gemini image response: %w", err)
	}

	candidates, _ := raw["candidates"].([]interface{})
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned by Gemini")
	}

	images := make([]ImageResult, 0, len(candidates))
	for _, rawCandidate := range candidates {
		candidate, _ := rawCandidate.(map[string]interface{})
		content, _ := candidate["content"].(map[string]interface{})
		parts, _ := content["parts"].([]interface{})
		revisedPrompt := ""
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]interface{})
			if !ok {
				continue
			}
			if thought, _ := part["thought"].(bool); thought {
				continue
			}
			if text, ok := part["text"].(string); ok && text != "" && revisedPrompt == "" {
				revisedPrompt = text
			}

			var inlineData map[string]interface{}
			if value, ok := part["inlineData"].(map[string]interface{}); ok {
				inlineData = value
			} else if value, ok := part["inline_data"].(map[string]interface{}); ok {
				inlineData = value
			}
			if inlineData == nil {
				continue
			}
			mimeType, _ := inlineData["mimeType"].(string)
			if mimeType == "" {
				mimeType, _ = inlineData["mime_type"].(string)
			}
			data, _ := inlineData["data"].(string)
			if strings.HasPrefix(mimeType, "image/") && data != "" {
				images = append(images, ImageResult{B64JSON: data, RevisedPrompt: revisedPrompt})
			}
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by Gemini")
	}
	return images, nil
}
