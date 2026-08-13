package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ImageGeometryMode describes how Image Studio should choose output geometry.
// It is intentionally semantic rather than provider-specific so an edit can
// preserve its source by default without relying on a magic WxH string.
type ImageGeometryMode string

const (
	ImageGeometryPreserveSource ImageGeometryMode = "preserve_source"
	ImageGeometryProviderAuto   ImageGeometryMode = "provider_auto"
	ImageGeometryExplicit       ImageGeometryMode = "explicit"
)

// ImageGeometry carries the user-visible output geometry intent.
type ImageGeometry struct {
	Mode ImageGeometryMode `json:"mode,omitempty"`
	Size string            `json:"size,omitempty"`
}

// ImageStudioRequest extends the legacy ImageRequest with semantic geometry
// and advanced fields used by the studio. Keeping the legacy request embedded
// lets non-Studio image callers continue using ImageGenerate unchanged.
type ImageStudioRequest struct {
	ImageRequest
	Geometry ImageGeometry `json:"geometry,omitempty"`
	Seed     *int          `json:"seed,omitempty"`
	Guidance *float64      `json:"guidance,omitempty"`
}

type imageStudioGeometryResolution struct {
	Mode        ImageGeometryMode
	LegacySize  string
	Size        string
	AspectRatio string
}

// ImageStudioGenerate dispatches an Image Studio request using the provider's
// documented image transport. It is deliberately separate from ImageGenerate
// while the older generic image API remains backward compatible.
func (s *Service) ImageStudioGenerate(ctx context.Context, req ImageStudioRequest) (*ImageResponse, error) {
	provider, err := s.resolveProviderProfile(req.Provider)
	if err != nil {
		return nil, err
	}
	baseURL, apiKey, _, providerType, err := s.extractProviderDetails(*provider)
	if err != nil {
		return nil, err
	}
	if !isImageCapableProvider(providerType) {
		return nil, fmt.Errorf("provider type '%s' does not support image generation", providerType)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" && provider.DefaultImageModel != nil {
		model = strings.TrimSpace(*provider.DefaultImageModel)
	}
	if model == "" {
		model = getDefaultImageModel(providerType)
	}

	caps := GetEffectiveImageCapabilities(strings.ToLower(providerType), model)
	if !caps.SupportsGeneration {
		return nil, fmt.Errorf("selected provider/model does not support image generation")
	}
	if req.OperationType == "edit" && !caps.SupportsEditing {
		return nil, fmt.Errorf("selected provider/model does not support image editing")
	}
	if err := validateImageStudioRequestCapabilities(caps, req); err != nil {
		return nil, err
	}
	if req.MaskImage != nil && strings.TrimSpace(req.MaskImage.Data) != "" {
		if !caps.SupportsMasking || caps.MaskingMode == ImageMaskingNone {
			return nil, fmt.Errorf("selected provider/model does not support area-mask editing")
		}
		if caps.MaskingMode == ImageMaskingPixel {
			if req.ReferenceImage == nil {
				return nil, fmt.Errorf("pixel mask requires a base image")
			}
			if err := validatePixelMaskPair(model, *req.ReferenceImage, *req.MaskImage); err != nil {
				return nil, fmt.Errorf("invalid pixel mask: %w", err)
			}
		}
	}

	geometry, err := resolveImageStudioGeometry(providerType, req.OperationType, req.Size, req.Geometry)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(providerType) {
	case "gemini":
		return s.geminiStudioImageGenerate(ctx, baseURL, apiKey, model, req, geometry)
	case "openrouter":
		return s.openRouterStudioImageGenerate(ctx, baseURL, apiKey, model, req, geometry)
	case "together":
		return s.togetherStudioImageGenerate(ctx, baseURL, apiKey, model, req, geometry)
	default:
		legacy := req.ImageRequest
		legacy.Model = model
		legacy.Size = geometry.LegacySize
		return s.ImageGenerate(ctx, legacy)
	}
}

func normalizeImageGeometryMode(operation, legacySize string, geometry ImageGeometry) ImageGeometryMode {
	if geometry.Mode != "" {
		return geometry.Mode
	}
	if strings.EqualFold(operation, "edit") {
		if strings.TrimSpace(legacySize) == "" {
			return ImageGeometryPreserveSource
		}
	}
	if strings.EqualFold(strings.TrimSpace(legacySize), "auto") || strings.TrimSpace(legacySize) == "" {
		return ImageGeometryProviderAuto
	}
	return ImageGeometryExplicit
}

