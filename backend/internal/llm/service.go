package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// ChatMessage represents a single message in a chat request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TitleTimeout is the max time allowed for generating a conversation title.
const TitleTimeout = 10 * time.Second

// ReferenceImage is an optional input image for editing/inpainting.
type ReferenceImage struct {
	Data     string `json:"data"`      // base64-encoded image
	MimeType string `json:"mime_type"` // e.g. "image/png"
}

// ImageRequest holds the parameters for an image generation request.
type ImageRequest struct {
	Provider             string           `json:"provider"`
	Model                string           `json:"model"`
	Prompt               string           `json:"prompt"`
	Size                 string           `json:"size,omitempty"`                   // e.g. "1024x1024" (default)
	Quality              string           `json:"quality,omitempty"`                // "standard", "medium", "high", "low", "auto"
	N                    int              `json:"n,omitempty"`                      // number of images (default 1)
	ReferenceImage       *ReferenceImage  `json:"reference_image,omitempty"`        // for editing a previous image
	MaskImage            *ReferenceImage  `json:"mask_image,omitempty"`             // mask for region-aware editing
	OperationType        string           `json:"operation_type,omitempty"`         // generate | edit | variation
	Strength             *float64         `json:"strength,omitempty"`               // edit intensity 0.0–1.0
	ReferenceImages      []ReferenceImage `json:"reference_images,omitempty"`       // multiple content references
	StyleReferenceImages []ReferenceImage `json:"style_reference_images,omitempty"` // style references
}

// ImageResult holds a single generated image.
type ImageResult struct {
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageResponse holds the result of an image generation request.
type ImageResponse struct {
	Images   []ImageResult `json:"images"`
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
}

// EmbeddingRequest holds the parameters for an embedding request.
type EmbeddingRequest struct {
	Provider string   `json:"provider"`
	Model    string   `json:"model"` // e.g. "text-embedding-3-small"
	Input    []string `json:"input"` // texts to embed
}

// EmbeddingResponse holds the result of an embedding request.
type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Model      string      `json:"model"`
	Dimensions int         `json:"dimensions"`
}

// ChatRequest holds the parameters for a chat completion request.
type ChatRequest struct {
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Think    *bool         `json:"think,omitempty"` // Ollama-only: enable/disable thinking
}

// ChatResponse holds the result of a non-streaming chat completion.
type ChatResponse struct {
	Content     string `json:"content"`
	Thinking    string `json:"thinking,omitempty"` // Ollama-only: model's thinking content
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	TokenInput  *int   `json:"token_input,omitempty"`
	TokenOutput *int   `json:"token_output,omitempty"`
}

