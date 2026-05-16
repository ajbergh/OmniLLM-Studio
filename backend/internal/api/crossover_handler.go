package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// llmCompleter is a narrow interface so CrossoverHandler is mockable in tests
// without depending on all of llm.Service.
type llmCompleter interface {
	ChatComplete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// CrossoverHandler handles cross-studio domain translation requests.
type CrossoverHandler struct {
	llm          llmCompleter
	providerRepo *repository.ProviderRepo
}

// NewCrossoverHandler creates a CrossoverHandler.
func NewCrossoverHandler(llm llmCompleter, providerRepo *repository.ProviderRepo) *CrossoverHandler {
	return &CrossoverHandler{llm: llm, providerRepo: providerRepo}
}

// translateRequest is the shape of POST /v1/crossover/translate.
type translateRequest struct {
	Source  string              `json:"source"` // "music" | "image"
	Target  string              `json:"target"` // "music" | "image"
	Content translateContentReq `json:"content"`
}

type translateContentReq struct {
	Prompt      string   `json:"prompt"`
	Genre       string   `json:"genre,omitempty"`
	Mood        string   `json:"mood,omitempty"`
	Instruments []string `json:"instruments,omitempty"`
}

// Translate handles POST /v1/crossover/translate.
func (h *CrossoverHandler) Translate(w http.ResponseWriter, r *http.Request) {
	var req translateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "invalid request body", nil)
		return
	}

	// Validate source/target pair
	validSources := map[string]bool{"music": true, "image": true, "chat": true}
	validTargets := map[string]bool{"music": true, "image": true}
	if !validSources[req.Source] || !validTargets[req.Target] {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "source must be 'music', 'image', or 'chat'; target must be 'music' or 'image'", nil)
		return
	}
	if req.Source == req.Target {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "source and target must be different", nil)
		return
	}

	// Guard: short prompt is not useful for translation
	if len(strings.TrimSpace(req.Content.Prompt)) < 10 {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "prompt too short for translation (minimum 10 characters)", nil)
		return
	}

	// Resolve provider — pick first enabled provider
	provider, err := h.firstEnabledProvider(auth.UserIDFromContext(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == "" {
		respondErrorWithCode(w, http.StatusPreconditionFailed, "no_provider_configured", "no enabled LLM provider found; add a provider in Settings first", nil)
		return
	}

	// Build prompt
	systemPrompt, userPrompt := buildTranslationPrompt(req)

	chatReq := llm.ChatRequest{
		Provider: provider,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	resp, err := h.llm.ChatComplete(r.Context(), chatReq)
	if err != nil {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", fmt.Sprintf("LLM call failed: %v", err), nil)
		return
	}

	// Parse JSON from response
	raw := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present
	raw = stripJSONFences(raw)

	if raw == "" {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", "LLM returned empty response", nil)
		return
	}

	// Validate it's parseable JSON and return it directly
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", "LLM did not return valid JSON", nil)
		return
	}

	respondJSON(w, http.StatusOK, parsed)
}

// firstEnabledProvider returns the provider name/id of the first enabled provider profile.
// In solo mode userID is empty; we still pick the first enabled provider.
func (h *CrossoverHandler) firstEnabledProvider(_ string) (string, error) {
	providers, err := h.providerRepo.List()
	if err != nil {
		return "", fmt.Errorf("list providers: %w", err)
	}
	for _, p := range providers {
		if p.Enabled {
			// Use the provider name as the routing key (matches llm.Service.resolveProvider logic)
			return p.Name, nil
		}
	}
	return "", nil
}

// buildTranslationPrompt constructs the system + user prompts for domain translation.
func buildTranslationPrompt(req translateRequest) (system, user string) {
	switch {
	case req.Source == "music" && req.Target == "image":
		system = `You are a creative director specializing in translating musical descriptions into vivid visual concepts for image generation.
Your task: given a music prompt with optional genre, mood, and instruments, output a single JSON object with exactly this schema:
{"image_prompt": "<string>"}
The image_prompt should be a detailed, concrete visual description (100-200 words) suitable for an AI image generator. Include lighting, color palette, scene elements, artistic style, and atmosphere that evoke the feeling of the music. Do NOT include explanations or markdown — output ONLY the JSON object.`
		parts := []string{fmt.Sprintf("Music prompt: %s", req.Content.Prompt)}
		if req.Content.Genre != "" {
			parts = append(parts, fmt.Sprintf("Genre: %s", req.Content.Genre))
		}
		if req.Content.Mood != "" {
			parts = append(parts, fmt.Sprintf("Mood: %s", req.Content.Mood))
		}
		if len(req.Content.Instruments) > 0 {
			parts = append(parts, fmt.Sprintf("Instruments: %s", strings.Join(req.Content.Instruments, ", ")))
		}
		user = strings.Join(parts, "\n")

	case req.Source == "image" && req.Target == "music":
		system = `You are a music director specializing in translating visual concepts into music production briefs.
Your task: given an image prompt or visual description, output a single JSON object with exactly this schema:
{"prompt": "<string>", "genre": "<string>", "mood": "<string>", "instruments": ["<string>", ...], "tempo": "<string>"}
The prompt should be a concise music prompt (50-100 words). genre should be a specific music genre. mood should be one word or short phrase. instruments should be an array of 3-6 instrument names. tempo should be one of: slow, moderate, upbeat, fast. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Image/visual description: %s", req.Content.Prompt)

	case req.Source == "chat" && req.Target == "music":
		system = `You are a Lyria music prompt specialist. Extract and distill only the musical description from the following text — which may be a full LLM chat response containing conversational preamble, explanations, and postscript.
Remove ALL of the following: greetings, commentary, meta-commentary ("here's a prompt", "I'd suggest", "this captures..."), markdown headers, and any non-musical sentences.
Return a single JSON object with exactly this schema:
{"prompt": "<string>", "genre": "<string>", "mood": "<string>", "instruments": ["<string>", ...], "tempo": "<string>"}
The prompt field must be a clean, self-contained Lyria music generation prompt (30-100 words) describing only sonic/musical qualities. genre should be a specific music genre. mood should be one word or short phrase. instruments should be an array of 3-6 instrument names inferred from the text. tempo should be one of: slow, moderate, upbeat, fast. If no clear musical content is found, infer a reasonable interpretation. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Chat response to distill:\n%s", req.Content.Prompt)

	default:
		// Shouldn't reach here due to earlier validation
		system = "Translate the input to the target domain. Output only JSON."
		user = req.Content.Prompt
	}
	return system, user
}

// stripJSONFences removes ```json ... ``` or ``` ... ``` fences from LLM output.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove first line (```json or ```)
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
		// Remove trailing ```
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