func resolveImageStudioGeometry(providerType, operation, legacySize string, geometry ImageGeometry) (imageStudioGeometryResolution, error) {
	mode := normalizeImageGeometryMode(operation, legacySize, geometry)
	size := strings.TrimSpace(geometry.Size)
	if size == "" {
		size = strings.TrimSpace(legacySize)
	}
	resolution := imageStudioGeometryResolution{Mode: mode}
	switch mode {
	case ImageGeometryPreserveSource:
		if !strings.EqualFold(operation, "edit") {
			return resolution, fmt.Errorf("preserve_source geometry is only valid for image edits")
		}
		switch strings.ToLower(providerType) {
		case "openai":
			resolution.LegacySize = "auto"
		case "gemini", "openrouter":
			resolution.LegacySize = ""
		default:
			resolution.LegacySize = ""
		}
		return resolution, nil
	case ImageGeometryProviderAuto:
		if strings.EqualFold(providerType, "openai") {
			resolution.LegacySize = "auto"
		}
		return resolution, nil
	case ImageGeometryExplicit:
		if size == "" || strings.EqualFold(size, "auto") {
			return resolution, fmt.Errorf("explicit image geometry requires a concrete size")
		}
		if _, _, ok := parseImageSize(size); !ok {
			return resolution, fmt.Errorf("invalid image size %q; expected WIDTHxHEIGHT", size)
		}
		resolution.Size = size
		resolution.LegacySize = size
		resolution.AspectRatio = imageAspectRatioForSize(size)
		return resolution, nil
	default:
		return resolution, fmt.Errorf("unsupported image geometry mode %q", mode)
	}
}

func parseImageSize(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func imageAspectRatioForSize(size string) string {
	w, h, ok := parseImageSize(size)
	if !ok {
		return ""
	}
	g := gcdImageDimension(w, h)
	return fmt.Sprintf("%d:%d", w/g, h/g)
}

func gcdImageDimension(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	if a == 0 {
		return 1
	}
	return a
}

func imageReferenceDataURL(ref ReferenceImage) string {
	mimeType := strings.TrimSpace(ref.MimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + ref.Data
}

func collectStudioInputReferences(req ImageStudioRequest) []map[string]interface{} {
	refs := make([]map[string]interface{}, 0, 1+len(req.ReferenceImages)+len(req.StyleReferenceImages))
	appendRef := func(ref ReferenceImage) {
		if strings.TrimSpace(ref.Data) == "" {
			return
		}
		refs = append(refs, map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]interface{}{"url": imageReferenceDataURL(ref)},
		})
	}
	if req.ReferenceImage != nil {
		appendRef(*req.ReferenceImage)
	}
	for _, ref := range req.ReferenceImages {
		appendRef(ref)
	}
	for _, ref := range req.StyleReferenceImages {
		appendRef(ref)
	}
	return refs
}