// StreamChunk represents a single token/chunk from a streaming response.
type StreamChunk struct {
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"` // Ollama-only: thinking content
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Service orchestrates LLM calls using configured provider profiles.
type Service struct {
	providerRepo *repository.ProviderRepo
	settingsRepo *repository.SettingsRepo
	httpClient   *http.Client

	ollamaModelCacheMu sync.Mutex
	ollamaModelCache   map[string]struct{}
}

// NewService creates a new LLM service.
func NewService(providerRepo *repository.ProviderRepo, settingsRepo *repository.SettingsRepo) *Service {
	return &Service{
		providerRepo: providerRepo,
		settingsRepo: settingsRepo,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		ollamaModelCache: make(map[string]struct{}),
	}
}

// resolveProviderProfile determines the provider profile to use.
// Priority: exact ID match > name match > type match > first enabled.
func (s *Service) resolveProviderProfile(providerName string) (*models.ProviderProfile, error) {
	providers, err := s.providerRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	if providerName != "" {
		// Priority 1: exact ID match
		for i := range providers {
			if providers[i].Enabled && providers[i].ID == providerName {
				return &providers[i], nil
			}
		}
		// Priority 2: exact name match
		for i := range providers {
			if providers[i].Enabled && providers[i].Name == providerName {
				return &providers[i], nil
			}
		}
		// Priority 3: type match
		for i := range providers {
			if providers[i].Enabled && providers[i].Type == providerName {
				return &providers[i], nil
			}
		}
	}

	// Fall back to first enabled provider
	for i := range providers {
		if providers[i].Enabled {
			return &providers[i], nil
		}
	}

	return nil, fmt.Errorf("no enabled provider found")
}

// resolveProvider determines the provider profile and API key to use.
func (s *Service) resolveProvider(providerName string) (baseURL, apiKey, model, providerType string, err error) {
	provider, err := s.resolveProviderProfile(providerName)
	if err != nil {
		return "", "", "", "", err
	}
	return s.extractProviderDetails(*provider)
}

// ResolveProviderType returns the provider type string for a given provider name/ID.
func (s *Service) ResolveProviderType(providerName string) (string, error) {
	_, _, _, providerType, err := s.resolveProvider(providerName)
	if err != nil {
		return "", err
	}
	return providerType, nil
}

func (s *Service) extractProviderDetails(p models.ProviderProfile) (baseURL, apiKey, model, providerType string, err error) {
	key, err := s.providerRepo.GetAPIKey(p.ID)
	if err != nil {
		return "", "", "", "", fmt.Errorf("get api key: %w", err)
	}
	url := getBaseURL(p.Type, p.BaseURL)
	defaultModel := ""
	if p.DefaultModel != nil {
		defaultModel = *p.DefaultModel
	}
	return url, key, defaultModel, p.Type, nil
}

func getBaseURL(providerType string, customURL *string) string {
	if customURL != nil && *customURL != "" {
		return *customURL
	}

	switch strings.ToLower(providerType) {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "https://api.openai.com/v1"
	}
}

func getDefaultModel(providerType string) string {
	switch strings.ToLower(providerType) {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "ollama":
		return "llama3.2"
	case "openrouter":
		return "openai/gpt-4o"
	case "groq":
		return "llama-3.3-70b-versatile"
	case "together":
		return "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
	case "mistral":
		return "mistral-large-latest"
	case "gemini":
		return "gemini-2.0-flash"
	default:
		return "gpt-4o"
	}
}

func getDefaultEmbeddingModel(providerType string) string {
	switch strings.ToLower(providerType) {
	case "ollama":
		return "nomic-embed-text"
	default:
		return "text-embedding-3-small"
	}
}

func normalizeEmbeddingModel(providerType, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		return getDefaultEmbeddingModel(providerType)
	}

	if strings.ToLower(providerType) == "ollama" {
		switch strings.ToLower(model) {
		case "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002":
			// OpenAI embedding names are a common default, but Ollama requires local models.
			return "nomic-embed-text"
		}
	}

	return model
}

func ollamaAPIRoot(baseURL string) string {
	trimmed := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimSuffix(trimmed, "/v1")
	}

	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/v1") {
		path = strings.TrimSuffix(path, "/v1")
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""

	root := strings.TrimRight(parsed.String(), "/")
	if root == "" {
		return strings.TrimSuffix(trimmed, "/v1")
	}
	return root
}

func ollamaModelNameMatches(installed, requested string) bool {
	installed = strings.ToLower(strings.TrimSpace(installed))
	requested = strings.ToLower(strings.TrimSpace(requested))
	if installed == "" || requested == "" {
		return false
	}
	if installed == requested {
		return true
	}

	if strings.HasSuffix(installed, ":latest") && strings.TrimSuffix(installed, ":latest") == requested {
		return true
	}
	if strings.HasSuffix(requested, ":latest") && strings.TrimSuffix(requested, ":latest") == installed {
		return true
	}

	return false
}

func (s *Service) hasCachedOllamaModel(rootURL, model string) bool {
	cacheKey := strings.ToLower(rootURL + "|" + model)
	s.ollamaModelCacheMu.Lock()
	defer s.ollamaModelCacheMu.Unlock()
	_, ok := s.ollamaModelCache[cacheKey]
	return ok
}

func (s *Service) cacheOllamaModel(rootURL, model string) {
	cacheKey := strings.ToLower(rootURL + "|" + model)
	s.ollamaModelCacheMu.Lock()
	defer s.ollamaModelCacheMu.Unlock()
	s.ollamaModelCache[cacheKey] = struct{}{}
}

func (s *Service) ollamaModelExists(ctx context.Context, rootURL, model string) (bool, error) {
	tagsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(tagsCtx, http.MethodGet, rootURL+"/api/tags", nil)
	if err != nil {
		return false, fmt.Errorf("create tags request: %w", err)
	}

	pullClient := &http.Client{Timeout: 20 * time.Minute}
	resp, err := pullClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("request tags: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read tags response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("tags status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(bodyBytes, &tagsResp); err != nil {
		return false, fmt.Errorf("decode tags response: %w", err)
	}

	for _, m := range tagsResp.Models {
		if ollamaModelNameMatches(m.Name, model) {
			return true, nil
		}
	}

	return false, nil
}

func (s *Service) pullOllamaModel(ctx context.Context, rootURL, model string) error {
	pullCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	payload := map[string]interface{}{
		"model":  model,
		"stream": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pull payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(pullCtx, http.MethodPost, rootURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request pull: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read pull response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pull status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (s *Service) ensureOllamaEmbeddingModel(ctx context.Context, baseURL, model string) error {
	rootURL := ollamaAPIRoot(baseURL)
	if s.hasCachedOllamaModel(rootURL, model) {
		return nil
	}

	exists, err := s.ollamaModelExists(ctx, rootURL, model)
	if err != nil {
		return fmt.Errorf("check ollama model %q: %w", model, err)
	}
	if !exists {
		log.Printf("[llm] ollama embedding model %q not found; auto-pulling", model)
		if err := s.pullOllamaModel(ctx, rootURL, model); err != nil {
			return fmt.Errorf("auto-pull ollama model %q failed: %w", model, err)
		}
		log.Printf("[llm] ollama embedding model %q pulled successfully", model)
	}

	s.cacheOllamaModel(rootURL, model)
	return nil
}

// ChatComplete performs a non-streaming chat completion.
func (s *Service) ChatComplete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	baseURL, apiKey, defaultModel, providerType, err := s.resolveProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = defaultModel
	}
	if model == "" {
		model = getDefaultModel(providerType)
	}

	// Use OpenAI-compatible API format (works for most providers)
	body := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   false,
	}

	// Ollama-only: pass think parameter when explicitly set
	if strings.ToLower(providerType) == "ollama" && req.Think != nil {
		body["think"] = *req.Think
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		if strings.ToLower(providerType) == "anthropic" {
			httpReq.Header.Set("x-api-key", apiKey)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	tokenIn := result.Usage.PromptTokens
	tokenOut := result.Usage.CompletionTokens

	return &ChatResponse{
		Content:     content,
		Provider:    providerType,
		Model:       model,
		TokenInput:  &tokenIn,
		TokenOutput: &tokenOut,
	}, nil
}

// ChatStream performs a streaming chat completion, calling onChunk for each token.
func (s *Service) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) error {
	baseURL, apiKey, defaultModel, providerType, err := s.resolveProvider(req.Provider)
	if err != nil {
		return err
	}

	model := req.Model
	if model == "" {
		model = defaultModel
	}
	if model == "" {
		model = getDefaultModel(providerType)
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   true,
	}

	// Ollama-only: pass think parameter when explicitly set
	if strings.ToLower(providerType) == "ollama" && req.Think != nil {
		body["think"] = *req.Think
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Use a client without timeout for streaming
	streamClient := &http.Client{}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		if strings.ToLower(providerType) == "anthropic" {
			httpReq.Header.Set("x-api-key", apiKey)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse SSE stream
	reader := io.Reader(resp.Body)
	buf := make([]byte, 4096)
	var leftover string

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := leftover + string(buf[:n])
			leftover = ""

			lines := strings.Split(data, "\n")
			for i, line := range lines {
				// Last line might be incomplete
				if i == len(lines)-1 && !strings.HasSuffix(data, "\n") {
					leftover = line
					continue
				}

				line = strings.TrimSpace(line)
				if line == "" || line == ":" {
					continue
				}

				if strings.HasPrefix(line, "data: ") {
					payload := strings.TrimPrefix(line, "data: ")
					if payload == "[DONE]" {
						return nil
					}

					var chunk struct {
						Choices []struct {
							Delta struct {
								Content  string `json:"content"`
								Thinking string `json:"thinking"`
							} `json:"delta"`
						} `json:"choices"`
					}

					if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
						continue // skip malformed chunks
					}

					if len(chunk.Choices) > 0 {
						delta := chunk.Choices[0].Delta
						if delta.Content != "" || delta.Thinking != "" {
							onChunk(StreamChunk{
								Content:  delta.Content,
								Thinking: delta.Thinking,
								Provider: providerType,
								Model:    model,
							})
						}
					}
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stream: %w", err)
		}
	}

	return nil
}

// isImageCapableProvider returns true if the given provider type supports
// image generation (via OpenAI-compat /images/generations or native API).
func isImageCapableProvider(providerType string) bool {
	switch strings.ToLower(providerType) {
	case "openai", "together", "openrouter":
		return true
	case "gemini":
		// Gemini uses the native generateContent API (not OpenAI-compat).
		return true
	default:
		return false
	}
}

// IsImageCapableProvider is the exported version for use by API handlers.
func IsImageCapableProvider(providerType string) bool {
	return isImageCapableProvider(providerType)
}

// getDefaultImageModel returns a sensible default image model for the provider type.
// Only call this for image-capable providers (see isImageCapableProvider).
func getDefaultImageModel(providerType string) string {
	caps := GetImageCapabilities(strings.ToLower(providerType))
	if caps.DefaultImageModel != "" {
		return caps.DefaultImageModel
	}
	switch strings.ToLower(providerType) {
	case "openai":
		return "gpt-image-1"
	case "together":
		return "black-forest-labs/FLUX.1-schnell-Free"
	case "openrouter":
		return "openai/dall-e-3"
	case "gemini":
		return "gemini-2.0-flash-preview-image-generation"
	default:
		return ""
	}
}

// geminiNativeBaseURL converts an OpenAI-compat Gemini base URL to the native API base URL.
// e.g. "https://generativelanguage.googleapis.com/v1beta/openai" → "https://generativelanguage.googleapis.com/v1beta"
func geminiNativeBaseURL(baseURL string) string {
	return strings.TrimSuffix(baseURL, "/openai")
}

// sizeToGeminiAspectRatio converts an OpenAI-style image size (e.g. "1024x1024") to a Gemini aspect ratio.
func sizeToGeminiAspectRatio(size string) string {
	// Direct lookup covers all WxH strings from capabilities.SupportedSizes.
	known := map[string]string{
		"1024x1024": "1:1",
		"1024x1536": "2:3",
		"1536x1024": "3:2",
		"768x1024":  "3:4",
		"1024x768":  "4:3",
		"1024x1280": "4:5",
		"1280x1024": "5:4",
		"576x1024":  "9:16",
		"1024x576":  "16:9",
		"1344x576":  "21:9",
		"512x2048":  "1:4",
		"2048x512":  "4:1",
		"384x3072":  "1:8",
		"3072x384":  "8:1",
		// Legacy sizes
		"512x512":   "1:1",
		"1792x1024": "16:9",
		"1024x1792": "9:16",
	}
	if r, ok := known[size]; ok {
		return r
	}
	return "1:1"
}

// geminiImageGenerate uses the native Gemini generateContent API for image generation.
// Gemini image models (gemini-2.5-flash-image, gemini-3.1-flash-image-preview, etc.)
// do not support the OpenAI-compat /images/generations endpoint.
func (s *Service) geminiImageGenerate(ctx context.Context, baseURL, apiKey, model string, req ImageRequest) (*ImageResponse, error) {
	nativeBase := geminiNativeBaseURL(baseURL)
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", nativeBase, model)

	// Build generationConfig
	genConfig := map[string]interface{}{
		"responseModalities": []string{"IMAGE", "TEXT"},
	}

	if req.N > 1 {
		genConfig["candidateCount"] = req.N
	}

	imageConfig := map[string]interface{}{}
	if req.Size != "" {
		imageConfig["aspectRatio"] = sizeToGeminiAspectRatio(req.Size)
	}
	if len(imageConfig) > 0 {
		genConfig["imageConfig"] = imageConfig
	}

	// Build parts: text prompt + optional reference/mask/style images
	reqParts := []map[string]interface{}{
		{"text": req.Prompt},
	}
	if req.ReferenceImage != nil && req.ReferenceImage.Data != "" {
		reqParts = append(reqParts, map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": req.ReferenceImage.MimeType,
				"data":     req.ReferenceImage.Data,
			},
		})
		log.Printf("[gemini-image] including reference image (%s, %d bytes b64) for editing",
			req.ReferenceImage.MimeType, len(req.ReferenceImage.Data))
	}

	// Additional content references
	for i, ref := range req.ReferenceImages {
		if ref.Data == "" {
			continue
		}
		reqParts = append(reqParts, map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": ref.MimeType,
				"data":     ref.Data,
			},
		})
		log.Printf("[gemini-image] including additional reference image %d (%s)", i+1, ref.MimeType)
	}

	// Mask image
	if req.MaskImage != nil && req.MaskImage.Data != "" {
		reqParts = append(reqParts, map[string]interface{}{
			"text": "Use this mask to identify the region to edit:",
		})
		reqParts = append(reqParts, map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": req.MaskImage.MimeType,
				"data":     req.MaskImage.Data,
			},
		})
		log.Printf("[gemini-image] including mask image (%s)", req.MaskImage.MimeType)
	}

	// Style reference images
	for i, ref := range req.StyleReferenceImages {
		if ref.Data == "" {
			continue
		}
		reqParts = append(reqParts, map[string]interface{}{
			"text": "Use this as a style reference:",
		})
		reqParts = append(reqParts, map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": ref.MimeType,
				"data":     ref.Data,
			},
		})
		log.Printf("[gemini-image] including style reference image %d (%s)", i+1, ref.MimeType)
	}

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": reqParts,
			},
		},
		"generationConfig": genConfig,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	log.Printf("[gemini-image] POST %s model=%s", endpoint, model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	}

	// Use a longer timeout for image generation
	imgClient := &http.Client{Timeout: 180 * time.Second}
	resp, err := imgClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse generically to handle both camelCase and snake_case JSON field names.
	// The Gemini REST API documentation shows both conventions in different places.
	var rawResp map[string]interface{}
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	candidates, _ := rawResp["candidates"].([]interface{})
	if len(candidates) == 0 {
		log.Printf("[gemini-image] no candidates in response: %s", string(respBody[:min(len(respBody), 500)]))
		return nil, fmt.Errorf("no candidates returned by Gemini")
	}

	var images []ImageResult
	var revisedPrompt string

	for _, c := range candidates {
		candidate, _ := c.(map[string]interface{})
		content, _ := candidate["content"].(map[string]interface{})
		parts, _ := content["parts"].([]interface{})

		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			// Skip thinking/thought parts
			if thought, _ := part["thought"].(bool); thought {
				continue
			}

			// Collect text for revised prompt
			if text, ok := part["text"].(string); ok && text != "" && revisedPrompt == "" {
				revisedPrompt = text
			}

			// Check for inline_data (snake_case) OR inlineData (camelCase)
			var inlineData map[string]interface{}
			if id, ok := part["inline_data"].(map[string]interface{}); ok {
				inlineData = id
			} else if id, ok := part["inlineData"].(map[string]interface{}); ok {
				inlineData = id
			}

			if inlineData != nil {
				mimeType := ""
				if mt, ok := inlineData["mime_type"].(string); ok {
					mimeType = mt
				} else if mt, ok := inlineData["mimeType"].(string); ok {
					mimeType = mt
				}

				data, _ := inlineData["data"].(string)

				if strings.HasPrefix(mimeType, "image/") && data != "" {
					images = append(images, ImageResult{
						B64JSON:       data,
						RevisedPrompt: revisedPrompt,
					})
				}
			}
		}
	}

	if len(images) == 0 {
		// Log a snippet of the response for debugging
		snippet := string(respBody)
		if len(snippet) > 1000 {
			snippet = snippet[:1000] + "..."
		}
		log.Printf("[gemini-image] no images found in response candidates (count=%d): %s", len(candidates), snippet)
		return nil, fmt.Errorf("no images returned by Gemini")
	}

	log.Printf("[gemini-image] success: %d image(s) from model %s", len(images), model)

	return &ImageResponse{
		Images:   images,
		Provider: "gemini",
		Model:    model,
	}, nil
}