func buildOpenRouterStudioImageBody(model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) map[string]interface{} {
	body := map[string]interface{}{"model": model, "prompt": req.Prompt}
	if req.N > 0 {
		body["n"] = req.N
	}
	if req.Quality != "" {
		body["quality"] = req.Quality
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	if geometry.Mode == ImageGeometryExplicit && geometry.AspectRatio != "" {
		// OpenRouter's Image API normalizes dimensions per endpoint. Sending both
		// explicit pixel size and aspect_ratio can conflict with endpoint-specific
		// capabilities, so Image Studio sends the portable aspect ratio only.
		body["aspect_ratio"] = geometry.AspectRatio
	}
	if refs := collectStudioInputReferences(req); len(refs) > 0 {
		body["input_references"] = refs
	}
	return body
}

func buildOpenRouterStudioMinimalBody(model string, req ImageStudioRequest) map[string]interface{} {
	body := map[string]interface{}{"model": model, "prompt": req.Prompt}
	if refs := collectStudioInputReferences(req); len(refs) > 0 {
		body["input_references"] = refs
	}
	return body
}

func (s *Service) openRouterStudioImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	if req.MaskImage != nil && req.MaskImage.Data != "" {
		return nil, fmt.Errorf("OpenRouter dedicated Images API does not expose a pixel-mask parameter for this Image Studio transport")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/images"
	body := buildOpenRouterStudioImageBody(model, req, geometry)
	respBody, status, err := s.doOpenRouterStudioImageRequest(ctx, endpoint, apiKey, body)
	if err != nil {
		return nil, err
	}

	// OpenRouter image endpoints expose model-specific optional parameters. If a
	// provider rejects a portable option such as aspect_ratio/quality/seed, retry
	// one time with only the required fields (plus references when requested).
	// Authentication and authorization errors are never retried.
	if (status == http.StatusBadRequest || status == http.StatusUnprocessableEntity) && len(body) > 2 {
		minimalBody := buildOpenRouterStudioMinimalBody(model, req)
		if len(minimalBody) < len(body) {
			fallbackBody, fallbackStatus, fallbackErr := s.doOpenRouterStudioImageRequest(ctx, endpoint, apiKey, minimalBody)
			if fallbackErr == nil && fallbackStatus >= 200 && fallbackStatus < 300 {
				respBody, status = fallbackBody, fallbackStatus
			} else if fallbackErr != nil {
				return nil, fmt.Errorf("OpenRouter image retry: %w", fallbackErr)
			} else {
				return nil, fmt.Errorf("OpenRouter image API returned status %d: %s; minimal retry returned status %d: %s", status, strings.TrimSpace(string(respBody)), fallbackStatus, strings.TrimSpace(string(fallbackBody)))
			}
		}
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("OpenRouter image API returned status %d: %s", status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode OpenRouter image response: %w", err)
	}
	images := make([]ImageResult, 0, len(result.Data))
	for _, item := range result.Data {
		if item.B64JSON != "" {
			images = append(images, ImageResult{B64JSON: item.B64JSON})
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by OpenRouter")
	}
	return &ImageResponse{Images: images, Provider: "openrouter", Model: model}, nil
}

func (s *Service) doOpenRouterStudioImageRequest(ctx context.Context, endpoint, apiKey string, body map[string]interface{}) ([]byte, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal OpenRouter image request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create OpenRouter image request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/ajbergh/OmniLLM-Studio")
	httpReq.Header.Set("X-Title", "OmniLLM-Studio")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 180 * time.Second, Transport: s.httpClient.Transport}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("OpenRouter image request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read OpenRouter image response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

func togetherModelUsesAspectRatio(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "schnell") || strings.Contains(lower, "kontext")
}

func buildTogetherStudioImageBody(model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (map[string]interface{}, error) {
	if req.OperationType == "edit" || req.ReferenceImage != nil || len(req.ReferenceImages) > 0 || len(req.StyleReferenceImages) > 0 {
		return nil, fmt.Errorf("selected Together model transport is generation-only in Image Studio")
	}
	body := map[string]interface{}{"model": model, "prompt": req.Prompt, "response_format": "base64"}
	if req.N > 0 {
		body["n"] = req.N
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	if req.Guidance != nil {
		body["guidance_scale"] = *req.Guidance
	}
	if geometry.Mode == ImageGeometryExplicit {
		if togetherModelUsesAspectRatio(model) {
			body["aspect_ratio"] = geometry.AspectRatio
		} else {
			w, h, ok := parseImageSize(geometry.Size)
			if !ok {
				return nil, fmt.Errorf("invalid Together image size %q", geometry.Size)
			}
			body["width"] = w
			body["height"] = h
		}
	}
	return body, nil
}

func (s *Service) togetherStudioImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageStudioRequest, geometry imageStudioGeometryResolution) (*ImageResponse, error) {
	body, err := buildTogetherStudioImageBody(model, req, geometry)
	if err != nil {
		return nil, err
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal Together image request: %w", err)
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/images/generations"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create Together image request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 180 * time.Second, Transport: s.httpClient.Transport}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Together image request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Together image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Together image API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode Together image response: %w", err)
	}
	images := make([]ImageResult, 0, len(result.Data))
	for _, item := range result.Data {
		if item.B64JSON != "" {
			images = append(images, ImageResult{B64JSON: item.B64JSON})
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by Together")
	}
	return &ImageResponse{Images: images, Provider: "together", Model: model}, nil
}