// openaiImageEdit sends a multipart form request to the OpenAI-compatible
// /images/edits endpoint. The image and optional mask are decoded from base64
// and included as file parts.
func (s *Service) openaiImageEdit(ctx context.Context, baseURL, apiKey, model string, req ImageRequest) (*ImageResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// model field
	if err := w.WriteField("model", model); err != nil {
		return nil, fmt.Errorf("write model field: %w", err)
	}
	// prompt
	if err := w.WriteField("prompt", req.Prompt); err != nil {
		return nil, fmt.Errorf("write prompt field: %w", err)
	}
	// n
	n := req.N
	if n <= 0 {
		n = 1
	}
	if err := w.WriteField("n", fmt.Sprintf("%d", n)); err != nil {
		return nil, fmt.Errorf("write n field: %w", err)
	}
	// size
	size := req.Size
	if size == "" {
		size = "1024x1024"
	}
	if err := w.WriteField("size", size); err != nil {
		return nil, fmt.Errorf("write size field: %w", err)
	}
	// response_format
	if err := w.WriteField("response_format", "b64_json"); err != nil {
		return nil, fmt.Errorf("write response_format field: %w", err)
	}

	// image file part (base64 → raw bytes)
	imgData, err := base64.StdEncoding.DecodeString(req.ReferenceImage.Data)
	if err != nil {
		return nil, fmt.Errorf("decode base image: %w", err)
	}
	ext := "png"
	if strings.Contains(req.ReferenceImage.MimeType, "webp") {
		ext = "webp"
	} else if strings.Contains(req.ReferenceImage.MimeType, "jpeg") || strings.Contains(req.ReferenceImage.MimeType, "jpg") {
		ext = "jpg"
	}
	imagePart, err := w.CreateFormFile("image", "image."+ext)
	if err != nil {
		return nil, fmt.Errorf("create image part: %w", err)
	}
	if _, err := imagePart.Write(imgData); err != nil {
		return nil, fmt.Errorf("write image part: %w", err)
	}

	// optional mask file part
	if req.MaskImage != nil && req.MaskImage.Data != "" {
		maskData, err := base64.StdEncoding.DecodeString(req.MaskImage.Data)
		if err != nil {
			return nil, fmt.Errorf("decode mask image: %w", err)
		}
		maskPart, err := w.CreateFormFile("mask", "mask.png")
		if err != nil {
			return nil, fmt.Errorf("create mask part: %w", err)
		}
		if _, err := maskPart.Write(maskData); err != nil {
			return nil, fmt.Errorf("write mask part: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/images/edits", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	imgClient := &http.Client{Timeout: 180 * time.Second}
	resp, err := imgClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	images := make([]ImageResult, 0, len(result.Data))
	for _, d := range result.Data {
		images = append(images, ImageResult{
			B64JSON:       d.B64JSON,
			RevisedPrompt: d.RevisedPrompt,
		})
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by provider")
	}

	return &ImageResponse{
		Images:   images,
		Provider: "openai",
		Model:    model,
	}, nil
}

// ImageGenerate performs an image generation request. For most providers this uses
// the OpenAI-compatible /images/generations endpoint. For Gemini, it uses the
// native generateContent API.
func (s *Service) ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	provider, err := s.resolveProviderProfile(req.Provider)
	if err != nil {
		return nil, err
	}

	baseURL, apiKey, _, providerType, err := s.extractProviderDetails(*provider)
	if err != nil {
		return nil, err
	}

	// Reject providers that don't support image generation
	if !isImageCapableProvider(providerType) {
		return nil, fmt.Errorf("provider type '%s' does not support image generation", providerType)
	}

	// For image generation, ignore the provider's chat default model and use
	// either the explicitly requested model or the provider-specific image default.
	model := req.Model
	if model == "" {
		if provider.DefaultImageModel != nil {
			model = strings.TrimSpace(*provider.DefaultImageModel)
		}
		if model == "" {
			model = getDefaultImageModel(providerType)
		}
	}

	// Route Gemini to its native generateContent API
	if strings.EqualFold(providerType, "gemini") {
		return s.geminiImageGenerate(ctx, baseURL, apiKey, model, req)
	}

	// Route edit requests to the OpenAI /images/edits multipart endpoint
	if req.OperationType == "edit" && req.ReferenceImage != nil {
		return s.openaiImageEdit(ctx, baseURL, apiKey, model, req)
	}

	n := req.N
	if n <= 0 {
		n = 1
	}

	size := req.Size
	if size == "" {
		size = "1024x1024"
	}

	quality := req.Quality
	if quality == "" {
		quality = "auto"
	}

	body := map[string]interface{}{
		"model":           model,
		"prompt":          req.Prompt,
		"n":               n,
		"size":            size,
		"quality":         quality,
		"response_format": "b64_json",
	}

	// gpt-image-1 supports reference images via the "image" array field
	var imageInputs []map[string]interface{}
	if req.ReferenceImage != nil && req.ReferenceImage.Data != "" {
		imageInputs = append(imageInputs, map[string]interface{}{
			"type": "base64",
			"data": req.ReferenceImage.Data,
		})
	}
	for _, ref := range req.ReferenceImages {
		if ref.Data != "" {
			imageInputs = append(imageInputs, map[string]interface{}{
				"type": "base64",
				"data": ref.Data,
			})
		}
	}
	if len(imageInputs) > 0 {
		body["image"] = imageInputs
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/images/generations", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Use a longer timeout for image generation
	imgClient := &http.Client{Timeout: 180 * time.Second}
	resp, err := imgClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	images := make([]ImageResult, 0, len(result.Data))
	for _, d := range result.Data {
		images = append(images, ImageResult{
			B64JSON:       d.B64JSON,
			RevisedPrompt: d.RevisedPrompt,
		})
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images returned by provider")
	}

	return &ImageResponse{
		Images:   images,
		Provider: providerType,
		Model:    model,
	}, nil
}

// Embed calls the provider's /embeddings endpoint (OpenAI-compatible) and
// returns one embedding vector per input string.
func (s *Service) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	baseURL, apiKey, _, providerType, err := s.resolveProvider(req.Provider)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	model := normalizeEmbeddingModel(providerType, req.Model)
	if strings.EqualFold(providerType, "ollama") {
		if err := s.ensureOllamaEmbeddingModel(ctx, baseURL, model); err != nil {
			return nil, err
		}
	}

	body := map[string]interface{}{
		"model": model,
		"input": req.Input,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if apiKey != "" {
		if providerType == "anthropic" {
			httpReq.Header.Set("x-api-key", apiKey)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if strings.ToLower(providerType) == "ollama" {
			return nil, fmt.Errorf("provider returned status %d: %s (hint: pull and use an Ollama embedding model such as 'nomic-embed-text')", resp.StatusCode, string(bodyBytes))
		}
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned by provider")
	}

	// Sort by index to guarantee ordering matches input
	sort.Slice(result.Data, func(i, j int) bool {
		return result.Data[i].Index < result.Data[j].Index
	})

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}

	dims := 0
	if len(embeddings) > 0 {
		dims = len(embeddings[0])
	}

	return &EmbeddingResponse{
		Embeddings: embeddings,
		Model:      result.Model,
		Dimensions: dims,
	}, nil
}
